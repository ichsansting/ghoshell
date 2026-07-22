package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"

	"golang.org/x/term"

	"ghoshell/internal/vault"
)

// promptPassphrase reads the passphrase from the controlling TTY with echo off.
// decrypt-then-pick (spec): the passphrase is asked before anything is shown.
func promptPassphrase() (string, error) {
	fmt.Fprint(os.Stderr, "passphrase: ")
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// manifestPath is where the manifest JSON lives inside the sealed vault. The
// vault is a single blob of path-keyed files (seam 1); the manifest is one of
// them and is the source of truth every launch composes from.
const manifestPath = "manifest.json"

// launchConfig carries the two seams the launch pipeline needs injected so it
// can be driven end-to-end in a test without a real TTY or a real shell:
// readPassphrase supplies the passphrase, run executes the composed shell.
type launchConfig struct {
	vaultPath      string
	readPassphrase func() (string, error)
	run            func(*exec.Cmd) error // default (*exec.Cmd).Run
}

func runLaunch(args []string, _, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "gho launch: need a vault path")
		return 2
	}
	cfg := launchConfig{
		vaultPath:      args[0],
		readPassphrase: promptPassphrase,
		run:            (*exec.Cmd).Run,
	}
	if err := launch(cfg); err != nil {
		fmt.Fprintf(stderr, "gho launch: %v\n", err)
		return 1
	}
	return 0
}

// launch is the whole tracer-bullet path: read the vault, unlock it, compose
// the profile, materialize it into an ephemeral HOME, spawn the shell, and wipe
// on the way out — whether that exit is normal or a signal.
func launch(cfg launchConfig) error {
	blob, err := os.ReadFile(cfg.vaultPath)
	if err != nil {
		return fmt.Errorf("read vault: %w", err)
	}

	pass, err := cfg.readPassphrase()
	if err != nil {
		return fmt.Errorf("read passphrase: %w", err)
	}

	// Unseal fails loud on a wrong passphrase or any tampering (seam 1) — a
	// broken unlock never yields a partial or empty environment.
	files, err := vault.Unseal(pass, blob)
	if err != nil {
		return err
	}

	m, err := manifestFromVault(files)
	if err != nil {
		return err
	}
	profile, err := soleProfile(m)
	if err != nil {
		return err
	}
	composed, err := Compose(m, profile)
	if err != nil {
		return err
	}

	sess, err := newSession()
	if err != nil {
		return err
	}
	// One wipe, whichever way we leave: the deferred call covers normal return
	// and panics; the signal goroutine covers Ctrl-C / kill (which would skip
	// the defer via os.Exit). session.wipe is sync.Once-guarded so both are safe.
	defer sess.wipe()
	stop := make(chan struct{})
	defer close(stop)
	installSignalWipe(sess, stop)

	if err := sess.materialize(composed); err != nil {
		return err
	}

	// mise x -- fish: tools lazy-install on first use and land on PATH for the
	// shell and anything it spawns. mise finds the composed mise.toml because
	// the child runs with cwd = HOME.
	cmd := exec.Command("mise", "x", "--", "fish")
	cmd.Dir = sess.home
	cmd.Env = append(os.Environ(), "HOME="+sess.home)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cfg.run(cmd)
}

// installSignalWipe wipes the session on a real kill (SIGTERM) or terminal
// hangup (SIGHUP), then exits — the ghost safety net for a teardown the normal
// exit path would otherwise skip. SIGINT is deliberately NOT a trigger: gho and
// the shell share the foreground process group, so an interactive Ctrl-C hits
// both, and it belongs to fish (which cancels its command line and keeps
// running). Wiping on it would delete HOME out from under a live shell, so gho
// ignores it and lets fish own it. It stops listening when stop is closed
// (normal return), which also resets SIGINT so a test process isn't left deaf
// to Ctrl-C. ponytail: wipes without reaping the child; a SIGTERM aimed only at
// gho orphans the shell (it dies on terminal close). Reap it if that matters.
func installSignalWipe(sess *session, stop <-chan struct{}) {
	signal.Ignore(syscall.SIGINT)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		defer func() {
			signal.Stop(sigCh)
			signal.Reset(syscall.SIGINT)
		}()
		select {
		case <-sigCh:
			sess.wipe()
			os.Exit(143) // 128 + SIGTERM
		case <-stop:
		}
	}()
}

func manifestFromVault(files []vault.File) (Manifest, error) {
	for _, f := range files {
		if f.Path == manifestPath {
			return ParseManifest(f.Data)
		}
	}
	return Manifest{}, fmt.Errorf("vault has no %s", manifestPath)
}

// soleProfile picks the profile to launch. There is no picker yet (ticket 05),
// so it takes the first profile by sorted name — deterministic for the single
// and multi-profile cases alike.
func soleProfile(m Manifest) (string, error) {
	if len(m.Profiles) == 0 {
		return "", fmt.Errorf("manifest has no profiles")
	}
	names := make([]string, 0, len(m.Profiles))
	for n := range m.Profiles {
		names = append(names, n)
	}
	sort.Strings(names)
	return names[0], nil
}

// session is one ephemeral HOME and the state its wipe needs.
type session struct {
	home        string
	onTmpfs     bool     // true on the linux/tmpfs path → wipe is structural
	secretPaths []string // absolute paths of 0600 files, scrubbed on the disk fallback
	once        sync.Once
}

// newSession creates the ephemeral HOME, preferring a RAM-backed tmpfs dir so
// the wipe is structural, and falling back to the OS temp dir (e.g. on mac,
// which has no tmpfs) where the wipe scrubs secrets before unlink.
func newSession() (*session, error) {
	bases := []struct {
		path  string
		tmpfs bool
	}{
		{"/dev/shm", true},    // linux tmpfs
		{os.TempDir(), false}, // mac / anywhere without tmpfs
	}
	for _, b := range bases {
		dir, err := os.MkdirTemp(b.path, "ghoshell-")
		if err != nil {
			continue
		}
		// The session root holds credentials — never group/world reachable.
		if err := os.Chmod(dir, 0o700); err != nil {
			os.RemoveAll(dir)
			return nil, fmt.Errorf("lock session dir: %w", err)
		}
		return &session{home: dir, onTmpfs: b.tmpfs}, nil
	}
	return nil, fmt.Errorf("no writable session dir (tried /dev/shm and %s)", os.TempDir())
}

// materialize writes the composed files and the generated mise.toml into HOME.
// Secret files land 0600; a manifest path that would escape HOME is rejected so
// a launch can never touch the host's real home (the ghost invariant).
func (s *session) materialize(c Composed) error {
	for _, f := range c.Files {
		dst := filepath.Join(s.home, f.Path)
		if dst != s.home && !strings.HasPrefix(dst, s.home+string(os.PathSeparator)) {
			return fmt.Errorf("file %q escapes session HOME", f.Path)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
			return fmt.Errorf("mkdir for %q: %w", f.Path, err)
		}
		mode := os.FileMode(0o644)
		if f.Secret {
			mode = 0o600
		}
		if err := writeExact(dst, []byte(f.Content), mode); err != nil {
			return fmt.Errorf("write %q: %w", f.Path, err)
		}
		if f.Secret {
			s.secretPaths = append(s.secretPaths, dst)
		}
	}
	mise := filepath.Join(s.home, "mise.toml")
	if err := writeExact(mise, []byte(c.MiseToml), 0o644); err != nil {
		return fmt.Errorf("write mise.toml: %w", err)
	}
	return nil
}

// writeExact writes data at exactly mode, defeating umask (WriteFile alone would
// let umask clear bits — a secret meant for 0600 could land more permissive).
func writeExact(path string, data []byte, mode os.FileMode) error {
	if err := os.WriteFile(path, data, mode); err != nil {
		return err
	}
	return os.Chmod(path, mode)
}

// wipe tears the session down exactly once. On tmpfs, removing the tree frees
// the RAM — structurally complete. On the disk fallback there is no tmpfs to
// lean on, so secret files are scrubbed before the tree is unlinked.
func (s *session) wipe() {
	s.once.Do(func() {
		if !s.onTmpfs {
			for _, p := range s.secretPaths {
				scrub(p)
			}
		}
		os.RemoveAll(s.home)
	})
}

// scrub overwrites a file's bytes with zeros before it is unlinked.
// ponytail: single-pass zero overwrite, best-effort — on CoW/SSD it is not a
// guarantee. tmpfs (the linux path) is the real defense; this only softens the
// mac fallback's honest ceiling.
func scrub(path string) {
	fi, err := os.Stat(path)
	if err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return
	}
	defer f.Close()
	if _, err := f.WriteAt(make([]byte, fi.Size()), 0); err == nil {
		f.Sync()
	}
}

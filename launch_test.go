package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"ghoshell/internal/vault"
)

// fixtureVault seals a manifest + its referenced config/secret files into a blob
// on disk and returns the path. This is the "fixture vault" seam 3 drives.
func fixtureVault(t *testing.T, pass string) string {
	t.Helper()
	m := Manifest{
		Profiles: map[string][]string{"work": {"base", "secrets"}},
		Components: map[string]Component{
			"base": {
				Tools: []string{"fish", "python@3.12"},
				Files: []File{{Path: ".config/fish/config.fish", Content: "set -g fish_greeting"}},
			},
			"secrets": {
				Files: []File{{Path: ".ssh/id_ed25519", Content: "PRIVATE-KEY", Secret: true}},
			},
		},
	}
	manifestJSON, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	blob, err := vault.Seal(pass, []vault.File{
		{Path: manifestPath, Mode: 0o600, Data: manifestJSON},
	})
	if err != nil {
		t.Fatalf("seal fixture: %v", err)
	}
	path := filepath.Join(t.TempDir(), "vault.blob")
	if err := os.WriteFile(path, blob, 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

// TestLaunchEndToEnd is seam 3: drive launch against a fixture vault and assert
// the observable outcome — HOME holds exactly the composed set at correct perms,
// mise.toml matches the composition, the shell is spawned as `mise x -- fish`,
// and the session dir is gone after teardown.
func TestLaunchEndToEnd(t *testing.T) {
	const pass = "correct horse battery staple"
	vaultPath := fixtureVault(t, pass)

	var gotArgs []string
	var homeAtSpawn, gotToml string
	var snapshot map[string]os.FileMode

	cfg := launchConfig{
		vaultPath:      vaultPath,
		readPassphrase: func() (string, error) { return pass, nil },
		// The injected run stands in for the real `mise x -- fish`: it records
		// argv and photographs HOME at the instant the shell would start —
		// after materialize, before wipe.
		run: func(cmd *exec.Cmd) error {
			gotArgs = cmd.Args
			homeAtSpawn = cmd.Dir
			snapshot = snapshotPerms(t, cmd.Dir)
			b, _ := os.ReadFile(filepath.Join(cmd.Dir, "mise.toml"))
			gotToml = string(b)
			return nil
		},
	}

	if err := launch(cfg); err != nil {
		t.Fatalf("launch: %v", err)
	}

	// Spawned as `mise x -- fish`.
	if strings.Join(gotArgs, " ") != "mise x -- fish" {
		t.Errorf("spawned %v, want [mise x -- fish]", gotArgs)
	}

	// Exactly the composed set (2 files + mise.toml), at the right perms.
	want := map[string]os.FileMode{
		".config/fish/config.fish": 0o644,
		".ssh/id_ed25519":          0o600, // secret
		"mise.toml":                0o644,
	}
	for rel, mode := range want {
		got, ok := snapshot[rel]
		if !ok {
			t.Errorf("HOME missing %q", rel)
			continue
		}
		if got.Perm() != mode {
			t.Errorf("%q perm = %o, want %o", rel, got.Perm(), mode)
		}
	}
	for rel := range snapshot {
		if _, ok := want[rel]; !ok {
			t.Errorf("HOME has unexpected file %q", rel)
		}
	}

	// mise.toml matches the composition (union of both components' tools).
	wantToml := "[tools]\n\"fish\" = \"latest\"\n\"python\" = \"3.12\"\n"
	if gotToml != wantToml {
		t.Errorf("mise.toml =\n%q\nwant\n%q", gotToml, wantToml)
	}

	// Session dir gone after teardown.
	if _, err := os.Stat(homeAtSpawn); !os.IsNotExist(err) {
		t.Errorf("session dir %s survived teardown (err=%v)", homeAtSpawn, err)
	}
}

// snapshotPerms records every regular file under root, keyed by its path
// relative to root, with its mode — the shape seam 3 asserts on.
func snapshotPerms(t *testing.T, root string) map[string]os.FileMode {
	t.Helper()
	out := map[string]os.FileMode{}
	err := filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		out[rel] = fi.Mode()
		return nil
	})
	if err != nil {
		t.Fatalf("walk HOME: %v", err)
	}
	return out
}

func TestLaunchWrongPassphraseFailsLoud(t *testing.T) {
	vaultPath := fixtureVault(t, "right")
	err := launch(launchConfig{
		vaultPath:      vaultPath,
		readPassphrase: func() (string, error) { return "wrong", nil },
		run:            func(*exec.Cmd) error { t.Fatal("shell spawned on wrong passphrase"); return nil },
	})
	if err == nil {
		t.Fatal("launch with wrong passphrase returned nil — must fail loud")
	}
}

// TestSessionWipeScrubsSecrets checks the mac-fallback path: a non-tmpfs session
// zeroes secret bytes before unlink. We can't observe the freed bytes, so we
// assert wipe removes the tree and that scrub survives a since-deleted file.
func TestSessionWipeScrubsSecrets(t *testing.T) {
	s := &session{home: t.TempDir(), onTmpfs: false}
	if err := s.materialize(Composed{
		Files:    []File{{Path: ".ssh/key", Content: "SECRET", Secret: true}},
		MiseToml: "[tools]\n",
	}); err != nil {
		t.Fatalf("materialize: %v", err)
	}
	secret := filepath.Join(s.home, ".ssh/key")
	if fi, err := os.Stat(secret); err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("secret perms = %v (err=%v), want 0600", fi, err)
	}
	s.wipe()
	if _, err := os.Stat(s.home); !os.IsNotExist(err) {
		t.Errorf("home survived wipe: %v", err)
	}
	s.wipe() // idempotent — sync.Once guards a double teardown
}

func TestScrubHandlesMissingFile(t *testing.T) {
	scrub(filepath.Join(t.TempDir(), "does-not-exist")) // must not panic
}

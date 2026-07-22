package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"ghoshell/internal/vault"
)

// packConfig carries the seams pack needs injected so it can be driven
// end-to-end in a test without a real TTY, a real editor session, or a real
// git remote: readPassphrase supplies the passphrase, stdin/stdout drive the
// TUI, and runGit executes the git commands that publish the re-sealed blob.
type packConfig struct {
	vaultPath      string
	readPassphrase func() (string, error)
	stdin          io.Reader
	stdout         io.Writer
	runGit         func(*exec.Cmd) error // default (*exec.Cmd).Run
}

func runPack(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "gho pack: need a vault path")
		return 2
	}
	cfg := packConfig{
		vaultPath:      args[0],
		readPassphrase: promptPassphrase,
		stdin:          os.Stdin,
		stdout:         stdout,
		runGit:         (*exec.Cmd).Run,
	}
	if err := pack(cfg); err != nil {
		fmt.Fprintf(stderr, "gho pack: %v\n", err)
		return 1
	}
	return 0
}

// pack is the whole authoring round trip: unlock the vault (or, if vaultPath
// doesn't exist yet, start from an empty one — the only way the very first
// vault ever gets created, since there's no separate init command), run the
// in-binary TUI over its manifest, and — only if the user saves — re-seal
// (vault.Seal always rotates salt+nonce) and push the new blob so a launch
// elsewhere pulls the change.
func pack(cfg packConfig) error {
	blob, err := os.ReadFile(cfg.vaultPath)
	bootstrap := os.IsNotExist(err)
	if err != nil && !bootstrap {
		return fmt.Errorf("read vault: %w", err)
	}
	pass, err := cfg.readPassphrase()
	if err != nil {
		return fmt.Errorf("read passphrase: %w", err)
	}

	var files []vault.File
	m := Manifest{Profiles: map[string][]string{}, Components: map[string]Component{}}
	if !bootstrap {
		files, err = vault.Unseal(pass, blob)
		if err != nil {
			return err
		}
		m, err = manifestFromVault(files)
		if err != nil {
			return err
		}
	} else {
		fmt.Fprintf(cfg.stdout, "pack: no vault at %s — creating a new one\n", cfg.vaultPath)
	}

	edited, save, err := runTUI(m, cfg.stdin, cfg.stdout)
	if err != nil {
		return fmt.Errorf("edit manifest: %w", err)
	}
	if !save {
		fmt.Fprintln(cfg.stdout, "pack: no changes saved")
		return nil
	}

	manifestJSON, err := json.Marshal(edited)
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	newBlob, err := vault.Seal(pass, spliceManifest(files, manifestJSON))
	if err != nil {
		return fmt.Errorf("seal vault: %w", err)
	}
	if err := os.WriteFile(cfg.vaultPath, newBlob, 0o600); err != nil {
		return fmt.Errorf("write vault: %w", err)
	}

	return pushVault(cfg)
}

// spliceManifest returns files with manifest.json's content replaced by data,
// preserving every other vault entry untouched — re-sealing must carry the
// whole vault forward, not just rebuild it from the one entry pack edits.
func spliceManifest(files []vault.File, data []byte) []vault.File {
	next := make([]vault.File, 0, len(files)+1)
	replaced := false
	for _, f := range files {
		if f.Path == manifestPath {
			f.Data = data
			replaced = true
		}
		next = append(next, f)
	}
	if !replaced {
		next = append(next, vault.File{Path: manifestPath, Mode: 0o600, Data: data})
	}
	return next
}

// pushVault commits the re-sealed blob and pushes it. Git resolves the repo
// from the vault's directory, so the vault must already live inside a clone
// with a remote configured.
func pushVault(cfg packConfig) error {
	dir := filepath.Dir(cfg.vaultPath)
	base := filepath.Base(cfg.vaultPath)
	steps := [][]string{
		{"add", base},
		{"commit", "-m", "pack: update vault"},
		{"push"},
	}
	for _, args := range steps {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if err := cfg.runGit(cmd); err != nil {
			return fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
		}
	}
	return nil
}

// runTUI is the in-binary manifest editor: a line-oriented REPL over the
// injected stdin/stdout so it can be driven by a real terminal or, in tests,
// by a canned script. It never touches disk — the caller decides whether to
// persist the returned manifest based on the save flag. Every mutation is
// validated against 03's disjointness rule before it's kept, so the in-memory
// manifest is never left in an ambiguous state.
func runTUI(m Manifest, in io.Reader, out io.Writer) (Manifest, bool, error) {
	// Edits land on a private working copy — Profiles/Components are maps, so
	// mutating m in place would leak edits into the caller even on quit. The
	// original is what quit hands back untouched.
	original := m
	work := cloneManifest(m)

	sc := bufio.NewScanner(in)
	fmt.Fprintln(out, "gho pack — manifest editor. Type 'help' for commands.")
	for {
		fmt.Fprint(out, "pack> ")
		if !sc.Scan() {
			return original, false, sc.Err()
		}
		fields := strings.Fields(sc.Text())
		if len(fields) == 0 {
			continue
		}
		var err error
		switch fields[0] {
		case "help":
			printHelp(out)
		case "profiles":
			listProfiles(out, work)
		case "components":
			listComponents(out, work)
		case "profile":
			err = handleProfile(&work, fields[1:], out)
		case "component":
			err = handleComponent(&work, fields[1:], out)
		case "save":
			if verr := validateProfiles(work, allProfileNames(work)); verr != nil {
				fmt.Fprintln(out, "cannot save:", verr)
				continue
			}
			return work, true, nil
		case "quit", "q":
			return original, false, nil
		default:
			fmt.Fprintf(out, "unknown command %q — type 'help'\n", fields[0])
		}
		if err != nil {
			fmt.Fprintln(out, "error:", err)
		}
	}
}

func printHelp(out io.Writer) {
	fmt.Fprint(out, `commands:
  profiles                              list profiles and their components
  components                            list components
  profile new <name>                    create an empty profile
  profile rm <name>                     delete a profile
  profile <name>                        show a profile's components
  profile <name> add <component>        add a component to a profile
  profile <name> rm <component>         remove a component from a profile
  component new <name>                  create an empty component
  component rm <name>                   delete a component
  component <name>                      show a component's tools and files
  component <name> tool add <spec>      add a mise tool spec, e.g. python@3.12
  component <name> tool rm <tool>       remove a tool by name
  component <name> file set <path> <content...>     add/replace a config file
  component <name> file secret <path> <content...>  add/replace a secret file
  component <name> file rm <path>       remove a file
  save                                  validate and save, then re-seal + push
  quit                                  discard changes and exit
`)
}

// cloneManifest deep-copies the maps/slices runTUI mutates in place, so an
// edit session's working copy never aliases the manifest the caller passed in.
func cloneManifest(m Manifest) Manifest {
	profiles := make(map[string][]string, len(m.Profiles))
	for name, comps := range m.Profiles {
		profiles[name] = append([]string{}, comps...)
	}
	components := make(map[string]Component, len(m.Components))
	for name, c := range m.Components {
		components[name] = Component{
			Tools: append([]string{}, c.Tools...),
			Files: append([]File{}, c.Files...),
		}
	}
	return Manifest{Profiles: profiles, Components: components}
}

func allProfileNames(m Manifest) []string {
	names := make([]string, 0, len(m.Profiles))
	for n := range m.Profiles {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// profilesUsing returns the (sorted) profiles that list component name — the
// set that must stay valid after an edit to that component.
func profilesUsing(m Manifest, name string) []string {
	var names []string
	for p, comps := range m.Profiles {
		for _, c := range comps {
			if c == name {
				names = append(names, p)
				break
			}
		}
	}
	sort.Strings(names)
	return names
}

// applyIfValid runs mutate against m, validates the profiles it affects (03's
// disjointness rule via Compose), and rolls back to the pre-mutation state on
// failure — the "at author time" enforcement every kind of edit in this TUI
// shares.
func applyIfValid(m *Manifest, affected []string, mutate func(*Manifest)) error {
	before := cloneManifest(*m)
	mutate(m)
	if err := validateProfiles(*m, affected); err != nil {
		*m = before
		return err
	}
	return nil
}

func validateProfiles(m Manifest, names []string) error {
	for _, name := range names {
		if _, err := Compose(m, name); err != nil {
			return err
		}
	}
	return nil
}

func listProfiles(out io.Writer, m Manifest) {
	for _, name := range allProfileNames(m) {
		fmt.Fprintf(out, "%s: %s\n", name, strings.Join(m.Profiles[name], ", "))
	}
}

func listComponents(out io.Writer, m Manifest) {
	names := make([]string, 0, len(m.Components))
	for n := range m.Components {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		c := m.Components[name]
		fmt.Fprintf(out, "%s: %d tool(s), %d file(s)\n", name, len(c.Tools), len(c.Files))
	}
}

func printComponent(out io.Writer, name string, c Component) {
	fmt.Fprintf(out, "%s tools: %s\n", name, strings.Join(c.Tools, ", "))
	for _, f := range c.Files {
		secret := ""
		if f.Secret {
			secret = " (secret)"
		}
		fmt.Fprintf(out, "%s file: %s%s\n", name, f.Path, secret)
	}
}

// handleProfile dispatches "profile ..." subcommands. args is the line's
// fields after "profile".
func handleProfile(m *Manifest, args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: profile new|rm|<name> ...")
	}
	switch args[0] {
	case "new":
		if len(args) != 2 {
			return fmt.Errorf("usage: profile new <name>")
		}
		name := args[1]
		if _, exists := m.Profiles[name]; exists {
			return fmt.Errorf("profile %q already exists", name)
		}
		m.Profiles[name] = []string{}
		return nil
	case "rm":
		if len(args) != 2 {
			return fmt.Errorf("usage: profile rm <name>")
		}
		name := args[1]
		if _, exists := m.Profiles[name]; !exists {
			return fmt.Errorf("profile %q does not exist", name)
		}
		delete(m.Profiles, name)
		return nil
	default:
		name := args[0]
		comps, exists := m.Profiles[name]
		if !exists {
			return fmt.Errorf("profile %q does not exist", name)
		}
		if len(args) == 1 {
			fmt.Fprintf(out, "%s: %s\n", name, strings.Join(comps, ", "))
			return nil
		}
		if len(args) != 3 {
			return fmt.Errorf("usage: profile %s add|rm <component>", name)
		}
		action, comp := args[1], args[2]
		var next []string
		switch action {
		case "add":
			if _, ok := m.Components[comp]; !ok {
				return fmt.Errorf("component %q does not exist", comp)
			}
			for _, c := range comps {
				if c == comp {
					return fmt.Errorf("profile %q already has component %q", name, comp)
				}
			}
			next = append(append([]string{}, comps...), comp)
		case "rm":
			for _, c := range comps {
				if c != comp {
					next = append(next, c)
				}
			}
			if len(next) == len(comps) {
				return fmt.Errorf("profile %q does not have component %q", name, comp)
			}
		default:
			return fmt.Errorf("usage: profile %s add|rm <component>", name)
		}
		return applyIfValid(m, []string{name}, func(m *Manifest) { m.Profiles[name] = next })
	}
}

// handleComponent dispatches "component ..." subcommands. args is the line's
// fields after "component".
func handleComponent(m *Manifest, args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: component new|rm|<name> ...")
	}
	switch args[0] {
	case "new":
		if len(args) != 2 {
			return fmt.Errorf("usage: component new <name>")
		}
		name := args[1]
		if _, exists := m.Components[name]; exists {
			return fmt.Errorf("component %q already exists", name)
		}
		m.Components[name] = Component{}
		return nil
	case "rm":
		if len(args) != 2 {
			return fmt.Errorf("usage: component rm <name>")
		}
		name := args[1]
		if _, exists := m.Components[name]; !exists {
			return fmt.Errorf("component %q does not exist", name)
		}
		delete(m.Components, name)
		return nil
	default:
		name := args[0]
		c, exists := m.Components[name]
		if !exists {
			return fmt.Errorf("component %q does not exist", name)
		}
		if len(args) == 1 {
			printComponent(out, name, c)
			return nil
		}
		switch args[1] {
		case "tool":
			return handleComponentTool(m, name, args[2:])
		case "file":
			return handleComponentFile(m, name, args[2:])
		default:
			return fmt.Errorf("usage: component %s tool|file ...", name)
		}
	}
}

func handleComponentTool(m *Manifest, name string, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: component %s tool add|rm <spec>", name)
	}
	c := m.Components[name]
	var next []string
	switch args[0] {
	case "add":
		next = append(append([]string{}, c.Tools...), args[1])
	case "rm":
		toolName := args[1]
		for _, t := range c.Tools {
			n, _ := splitToolSpec(t)
			if n != toolName {
				next = append(next, t)
			}
		}
	default:
		return fmt.Errorf("usage: component %s tool add|rm <spec>", name)
	}
	return applyIfValid(m, profilesUsing(*m, name), func(m *Manifest) {
		c := m.Components[name]
		c.Tools = next
		m.Components[name] = c
	})
}

// handleComponentFile handles "file set|secret|rm ..." for one component.
// Content collapses interior whitespace runs to single spaces — a documented
// v1 ceiling; ponytail: fine for one-line config snippets, raw multi-line
// entry would need a different input mode if that's ever wanted.
func handleComponentFile(m *Manifest, name string, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: component %s file set|secret|rm <path> [content...]", name)
	}
	c := m.Components[name]
	var next []File
	switch args[0] {
	case "set", "secret":
		if len(args) < 3 {
			return fmt.Errorf("usage: component %s file %s <path> <content...>", name, args[0])
		}
		path, content := args[1], strings.Join(args[2:], " ")
		next = setFile(c.Files, File{Path: path, Content: content, Secret: args[0] == "secret"})
	case "rm":
		next = removeFile(c.Files, args[1])
		if len(next) == len(c.Files) {
			return fmt.Errorf("component %q has no file %q", name, args[1])
		}
	default:
		return fmt.Errorf("usage: component %s file set|secret|rm <path> [content...]", name)
	}
	return applyIfValid(m, profilesUsing(*m, name), func(m *Manifest) {
		c := m.Components[name]
		c.Files = next
		m.Components[name] = c
	})
}

func setFile(files []File, f File) []File {
	next := make([]File, 0, len(files)+1)
	replaced := false
	for _, existing := range files {
		if existing.Path == f.Path {
			next = append(next, f)
			replaced = true
			continue
		}
		next = append(next, existing)
	}
	if !replaced {
		next = append(next, f)
	}
	return next
}

func removeFile(files []File, path string) []File {
	var next []File
	for _, f := range files {
		if f.Path != path {
			next = append(next, f)
		}
	}
	return next
}

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"ghoshell/internal/vault"
)

// runScript drives runTUI with a canned command script and returns the
// resulting manifest, whether it was saved, and everything written to stdout.
func runScript(t *testing.T, m Manifest, script string) (Manifest, bool, string) {
	t.Helper()
	var out bytes.Buffer
	got, save, err := runTUI(m, strings.NewReader(script), &out)
	if err != nil {
		t.Fatalf("runTUI: %v", err)
	}
	return got, save, out.String()
}

func baseManifest() Manifest {
	return Manifest{
		Profiles: map[string][]string{
			"work": {"base"},
		},
		Components: map[string]Component{
			"base": {
				Tools: []string{"fish"},
				Files: []File{{Path: ".config/fish/config.fish", Content: "set -g fish_greeting"}},
			},
		},
	}
}

func TestRunTUIQuitDiscardsChanges(t *testing.T) {
	m := baseManifest()
	got, save, _ := runScript(t, m, "profile new personal\nquit\n")
	if save {
		t.Fatal("quit reported save=true")
	}
	if _, exists := got.Profiles["personal"]; exists {
		t.Fatal("quit kept the edit — should discard")
	}
}

func TestRunTUISaveAppliesEdits(t *testing.T) {
	m := baseManifest()
	got, save, _ := runScript(t, m, strings.Join([]string{
		"component new python-dev",
		"component python-dev tool add python@3.12",
		"component python-dev file set .config/pyright.json {}",
		"profile new personal",
		"profile personal add base",
		"profile personal add python-dev",
		"save",
	}, "\n")+"\n")
	if !save {
		t.Fatal("save reported save=false")
	}
	if got.Profiles["personal"] == nil {
		t.Fatal("new profile missing")
	}
	if comps := got.Profiles["personal"]; len(comps) != 2 {
		t.Fatalf("personal components = %v, want 2", comps)
	}
	c, ok := got.Components["python-dev"]
	if !ok {
		t.Fatal("new component missing")
	}
	if len(c.Tools) != 1 || c.Tools[0] != "python@3.12" {
		t.Fatalf("tools = %v", c.Tools)
	}
	if len(c.Files) != 1 || c.Files[0].Path != ".config/pyright.json" || c.Files[0].Content != "{}" {
		t.Fatalf("files = %v", c.Files)
	}

	// Composing the edited manifest must still succeed — the TUI's own
	// validation and 03's Compose agree.
	if _, err := Compose(got, "personal"); err != nil {
		t.Fatalf("Compose(personal): %v", err)
	}
}

func TestRunTUIRejectsConflictingToolAtAuthorTime(t *testing.T) {
	m := Manifest{
		Profiles: map[string][]string{"p": {"a"}},
		Components: map[string]Component{
			"a": {Tools: []string{"python@3.11"}},
			"b": {Tools: []string{"python@3.12"}},
		},
	}
	got, _, out := runScript(t, m, "profile p add b\nquit\n")
	if !strings.Contains(out, "error:") {
		t.Fatalf("expected an error on conflicting add, got:\n%s", out)
	}
	if len(got.Profiles["p"]) != 1 {
		t.Fatalf("conflicting add was not rolled back: %v", got.Profiles["p"])
	}
}

func TestRunTUIRejectsSamePathFileAtAuthorTime(t *testing.T) {
	m := Manifest{
		Profiles: map[string][]string{"p": {"a"}},
		Components: map[string]Component{
			"a": {Files: []File{{Path: ".bashrc", Content: "a"}}},
			"b": {Files: []File{{Path: ".bashrc", Content: "b"}}},
		},
	}
	got, _, out := runScript(t, m, "profile p add b\nquit\n")
	if !strings.Contains(out, "error:") {
		t.Fatalf("expected an error on conflicting path, got:\n%s", out)
	}
	if len(got.Profiles["p"]) != 1 {
		t.Fatalf("conflicting add was not rolled back: %v", got.Profiles["p"])
	}
}

func TestRunTUISaveRefusesInvalidManifest(t *testing.T) {
	// Two components each fine alone; hand-craft a manifest that is already
	// invalid (bypassing the TUI's own guards) to prove save() re-validates
	// as a final safety net.
	m := Manifest{
		Profiles: map[string][]string{"p": {"a", "b"}},
		Components: map[string]Component{
			"a": {Tools: []string{"python@3.11"}},
			"b": {Tools: []string{"python@3.12"}},
		},
	}
	_, save, out := runScript(t, m, "save\nquit\n")
	if save {
		t.Fatal("save on an invalid manifest returned save=true")
	}
	if !strings.Contains(out, "cannot save") {
		t.Fatalf("expected 'cannot save', got:\n%s", out)
	}
}

func TestRunTUIToolRmAndFileRm(t *testing.T) {
	m := Manifest{
		Profiles: map[string][]string{"p": {"a"}},
		Components: map[string]Component{
			"a": {
				Tools: []string{"fish", "python@3.12"},
				Files: []File{{Path: ".bashrc", Content: "x"}},
			},
		},
	}
	got, save, _ := runScript(t, m, strings.Join([]string{
		"component a tool rm python",
		"component a file rm .bashrc",
		"save",
	}, "\n")+"\n")
	if !save {
		t.Fatal("save=false")
	}
	c := got.Components["a"]
	if len(c.Tools) != 1 || c.Tools[0] != "fish" {
		t.Fatalf("tools after rm = %v", c.Tools)
	}
	if len(c.Files) != 0 {
		t.Fatalf("files after rm = %v", c.Files)
	}
}

// fixtureVaultAt seals m at path under pass — the pack-side counterpart of
// launch_test.go's fixtureVault, parameterized on path since pack needs the
// vault to live inside a git working tree it controls.
func fixtureVaultAt(t *testing.T, path, pass string, m Manifest) {
	t.Helper()
	manifestJSON, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	blob, err := vault.Seal(pass, []vault.File{{Path: manifestPath, Mode: 0o600, Data: manifestJSON}})
	if err != nil {
		t.Fatalf("seal fixture: %v", err)
	}
	if err := os.WriteFile(path, blob, 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
}

func runGitOrSkip(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// TestPackRoundTrip is 07's own round-trip check: pack a change through a
// real local git remote, then unseal the pushed blob from a fresh clone and
// confirm the edit is present.
func TestPackRoundTrip(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	const pass = "correct horse battery staple"

	remote := t.TempDir()
	runGitOrSkip(t, remote, "init", "--bare", "-b", "main", ".")

	clone := t.TempDir()
	runGitOrSkip(t, clone, "clone", remote, ".")
	runGitOrSkip(t, clone, "config", "user.email", "test@example.com")
	runGitOrSkip(t, clone, "config", "user.name", "Test")

	vaultPath := filepath.Join(clone, "vault.blob")
	fixtureVaultAt(t, vaultPath, pass, baseManifest())
	runGitOrSkip(t, clone, "add", "vault.blob")
	runGitOrSkip(t, clone, "commit", "-m", "initial vault")
	runGitOrSkip(t, clone, "push", "-u", "origin", "main")

	cfg := packConfig{
		vaultPath:      vaultPath,
		readPassphrase: func() (string, error) { return pass, nil },
		stdin:          strings.NewReader("profile new personal\nprofile personal add base\nsave\n"),
		stdout:         &bytes.Buffer{},
		runGit:         (*exec.Cmd).Run,
	}
	if err := pack(cfg); err != nil {
		t.Fatalf("pack: %v", err)
	}

	// Fetch from the remote into a second clone to prove the push actually
	// landed there, not just in the working clone.
	fetch := t.TempDir()
	runGitOrSkip(t, fetch, "clone", remote, ".")
	blob, err := os.ReadFile(filepath.Join(fetch, "vault.blob"))
	if err != nil {
		t.Fatalf("read pushed vault: %v", err)
	}
	files, err := vault.Unseal(pass, blob)
	if err != nil {
		t.Fatalf("unseal pushed vault: %v", err)
	}
	m, err := manifestFromVault(files)
	if err != nil {
		t.Fatalf("manifestFromVault: %v", err)
	}
	if _, ok := m.Profiles["personal"]; !ok {
		t.Fatalf("pushed vault missing the packed edit: %+v", m.Profiles)
	}
}

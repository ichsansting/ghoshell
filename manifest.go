package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// File is one path-keyed payload entry. secret:true files are materialized 0600.
type File struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Secret  bool   `json:"secret,omitempty"`
}

// Component bundles mise tool specs and files. Tools are mise spec strings
// ("fish", "python@3.12"); a bare name floats @latest.
type Component struct {
	Tools []string `json:"tools,omitempty"`
	Files []File   `json:"files,omitempty"`
}

// Manifest is the whole payload source of truth: profiles map to component
// names, components hold the actual tools/files.
type Manifest struct {
	Profiles   map[string][]string  `json:"profiles"`
	Components map[string]Component `json:"components"`
}

// Composed is the flat union a launch materializes: files (sorted by path) plus
// a generated mise.toml [tools] table.
type Composed struct {
	Files    []File
	MiseToml string
}

// ParseManifest decodes the JSON manifest. Manifests are TUI-authored, not
// hand-written, so JSON (stdlib) is enough — no YAML dependency.
func ParseManifest(data []byte) (Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("parse manifest: %w", err)
	}
	return m, nil
}

// Compose returns the flat union of profileName's components as files + a
// mise.toml. It is pure (no I/O) and enforces the disjointness rule: two
// components in the profile may not place a file at the same path or name the
// same tool with conflicting versions.
func Compose(m Manifest, profileName string) (Composed, error) {
	comps, ok := m.Profiles[profileName]
	if !ok {
		return Composed{}, fmt.Errorf("unknown profile %q", profileName)
	}

	files := map[string]File{}   // path -> File, for conflict detection
	tools := map[string]string{} // tool name -> version, "" means @latest

	for _, name := range comps {
		c, ok := m.Components[name]
		if !ok {
			return Composed{}, fmt.Errorf("profile %q references unknown component %q", profileName, name)
		}
		for _, f := range c.Files {
			if _, dup := files[f.Path]; dup {
				return Composed{}, fmt.Errorf("profile %q: two components place a file at path %q", profileName, f.Path)
			}
			files[f.Path] = f
		}
		for _, spec := range c.Tools {
			tool, version := splitToolSpec(spec)
			if prev, seen := tools[tool]; seen && prev != version {
				return Composed{}, fmt.Errorf("profile %q: conflicting versions for tool %q (%q vs %q)", profileName, tool, prev, version)
			}
			tools[tool] = version
		}
	}

	return Composed{Files: sortedFiles(files), MiseToml: renderMiseToml(tools)}, nil
}

// splitToolSpec parses a mise spec: "python@3.12" -> ("python", "3.12"), a bare
// "fish" -> ("fish", "") which renders as @latest.
func splitToolSpec(spec string) (name, version string) {
	if name, version, found := strings.Cut(spec, "@"); found {
		return name, version
	}
	return spec, ""
}

func sortedFiles(files map[string]File) []File {
	out := make([]File, 0, len(files))
	for _, f := range files {
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// renderMiseToml emits a [tools] table sorted by name. Bare tools float latest.
func renderMiseToml(tools map[string]string) string {
	names := make([]string, 0, len(tools))
	for name := range tools {
		names = append(names, name)
	}
	sort.Strings(names)

	var b strings.Builder
	b.WriteString("[tools]\n")
	for _, name := range names {
		version := tools[name]
		if version == "" {
			version = "latest"
		}
		fmt.Fprintf(&b, "%q = %q\n", name, version)
	}
	return b.String()
}

package main

import (
	"strings"
	"testing"
)

// A small manifest reused across composition tests: three components, two
// profiles. "unused" is never listed by any tested profile.
const sampleJSON = `{
  "profiles": {
    "work":     ["base", "python-dev"],
    "personal": ["base"]
  },
  "components": {
    "base": {
      "tools": ["fish", "starship"],
      "files": [
        {"path": ".config/fish/config.fish", "content": "set -g fish_greeting"},
        {"path": ".ssh/id_ed25519", "content": "KEY", "secret": true}
      ]
    },
    "python-dev": {
      "tools": ["python@3.12", "pyright"],
      "files": [
        {"path": ".config/pyright.json", "content": "{}"}
      ]
    },
    "unused": {
      "tools": ["should-not-appear"],
      "files": [{"path": ".should-not-appear", "content": "x"}]
    }
  }
}`

func mustParse(t *testing.T, s string) Manifest {
	t.Helper()
	m, err := ParseManifest([]byte(s))
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	return m
}

func TestCompose(t *testing.T) {
	m := mustParse(t, sampleJSON)

	t.Run("union of a profile's components", func(t *testing.T) {
		got, err := Compose(m, "work")
		if err != nil {
			t.Fatalf("Compose: %v", err)
		}
		wantPaths := []string{".config/fish/config.fish", ".config/pyright.json", ".ssh/id_ed25519"}
		var gotPaths []string
		for _, f := range got.Files {
			gotPaths = append(gotPaths, f.Path)
		}
		if strings.Join(gotPaths, ",") != strings.Join(wantPaths, ",") {
			t.Errorf("files = %v, want %v (sorted union)", gotPaths, wantPaths)
		}
		// secret flag carried through the union.
		for _, f := range got.Files {
			if f.Path == ".ssh/id_ed25519" && !f.Secret {
				t.Errorf("secret file lost secret flag: %+v", f)
			}
		}
		wantToml := "[tools]\n\"fish\" = \"latest\"\n\"pyright\" = \"latest\"\n\"python\" = \"3.12\"\n\"starship\" = \"latest\"\n"
		if got.MiseToml != wantToml {
			t.Errorf("mise.toml =\n%q\nwant\n%q", got.MiseToml, wantToml)
		}
	})

	t.Run("non-member components never appear", func(t *testing.T) {
		got, err := Compose(m, "personal")
		if err != nil {
			t.Fatalf("Compose: %v", err)
		}
		for _, f := range got.Files {
			if strings.Contains(f.Path, "should-not-appear") {
				t.Errorf("leaked non-member file %q", f.Path)
			}
		}
		if strings.Contains(got.MiseToml, "should-not-appear") {
			t.Errorf("leaked non-member tool in %q", got.MiseToml)
		}
	})

	t.Run("bare tool floats @latest", func(t *testing.T) {
		got, _ := Compose(m, "personal")
		if !strings.Contains(got.MiseToml, "\"fish\" = \"latest\"") {
			t.Errorf("bare tool did not float latest: %q", got.MiseToml)
		}
	})

	t.Run("same tool same version dedupes", func(t *testing.T) {
		m := mustParse(t, `{
          "profiles": {"p": ["a", "b"]},
          "components": {
            "a": {"tools": ["node@20"]},
            "b": {"tools": ["node@20"]}
          }
        }`)
		got, err := Compose(m, "p")
		if err != nil {
			t.Fatalf("Compose: %v", err)
		}
		if n := strings.Count(got.MiseToml, "node"); n != 1 {
			t.Errorf("node appears %d times, want 1 (deduped): %q", n, got.MiseToml)
		}
	})
}

func TestComposeRejects(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		profile string
		wantErr string
	}{
		{
			// Same path is rejected even when content is byte-identical:
			// disjointness is a path rule, and a silent last-write-wins could
			// downgrade a secret file's flags depending on component order.
			name: "two components place a file at the same path",
			json: `{
              "profiles": {"p": ["a", "b"]},
              "components": {
                "a": {"files": [{"path": ".bashrc", "content": "same"}]},
                "b": {"files": [{"path": ".bashrc", "content": "same"}]}
              }
            }`,
			profile: "p",
			wantErr: "path",
		},
		{
			name: "same tool with conflicting versions",
			json: `{
              "profiles": {"p": ["a", "b"]},
              "components": {
                "a": {"tools": ["python@3.11"]},
                "b": {"tools": ["python@3.12"]}
              }
            }`,
			profile: "p",
			wantErr: "python",
		},
		{
			name:    "unknown profile",
			json:    `{"profiles": {"p": []}, "components": {}}`,
			profile: "nope",
			wantErr: "profile",
		},
		{
			name: "profile references unknown component",
			json: `{
              "profiles": {"p": ["ghost"]},
              "components": {}
            }`,
			profile: "p",
			wantErr: "component",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := mustParse(t, tt.json)
			_, err := Compose(m, tt.profile)
			if err == nil {
				t.Fatalf("Compose(%q) = nil error, want error containing %q", tt.profile, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestParseManifestRejectsGarbage(t *testing.T) {
	if _, err := ParseManifest([]byte("not json")); err == nil {
		t.Error("ParseManifest(garbage) = nil error, want parse error")
	}
}

# 03 — Manifest schema + profile composition

**What to build:** The manifest data model and the pure composition function (Seam 2).
Parse a manifest with a top-level `profiles:` map (each profile = a list of component
names) and components (each = mise tool specs + config files + secret files). Given a
manifest and a profile name, compute the flat union of that profile's components as the set
of files plus a generated `mise.toml` `[tools]` table. Author-time validation rejects
ambiguous profiles.

**Blocked by:** 01.

**Status:** done

- [x] Parse the manifest: `profiles:` map of name → component-name list, and component
      definitions of `{ mise specs + config files + secret files }`.
- [x] `compose(manifest, profileName) → { files, miseToml }` returns the flat union of the
      profile's components' files and `[tools]` specs.
- [x] Disjointness rule enforced at author/validation time: reject a profile whose
      components place two files at the same path, or name the same tool with conflicting
      versions.
- [x] Components not listed by the profile never appear in the output.
- [x] Tool specs float `@latest` (no pinning/lockfile in v1).
- [x] Pure — no filesystem or network I/O. Table-driven `_test.go` for union, both
      disjointness rejections, and exclusion of non-member components.

Manifest is JSON (`encoding/json`, stdlib), not YAML: ticket says only "parse the
manifest", the manifest is TUI-authored (07) not hand-written, and this keeps the
single-`x/crypto`-dependency posture. TUI in 07 must emit JSON.

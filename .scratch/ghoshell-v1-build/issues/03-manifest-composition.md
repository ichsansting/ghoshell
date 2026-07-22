# 03 — Manifest schema + profile composition

**What to build:** The manifest data model and the pure composition function (Seam 2).
Parse a manifest with a top-level `profiles:` map (each profile = a list of component
names) and components (each = mise tool specs + config files + secret files). Given a
manifest and a profile name, compute the flat union of that profile's components as the set
of files plus a generated `mise.toml` `[tools]` table. Author-time validation rejects
ambiguous profiles.

**Blocked by:** 01.

**Status:** ready-for-agent

- [ ] Parse the manifest: `profiles:` map of name → component-name list, and component
      definitions of `{ mise specs + config files + secret files }`.
- [ ] `compose(manifest, profileName) → { files, miseToml }` returns the flat union of the
      profile's components' files and `[tools]` specs.
- [ ] Disjointness rule enforced at author/validation time: reject a profile whose
      components place two files at the same path, or name the same tool with conflicting
      versions.
- [ ] Components not listed by the profile never appear in the output.
- [ ] Tool specs float `@latest` (no pinning/lockfile in v1).
- [ ] Pure — no filesystem or network I/O. Table-driven `_test.go` for union, both
      disjointness rejections, and exclusion of non-member components.

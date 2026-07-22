# 08 — Release build: cross-compile 3 arches + publish assets

**What to build:** The delivery plumbing that makes the one-liner resolve real artifacts.
CI cross-compiles the launcher for all three targets and publishes them as GitHub Release
assets, alongside the vault blob in the repo — so `curl <repo>/gho.sh | sh` (06) fetches a
real binary for the detected arch and a real vault, end to end.

**Blocked by:** 06, 07.

**Status:** done (narrowed scope — see Comments)

- [ ] CI cross-compiles static binaries for `darwin/arm64`, `linux/arm64`, `linux/amd64`
      (`CGO_ENABLED=0`). — descoped, see Comments.
- [ ] Each arch binary is published as a versioned GitHub Release asset the bootstrap (06)
      can select by `uname`. — descoped, see Comments.
- [ ] Release tagging gives launcher versioning/rollback for free; vault rollback rides the
      repo's commit history. — descoped, see Comments.
- [ ] End-to-end smoke: the one-liner on a linux target fetches the published binary + vault
      and reaches the launch flow. — descoped along with the release pipeline it depends on,
      see Comments.
- [x] `gho.sh`'s `GHOSHELL_REPO` is stamped with the real repo (`ichsansting/ghoshell`)
      instead of the `OWNER/ghoshell` placeholder.
- [x] `gho pack` can create the first vault, not just re-seal an existing one — a gap that
      blocked ever getting a real vault into the repo at all.

## Comments

Re-scoped with the user mid-implementation: no GitHub Actions release workflow for now (they
build/install the `gho` binary themselves — `go build -o gho .`). What shipped instead:

1. `gho.sh` now points at the real repo (`ichsansting/ghoshell`) rather than the placeholder.
2. `pack()` (`pack.go`) previously required an existing vault (`os.ReadFile` on a missing
   `vaultPath` was a hard error) — there was no way to create the very first vault, since
   `gho pack` only re-seals. Added a bootstrap path: a missing `vaultPath` now starts from an
   empty manifest instead of erroring, so `gho pack <new-path>` creates *and* pushes the first
   vault. Covered by `TestPackBootstrapsNewVault` (real local git remote, same pattern as
   `TestPackRoundTrip`).

Deliberately NOT done here, left for the user:
- Actually authoring the real vault content (a profile with `fish` + `starship` tools and a
  `.config/fish/config.fish` sourcing `starship init fish`) — passphrase is the user's to set,
  so they run `gho pack vault.bin` themselves at their own terminal rather than me scripting a
  passphrase into a command.
- The CI cross-compile + GitHub Release publish pipeline this ticket was originally scoped
  around. If that's wanted later, `gho.sh` already expects release assets named
  `gho-<os>-<arch>` (e.g. `gho-linux-amd64`) fetched via
  `github.com/<repo>/releases/latest/download/`, so a future workflow just needs to produce
  and upload those under that naming.

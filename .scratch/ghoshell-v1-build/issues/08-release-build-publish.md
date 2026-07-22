# 08 — Release build: cross-compile 3 arches + publish assets

**What to build:** The delivery plumbing that makes the one-liner resolve real artifacts.
CI cross-compiles the launcher for all three targets and publishes them as GitHub Release
assets, alongside the vault blob in the repo — so `curl <repo>/gho.sh | sh` (06) fetches a
real binary for the detected arch and a real vault, end to end.

**Blocked by:** 06, 07.

**Status:** done (manual release, no CI — see Comments)

- [x] Binaries cross-compiled for `darwin/arm64`, `linux/arm64`, `linux/amd64`
      (`CGO_ENABLED=0`) — done manually (`go build`), not via CI. See Comments.
- [x] Each arch binary published as a GitHub Release asset (`gho-<os>-<arch>`) the
      bootstrap (06) selects by `uname` — done manually via `gh release create`, tag
      `v0.1.0`. Not wired to CI, so cutting the next release is a manual repeat of the
      same steps.
- [ ] Release tagging gives launcher versioning/rollback for free — the mechanism exists
      (tags), but nothing automates cutting a new one; each release is a manual step for
      now.
- [x] End-to-end smoke: verified against the real repo — `gho.sh` fetched unmodified from
      `raw.githubusercontent.com`, resolves the release asset via a real 302 redirect to
      `releases/download/v0.1.0/gho-linux-amd64`, and `vault.bin` fetches 200 from
      `main`. Ran the fetched binary against the fetched vault directly (this sandbox's
      `/dev/shm` is mounted `noexec`, so the one-liner's own `exec` step can't be
      exercised here — see Comments) and confirmed it reaches `gho launch`'s passphrase
      prompt.
- [x] `gho.sh`'s `GHOSHELL_REPO` is stamped with the real repo (`ichsansting/ghoshell`)
      instead of the `OWNER/ghoshell` placeholder.
- [x] `gho pack` can create the first vault, not just re-seal an existing one — a gap that
      blocked ever getting a real vault into the repo at all.

## Comments

Re-scoped with the user mid-implementation: no GitHub Actions workflow — they build the
`gho` binary and (for now) cut releases manually via `gh release create`/`gh release
upload` rather than through CI. What shipped:

1. `gho.sh` now points at the real repo (`ichsansting/ghoshell`) rather than the placeholder.
2. `pack()` (`pack.go`) previously required an existing vault (`os.ReadFile` on a missing
   `vaultPath` was a hard error) — there was no way to create the very first vault, since
   `gho pack` only re-seals. Added a bootstrap path: a missing `vaultPath` now starts from an
   empty manifest instead of erroring, so `gho pack <new-path>` creates *and* pushes the first
   vault. Covered by `TestPackBootstrapsNewVault` (real local git remote, same pattern as
   `TestPackRoundTrip`). The user ran this themselves to author a real profile (`fish` +
   `starship` tools, a `.config/fish/config.fish` sourcing `starship init fish`) — passphrase
   was theirs, entered at their own terminal, never seen or scripted by the agent.
3. Cross-compiled all three targets (`CGO_ENABLED=0`) and published them as release `v0.1.0`
   via `gh release create` — a one-off manual run, not a workflow. Verified end-to-end against
   the real GitHub repo (raw fetch of `gho.sh`, redirect resolution of the release asset,
   fetch of `vault.bin`, and running the fetched binary against the fetched vault to confirm
   it reaches the passphrase prompt). The one-liner itself (`curl | sh`) could not be run
   start-to-finish in this sandbox because its container mounts `/dev/shm` `noexec`, which
   blocks `gho.sh`'s own `exec` of the binary it downloads there — that's this sandbox's
   restriction, not a bug in `gho.sh` (ticket 06); worth knowing about if it ever shows up on
   a real machine with a hardened `/dev/shm`, since `gho_workdir` currently checks only `-d`
   and `-w`, not executability, before picking a base dir.

Deliberately not automated: cutting a *new* release still means re-running the cross-compile
+ `gh release create` steps by hand. `gho.sh` already expects release assets named
`gho-<os>-<arch>` fetched via `github.com/<repo>/releases/latest/download/`, so a future CI
workflow just needs to produce and upload those under that naming — no changes needed on the
`gho.sh` side.

# 08 — Release build: cross-compile 3 arches + publish assets

**What to build:** The delivery plumbing that makes the one-liner resolve real artifacts.
CI cross-compiles the launcher for all three targets and publishes them as GitHub Release
assets, alongside the vault blob in the repo — so `curl <repo>/gho.sh | sh` (06) fetches a
real binary for the detected arch and a real vault, end to end.

**Blocked by:** 06, 07.

**Status:** ready-for-agent

- [ ] CI cross-compiles static binaries for `darwin/arm64`, `linux/arm64`, `linux/amd64`
      (`CGO_ENABLED=0`).
- [ ] Each arch binary is published as a versioned GitHub Release asset the bootstrap (06)
      can select by `uname`.
- [ ] Release tagging gives launcher versioning/rollback for free; vault rollback rides the
      repo's commit history.
- [ ] End-to-end smoke: the one-liner on a linux target fetches the published binary + vault
      and reaches the launch flow.

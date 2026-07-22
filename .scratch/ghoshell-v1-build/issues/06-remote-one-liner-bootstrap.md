# 06 — Remote one-liner: POSIX-sh bootstrap + Releases + vault pull

**What to build:** The paste-anywhere path. A user pastes `curl <repo-url>/gho.sh | sh`
into any shell; a POSIX-sh bootstrap detects OS/arch via `uname`, fetches the matching
launcher binary from GitHub Releases and the vault blob over credentialless HTTPS, then
execs launch. The paste carries only the repo URL — no secret, no profile name.

**Blocked by:** 04.

**Status:** ready-for-agent

- [ ] A POSIX-sh bootstrap script runs under a bare shell with only `curl`-or-`wget`.
- [ ] `uname`-detects OS/arch and selects the matching launcher Release asset (3-target
      matrix; missing arch fails with a clear message).
- [ ] Fetches the vault blob over plain HTTPS GET (no credentials).
- [ ] Repo location comes from the pasted URL; the paste contains no secret and no profile
      name.
- [ ] Hands off to the launch flow from 04 (passphrase → decrypt → materialize → shell).
- [ ] Integrity is TLS-only for v1 (checksum/signing are documented later bolt-ons).

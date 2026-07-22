# 06 — Remote one-liner: POSIX-sh bootstrap + Releases + vault pull

**What to build:** The paste-anywhere path. A user pastes `curl <repo-url>/gho.sh | sh`
into any shell; a POSIX-sh bootstrap detects OS/arch via `uname`, fetches the matching
launcher binary from GitHub Releases and the vault blob over credentialless HTTPS, then
execs launch. The paste carries only the repo URL — no secret, no profile name.

**Blocked by:** 04.

**Status:** done

- [x] A POSIX-sh bootstrap script runs under a bare shell with only `curl`-or-`wget`.
- [x] `uname`-detects OS/arch and selects the matching launcher Release asset (3-target
      matrix; missing arch fails with a clear message).
- [x] Fetches the vault blob over plain HTTPS GET (no credentials).
- [x] Repo location comes from the pasted URL; the paste contains no secret and no profile
      name.
- [x] Hands off to the launch flow from 04 (passphrase → decrypt → materialize → shell).
- [x] Integrity is TLS-only for v1 (checksum/signing are documented later bolt-ons).

## Comments

`gho.sh` (repo root) is the bootstrap; `gho_test.sh` is its self-check (no
network, no framework — mocks `uname` and the internal `gho_has` curl/wget
switch). Verified separately against a real local HTTP server that the
actual `curl -fsSL -o` / `wget -q -O` invocations fetch bytes correctly and
the result chmods+execs.

Repo identity (`GHOSHELL_REPO`/`GHOSHELL_BRANCH`/`GHOSHELL_VAULT_PATH`) is
baked into the script as constants rather than parsed from the fetch URL —
a piped POSIX-sh script can't see the URL it was fetched from. "Repo
location comes from the pasted URL" holds because the script lives *in*
that same repo: whichever URL you paste is where these constants point.
The constants here are still the `OWNER/ghoshell` placeholder — 08 (publish)
must stamp in the real repo before the pasted one-liner resolves to
anything; this ticket built the mechanism, 08 wires it live.

On the download scratch dir: `exec` hands off to `gho launch` on success, so
the trap that cleans up `dir` on early failure never runs on that path — the
downloaded binary and vault copy are left behind. Accepted ceiling, noted
inline in `gho.sh`: neither file is secret (the vault is public+encrypted
per spec), and `dir` is RAM-backed or `$TMPDIR`-bounded either way.

**Follow-up bug (found during 08's real-repo verification):** `curl -fsSL url
| sh` makes `sh`'s stdin the pipe carrying gho.sh's own source, not the
terminal — `exec` inherited that fd, so `gho launch`'s passphrase prompt hit
ENOTTY on every real one-liner run; it never actually worked interactively.
Fixed by redirecting stdin from `/dev/tty` immediately before the `exec`
handoff. Verified against the real repo in a pty (`curl | sh` end to end).

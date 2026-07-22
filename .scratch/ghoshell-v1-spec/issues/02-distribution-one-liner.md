# 02 — Distribution & the one-liner bootstrap

Type: grilling
Status: resolved
Blocked by: 01

## Question

What does the one-liner actually fetch and run, and how does it survive a bare target?

Sub-questions to resolve:

- The literal shape of the one-liner (`curl … | sh`? something that works without curl?).
- Where the artifact is hosted (GitHub releases? object storage? self-hosted?).
- How the bootstrap behaves when exec'd into a running container with a minimal base —
  no assumptions about installed tooling.
- Integrity/authenticity of the fetched artifact (so the one-liner can't be MITM'd).
- How the passphrase is entered at this stage (prompt? env? piped?).

## Answer

**The paste delivers only the tool; the environment is chosen interactively after unlock.**

1. **What the one-liner carries** — nothing profile-specific, nothing secret. It fetches
   the bootstrap script, which fetches the binary and execs `launch`. Profile selection is
   *not* in the paste (user doesn't want to memorize handles), so the paste is byte-identical
   every time and safe in shell history.

2. **Trigger sequence (fixed):**
   `paste → fetch bootstrap → detect arch → fetch binary → exec → prompt passphrase →
   decrypt vault → built-in fuzzy picker over profiles → materialize chosen profile`.

3. **Stable typed URL.** What the user types is one arch-independent, version-independent
   URL — a POSIX-sh **bootstrap script** published as a release asset:
   `curl -fsSL https://github.com/USER/ghoshell/releases/latest/download/gho | sh`.
   The asset name (`gho`) is constant across OS/arch, and `/latest/download/` is GitHub's
   stable redirect to the newest release — so the URL is identical wherever it's typed.
   Arch detection (`uname`) lives *in the fetched script*, not in the typed text — consistent
   with 01's "thin one-liner, real logic downstream". The script pulls the matching
   `gho-<os>-<arch>` binary from the same release and execs it.

4. **Profile picker is built into the Go binary** — a self-contained TUI reading `/dev/tty`.
   No `fzf`/`fzy` dependency (bare-container + ghost floor forbids assuming a finder exists;
   `ponytail:` we don't shell out to a finder we can't guarantee). **Constraint 04 inherits:
   one passphrase unlocks a vault holding many profiles.**

5. **Order = decrypt-then-pick.** The passphrase unlocks first; profile *names* never sit in
   the clear on the host. Metadata hygiene (the existence of "work"/"personal") over saved
   keystrokes.

6. **Hosting = GitHub Releases.** Zero infra, versioned immutable assets, checksums for free.
   Bootstrap script + per-arch binaries are all release assets of the same release.

7. **Integrity = TLS-only for v1**, documented ceiling. Script and binary both fetched over
   TLS. **Residual risk consciously accepted:** a TLS-intercepting corporate proxy could swap
   the binary. Upgrade path named: pin a SHA256 of the binary *inside* the bootstrap script
   (the script is the natural home for a checksum now that it exists) or move to signing when
   that threat comes into scope.

**Known ceilings (parked, not ticketed — polish past v1's need):**
- **Version pinning** — `/latest/` always serves newest; no way to fetch an old build from the
  typed URL. Fine for a personal tool.
- **Typed-URL ergonomics** — long GitHub URL accepted; a Pages redirector or free subdomain
  bolts on later without changing the fetch path.
- **Integrity hardening** — checksum-in-script / signing, per #7.

All three bolt on without changing the fetch path. Typed URL leaks the GitHub handle —
non-secret by design (the passphrase is the lock).

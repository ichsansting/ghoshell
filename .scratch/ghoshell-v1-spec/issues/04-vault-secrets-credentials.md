# 04 — Vault: secrets, credentials & auth state

Type: grilling
Status: resolved
Blocked by: 01

## Question

How does ghoshell carry and unlock secrets + live auth state so you're "still logged in
anywhere" — and wipe them cleanly on exit (ghost principle)?

Sub-questions to resolve:

- Encryption scheme & the passphrase-unlock model (what KDF, what cipher, symmetric?).
- What's in the vault: SSH keys, `gh`/`aws`/`claude` tokens, env secrets, configs.
- "Still logged in" — restoring token/credential files so tools are pre-authenticated.
- **Trust-boundary / wipe stance (required, not optional):** long-lived creds get
  materialized into possibly-shared/ephemeral container filesystems. Decide: tmpfs /
  memory-only where possible, `ghoshell exit` wipes materialized secrets, decrypt-at-
  launch so nothing sits unlocked on disk. This facet must be resolved here.

## Answer

**One passphrase unlocks a single sealed blob into the ephemeral HOME; the vault is a
credential-agnostic, shell-agnostic file-placer; wipe is structural on tmpfs, best-effort
on the mac fallback.**

1. **Crypto primitives.** KDF = **Argon2id** (`golang.org/x/crypto/argon2`, memory-hard);
   cipher = **XChaCha20-Poly1305** (`golang.org/x/crypto/chacha20poly1305`, AEAD, 24-byte
   random nonce so no nonce-management footgun and no reliance on AES hardware). Both in
   the one `x/crypto` dep 01 already blessed as "stdlib crypto." Rejected the stricter
   stdlib-only AES-256-GCM + scrypt path: AES-GCM's 12-byte nonce is a sharper footgun and
   PBKDF2/scrypt is weaker than Argon2id against modern cracking. AEAD means a wrong
   passphrase or tampering **fails loud** — never silent garbage. Symmetric only: no key
   server, no asymmetric keys (nothing to phone home — honors the sealed-container spirit).

2. **Vault structure = single sealed blob.** Layout: **plaintext header** (Argon2id
   params + salt + nonce) ‖ **XChaCha20-Poly1305 ciphertext** of a **`tar`**
   (`archive/tar`, stdlib) preserving destination paths + POSIX perms. Header is plaintext
   so *any* binary version can unlock *any* vault. One decrypt → one archive in RAM →
   extract into ephemeral HOME. Rejected per-entry / encrypted-FS: the vault is always
   opened whole by one passphrase for one session and 05 moves it as one unit, so selective
   decryption buys nothing but nonce/tag bookkeeping. `pack` re-seals the whole (tiny) blob
   on any change — it's the edit surface anyway.

3. **Contents = credential-agnostic, path-keyed files.** Vault entries are **opaque files
   keyed by destination path under HOME**, with a **`secret: true`** flag →
   extracted mode **0600** + named in the wipe contract (facet 5). ghoshell has **no
   tool-aware types** — it doesn't know what `gh`/`aws`/`ssh` are; it places bytes at
   paths. SSH keys, `~/.aws/credentials`, `gh` hosts, `.netrc`, token JSON — all just
   files. Adding a new tool's creds needs **zero ghoshell changes**. `ponytail:` ghoshell
   is *not a credential manager*, it's an encrypted file-placer — the same YAGNI stance 03
   took on tools.

4. **"Still logged in" = the file-placer, no special mechanism.** File-authenticating
   tools get their creds placed at the right path (facet 3). Env-authenticating tools
   (`ANTHROPIC_API_KEY`, `GITHUB_TOKEN`, …) are handled **because the shell is fish**:
   fish auto-sources `~/.config/fish/conf.d/*.fish` at startup, so an env var *is* set by a
   file — a `secret: true` snippet at `$HOME/.config/fish/conf.d/NN-secrets.fish` (0600)
   containing `set -gx KEY value`. `-x` exports to child processes = still logged in. This
   **collapses the env-var special case entirely into facet 3** — no rc-sourcing, no
   process injection. ghoshell stays **shell-agnostic** (never learns what fish is). Boundary
   (not a v1 blocker): a profile spawning a non-fish shell must author its env snippet in
   that shell's syntax — the profile's concern, not ghoshell's.

5. **Wipe contract (the required facet) — three parts:**
   - **Normal + signal exit: `rm -rf` the session dir**, unconditionally, via a
     `defer`/signal-trap in `launch` firing on clean exit *and* SIGINT/SIGTERM/SIGHUP. On
     Linux/tmpfs (`/dev/shm`, `/run`) this is the whole contract — plaintext never touched a
     physical device.
   - **mac fallback only** (01: no tmpfs, `$TMPDIR`/`/tmp` is real SSD): **best-effort
     scrub-before-unlink of `secret:true` files.** `ponytail:` overwrite-in-place is *not* a
     guaranteed physical erase on SSD (wear-leveling, APFS copy-on-write) — it shrinks the
     window, doesn't defeat forensic recovery. Documented ceiling, not a false guarantee.
   - **Crash / `kill -9` / power-loss: rely on structural teardown.** A hard-killed process
     can't run its trap. Linux/tmpfs → still safe (RAM dies with container/reboot). mac →
     documented residual risk, mitigated by random-named dir + vault-at-rest always
     encrypted (only the *materialized* session is exposed, until reboot/manual clean).

   Honest framing: **ghost is structural-and-complete on Linux/tmpfs (the primary target —
   you ghost *into* containers) and best-effort on the mac-launch fallback.** That asymmetry
   is inherent to mac lacking tmpfs, not a v1 design gap. Rejected guaranteed secure-erase on
   mac (multi-pass `srm`, crypto-erase APIs): meaningless on SSD / per-volume-not-per-file —
   documenting the ceiling beats faking the guarantee.

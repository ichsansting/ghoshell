# 05 — Bundle storage & sync

Type: grilling
Status: resolved
Blocked by: 04

## Question

Where does the locked bundle (payload manifest + encrypted vault) live, and how does it
move machine-to-machine so the one-liner can reach it from anywhere?

Sub-questions to resolve:

- Storage backend — git repo? object storage (S3/R2)? self-hosted? Trade-offs vs. the
  one-liner reachability from a bare container.
- Is the encrypted vault stored alongside the payload manifest, or separately?
- How updates propagate — push a new locked bundle, pull latest on next launch.
- Versioning / rollback of the bundle.
- Depends on 04 because the vault's on-disk format determines what actually gets stored.

## Answer

**One repo holds everything; the launcher binary ships as a Release asset; ghoshell no
longer stores tools at all — mise resolves them on-demand at launch.**

Note: the ticket's "manifest + vault as two things" framing is stale — 04 put the manifest
*inside* the sealed blob. There is **one mutable artifact to sync: the vault blob.**

1. **Public + encrypted-at-rest.** The vault blob sits at a plain HTTPS URL, no access
   control. Confidentiality rests entirely on the passphrase — exactly what 04's crypto
   (Argon2id + XChaCha20-Poly1305, fails-loud) was built for. Credentialless GET keeps the
   bootstrap secret-free (honors 02) and reachable from a bare container with the
   curl-or-wget floor. **Documented ceiling:** a public blob is offline-crackable —
   passphrase strength is the *entire* defense, which is why 04 chose memory-hard Argon2id.
   An unguessable/random-named URL ("public but unlisted") is optional obscurity, not
   security — not core.

2. **Backend = GitHub, one repo for everything.** The user's own repo holds: their source
   (including ghoshell config), the **built launcher binary as a Release asset**
   (arch-specific, 3 targets from 01), and the **vault blob**. No object storage, no
   self-hosting, no second "upstream" repo — the earlier two-repo split was wrong. One
   origin on the launch path. Rejected: S3/R2 (new account + creds for no gain on a tiny
   blob); git-clone-to-fetch (bare container may lack `git`; raw HTTPS GET is simpler).

3. **Reachability — location rides in the pasted URL.** The paste is a raw URL into the
   user's repo (`curl http://<repo-url>/gho.sh | sh`). The universal POSIX-sh bootstrap
   `uname`-detects arch and downloads the matching launcher Release asset from that same
   repo. The repo coordinates are *in the URL*, so the paste carries **no profile name and
   no secret** (02 honored — a repo URL is neither). No `-s <owner>` arg, no interactive
   prompt: the URL says *where*, the passphrase says *who*, the picker says *which profile*.

4. **Tools = mise, not ghoshell (reverses 03).** ghoshell **stops hosting tool binaries
   entirely.** The launcher fetches the `mise` binary from mise's GitHub, then resolves
   every tool through mise (`mise x python@latest -- python …`, `github:rg@latest`, etc. —
   likely wrapped as shell functions in the profile's fish config). Consequences:
   - The **vault holds only manifest + configs + secrets** — no content-addressed tool
     assets to store, hash, or sync. Big simplification for *this* ticket.
   - **mise is now the package manager** — consciously reversing 03's `ponytail:` "ghoshell
     is not a package manager / supply your own static builds." See map ripple on 03.
   - **Portability ceiling:** 03 guaranteed static tools that run on any musl/busybox base.
     mise's own binary is static, but tools it fetches are **not guaranteed static** — a
     mise-installed python/node is typically glibc-dynamic and won't run on a pure
     musl/busybox host. Accepted because the real target is exec'ing into glibc dev
     containers; musl/busybox tool portability becomes a **documented ceiling**, not a
     v1 blocker. `github:`-style static single-file tools (rg, fd, etc.) still work anywhere.

5. **Sync, updates, versioning/rollback — GitHub-native, nearly free.**
   - **Update propagation:** `pack` re-seals the whole vault (04) and pushes it to the repo;
     next launch fetches latest. Pull-latest-on-launch, no daemon.
   - **Versioning/rollback:** GitHub gives it for free — the launcher via Release tags, the
     vault via its committed history (fetch a prior blob by SHA/tag). Re-lock hygiene from
     04 (rotate salt/nonce on every re-seal) means each stored blob is a fully-independent
     ciphertext, so rollback is just "fetch an older blob and unlock it."

<!-- wayfinder:map -->
# ghoshell v1 spec

## Destination

A locked **v1 design spec** (`spec.md`) for ghoshell — every load-bearing decision
resolved with rationale, ready to hand to an implementer who can start building
without re-deciding anything.

**What ghoshell is:** launch your full working environment — configs, packages,
credentials, live auth state — anywhere (mac / linux / exec'd-into container) via a
passphrase-locked one-liner, that feels like home and leaves no trace when done.

## Notes

Two defining principles cut across every ticket (they are constraints, not tickets):

1. **Ghost** — ephemeral, wipe-on-exit, no trace left on host or container.
2. **One-liner, minimal assumptions** — the bootstrap must survive being dropped into
   an arbitrary running container with a bare base (maybe no `curl`, musl/busybox).

Skills every session should consult: `/grilling` + `/domain-modeling` (the tickets are
grilling type by default). Resolve one ticket per session.

## Decisions so far

<!-- one line per closed ticket: gist + link -->

- [01 — Launcher language & runtime](issues/01-launcher-language-runtime.md) — ghoshell is **one static Go binary** (`CGO_ENABLED=0`, no libc dep, cross-compiles to arm64+x86_64 × mac+linux), bootstrapped by a thin POSIX-sh one-liner; **curl-or-wget** fetch floor (base64-embed only a documented escape hatch); binary lands RAM-backed (tmpfs) with temp-dir fallback; single binary with `pack`/`launch` subcommands.
- [02 — Distribution & the one-liner bootstrap](issues/02-distribution-one-liner.md) — **stable typed URL** (`.../releases/latest/download/gho | sh`, byte-identical across OS/arch/version) fetches a POSIX-sh **bootstrap script** that `uname`-detects and pulls the matching binary from **GitHub Releases**; the paste carries **nothing profile-specific or secret**; sequence is **decrypt-then-pick** — passphrase unlocks the vault, then a **binary-built-in fuzzy picker** (no `fzf` dep) selects the profile; **TLS-only** integrity for v1 (documented ceiling: checksum-in-script → signing; version pinning; URL ergonomics all bolt on later).
- [03 — Payload definition](issues/03-payload-definition.md) — payload is a **manifest, TUI-managed** (source of truth, edited only via a binary TUI, never hand-written; `pack` opens it). Tools = **bundled prebuilt static binaries**, published as **public content-addressed assets** (`gho-tool-<sha>-<os>-<arch>`) on Releases, fetched **by arch and by hash** after decrypt-then-pick — so public names leak no profile, shared tools dedupe, and fetch gets integrity for free. Vault holds only **manifest + configs + secrets** (tools are external). Configs materialize into an **ephemeral HOME** (RAM dir from 01) — real home never touched, wipe is structural. **No tiers** (lean = a leaner profile; 02's picker covers it). `pack` bundles+publishes the tool assets. `ponytail:` ghoshell is not a package manager — you supply each tool's static build.
- [04 — Vault: secrets, credentials & auth state](issues/04-vault-secrets-credentials.md) — crypto = **Argon2id + XChaCha20-Poly1305** (AEAD, fails loud on wrong passphrase; the one `x/crypto` dep). Vault = **single sealed blob**: plaintext header (Argon2id params + salt + nonce) ‖ ciphertext of a `tar` (paths + POSIX perms); `pack` re-seals whole, `launch` = one decrypt → extract into ephemeral HOME. Contents = **credential-agnostic, path-keyed files** with a `secret:true` flag (0600) — ghoshell is *not a credential manager*, it places bytes at paths; a new tool's creds need zero ghoshell changes. **"Still logged in" = the file-placer, no special mechanism**: file-auth tools get creds at their path; env-auth tools ride a `secret:true` fish `conf.d` snippet (`set -gx`) — the env-var case collapses into files because the shell is fish. **Wipe contract:** structural `rm -rf` on exit/signal (complete on Linux/tmpfs — the primary target); best-effort scrub-before-unlink for `secret` files on the mac fallback (no tmpfs → documented SSD ceiling, not a false guarantee); hard-kill relies on teardown + always-encrypted vault-at-rest.
- [05 — Bundle storage & sync](issues/05-bundle-storage-sync.md) — **one repo holds everything** (source incl. ghoshell, the launcher binary as a **Release asset**, and the **vault blob**); no upstream/second repo. Vault is **public + encrypted-at-rest** — credentialless HTTPS GET, passphrase is the whole defense (documented ceiling: offline-crackable → 04's memory-hard Argon2id). Location **rides in the pasted URL** (`curl http://<repo>/gho.sh | sh`) — a repo URL is neither a profile name nor a secret, so 02's secret-free paste holds. Sync = `pack` pushes re-sealed blob, launch pulls latest; **versioning/rollback free from GitHub** (Release tags for launcher, commit history + 04's per-lock nonce rotation for the vault). **Tools = mise, not ghoshell (reverses 03):** ghoshell stops hosting tool binaries; launcher fetches `mise`, resolves every tool on-demand (`mise x`, `github:rg@latest`). Vault now holds **only manifest + configs + secrets**. Ceiling: mise-fetched tools aren't guaranteed static — glibc-dynamic tools won't run on musl/busybox; accepted because target is glibc dev containers.

## Not yet specified

- **Profile inheritance / shared base** (surfaced by [03]) — a common dotfile/tool base shared across profiles to avoid duplication. Post-v1; only matters once multiple profiles exist and drift. [06] decides v1's duplicate-vs-share stance without building inheritance.
- **mise reproducibility / version pinning** (surfaced by [05]) — a profile lists mise tool specs; does v1 pin exact versions (a mise lockfile in the vault) or float `@latest`? Likely folds into [06]'s schema decision, not its own ticket.

## Out of scope

- **Arch matrix** — fixed constraint, not a decision: **3 targets** — `darwin/arm64`, `linux/arm64`, `linux/amd64` (no mac-x86; Apple is arm-only). Corrected during [03].
- **Container image building** — only exec-into a running container is supported.
- **Process / shell-history session restore** — "sessions" means auth state only
  (still logged in), not resuming running processes.

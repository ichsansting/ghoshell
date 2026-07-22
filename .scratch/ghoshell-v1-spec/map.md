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

## Not yet specified

- Exact wipe **contract** (scrub-on-exit vs. rely on tmpfs teardown vs. both) — [01] fixed *where* the binary lands (RAM-preferred) and [03] put the whole session (binary + tools + configs) under one **ephemeral HOME** in that RAM dir, so the contract is now "tear down one dir" — sharpens into a ticket after [04] pins the vault/secret shape.
- Update / re-lock flow — how a changed environment gets re-bundled. **Now touches [03]:** re-pack must re-hash changed tools and re-publish content-addressed assets; the TUI is the edit surface.
- **Profile inheritance / shared base** (surfaced by [03]) — a common dotfile/tool base shared across profiles to avoid duplication. Post-v1; only matters once multiple profiles exist and drift.
- Multi-profile support (more than one environment in one vault). **Now constrained by [02]:**
  one passphrase unlocks a vault holding *many* profiles (work/personal, and auth variants like
  Claude API vs subscription); profiles are chosen by a binary-built-in fuzzy picker *after*
  decrypt, so profile names live only inside the encrypted vault. Sharpens into tickets once
  [04] defines the vault's shape.

## Out of scope

- **Arch matrix** — fixed constraint, not a decision: **3 targets** — `darwin/arm64`, `linux/arm64`, `linux/amd64` (no mac-x86; Apple is arm-only). Corrected during [03].
- **Container image building** — only exec-into a running container is supported.
- **Process / shell-history session restore** — "sessions" means auth state only
  (still logged in), not resuming running processes.

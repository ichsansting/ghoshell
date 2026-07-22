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

## Not yet specified

- Exact wipe **contract** (scrub-on-exit vs. rely on tmpfs teardown vs. both) — [01] fixed *where* the binary lands (RAM-preferred); the scrub contract sharpens after [04].
- Update / re-lock flow — how a changed environment gets re-bundled.
- Multi-profile support (more than one environment in one vault).

## Out of scope

- **Arch matrix** — fixed constraint, not a decision: target arm64 + x86_64, mac + linux.
- **Container image building** — only exec-into a running container is supported.
- **Process / shell-history session restore** — "sessions" means auth state only
  (still logged in), not resuming running processes.

# 01 — Binary skeleton + pack/launch subcommands

**What to build:** A single static Go binary that builds clean and dispatches two
subcommands. Running `gho launch` or `gho pack` executes the right (stubbed) code path and
exits; any other invocation prints usage. This is the prefactor every other ticket sits on
— nothing here is user-facing beyond "the binary exists and routes."

**Blocked by:** None — can start immediately.

**Status:** done

- [x] Go module initialized; binary builds with `CGO_ENABLED=0` (no libc dependency).
- [x] `gho launch` and `gho pack` each dispatch to a distinct entry point (stub bodies OK).
- [x] Unknown/absent subcommand prints usage and exits non-zero.
- [x] Cross-compiles for `linux/arm64` locally as a smoke check (full 3-arch matrix is 08).

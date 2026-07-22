# 01 — Launcher language & runtime

Type: grilling
Status: resolved
Blocked by: —

## Question

What is ghoshell itself written in / delivered as? This is the root decision — it
disciplines distribution (02), payload materialization (03), and the vault (04).

Sub-questions to resolve:

- Single static binary (Rust / Go / Zig) vs. a POSIX-sh bootstrap that fetches a binary
  vs. pure shell?
- Given the bare-container constraint (maybe no `curl`, musl/busybox base), what can the
  bootstrap assume exists on the target?
- Given the ghost principle (wipe-on-exit, minimal footprint), how does the runtime
  choice affect what gets left behind?
- Static linking / musl for portability across glibc/musl hosts?

## Answer

**ghoshell is one static Go binary, bootstrapped by a thin POSIX-sh one-liner.**

1. **Shape** — thin POSIX-sh bootstrap (the pasted one-liner) + a single static binary
   holding all real logic (vault decrypt, payload materialize, wipe). The one-liner does
   only detect-fetch-exec.
2. **Fetch floor** — declared prerequisite is **curl-or-wget**. The one-liner is a clean
   fetch-and-exec; no `/dev/tcp` or busybox gymnastics. A base64-embed of the binary is
   *documented* as a manual escape hatch for the genuinely sealed (no-network) container,
   but is not the default path. Chasing the truly-bare box was ruled not worth the
   megabyte-paste complexity.
3. **Language** — **Go**, built `CGO_ENABLED=0` → fully static, no libc dependency. This
   dissolves the musl-vs-glibc sub-question (same binary runs on both) and cross-compiles
   to arm64 + x86_64 × mac + linux from one machine. Go's stdlib crypto keeps the vault
   work dependency-free. Rust was considered for tighter binaries + secret-zeroing rigor
   but the cross-compile tax and GC-non-issue made it over-engineering for v1.
4. **Where the binary lands** — prefer a RAM-backed tmpfs path (`/dev/shm`, `/run`) so it
   never touches disk and self-cleans on container teardown; fall back to a random-named
   temp dir (`$TMPDIR`/`/tmp`, the path mac takes). `memfd`-exec rejected: Linux-only and
   fights the shell bootstrap. This fixes only *where* the binary materializes; the
   scrub-vs-tmpfs **wipe contract** stays deferred (graduates after 04).
5. **Artifact count** — **one binary, subcommands** (`ghoshell pack` / `ghoshell launch`),
   not a two-artifact author/launch split. Splitting deferred until launch-binary size is
   a real problem, which it is not at v1.

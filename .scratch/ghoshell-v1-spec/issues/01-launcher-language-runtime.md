# 01 — Launcher language & runtime

Type: grilling
Status: open
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

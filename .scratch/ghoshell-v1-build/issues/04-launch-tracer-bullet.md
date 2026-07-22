# 04 — Launch tracer bullet: unlock → materialize → mise x -- fish → wipe

**What to build:** The thinnest complete launch, end to end and local (Seam 3). Point
`gho launch` at a fixture vault on disk: it prompts for the passphrase, decrypts, composes
the profile, materializes the files into an ephemeral RAM-backed HOME, writes the composed
`mise.toml`, and drops the user into their shell as `mise x -- fish`. On exit or signal it
wipes the session. No network, no picker yet (use the sole/first profile) — this proves the
whole vertical path holds together.

**Blocked by:** 02, 03.

**Status:** done

- [x] `gho launch <fixture-vault>` prompts passphrase → decrypts via 02 → composes via 03.
- [x] Sets `HOME` to a RAM-backed (tmpfs) session dir with a temp-dir fallback; materializes
      the composed files there; secret files written `0600`.
- [x] Writes the composed `mise.toml` into the ephemeral HOME.
- [x] Spawns the shell as `mise x -- fish` so tools lazy-install on first use and land on
      `PATH` for the shell and any process it spawns.
- [x] On normal exit AND on signal, tears down the session dir — structural wipe on
      linux/tmpfs; best-effort scrub-then-unlink of secret files on the mac fallback.
- [x] Integration test drives launch against a fixture vault and asserts: ephemeral HOME
      contains exactly the composed set with correct perms, `mise.toml` matches, shell
      spawned as `mise x -- fish`, and session dir gone after teardown.

## Comments

Signal teardown wipes on SIGTERM/SIGHUP, not SIGINT. gho and the shell share
the foreground process group, so an interactive Ctrl-C belongs to fish (it
cancels the command line and keeps running) — wiping on it would delete HOME
under a live shell. `kill`/terminal-close still trigger the wipe safety net.

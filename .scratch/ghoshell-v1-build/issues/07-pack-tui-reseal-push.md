# 07 — pack: TUI manifest editor + re-seal + push

**What to build:** The authoring round trip. `gho pack` opens an in-binary TUI to edit the
manifest (profiles + components; never hand-edited, no add/rm subcommands), then re-seals
the whole vault via 02 and pushes it to the repo. Author on one machine, launch on another.

**Blocked by:** 02, 03.

**Status:** done

- [x] `gho pack` opens a TUI that edits the manifest — the source of truth — including
      profiles and their component lists.
- [x] The TUI enforces 03's disjointness rule at author time (can't save a profile with
      same-path files or conflicting tool versions).
- [x] On save, re-seals the entire vault via 02 with rotated salt + nonce.
- [x] Pushes the re-sealed blob to the repo (commit → push), so launch elsewhere pulls it.
- [x] Round-trip check: pack a change, then unseal the pushed blob and confirm the edit is
      present.

## Comments

The "TUI" is a line-oriented REPL over stdin/stdout (`profile`/`component` commands,
`save`/`quit`), not a curses screen — no new dependency, and it's fully scriptable for tests.
"No add/rm subcommands" is about the outer CLI surface: `gho pack <vault-path>` takes no
flags: all editing happens inside the one interactive session, never as separate top-level
subcommands.

Disjointness is enforced twice: every mutation (`profile ... add/rm`, `component ... tool/file
...`) is applied to a scratch copy, validated via 03's `Compose`, and rolled back on failure —
so a single bad edit never sticks. `save` re-validates every profile as a final gate before
persisting, in case something upstream is ever wrong.

The vault currently holds one entry (`manifest.json`) since config/secret file content lives
inline in the manifest, but re-sealing splices the new manifest into whatever `Unseal` returned
rather than rebuilding the vault from scratch — future non-manifest vault entries won't be
silently dropped.

`TestPackRoundTrip` spins up a real local bare git remote + two clones (no mocking): packs a
change against one clone, then fetches a fresh clone and unseals the pushed blob to confirm the
edit landed.

# 07 — pack: TUI manifest editor + re-seal + push

**What to build:** The authoring round trip. `gho pack` opens an in-binary TUI to edit the
manifest (profiles + components; never hand-edited, no add/rm subcommands), then re-seals
the whole vault via 02 and pushes it to the repo. Author on one machine, launch on another.

**Blocked by:** 02, 03.

**Status:** ready-for-agent

- [ ] `gho pack` opens a TUI that edits the manifest — the source of truth — including
      profiles and their component lists.
- [ ] The TUI enforces 03's disjointness rule at author time (can't save a profile with
      same-path files or conflicting tool versions).
- [ ] On save, re-seals the entire vault via 02 with rotated salt + nonce.
- [ ] Pushes the re-sealed blob to the repo (commit → push), so launch elsewhere pulls it.
- [ ] Round-trip check: pack a change, then unseal the pushed blob and confirm the edit is
      present.

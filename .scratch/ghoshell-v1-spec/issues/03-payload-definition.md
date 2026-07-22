# 03 — Payload definition

Type: grilling
Status: resolved
Blocked by: 01

## Question

How is the "what to install / materialize" set declared, and how does it land on a target?

Sub-questions to resolve:

- Declarative manifest format for the payload (packages, configs, dotfiles).
- Tiers — minimal (fast, for a quick exec-in) vs. full (complete home environment)?
- How packages are installed on a target with no assumed package manager — bundle
  prebuilt binaries, or drive the host's manager, or both?
- How config files (shell, editor, prompt) are placed, and how they're wiped on exit
  per the ghost principle.
- Relationship between payload materialization and the bootstrap (02).

## Answer

> **SUPERSEDED IN PART by [05](05-bundle-storage-sync.md):** tool provisioning changed
> from ghoshell-bundled content-addressed static binaries to **mise** resolving tools
> on-demand. Points **2, 4, 5, 8 below no longer hold** — ghoshell hosts no tool assets;
> there are no `gho-tool-<sha>` bundles; the launcher fetches `mise` and runs tools via
> `mise x` / `github:…`. Still standing: **1** (manifest, TUI-managed), **3** (3-arch
> matrix), **6** (ephemeral HOME), **7** (no tiers → profiles). The `ponytail:` "not a
> package manager" stance is explicitly reversed — mise *is* the package manager now.

**A curated manifest declares the payload; tools are public content-addressed assets fetched by arch; everything materializes into an ephemeral HOME.**

1. **Authoring = manifest, TUI-managed.** A declarative manifest is the source of
   truth, but edited only through a TUI inside the binary (reusing 02's `/dev/tty`
   TUI muscle) — never hand-edited, never `add`/`rm` subcommands. `pack` opens the
   editor TUI; the manifest lives inside the vault. Snapshotting `~` was rejected:
   drags in secrets, machine-specific paths, and junk — the opposite of a clean
   reproducible ghost payload.

2. **Tool provisioning = bundled prebuilt static binaries.** The only option
   satisfying all three locked constraints at once — works on the bare no-manager
   box, needs no network for *tooling* at launch, leaves zero trace. Driving the
   host package manager (apt/apk/brew) was rejected: violates ghost, needs root +
   network, breaks on the bare box. `ponytail:` ghoshell is not a package manager —
   "add a tool" means *supplying* its static build; ghoshell bundles what you give it.

3. **Arch matrix corrected to 3:** `darwin/arm64`, `linux/arm64`, `linux/amd64`
   (no mac-x86 — Apple is arm-only now). Built for all 3; arch selected at fetch
   time by the bootstrap's existing `uname` detection (02).

4. **Tools live as public per-arch assets on GitHub Releases, fetched by matching
   arch** — *not* in the vault. Tool binaries (ripgrep, etc.) aren't secret, so
   encrypting them buys nothing but bulk. The vault carries only manifest + configs
   + secrets. Fetching the matching tool set adds no *new* network dependency (02
   already fetches the binary over the network at launch). Consciously accepted: a
   no-network launch can't pull tools even with a cached vault — fine, since
   ghost-into-a-live-container always has network.

5. **`pack` builds and publishes the tool assets.** Toggle a tool in the TUI →
   `pack` sources (from the static build you supplied), bundles per-arch, and
   uploads to the release. Single interface; no hand-managed bundles (you said you
   won't hand-manage files). Sourcing the static build is still yours; bundling +
   publishing is automated.

6. **Config placement = ephemeral HOME.** Launch sets `HOME` to the RAM-backed
   session dir (from 01) and materializes all configs *there*; the shell spawns
   with that HOME. Binary + tools + configs are one throwaway directory; the host's
   real home is never touched, even on crash. This answers "how are configs wiped on
   exit" **structurally** — there's nothing in the real home to scrub. Symlink-into-
   real-home and copy-into-real-home both rejected: they mutate the real home, which
   is the exact trace ghost forbids. Documented edge (not a v1 blocker): tools that
   ignore `$HOME` and read `/etc` or hardcoded paths; `HOME`+`PATH` cover ~95% of the
   dotfile case. The exact teardown mechanics (scrub vs rely-on-tmpfs) stay in the
   **wipe-contract** fog (post-04) — ephemeral HOME makes that contract simpler
   whatever it lands on.

7. **No tiers.** "Minimal vs full" collapses into **profiles** — 02's post-decrypt
   picker already selects them, so a lean quick-exec setup is just a leaner profile.
   A second tier axis would duplicate the picker (over-engineering). Profile
   *inheritance* (a shared dotfile base to avoid duplication across profiles) is real
   but post-v1 fog.

8. **Content-addressed tool bundles** — `gho-tool-<sha256>-<os>-<arch>`. The manifest
   maps `name → sha`; after decrypt-then-pick, launch fetches exactly the hashes the
   chosen profile lists, for the detected arch. This simultaneously: (a) keeps profile
   names secret — public asset names are hashes, leaking no "work"/"personal"; (b)
   fetches only what the profile needs; (c) dedupes tools shared across profiles for
   free; (d) gives fetch-integrity — the hash *is* the name, so launch verifies what
   it fetched, strictly better than 02's TLS-only floor on the tool path. `pack`
   hashes each tool and publishes under its hash; the TUI never shows a hash.

**Materialization ↔ bootstrap (02) relationship, concretely:**
`paste → fetch bootstrap → uname → fetch gho binary (arch) → exec → prompt passphrase
→ decrypt vault → picker over profiles → read chosen profile's manifest → fetch its
tool hashes (arch) into the RAM dir → materialize configs under ephemeral HOME → spawn
shell`.

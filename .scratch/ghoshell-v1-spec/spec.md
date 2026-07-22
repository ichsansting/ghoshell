# ghoshell v1 — Design Spec

Status: ready-for-agent

> Synthesized from the resolved wayfinder map `ghoshell-v1-spec` (tickets 01–06). Each
> Implementation Decision links the ticket that resolved it; the ticket bodies hold the
> rejected-alternatives detail this spec only gists.

## Problem Statement

I work across many machines — my mac, remote linux boxes, and containers I `exec` into.
Every time I land somewhere new, I don't feel at home: my shell, prompt, editor config,
CLI tools, and — worst of all — my **auth state** (still-logged-in credentials for the
services I use) aren't there. Setting them up by hand each time is slow, and whatever I
install leaves a mess behind on a machine that isn't mine. I want my full working
environment to appear anywhere on demand, and to vanish without a trace when I leave.

## Solution

**ghoshell** launches my complete working environment anywhere via a single
passphrase-locked one-liner I can paste into any shell. I pick a **profile** (work,
personal, an auth variant), and it materializes my tools, configs, and live auth state
into a throwaway home directory, drops me into my shell feeling exactly like home — then
wipes everything on exit, leaving no trace on the host.

Two principles are absolute (they constrain every decision, they are not themselves
features):

1. **Ghost** — ephemeral; wipe-on-exit; no trace left on host or container.
2. **One-liner, minimal assumptions** — the bootstrap must survive being dropped into an
   arbitrary running container with a bare base (maybe no `curl`, musl/busybox).

## User Stories

1. As a developer landing in a fresh container, I want to paste one line and get my full
   environment, so that I don't hand-configure every new machine.
2. As a developer on a bare base image, I want the bootstrap to work with only
   `curl`-or-`wget` present, so that a minimal container isn't a blocker.
3. As a developer on an unknown host, I want the bootstrap to auto-detect OS and arch, so
   that I paste the *same* line everywhere regardless of platform.
4. As a mac (arm) and linux (arm/amd64) user, I want a binary built for my platform, so
   that the launcher runs natively without emulation.
5. As a security-conscious user, I want the one-liner I paste to carry no secret and no
   profile name, so that pasting it in a shared terminal or logging it leaks nothing.
6. As a user with multiple contexts, I want to choose a profile at launch, so that I can
   land as "work" or "personal" from the same command.
7. As a user with auth variants, I want profiles like "Claude API" vs "Claude
   subscription", so that I can switch which credential set is active without separate
   setups.
8. As a user, I want to unlock everything with a single passphrase, so that I memorize one
   secret, not many.
9. As a user, I want a wrong passphrase to fail loudly and immediately, so that I'm never
   silently dropped into a broken or empty environment.
10. As a user, I want my profile/component names to live only inside the encrypted vault,
    so that nothing public ever reveals that "work" or "personal" exists.
11. As a user, I want to select a profile with a built-in fuzzy picker, so that I don't
    depend on `fzf` being installed on the host.
12. As a user, I want the passphrase prompt *before* the picker (decrypt-then-pick), so
    that profile names are never shown until the vault is open.
13. As a user, I want my CLI tools (ripgrep, python, node, …) available in the launched
    shell, so that my environment is actually usable, not just my dotfiles.
14. As a user, I want tools resolved by mise on demand, so that I don't hand-manage or
    pre-bundle static binaries.
15. As a user, I want a tool to install on first use, so that I don't pay for tools a
    session never touches.
16. As a user, I want tools available on `PATH` (not just as interactive aliases), so that
    an editor or script that spawns `rg`/`python` finds them too.
17. As a user, I want my config files (shell, prompt, editor) placed in the launched
    environment, so that it looks and behaves like home.
18. As a user, I want my configs materialized into a throwaway HOME, so that the host's
    real home is never touched even if the session crashes.
19. As a user, I want my still-logged-in auth state restored, so that I don't re-login to
    every service on every machine.
20. As a user, I want file-based credentials placed at their expected paths, so that tools
    that read a credential file just work.
21. As a user, I want env-var-based credentials restored too, so that tools that read an
    env var are also authenticated.
22. As a user, I want adding a new tool's credential to require no ghoshell code change, so
    that ghoshell stays a byte-placer, not a per-service credential manager.
23. As a user, I want secret files stored with `0600` perms, so that credentials aren't
    world-readable in the session.
24. As a user, I want everything wiped on exit and on signal (Ctrl-C, kill), so that
    leaving is clean by default.
25. As a user on linux/tmpfs, I want the wipe to be structurally complete, so that nothing
    survives in RAM after teardown.
26. As a user on mac (no tmpfs), I want a best-effort scrub of secret files before unlink,
    so that the fallback path is honest about its ceiling rather than pretending.
27. As a user, I want the vault always encrypted at rest, so that a hard-kill that skips
    teardown still doesn't leave plaintext credentials behind.
28. As a user, I want to edit my payload through a TUI, so that I never hand-write a
    manifest or manage files manually.
29. As a user, I want `pack` to open that editor, re-seal the vault, and push it, so that
    updating my environment is one command.
30. As a user, I want `launch` to always pull the latest vault, so that changes I packed
    elsewhere are already present when I land somewhere new.
31. As a user, I want everything (source, launcher binary, vault) in one repo, so that
    there's a single origin to manage and reach.
32. As a user, I want the launcher binary delivered as a versioned release asset, so that I
    get versioning and rollback for free.
33. As a user, I want the vault rolled back via commit history, so that a bad pack is
    recoverable.
34. As a user, I want the repo URL to live in the pasted one-liner, so that "where" is in
    the URL while "who" is the passphrase and "which" is the picker.
35. As a user with overlapping profiles, I want to edit a shared piece once and have every
    profile that uses it pick up the change, so that I don't duplicate-edit dotfiles.
36. As a user, I want to compose a profile from reusable components, so that "work" and
    "personal" can share a base without an inheritance system.
37. As a user, I want one shared secret (e.g. an SSH key) usable by multiple profiles
    without duplicating it, so that shared credentials are stored once.
38. As a user, I want conflicting definitions within a profile rejected when I author them,
    so that a launch is never ambiguous about which file or tool version wins.
39. As a user, I want only the chosen profile's set materialized, so that other profiles'
    files and secrets never touch the disk this session.
40. As a user, I want tools to float at `@latest` in v1, so that I get current versions
    without maintaining a lockfile yet.
41. As a maintainer, I want the vault format to be a single sealed blob, so that pack is a
    whole-file re-seal and launch is a single decrypt.
42. As a maintainer, I want the launcher to be one static binary with `pack`/`launch`
    subcommands, so that there's one artifact to build and ship per arch.

## Implementation Decisions

### Launcher runtime & shape — [01](issues/01-launcher-language-runtime.md)
- ghoshell is **one static Go binary**, `CGO_ENABLED=0` (no libc dependency, so it runs on
  glibc and musl/busybox alike), cross-compiled to the 3-target arch matrix.
- A thin **POSIX-sh one-liner** bootstraps it. Fetch floor is **curl-or-wget**; base64-embed
  is a documented escape hatch only.
- The binary lands in a **RAM-backed dir (tmpfs)** with a temp-dir fallback.
- Single binary, two subcommands: **`pack`** (author + re-seal + push) and **`launch`**
  (fetch + unlock + materialize + spawn).

### Distribution & bootstrap — [02](issues/02-distribution-one-liner.md)
- The pasted one-liner fetches a **POSIX-sh bootstrap script** which `uname`-detects OS/arch
  and pulls the matching launcher binary from **GitHub Releases**.
- The paste carries **nothing profile-specific and nothing secret** — only the repo URL.
- Launch sequence is **decrypt-then-pick**: passphrase unlocks the vault first, *then* a
  **binary-built-in fuzzy picker** (no `fzf` dependency) selects the profile.
- Integrity for v1 is **TLS-only** (documented ceiling: checksum-in-script → signing →
  version pinning are later bolt-ons).

### Vault format & crypto — [04](issues/04-vault-secrets-credentials.md)
- Crypto = **Argon2id (memory-hard KDF) + XChaCha20-Poly1305 (AEAD)**. AEAD **fails loud on
  a wrong passphrase**. This is the single `golang.org/x/crypto` dependency.
- The vault is a **single sealed blob**: a plaintext header (Argon2id params + salt + nonce)
  concatenated with the ciphertext of a **`tar`** (file paths + POSIX perms). `pack`
  re-seals the whole blob; `launch` does one decrypt → extract.
- Contents are **credential-agnostic, path-keyed files** carrying a **`secret:true`** flag
  (materialized `0600`). ghoshell **places bytes at paths** — it is not a credential
  manager, so a new tool's credential needs zero ghoshell change.
- **"Still logged in" is just the file-placer**: file-auth tools get their credential file at
  its path; env-auth tools ride a `secret:true` **fish `conf.d` snippet** (`set -gx …`) — the
  env-var case collapses into files because the shell is fish.
- **Re-seal hygiene:** every `pack` rotates salt + nonce, so each stored blob is a fully
  independent ciphertext.

### Bundle storage & sync — [05](issues/05-bundle-storage-sync.md)
- **One repo holds everything**: source (including ghoshell config), the built launcher
  binary as a **Release asset** (3 arch targets), and the **vault blob**. No second/upstream
  repo.
- The vault is **public + encrypted-at-rest** — credentialless HTTPS GET, so the bootstrap
  stays secret-free and reachable from a bare container. **Documented ceiling:** a public
  blob is offline-crackable, so passphrase strength is the entire defense — which is why 04
  chose memory-hard Argon2id.
- The **repo location rides in the pasted URL** (`curl <repo-url>/gho.sh | sh`).
- **Sync:** `pack` pushes the re-sealed blob; `launch` pulls latest. Versioning/rollback is
  GitHub-native — Release tags for the launcher, commit history + per-lock nonce rotation for
  the vault.
- **Tools = mise, not ghoshell (this reverses 03):** ghoshell hosts **no** tool binaries. The
  launcher fetches the `mise` binary, then resolves every tool on demand. The vault therefore
  holds **only manifest + configs + secrets**. **Ceiling:** mise-fetched tools aren't
  guaranteed static (a glibc-dynamic python won't run on pure musl/busybox) — accepted
  because the real target is glibc dev containers; `github:`-style single-file static tools
  still run anywhere.

### Payload authoring — [03](issues/03-payload-definition.md) (tool-provisioning parts superseded by 05)
- **Authoring = a single manifest, TUI-managed.** The manifest is the source of truth, edited
  **only** through a TUI inside the binary — never hand-edited, no `add`/`rm` subcommands.
  `pack` opens the editor. Snapshotting `~` was rejected (drags in secrets and machine junk).
- **No tiers** — "minimal vs full" collapses into profiles; a lean setup is just a leaner
  profile (the picker already selects it).
- **Config placement = ephemeral HOME.** `launch` sets `HOME` to the RAM-backed session dir
  and materializes all configs there; the shell spawns with that HOME. Wipe is **structural**
  — there is nothing in the real home to scrub. (Edge, not a v1 blocker: tools that ignore
  `$HOME` and read `/etc` or hardcoded paths; `HOME`+`PATH` cover ~95% of the dotfile case.)

### Multi-profile manifest schema — [06](issues/06-multi-profile-manifest.md)
- **Schema shape:** one manifest with a top-level **`profiles:` map**. Each key is a profile
  name; its value is a **list of component names**. The picker (02) enumerates the map's keys.
- **Composition over inheritance (core decision):** a profile is a **composition of
  components**, not a private copy and not an inheritance chain.
  - A **component** bundles `{ mise tool specs + config files + secret files }` — e.g. `base`
    (fish + starship + common dotfiles), `python-dev` (`python@latest` + pyright config),
    `claude-api` (the API-key secret), `claude-sub` (subscription auth state), `personal-ssh`
    (an SSH key).
  - A **profile lists component names**; launch materializes the **flat union** of the chosen
    profile's components.
  - This gives "edit a component once → every profile that includes it updates" without an
    inheritance system.
- **Disjointness rule (the invariant that keeps composition flat), enforced by the TUI at
  author time:** within a single profile, no two components may (a) place a file at the same
  path, or (b) name the same tool with conflicting versions. Wanting "component A overrides
  component B" is the signal to build the post-v1 override system — v1 rejects it at author
  time.
- **Secrets scoping falls out of composition:** a shared secret (one SSH key for both work
  and personal) is a component (`personal-ssh`) included by multiple profiles — stored once,
  `secret:true`, materialized only when a chosen profile includes it. No separate
  secret-sharing mechanism.
- **Name secrecy:** profile *and* component names live only inside the sealed manifest; the
  picker reads them strictly post-decrypt. Scoped honestly — *profile/component* names are
  secret, but *mise tool names* are visible on the wire at launch (accepted 05 consequence;
  tool names aren't profile-identifying).
- **Tool versions float `@latest`** for v1. Documented ceiling: not byte-reproducible over
  time; pinning + a `mise.lock`-in-vault is a **post-v1 addition of a field, not a reshape**.
- **Tool mechanism — `mise x -- fish` at the session level, no shims (verified in-env):**
  launch composes the chosen profile → writes **one generated `mise.toml`** into the ephemeral
  HOME whose `[tools]` table is the **union** of the profile's components' specs → **spawns the
  shell wrapped as `mise x -- fish`**. With `auto_install=true`, a tool **lazy-installs on
  first use**, and `mise x` puts the tools' **real bin dirs on `PATH`**, which the shell *and
  every process it spawns* inherit — so apps that shell out find the tools. This was verified
  against a genuinely-uninstalled `rg`: first invocation installed it, and a child process
  resolved and ran it. Rejected: per-command `mise x` aliases (invisible to apps),
  `mise activate --shims` (eager + shim-dispatch cost, its only edge irrelevant to ghost),
  `mise activate fish` (works but resolves env at shell-init, not the pure on-trigger model).
  **Consequence accepted:** the tool env is built once at wrap time, so editing the manifest
  *inside* a live session won't expose new tools until relaunch (fine — `pack` runs from a
  normal session, not mid-ghost).

### Full launch contract (composed sequence across tickets)
```
paste one-liner → fetch bootstrap → uname detect → fetch gho binary (arch) → exec
→ prompt passphrase → decrypt vault (fail loud on wrong pass)
→ fuzzy picker over profiles: keys → user picks profile X
→ resolve X's component list → union their { files, tool specs }
→ materialize union's files into ephemeral HOME (secret files 0600)
→ write composed mise.toml ([tools] = union of specs) into ephemeral HOME
→ spawn shell as `mise x -- fish`  (auto_install lazy-installs tools on first use)
→ on exit/signal: teardown wipe (structural on linux/tmpfs; scrub-then-unlink on mac fallback)
```

## Testing Decisions

Good tests here assert **external behavior at a seam**, never internal structure. The two
load-bearing pieces are pure functions and get dedicated tests; the wiring gets one thin
end-to-end test. Three seams total (fewest that cover the risk):

- **Seam 1 — Vault seal/unseal round-trip (pure crypto core).**
  `seal(passphrase, tar) → blob` and `unseal(passphrase, blob) → tar`. Tests:
  `unseal(seal(x)) == x` preserving paths, POSIX perms, and `secret:true`/`0600`; a **wrong
  passphrase fails loud** (AEAD auth failure, never silent/partial); each re-seal rotates
  salt+nonce → **distinct ciphertext for identical plaintext**. This is the only seam where a
  bug loses data or leaks credentials, so it earns the most tests.
- **Seam 2 — Profile composition (pure, the 06 logic).**
  `compose(manifest, profileName) → { files, mise.toml }`. Tests: the result is the **union**
  of the profile's components' files and `[tools]`; the **disjointness rule rejects** same-path
  files and conflicting tool versions *at this boundary* (author-time validation); components
  **not** in the profile never appear in the output. Pure, no I/O — the highest seam for
  everything 06 decided.
- **Seam 3 — Launch integration (end-to-end, thin).**
  Drive `launch` against a **fixture vault** and assert the observable outcome: the ephemeral
  HOME contains exactly the composed file set (with correct perms), the generated `mise.toml`
  matches the composition, the shell is spawned as `mise x -- fish`, and **teardown wipes the
  session dir** on normal exit and on signal. Network/mise is mocked or exercised with a real
  single-file static tool in CI. Kept coarse and few — Seams 1 and 2 carry the correctness
  weight.

Prior art: none yet (greenfield repo). Seams 1 and 2 are standard Go table-driven `_test.go`
suites in the package under test; Seam 3 is a Go integration test that builds the binary and
drives the subcommand against a temp dir. No test framework beyond the standard library is
assumed (consistent with the single-`x/crypto`-dependency posture of 04).

## Out of Scope

- **Arch matrix as a decision** — it's a fixed constraint: 3 targets, `darwin/arm64`,
  `linux/arm64`, `linux/amd64` (no mac-x86; Apple is arm-only).
- **Container image building** — only `exec`-into a *running* container is supported.
- **Process / shell-history session restore** — "session" means **auth state only** (still
  logged in), not resuming running processes or reconstructing shell history.
- **Profile inheritance / override** — v1 is flat composition + the disjointness rule. A
  component whose file *replaces* another's at the same path, or multi-level bases, is
  deferred. It becomes worth building once "some but not all" or "same file, tweaked per
  profile" is common.
- **mise version pinning / lockfile** — v1 floats `@latest`; pinning + `mise.lock`-in-vault is
  a post-v1 field addition.
- **Integrity beyond TLS** — checksum-in-script, binary/vault signing, and version pinning are
  documented later bolt-ons, not v1.
- **musl/busybox tool portability** — the launcher and `github:` static tools run anywhere,
  but glibc-dynamic mise tools are not guaranteed to run on a pure musl/busybox host. Accepted
  ceiling, not a v1 goal.
- **Non-fish shells** — the design leans on fish specifically (`conf.d` snippets, `mise x --
  fish`); other shells are not a v1 target.

## Further Notes

- This spec is a **planning artifact**, not an implementation. It records *what* v1 is and
  *why*, decision-by-decision, so an implementer can start building without re-deciding. It
  deliberately contains no file paths or code (except the launch-contract pseudo-sequence and
  the verified `mise x` mechanism, which encode decisions more precisely than prose).
- **Cross-ticket reversal to keep in mind:** 03 originally specified ghoshell-bundled,
  content-addressed static tool binaries; **05 reversed this to mise** resolving tools on
  demand. Where 03 and 05 disagree, **05 wins** (and 06's tool mechanism builds on 05, not 03).
- The two absolute principles (ghost; one-liner/minimal-assumptions) are **constraints, not
  features** — every decision above was chosen because it honors both, and each documented
  "ceiling" marks exactly where a v1 simplification would need to grow if a principle later
  demands more.

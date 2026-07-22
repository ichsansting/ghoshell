# 06 — Multi-profile manifest schema

Type: grilling
Status: resolved
Blocked by: —

<!-- Builds on resolved 02 (post-decrypt picker), 03 (manifest, TUI-managed),
     04 (single sealed blob, path-keyed files), 05 (tools = mise, not ghoshell-hosted).
     All resolved → frontier. NOTE: 05 superseded 03's tools-by-hash — a profile now lists
     mise tool specs (e.g. python@latest, github:rg), not content-addressed hashes. -->

## Question

One passphrase unlocks one vault (04's single sealed blob) that holds **many** profiles
(02: work/personal, and auth variants like Claude API vs subscription; 02's post-decrypt
fuzzy picker selects one). How does the manifest *express* those profiles?

Sub-questions to resolve:

- **Schema shape** — one manifest with a top-level `profiles:` map, or N manifests inside
  the blob? What does the picker (02) enumerate?
- **Per-profile vs. shared** — does each profile own its full {mise tool specs (05), config
  files, secret files (04)} set, or reference a shared pool? (Full profile *inheritance* is
  explicitly post-v1 fog — this ticket only needs to decide whether v1 duplicates or shares,
  not build an inheritance system.)
- **Secrets scoping** — are `secret:true` files (04) per-profile, or can profiles share a
  secret? (e.g. one SSH key used by both work and personal.)
- **Name secrecy** — 02/03 require profile names leak nowhere public; confirm the schema
  keeps names inside the encrypted blob and the picker reads them only post-decrypt.
- **Launch selection** — after pick, launch materializes only the chosen profile's set into
  the ephemeral HOME. Confirm the schema makes "which files/tools belong to profile X"
  unambiguous.

## Answer

**One manifest, `profiles:` map. A profile is a composition of components (not inheritance);
launch materializes the flat union of the chosen profile's components, and spawns the shell
wrapped in `mise x` — no shims — so tools lazy-install on first use and are on `PATH` for apps.**

### 1. Schema shape — one manifest, `profiles:` map

A single TUI-managed manifest (03) with a top-level `profiles:` map. Each key is a profile
name; its value is a **list of component names**. The picker (02) enumerates the map's keys.
Rejected N-manifests-per-profile: re-litigates 03's single-manifest decision and buys nothing
at a handful-of-profiles scale.

### 2. Composition over inheritance — the core decision

A profile does **not** own a private copy of its files/tools, and does **not** inherit from a
base. A profile is a **composition of components**:

- A **component** is a self-contained bundle of `{ mise tool specs (05) + config files + secret
  files (04) }` — e.g. `base` (fish + starship + common dotfiles), `python-dev` (mise
  `python@latest` + pyright config), `claude-api` (the API-key secret), `claude-sub`
  (subscription auth state), `personal-ssh` (an SSH key).
- A **profile is a list of components**: `work = [base, python-dev, claude-api]`,
  `personal = [base, python-dev, claude-sub, personal-ssh]`.
- Launch materializes the **flat union** of the chosen profile's components.

This delivers the user's requirement — *edit a component once, every profile that includes it
gets the change* — with less machinery than a shared-layer, and it is **not** the inheritance
the map deferred: composition is a flat union of independent building blocks with **no parent
chain and no override semantics**. That distinction is enforced by one rule:

> **Disjointness rule (enforced by the TUI at author time):** within a single profile, no two
> components may (a) place a file at the same path, or (b) name the same tool with conflicting
> versions. The moment "component A overrides component B" is wanted, that's the signal to build
> the post-v1 inheritance/override system from the fog — v1 rejects it at author time instead.

Rejected alternatives: **per-profile duplication** (user wants edit-once-apply-everywhere);
**always-on `shared:` layer** (all-or-nothing — can't do "some but not all profiles"); **shared
pool + per-profile references** (id namespace + reference resolution = front half of inheritance).

### 3. Secrets scoping — a secret is just a component

`secret:true` files (04) are scoped by the component that carries them. A shared secret (one SSH
key used by work + personal) is a component (`personal-ssh`) included by multiple profiles —
stored **once**, `secret:true`, materialized only when a chosen profile includes it. No separate
secret-sharing mechanism; it falls out of composition.

### 4. Name secrecy — automatic

Profile **and** component names live only inside the manifest, which lives only inside the sealed
vault (04). The picker (02) reads them strictly post-decrypt. Nothing profile-shaped is ever
public. **Scoped honestly:** *profile/component* names are secret; **mise tool names are visible
on the wire** at launch (mise fetches `python`, `rg`, … from mise/GitHub) — already an accepted
05 consequence, and tool names are not profile-identifying.

### 5. Tool version pinning — float `@latest` for v1

Components store floating specs (`python@latest`, `github:rg@latest`) — the spec strings 05
already described, nothing more. **Documented ceiling:** not byte-reproducible over time (two
launches days apart may get different tool versions); mise supports pinning + a lockfile, so
"add `mise.lock` to the vault and re-pin at pack" bolts on post-v1 as a *new field, not a
reshape*. Accepted because ghost's premise is "feel at home in a live container," not
scientific build reproducibility. Resolves the map's parked *mise reproducibility* fog.

### 6. Tool mechanism — `mise x -- fish` at the session level, no shims

A component's mise specs are `[tools]` entries. Launch **composes the chosen profile → writes one
generated `mise.toml`** into the ephemeral HOME whose `[tools]` table is the **union** of the
profile's components' specs. Then launch **spawns the shell wrapped as `mise x -- fish`** (not
bare fish, not shims, not `mise activate`).

Verified in this environment (`mise 2026.6.13`, with a genuinely-uninstalled `rg`):
- **Lazy-install:** `mise x` triggers install on *first use* (`auto_install=true`) — no upfront
  `mise install`. This is the property the user specifically wanted from `mise x`.
- **Apps / PATH:** `mise x -- <cmd>` exports the tools' **real bin dirs** onto `PATH`, and a
  **child** process resolved and ran `rg`. Because ghost's entire process tree descends from the
  one wrapped fish, every app the shell spawns inherits the tools. No shims dir needed.

Rejected: **per-command `mise x` aliases** (fish functions — invisible to apps reading `PATH`,
the original failure); **`mise activate --shims`** (real files but resolves eagerly / adds shim
dispatch cost, and its only advantage — tools for processes that never inherited the shell env —
doesn't apply since ghost has none); **`mise activate fish`** (works, but resolves the env at
shell-init rather than the pure `mise x` on-trigger model the user chose).

**Consequence accepted:** `mise x` builds the tool env once at wrap time, so editing the manifest
*inside* a live ghost session won't expose new tools until relaunch — fine, since `pack` happens
from a normal session, not mid-ghost. First use of a tool pays install cost (network) at
launch-time inside the session — inherent to lazy-install, consistent with ghost (nothing
pre-baked, wiped on exit).

### Launch contract (updates 03's materialization sequence)

```
… decrypt vault → picker over profiles: keys → user picks profile X
→ resolve X's component list → union their { files, tool specs }
→ materialize the union's files into ephemeral HOME (04/03; secret files 0600)
→ write composed mise.toml ([tools] = union of specs) into ephemeral HOME
→ spawn shell as `mise x -- fish`  (auto_install lazy-installs tools on first use)
```

Components not in X stay sealed in the vault, unextracted. A component listed by no profile is
harmless dead weight (TUI can surface orphans). "Which files/tools belong to profile X" = the
union of X's components, disjoint by construction — the ticket's final sub-question, answered.

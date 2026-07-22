# 06 — Multi-profile manifest schema

Type: grilling
Status: open
Blocked by: —

<!-- Builds on resolved 02 (post-decrypt picker), 03 (manifest, tools-by-hash),
     04 (single sealed blob, path-keyed files). All three resolved → frontier. -->

## Question

One passphrase unlocks one vault (04's single sealed blob) that holds **many** profiles
(02: work/personal, and auth variants like Claude API vs subscription; 02's post-decrypt
fuzzy picker selects one). How does the manifest *express* those profiles?

Sub-questions to resolve:

- **Schema shape** — one manifest with a top-level `profiles:` map, or N manifests inside
  the blob? What does the picker (02) enumerate?
- **Per-profile vs. shared** — does each profile own its full {tools-by-hash (03), config
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

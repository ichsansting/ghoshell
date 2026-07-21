# 04 — Vault: secrets, credentials & auth state

Type: grilling
Status: open
Blocked by: 01

## Question

How does ghoshell carry and unlock secrets + live auth state so you're "still logged in
anywhere" — and wipe them cleanly on exit (ghost principle)?

Sub-questions to resolve:

- Encryption scheme & the passphrase-unlock model (what KDF, what cipher, symmetric?).
- What's in the vault: SSH keys, `gh`/`aws`/`claude` tokens, env secrets, configs.
- "Still logged in" — restoring token/credential files so tools are pre-authenticated.
- **Trust-boundary / wipe stance (required, not optional):** long-lived creds get
  materialized into possibly-shared/ephemeral container filesystems. Decide: tmpfs /
  memory-only where possible, `ghoshell exit` wipes materialized secrets, decrypt-at-
  launch so nothing sits unlocked on disk. This facet must be resolved here.

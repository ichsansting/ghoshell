# 02 — Distribution & the one-liner bootstrap

Type: grilling
Status: open
Blocked by: 01

## Question

What does the one-liner actually fetch and run, and how does it survive a bare target?

Sub-questions to resolve:

- The literal shape of the one-liner (`curl … | sh`? something that works without curl?).
- Where the artifact is hosted (GitHub releases? object storage? self-hosted?).
- How the bootstrap behaves when exec'd into a running container with a minimal base —
  no assumptions about installed tooling.
- Integrity/authenticity of the fetched artifact (so the one-liner can't be MITM'd).
- How the passphrase is entered at this stage (prompt? env? piped?).

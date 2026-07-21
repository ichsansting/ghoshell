# 03 — Payload definition

Type: grilling
Status: open
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

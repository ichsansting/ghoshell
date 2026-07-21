# 05 — Bundle storage & sync

Type: grilling
Status: open
Blocked by: 04

## Question

Where does the locked bundle (payload manifest + encrypted vault) live, and how does it
move machine-to-machine so the one-liner can reach it from anywhere?

Sub-questions to resolve:

- Storage backend — git repo? object storage (S3/R2)? self-hosted? Trade-offs vs. the
  one-liner reachability from a bare container.
- Is the encrypted vault stored alongside the payload manifest, or separately?
- How updates propagate — push a new locked bundle, pull latest on next launch.
- Versioning / rollback of the bundle.
- Depends on 04 because the vault's on-disk format determines what actually gets stored.

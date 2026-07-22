# 02 — Vault seal/unseal round-trip

**What to build:** The crypto core of the vault (Seam 1). Given a passphrase and a set of
path-keyed files, produce a single sealed blob; given the passphrase and the blob, get the
files back byte-identical. A wrong passphrase fails loudly rather than returning garbage.
This is the one place a bug loses data or leaks credentials, so it is built and tested in
isolation before anything materializes it.

**Blocked by:** 01.

**Status:** ready-for-agent

- [ ] `seal(passphrase, files) → blob` and `unseal(passphrase, blob) → files` round-trip:
      `unseal(seal(x)) == x`, preserving each file's path, POSIX perms, and `secret:true`.
- [ ] Crypto is Argon2id (memory-hard KDF) + XChaCha20-Poly1305 (AEAD), via `x/crypto` only.
- [ ] Blob layout is a plaintext header (Argon2id params + salt + nonce) followed by the
      ciphertext of a `tar` of the files.
- [ ] A wrong passphrase fails loud (AEAD auth failure) — never a silent or partial extract.
- [ ] Each `seal` of identical plaintext rotates salt + nonce → distinct ciphertext.
- [ ] Table-driven `_test.go` covering round-trip, perms/secret preservation, wrong-pass
      failure, and ciphertext-distinctness.

# 05 — Built-in fuzzy picker (decrypt-then-pick)

**What to build:** Profile selection after unlock. Once the vault is decrypted, launch
presents a built-in fuzzy picker over the `profiles:` map keys; the chosen profile is what
gets composed and materialized. No dependency on `fzf` being present on the host, and
profile names are read only after decrypt — never shown before the passphrase succeeds.

**Blocked by:** 04.

**Status:** ready-for-agent

- [ ] After decrypt, a built-in fuzzy picker lists the profile names and returns the choice.
- [ ] No external picker dependency (`fzf` etc.); the picker is in the binary.
- [ ] Picker runs strictly post-decrypt — profile names never surface before unlock.
- [ ] The picked profile feeds 04's compose → materialize path unchanged.
- [ ] Single-profile vault skips the prompt (or auto-selects) so 04's behavior still holds.

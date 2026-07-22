# 05 — Built-in fuzzy picker (decrypt-then-pick)

**What to build:** Profile selection after unlock. Once the vault is decrypted, launch
presents a built-in fuzzy picker over the `profiles:` map keys; the chosen profile is what
gets composed and materialized. No dependency on `fzf` being present on the host, and
profile names are read only after decrypt — never shown before the passphrase succeeds.

**Blocked by:** 04.

**Status:** done

- [x] After decrypt, a built-in fuzzy picker lists the profile names and returns the choice.
- [x] No external picker dependency (`fzf` etc.); the picker is in the binary.
- [x] Picker runs strictly post-decrypt — profile names never surface before unlock.
- [x] The picked profile feeds 04's compose → materialize path unchanged.
- [x] Single-profile vault skips the prompt (or auto-selects) so 04's behavior still holds.

## Comments

Picker lives in `picker.go`: raw-mode keystrokes drive live fuzzy filtering
(in-order subsequence match, fzf-style) over the manifest's profile names,
with arrow-key navigation and enter to select. No new dependency —
stdlib + the `golang.org/x/term` raw-mode support already used by
`promptPassphrase`. The filter/state-machine logic (`fuzzyScore`,
`filterProfiles`, `pickerState`, `readKey`) is pure and unit-tested; only the
terminal I/O loop (`pickProfilePrompt`) is thin, untested wiring, mirroring
`promptPassphrase`'s own pattern.

`launch()` calls `chooseProfile`, which auto-selects on a single-profile
manifest without ever invoking the picker (asserted by
`TestLaunchSingleProfileSkipsThePicker`), and otherwise defers to the
injected `pickProfile` seam — the same injection pattern as
`readPassphrase`, so the multi-profile path is covered by
`TestLaunchMultiProfileFeedsPickedChoiceIntoCompose` without a real TTY.

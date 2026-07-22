#!/bin/sh
# Self-check for gho.sh: exercises OS/arch detection and the curl-or-wget
# fallback without any network access. Run: sh gho_test.sh
set -eu

GHO_SH_TEST=1
. "$(dirname "$0")/gho.sh"

fails=0

assert_eq() {
    if [ "$1" != "$2" ]; then
        echo "FAIL: $3 — got '$1', want '$2'" >&2
        fails=$((fails + 1))
    fi
}

# assert_fails DESC CMD... — runs CMD in a subshell so its exit doesn't kill
# this test script, and checks it failed.
assert_fails() {
    desc=$1
    shift
    if ( "$@" ) >/dev/null 2>&1; then
        echo "FAIL: $desc — expected failure, got success" >&2
        fails=$((fails + 1))
    fi
}

# --- gho_target: 3-target matrix + rejections ---
# Shell functions shadow external commands for the rest of this script's
# lifetime — this `uname` (and `curl`/`wget` below) intercept every call
# gho.sh's functions make, so no real network or platform command runs.
uname() {
    case "$1" in
        -s) echo "$MOCK_UNAME_S" ;;
        -m) echo "$MOCK_UNAME_M" ;;
    esac
}

MOCK_UNAME_S=Linux MOCK_UNAME_M=x86_64
assert_eq "$(gho_target)" "linux-amd64" "linux/x86_64"

MOCK_UNAME_S=Linux MOCK_UNAME_M=aarch64
assert_eq "$(gho_target)" "linux-arm64" "linux/aarch64"

MOCK_UNAME_S=Darwin MOCK_UNAME_M=arm64
assert_eq "$(gho_target)" "darwin-arm64" "darwin/arm64"

MOCK_UNAME_S=Darwin MOCK_UNAME_M=x86_64
assert_fails "darwin/amd64 has no target (mac is arm64-only)" gho_target

MOCK_UNAME_S=SunOS MOCK_UNAME_M=x86_64
assert_fails "unsupported OS rejected" gho_target

MOCK_UNAME_S=Linux MOCK_UNAME_M=riscv64
assert_fails "unsupported arch rejected" gho_target

# --- gho_fetch: curl-or-wget floor, mocked via gho_has (not `command`) ---
gho_has() { [ "$1" = "$MOCK_HAVE" ]; }

MOCK_HAVE=curl
curl() { echo "curl called with: $*"; }
out=$(gho_fetch "http://example/x" /dev/null)
case "$out" in
    "curl called"*) ;;
    *) echo "FAIL: gho_fetch did not use curl when present" >&2; fails=$((fails + 1)) ;;
esac

MOCK_HAVE=wget
wget() { echo "wget called with: $*"; }
out=$(gho_fetch "http://example/x" /dev/null)
case "$out" in
    "wget called"*) ;;
    *) echo "FAIL: gho_fetch did not fall back to wget when curl absent" >&2; fails=$((fails + 1)) ;;
esac

MOCK_HAVE=none
assert_fails "gho_fetch fails clearly with neither curl nor wget" gho_fetch "http://example/x" /dev/null

# --- gho_workdir: RAM-preferred, real filesystem (no mock — the smallest
# check that fails if the RAM-or-fallback loop stops producing a dir) ---
gho_test_dir=$(gho_workdir)
if [ ! -d "$gho_test_dir" ] || [ ! -w "$gho_test_dir" ]; then
    echo "FAIL: gho_workdir did not return an existing, writable dir" >&2
    fails=$((fails + 1))
fi
rm -rf "$gho_test_dir"

if [ "$fails" -eq 0 ]; then
    echo "ok: gho.sh self-check"
else
    echo "FAILED: $fails check(s)"
    exit 1
fi

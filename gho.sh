#!/bin/sh
# ghoshell bootstrap.
#
#   curl -fsSL <repo-raw-url>/gho.sh | sh
#
# Detects OS/arch, fetches the matching gho launcher binary from GitHub
# Releases and the vault blob over plain HTTPS, then hands off to
# `gho launch <vault>`. Must survive a bare container with only curl-or-wget.
# No secret and no profile name ever appear here or in the paste — those
# live inside the sealed vault (decrypt-then-pick).
set -eu

# ponytail: repo identity is baked in here rather than parsed from the URL
# this script was fetched from — piped POSIX sh can't see its own URL. The
# publish step (ticket 08) sets these for the real repo; override for local
# testing.
: "${GHOSHELL_REPO:=OWNER/ghoshell}"
: "${GHOSHELL_BRANCH:=main}"
: "${GHOSHELL_VAULT_PATH:=vault.bin}"

gho_fail() {
    echo "gho: $1" >&2
    exit 1
}

gho_has() { command -v "$1" >/dev/null 2>&1; }

# gho_fetch URL DEST — curl-or-wget floor (spec: must survive a bare
# container that may have neither, and if so fails clearly).
gho_fetch() {
    if gho_has curl; then
        curl -fsSL "$1" -o "$2"
    elif gho_has wget; then
        wget -q "$1" -O "$2"
    else
        gho_fail "need curl or wget, found neither"
    fi
}

# gho_target prints "<os>-<arch>" for the 3-target matrix (darwin/arm64,
# linux/arm64, linux/amd64), or fails clearly for anything else.
gho_target() {
    os=$(uname -s)
    arch=$(uname -m)
    case "$os" in
        Darwin) os=darwin ;;
        Linux) os=linux ;;
        *) gho_fail "unsupported OS: $os (ghoshell ships darwin/linux only)" ;;
    esac
    case "$arch" in
        arm64|aarch64) arch=arm64 ;;
        x86_64|amd64) arch=amd64 ;;
        *) gho_fail "unsupported arch: $arch" ;;
    esac
    if [ "$os" = darwin ] && [ "$arch" = amd64 ]; then
        gho_fail "unsupported target: darwin-amd64 (mac is arm64-only)"
    fi
    echo "${os}-${arch}"
}

# gho_workdir makes a dir for the downloaded binary + vault, preferring
# RAM-backed /dev/shm with a temp-dir fallback (same tmpfs-first posture as
# the launched session).
gho_workdir() {
    for base in /dev/shm "${TMPDIR:-/tmp}"; do
        [ -d "$base" ] && [ -w "$base" ] || continue
        dir=$(mktemp -d "$base/ghoshell-boot.XXXXXX" 2>/dev/null) || continue
        echo "$dir"
        return 0
    done
    gho_fail "no writable RAM/temp dir (tried /dev/shm and ${TMPDIR:-/tmp})"
}

gho_main() {
    target=$(gho_target)
    dir=$(gho_workdir)
    # Cleans up dir on any early failure below (bad target, failed fetch).
    # ponytail: exec on the success path replaces this process, so the trap
    # never runs then and the binary + vault copy are left in dir. Accepted
    # ceiling — neither file is secret (vault is public+encrypted per spec),
    # and dir is RAM-backed or falls to $TMPDIR, both bounded by reboot/tmp
    # cleaners. Upgrade path if disk-fallback residency matters: a detached
    # `(sleep 1; rm -rf "$dir") &` fired just before exec.
    trap 'rm -rf "$dir"' EXIT

    bin="$dir/gho"
    gho_fetch "https://github.com/${GHOSHELL_REPO}/releases/latest/download/gho-${target}" "$bin"
    chmod +x "$bin"

    vault="$dir/vault.bin"
    gho_fetch "https://raw.githubusercontent.com/${GHOSHELL_REPO}/${GHOSHELL_BRANCH}/${GHOSHELL_VAULT_PATH}" "$vault"

    exec "$bin" launch "$vault"
}

# gho_test.sh sources this file with GHO_SH_TEST=1 to unit-test gho_target
# and gho_fetch without touching the network.
if [ "${GHO_SH_TEST:-}" != "1" ]; then
    gho_main
fi

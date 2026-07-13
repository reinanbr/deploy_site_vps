#!/bin/sh
# deploy_site — bundle installer
# Installs the deploy_site binary from an extracted release bundle.
#
# Usage:
#   sudo ./install.sh              # install
#   sudo ./install.sh --dry-run   # preview what would be done
#   sudo ./install.sh --help      # show this help
#   sudo ./install.sh --purge     # remove all installed files and dirs

set -e

# ─── paths ─────────────────────────────────────────────────

APP_NAME="deploy_site"
BIN_DEST="/usr/local/bin/${APP_NAME}"
CFG_DIR="/etc/${APP_NAME}"
CFG_EXAMPLE_DEST="${CFG_DIR}/config_deploy_site.example.json"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BIN_SRC="${SCRIPT_DIR}/${APP_NAME}"
CFG_EXAMPLE="${SCRIPT_DIR}/config_deploy_site.example.json"

# ─── flags ─────────────────────────────────────────────────

DRY_RUN=0
PURGE=0

for arg in "$@"; do
    case "$arg" in
        --dry-run)  DRY_RUN=1 ;;
        --purge)    PURGE=1 ;;
        --help|-h)
            cat <<EOF
deploy_site bundle installer

Usage:
  sudo ./install.sh              install deploy_site system-wide
  sudo ./install.sh --dry-run   preview actions without making changes
  sudo ./install.sh --purge     remove all installed files
  sudo ./install.sh --help      show this help

Installs to:
  ${BIN_DEST}       binary
  ${CFG_EXAMPLE_DEST}   example config for reference

This installer does not touch nginx, certbot, docker or any project config —
those are managed per-project with 'deploy_site init/dry-run/deploy'.
EOF
            exit 0
            ;;
        *)
            printf 'Unknown option: %s\n' "$arg" >&2
            printf "Run './install.sh --help' for usage.\n" >&2
            exit 1
            ;;
    esac
done

# ─── helpers ───────────────────────────────────────────────

info() { printf '  \033[1;34m::\033[0m %s\n' "$*"; }
ok()   { printf '  \033[1;32m+\033[0m  %s\n' "$*"; }
warn() { printf '  \033[1;33m!\033[0m  %s\n' "$*"; }
die()  { printf '  \033[1;31mx\033[0m  %s\n' "$*" >&2; exit 1; }
skip() { printf '  \033[2m-  %s (dry-run)\033[0m\n' "$*"; }

run() {
    if [ "$DRY_RUN" -eq 1 ]; then
        skip "$*"
    else
        "$@"
    fi
}

# ─── root check ────────────────────────────────────────────

if [ "$(id -u)" -ne 0 ] && [ "$DRY_RUN" -eq 0 ]; then
    die "Must be run as root. Try: sudo ./install.sh"
fi

# ─── purge ─────────────────────────────────────────────────

if [ "$PURGE" -eq 1 ]; then
    printf '\n\033[1mRemoving deploy_site...\033[0m\n\n'

    [ -e "$BIN_DEST" ] && run rm -f "$BIN_DEST" && ok "Removed $BIN_DEST" || true

    [ -d "$CFG_DIR" ] && run rm -rf "$CFG_DIR" && ok "Removed $CFG_DIR" || true

    printf '\n'
    ok "deploy_site removed."
    warn "nginx sites, certificates and any 'deploy_site service' systemd timer were left untouched."
    exit 0
fi

# ─── pre-flight ────────────────────────────────────────────

printf '\n\033[1mInstalling deploy_site...\033[0m\n\n'

[ -f "${BIN_SRC}" ] || die "Binary not found: ${BIN_SRC}
         Build it first:  go build -o packaging/linux/deploy_site main.go
         Or download a release bundle from:
         https://github.com/reinanbr/deploy_site/releases"

[ -f "${CFG_EXAMPLE}" ] || die "Config example not found: ${CFG_EXAMPLE}"

# verify sha256 checksum if a checksum file is present in the bundle
CHECKSUM_FILE="${SCRIPT_DIR}/sha256sums.txt"
if [ -f "$CHECKSUM_FILE" ]; then
    info "Verifying checksum..."
    if command -v sha256sum >/dev/null 2>&1; then
        if (cd "$SCRIPT_DIR" && sha256sum --check --ignore-missing "$CHECKSUM_FILE" >/dev/null 2>&1); then
            ok "Checksum OK"
        else
            die "Checksum mismatch — binary may be corrupted. Re-download the release bundle."
        fi
    else
        warn "sha256sum not found — skipping checksum verification"
    fi
fi

# ─── binary ────────────────────────────────────────────────

info "Installing binary       ${BIN_DEST}"
run install -Dm755 "${BIN_SRC}" "${BIN_DEST}"
ok "Binary installed"

# ─── example config ────────────────────────────────────────

info "Config dir              ${CFG_DIR}"
run install -d "${CFG_DIR}"
run install -m644 "${CFG_EXAMPLE}" "${CFG_EXAMPLE_DEST}"
ok "Example config:         ${CFG_EXAMPLE_DEST}"

# ─── dependency check ──────────────────────────────────────

for dep in docker nginx certbot; do
    if ! command -v "$dep" >/dev/null 2>&1; then
        warn "'${dep}' not found on PATH — install it before running 'deploy_site deploy'"
    fi
done

# ─── done ──────────────────────────────────────────────────

printf '\n'
ok "Installation complete"
printf '\n'
printf '  binary  : %s\n' "${BIN_DEST}"
printf '  example : %s\n' "${CFG_EXAMPLE_DEST}"
printf '\n'
printf '  Per project:\n'
printf '    cd /path/to/your/docker-compose/project\n'
printf '    deploy_site init && deploy_site dry-run && deploy_site deploy\n'
printf '\n'
printf '  Automatic renewal (installs a systemd timer):\n'
printf '    sudo deploy_site service install config_deploy_site.json\n'
printf '\n'

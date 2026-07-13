#!/usr/bin/env bash
set -euo pipefail

APP_NAME="deploy_site"
BIN_DEST="/usr/local/bin/${APP_NAME}"
CFG_DIR="/etc/${APP_NAME}"
PURGE="${1:-}"

require_root() {
  if [[ "${EUID}" -ne 0 ]]; then
    if command -v sudo >/dev/null 2>&1; then
      exec sudo bash "$0" "$@"
    fi
    echo "[ERROR] run as root or install sudo" >&2
    exit 1
  fi
}

main() {
  require_root "$@"

  rm -f "${BIN_DEST}"
  echo "[OK] removed binary: ${BIN_DEST}"

  if [[ "${PURGE}" == "--purge" ]]; then
    rm -rf "${CFG_DIR}"
    echo "[OK] removed config directory: ${CFG_DIR}"
  else
    echo "[INFO] keeping ${CFG_DIR}"
    echo "[INFO] use --purge to remove it"
  fi

  echo "[INFO] nginx sites, TLS certificates, docker containers and any"
  echo "[INFO] 'deploy_site service' systemd timer were left untouched — remove"
  echo "[INFO] them manually with 'deploy_site remove' / 'deploy_site service uninstall' first if needed."
}

main "$@"

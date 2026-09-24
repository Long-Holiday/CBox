#!/usr/bin/env bash
set -euo pipefail

echo "==> Building CBox binaries..."
make build

INSTALL_DIR="${HOME}/.local/bin"
if [ "$(id -u)" -eq 0 ]; then
    INSTALL_DIR="/usr/local/bin"
fi

mkdir -p "$INSTALL_DIR"
cp -f bin/cbox "${INSTALL_DIR}/cbox"
cp -f bin/cboxd "${INSTALL_DIR}/cboxd"
chmod +x "${INSTALL_DIR}/cbox" "${INSTALL_DIR}/cboxd"

echo "==> Installed cbox and cboxd to ${INSTALL_DIR}"

mkdir -p "${HOME}/.config/cbox"
mkdir -p "${HOME}/.local/share/cbox"

echo "==> Installation complete! Run 'cboxd' in background and use 'cbox' to start."

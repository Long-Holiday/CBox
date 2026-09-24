#!/usr/bin/env bash
set -euo pipefail

# CBox Worker Bootstrap Script
CBOX_ROOT="/content/.cbox"

echo "[cbox-worker] Initializing worker filesystem structure..."
mkdir -p "${CBOX_ROOT}/bin"
mkdir -p "${CBOX_ROOT}/containers"
mkdir -p "${CBOX_ROOT}/images"
mkdir -p "${CBOX_ROOT}/volumes"
mkdir -p "${CBOX_ROOT}/cache/images"
mkdir -p "${CBOX_ROOT}/cache/volumes"

# Ensure tmux and rsync are available
if ! command -v tmux >/dev/null 2>&1; then
    echo "[cbox-worker] Installing tmux..."
    apt-get update -qq && apt-get install -y -qq tmux >/dev/null 2>&1 || true
fi

if ! command -v rsync >/dev/null 2>&1; then
    echo "[cbox-worker] Installing rsync..."
    apt-get update -qq && apt-get install -y -qq rsync >/dev/null 2>&1 || true
fi

echo "{\"status\": \"ready\", \"timestamp\": $(date +%s)}" > "${CBOX_ROOT}/worker.json"
echo "CBOX_BOOTSTRAP_COMPLETE"

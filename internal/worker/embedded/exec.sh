#!/usr/bin/env bash
# CBox Remote Command Exec Wrapper
set -o pipefail

CONTAINER_ID="$1"
shift

CBOX_ROOT="/content/.cbox"
CONTAINER_DIR="${CBOX_ROOT}/containers/${CONTAINER_ID}"

if [ -f "${CONTAINER_DIR}/env.sh" ]; then
    source "${CONTAINER_DIR}/env.sh"
fi

WORKDIR=$(cat "${CONTAINER_DIR}/workdir" 2>/dev/null || echo "/workspace")
if [ -d "$WORKDIR" ]; then
    cd "$WORKDIR"
fi

exec "$@"

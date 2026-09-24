#!/usr/bin/env bash
# CBox Container Process Entrypoint Script
set -o pipefail

CONTAINER_ID="$1"
if [ -z "$CONTAINER_ID" ]; then
    echo "Usage: entrypoint.sh <container_id>" >&2
    exit 1
fi

CBOX_ROOT="/content/.cbox"
CONTAINER_DIR="${CBOX_ROOT}/containers/${CONTAINER_ID}"
CONFIG_FILE="${CONTAINER_DIR}/config.json"
STDOUT_LOG="${CONTAINER_DIR}/stdout.log"
STDERR_LOG="${CONTAINER_DIR}/stderr.log"
EXIT_FILE="${CONTAINER_DIR}/exit_code"
PID_FILE="${CONTAINER_DIR}/pid"
STATE_FILE="${CONTAINER_DIR}/state"

echo "$$" > "$PID_FILE"
echo "running" > "$STATE_FILE"

# Source environment variables if exported
if [ -f "${CONTAINER_DIR}/env.sh" ]; then
    source "${CONTAINER_DIR}/env.sh"
fi

WORKDIR=$(cat "${CONTAINER_DIR}/workdir" 2>/dev/null || echo "/workspace")
mkdir -p "$WORKDIR"
cd "$WORKDIR"

# Read execution command script
CMD_SCRIPT="${CONTAINER_DIR}/cmd.sh"
if [ ! -f "$CMD_SCRIPT" ]; then
    echo "Error: cmd.sh not found in ${CONTAINER_DIR}" >&2
    echo "1" > "$EXIT_FILE"
    echo "failed" > "$STATE_FILE"
    exit 1
fi

chmod +x "$CMD_SCRIPT"

# Execute command while teeing logs to stdout.log and stderr.log
"$CMD_SCRIPT" > >(tee -a "$STDOUT_LOG") 2> >(tee -a "$STDERR_LOG" >&2)
EXIT_CODE=$?

echo "$EXIT_CODE" > "$EXIT_FILE"
if [ "$EXIT_CODE" -eq 0 ]; then
    echo "exited" > "$STATE_FILE"
else
    echo "failed" > "$STATE_FILE"
fi

exit "$EXIT_CODE"

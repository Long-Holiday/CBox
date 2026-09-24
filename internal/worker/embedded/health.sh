#!/usr/bin/env bash
# CBox Worker Health and Metrics Extractor
set -o pipefail

# Collect GPU metrics
GPU_NAME="None"
GPU_MEM_USED=0
GPU_MEM_TOTAL=0
GPU_UTIL=0

if command -v nvidia-smi >/dev/null 2>&1; then
    SMI_OUT=$(nvidia-smi --query-gpu=name,memory.used,memory.total,utilization.gpu --format=csv,noheader,nounits 2>/dev/null | head -n 1 || true)
    if [ -n "$SMI_OUT" ]; then
        IFS=',' read -r GPU_NAME GPU_MEM_USED GPU_MEM_TOTAL GPU_UTIL <<< "$SMI_OUT"
        GPU_NAME=$(echo "$GPU_NAME" | xargs)
        GPU_MEM_USED=$(echo "$GPU_MEM_USED" | xargs)
        GPU_MEM_TOTAL=$(echo "$GPU_MEM_TOTAL" | xargs)
        GPU_UTIL=$(echo "$GPU_UTIL" | xargs)
    fi
fi

# Collect CPU metrics
CPU_UTIL=$(top -bn1 | grep "Cpu(s)" | sed "s/.*, *\([0-9.]*\)%* id.*/\1/" | awk '{print 100 - $1}' 2>/dev/null || echo "0")
if [ -z "$CPU_UTIL" ]; then
    CPU_UTIL="0"
fi

# Collect RAM metrics in MB
RAM_TOTAL=0
RAM_USED=0
if [ -f /proc/meminfo ]; then
    RAM_TOTAL=$(grep MemTotal /proc/meminfo | awk '{print int($2/1024)}')
    RAM_FREE=$(grep MemAvailable /proc/meminfo | awk '{print int($2/1024)}')
    RAM_USED=$((RAM_TOTAL - RAM_FREE))
fi

echo "{"
echo "  \"gpu_name\": \"${GPU_NAME}\","
echo "  \"gpu_mem_used_mb\": ${GPU_MEM_USED:-0},"
echo "  \"gpu_mem_total_mb\": ${GPU_MEM_TOTAL:-0},"
echo "  \"gpu_util_percent\": ${GPU_UTIL:-0},"
echo "  \"cpu_util_percent\": ${CPU_UTIL:-0},"
echo "  \"ram_used_mb\": ${RAM_USED:-0},"
echo "  \"ram_total_mb\": ${RAM_TOTAL:-0}"
echo "}"

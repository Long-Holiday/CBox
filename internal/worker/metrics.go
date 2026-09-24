package worker

import (
	"context"
	"strconv"
	"strings"
	"time"

	"cbox/internal/transport"
	pkgApi "cbox/pkg/api"
)

func CollectStats(ctx context.Context, trans transport.Transport, containerID string) (*pkgApi.StatsResponse, error) {
	// Query nvidia-smi and system memory
	cmd := `nvidia-smi --query-gpu=name,memory.used,memory.total,utilization.gpu --format=csv,noheader,nounits 2>/dev/null || echo "N/A,0,0,0"`
	res, err := trans.Exec(ctx, []string{"bash", "-c", cmd}, transport.ExecOptions{})
	if err != nil {
		return nil, err
	}

	stats := &pkgApi.StatsResponse{
		ContainerID: containerID,
		Timestamp:   time.Now().UTC(),
	}

	line := strings.TrimSpace(string(res.Stdout))
	parts := strings.Split(line, ",")
	if len(parts) >= 4 {
		stats.GPUName = strings.TrimSpace(parts[0])
		usedMB, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
		totalMB, _ := strconv.Atoi(strings.TrimSpace(parts[2]))
		util, _ := strconv.ParseFloat(strings.TrimSpace(parts[3]), 64)

		stats.GPUMemoryUsedMB = usedMB
		stats.GPUMemoryTotalMB = totalMB
		stats.GPUUtilization = util
	}

	// Query host memory
	memCmd := `free -m | awk '/Mem:/ {print $3","$2}' 2>/dev/null || echo "0,0"`
	memRes, err := trans.Exec(ctx, []string{"bash", "-c", memCmd}, transport.ExecOptions{})
	if err == nil {
		memParts := strings.Split(strings.TrimSpace(string(memRes.Stdout)), ",")
		if len(memParts) >= 2 {
			used, _ := strconv.ParseInt(strings.TrimSpace(memParts[0]), 10, 64)
			tot, _ := strconv.ParseInt(strings.TrimSpace(memParts[1]), 10, 64)
			stats.MemoryUsedMB = used
			stats.MemoryTotalMB = tot
		}
	}

	return stats, nil
}

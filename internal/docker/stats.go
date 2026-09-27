package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/docker/docker/api/types"
)

// ContainerStats is the computed, UI-friendly snapshot derived from one
// Docker stats sample.
type ContainerStats struct {
	CPUPercent    float64
	MemUsageBytes uint64
	MemLimitBytes uint64
	MemPercent    float64
	NetRxBytes    uint64
	NetTxBytes    uint64
}

// StreamStats calls fn with a computed ContainerStats snapshot roughly once
// per second until ctx is canceled or the stream ends. It returns any error
// from the underlying Docker connection (nil on a clean ctx cancellation).
func (c *Client) StreamStats(ctx context.Context, id string, fn func(ContainerStats)) error {
	resp, err := c.cli.ContainerStats(ctx, id, true)
	if err != nil {
		return fmt.Errorf("streaming stats for %s: %w", id, err)
	}
	defer resp.Body.Close()

	dec := json.NewDecoder(resp.Body)
	for {
		var raw types.StatsJSON
		if err := dec.Decode(&raw); err != nil {
			if err == io.EOF || ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("decoding stats for %s: %w", id, err)
		}
		fn(computeStats(raw))
	}
}

// SnapshotStats returns a single computed stats sample for a running
// container (like `docker stats --no-stream`), without holding open a
// continuous stream. Used to build aggregate, host-wide metrics by polling
// every running container on a timer instead of managing one long-lived
// stream per container.
func (c *Client) SnapshotStats(ctx context.Context, id string) (ContainerStats, error) {
	resp, err := c.cli.ContainerStats(ctx, id, false)
	if err != nil {
		return ContainerStats{}, fmt.Errorf("getting stats for %s: %w", id, err)
	}
	defer resp.Body.Close()

	var raw types.StatsJSON
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return ContainerStats{}, fmt.Errorf("decoding stats for %s: %w", id, err)
	}
	return computeStats(raw), nil
}

// computeStats mirrors the CPU%/memory calculations the Docker CLI itself
// uses (see docker/cli's calculateCPUPercentUnix / calculateMemUsageUnixNoCache),
// since the raw counters in a stats sample are cumulative, not point-in-time.
func computeStats(raw types.StatsJSON) ContainerStats {
	var cpuPercent float64
	cpuDelta := float64(raw.CPUStats.CPUUsage.TotalUsage) - float64(raw.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(raw.CPUStats.SystemUsage) - float64(raw.PreCPUStats.SystemUsage)
	if systemDelta > 0 && cpuDelta > 0 {
		onlineCPUs := float64(raw.CPUStats.OnlineCPUs)
		if onlineCPUs == 0 {
			onlineCPUs = float64(len(raw.CPUStats.CPUUsage.PercpuUsage))
		}
		if onlineCPUs == 0 {
			onlineCPUs = 1
		}
		cpuPercent = (cpuDelta / systemDelta) * onlineCPUs * 100.0
	}

	memUsage := raw.MemoryStats.Usage
	// Linux reports page cache as part of Usage; subtract it (cgroup v1's
	// "cache" or v2's "inactive_file") so this reads like real working-set
	// memory instead of including reclaimable cache.
	if cache, ok := raw.MemoryStats.Stats["cache"]; ok && cache < memUsage {
		memUsage -= cache
	} else if inactiveFile, ok := raw.MemoryStats.Stats["inactive_file"]; ok && inactiveFile < memUsage {
		memUsage -= inactiveFile
	}

	var memPercent float64
	if raw.MemoryStats.Limit > 0 {
		memPercent = float64(memUsage) / float64(raw.MemoryStats.Limit) * 100.0
	}

	var rx, tx uint64
	for _, n := range raw.Networks {
		rx += n.RxBytes
		tx += n.TxBytes
	}

	return ContainerStats{
		CPUPercent:    cpuPercent,
		MemUsageBytes: memUsage,
		MemLimitBytes: raw.MemoryStats.Limit,
		MemPercent:    memPercent,
		NetRxBytes:    rx,
		NetTxBytes:    tx,
	}
}

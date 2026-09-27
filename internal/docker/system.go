package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
)

// DiskUsageSummary is a simplified breakdown of on-disk space consumed by
// Docker objects, used to power the dashboard's storage chart.
type DiskUsageSummary struct {
	ImagesBytes     int64
	ContainersBytes int64
	VolumesBytes    int64
}

// DiskUsage reports how much disk space images, containers, and volumes are
// currently consuming (the same data `docker system df` shows).
func (c *Client) DiskUsage(ctx context.Context) (DiskUsageSummary, error) {
	du, err := c.cli.DiskUsage(ctx, types.DiskUsageOptions{})
	if err != nil {
		return DiskUsageSummary{}, fmt.Errorf("getting disk usage: %w", err)
	}

	var summary DiskUsageSummary
	for _, img := range du.Images {
		summary.ImagesBytes += img.Size
	}
	for _, ctr := range du.Containers {
		summary.ContainersBytes += ctr.SizeRw
	}
	for _, vol := range du.Volumes {
		if vol.UsageData != nil {
			summary.VolumesBytes += vol.UsageData.Size
		}
	}
	return summary, nil
}

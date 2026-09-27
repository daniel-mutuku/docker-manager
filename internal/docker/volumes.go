package docker

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/volume"
)

// VolumeSummary is the trimmed-down view used for the volumes list page.
type VolumeSummary struct {
	Name        string
	Driver      string
	Mountpoint  string
	CreatedAt   string
	UsedByNames []string // display names of containers that mount this volume
}

// ListVolumes returns every volume, annotated with which containers (by
// display name) currently mount each one. The container/volume
// cross-reference comes from the same container-list call used elsewhere
// (its Mounts field is populated without needing a per-container inspect).
func (c *Client) ListVolumes(ctx context.Context) ([]VolumeSummary, error) {
	volumesResp, err := c.cli.VolumeList(ctx, volume.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing volumes: %w", err)
	}

	usedBy, err := c.volumeUsage(ctx)
	if err != nil {
		return nil, err
	}

	summaries := make([]VolumeSummary, 0, len(volumesResp.Volumes))
	for _, v := range volumesResp.Volumes {
		summaries = append(summaries, VolumeSummary{
			Name:        v.Name,
			Driver:      v.Driver,
			Mountpoint:  v.Mountpoint,
			CreatedAt:   v.CreatedAt,
			UsedByNames: usedBy[v.Name],
		})
	}
	return summaries, nil
}

// volumeUsage maps each volume name to the display names of containers
// that mount it.
func (c *Client) volumeUsage(ctx context.Context) (map[string][]string, error) {
	containers, err := c.cli.ContainerList(ctx, containerListAllOptions())
	if err != nil {
		return nil, fmt.Errorf("listing containers for volume usage: %w", err)
	}

	usage := make(map[string][]string)
	for _, cnt := range containers {
		name := strings.TrimPrefix(firstOrEmpty(cnt.Names), "/")
		for _, m := range cnt.Mounts {
			if m.Type != mount.TypeVolume || m.Name == "" {
				continue
			}
			usage[m.Name] = append(usage[m.Name], name)
		}
	}
	return usage, nil
}

// CreateVolume creates a new named volume with the default local driver.
func (c *Client) CreateVolume(ctx context.Context, name string) error {
	_, err := c.cli.VolumeCreate(ctx, volume.CreateOptions{Name: name})
	if err != nil {
		return fmt.Errorf("creating volume %s: %w", name, err)
	}
	return nil
}

// RemoveVolume removes a volume by name.
func (c *Client) RemoveVolume(ctx context.Context, name string, force bool) error {
	if err := c.cli.VolumeRemove(ctx, name, force); err != nil {
		return fmt.Errorf("removing volume %s: %w", name, err)
	}
	return nil
}

// containerListAllOptions is the shared "every container, running or not"
// listing configuration used across container/volume/network cross-referencing.
func containerListAllOptions() types.ContainerListOptions {
	return types.ContainerListOptions{All: true}
}

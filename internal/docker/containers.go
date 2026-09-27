package docker

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
)

// ErrContainerNotFound is returned when a container id no longer exists.
var ErrContainerNotFound = errors.New("container not found")

// ContainerSummary is the trimmed-down view used for the container list page.
type ContainerSummary struct {
	ID        string
	ShortID   string
	Name      string
	Image     string
	Status    string
	State     string
	Ports     string
	CreatedAt int64
}

// ListContainers returns all containers, running and stopped.
func (c *Client) ListContainers(ctx context.Context) ([]ContainerSummary, error) {
	containers, err := c.cli.ContainerList(ctx, containerListAllOptions())
	if err != nil {
		return nil, fmt.Errorf("listing containers: %w", err)
	}

	summaries := make([]ContainerSummary, 0, len(containers))
	for _, cnt := range containers {
		summaries = append(summaries, toSummary(cnt))
	}
	return summaries, nil
}

// GetContainerSummary returns the list-style summary for a single container,
// which includes the human-readable Status string that inspect doesn't have.
// It returns ErrContainerNotFound if the container no longer exists.
func (c *Client) GetContainerSummary(ctx context.Context, id string) (ContainerSummary, error) {
	f := filters.NewArgs(filters.Arg("id", id))
	containers, err := c.cli.ContainerList(ctx, types.ContainerListOptions{All: true, Filters: f})
	if err != nil {
		return ContainerSummary{}, fmt.Errorf("listing container %s: %w", id, err)
	}
	if len(containers) == 0 {
		return ContainerSummary{}, ErrContainerNotFound
	}

	return toSummary(containers[0]), nil
}

// ListContainersByComposeProject returns every container (running or not)
// that Docker Compose tagged as belonging to the given project name, via
// the com.docker.compose.project label Compose sets automatically.
func (c *Client) ListContainersByComposeProject(ctx context.Context, project string) ([]ContainerSummary, error) {
	f := filters.NewArgs(filters.Arg("label", "com.docker.compose.project="+project))
	containers, err := c.cli.ContainerList(ctx, types.ContainerListOptions{All: true, Filters: f})
	if err != nil {
		return nil, fmt.Errorf("listing containers for compose project %s: %w", project, err)
	}

	summaries := make([]ContainerSummary, 0, len(containers))
	for _, cnt := range containers {
		summaries = append(summaries, toSummary(cnt))
	}
	return summaries, nil
}

// toSummary maps a Docker API container listing entry onto our trimmed-down
// ContainerSummary view.
func toSummary(cnt types.Container) ContainerSummary {
	return ContainerSummary{
		ID:        cnt.ID,
		ShortID:   shortID(cnt.ID),
		Name:      strings.TrimPrefix(firstOrEmpty(cnt.Names), "/"),
		Image:     cnt.Image,
		Status:    cnt.Status,
		State:     cnt.State,
		Ports:     formatPorts(cnt.Ports),
		CreatedAt: cnt.Created,
	}
}

// InspectContainer returns the full inspect result for a single container.
func (c *Client) InspectContainer(ctx context.Context, id string) (types.ContainerJSON, error) {
	info, err := c.cli.ContainerInspect(ctx, id)
	if err != nil {
		return types.ContainerJSON{}, fmt.Errorf("inspecting container %s: %w", id, err)
	}
	return info, nil
}

// StartContainer starts a stopped container.
func (c *Client) StartContainer(ctx context.Context, id string) error {
	if err := c.cli.ContainerStart(ctx, id, types.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("starting container %s: %w", id, err)
	}
	return nil
}

// StopContainer stops a running container, giving it up to its configured
// (or the default) grace period before killing it.
func (c *Client) StopContainer(ctx context.Context, id string) error {
	if err := c.cli.ContainerStop(ctx, id, container.StopOptions{}); err != nil {
		return fmt.Errorf("stopping container %s: %w", id, err)
	}
	return nil
}

// RestartContainer restarts a container.
func (c *Client) RestartContainer(ctx context.Context, id string) error {
	if err := c.cli.ContainerRestart(ctx, id, container.StopOptions{}); err != nil {
		return fmt.Errorf("restarting container %s: %w", id, err)
	}
	return nil
}

// RemoveContainer removes a container. It refuses to remove a running
// container unless force is set.
func (c *Client) RemoveContainer(ctx context.Context, id string, force bool) error {
	err := c.cli.ContainerRemove(ctx, id, types.ContainerRemoveOptions{Force: force})
	if err != nil {
		return fmt.Errorf("removing container %s: %w", id, err)
	}
	return nil
}

func firstOrEmpty(names []string) string {
	if len(names) == 0 {
		return ""
	}
	return names[0]
}

func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func formatPorts(ports []types.Port) string {
	if len(ports) == 0 {
		return ""
	}
	parts := make([]string, 0, len(ports))
	for _, p := range ports {
		switch {
		case p.PublicPort != 0 && p.IP != "":
			parts = append(parts, fmt.Sprintf("%s:%d->%d/%s", p.IP, p.PublicPort, p.PrivatePort, p.Type))
		case p.PublicPort != 0:
			parts = append(parts, fmt.Sprintf("%d->%d/%s", p.PublicPort, p.PrivatePort, p.Type))
		default:
			parts = append(parts, fmt.Sprintf("%d/%s", p.PrivatePort, p.Type))
		}
	}
	return strings.Join(parts, ", ")
}

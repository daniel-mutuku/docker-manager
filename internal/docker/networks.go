package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
)

// NetworkSummary is the trimmed-down view used for the networks list page.
type NetworkSummary struct {
	ID         string
	ShortID    string
	Name       string
	Driver     string
	Scope      string
	Internal   bool
	Containers []NetworkContainer
}

// NetworkContainer is one container attached to a network.
type NetworkContainer struct {
	Name        string
	IPv4Address string
}

// ListNetworks returns every Docker network, including which containers are
// attached to each one.
func (c *Client) ListNetworks(ctx context.Context) ([]NetworkSummary, error) {
	networks, err := c.cli.NetworkList(ctx, types.NetworkListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing networks: %w", err)
	}

	summaries := make([]NetworkSummary, 0, len(networks))
	for _, n := range networks {
		summaries = append(summaries, toNetworkSummary(n))
	}
	return summaries, nil
}

// GetNetwork returns a single network's detail, including attached
// containers, by ID or name.
func (c *Client) GetNetwork(ctx context.Context, id string) (NetworkSummary, error) {
	n, err := c.cli.NetworkInspect(ctx, id, types.NetworkInspectOptions{})
	if err != nil {
		return NetworkSummary{}, fmt.Errorf("inspecting network %s: %w", id, err)
	}
	return toNetworkSummary(n), nil
}

func toNetworkSummary(n types.NetworkResource) NetworkSummary {
	containers := make([]NetworkContainer, 0, len(n.Containers))
	for _, ep := range n.Containers {
		containers = append(containers, NetworkContainer{Name: ep.Name, IPv4Address: ep.IPv4Address})
	}
	return NetworkSummary{
		ID:         n.ID,
		ShortID:    shortID(n.ID),
		Name:       n.Name,
		Driver:     n.Driver,
		Scope:      n.Scope,
		Internal:   n.Internal,
		Containers: containers,
	}
}

// CreateNetwork creates a new network with the given name and driver (an
// empty driver defaults to "bridge", Docker's own default).
func (c *Client) CreateNetwork(ctx context.Context, name, driver string) error {
	_, err := c.cli.NetworkCreate(ctx, name, types.NetworkCreate{Driver: driver})
	if err != nil {
		return fmt.Errorf("creating network %s: %w", name, err)
	}
	return nil
}

// RemoveNetwork removes a network by ID or name.
func (c *Client) RemoveNetwork(ctx context.Context, id string) error {
	if err := c.cli.NetworkRemove(ctx, id); err != nil {
		return fmt.Errorf("removing network %s: %w", id, err)
	}
	return nil
}

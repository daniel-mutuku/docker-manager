// Package docker wraps the Docker Engine SDK client with the operations
// needed by the web manager (containers, and later images/volumes/networks).
package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/client"
)

// Client wraps the Docker SDK client.
type Client struct {
	cli *client.Client
}

// New creates a Docker client pointed at host (e.g. "unix:///var/run/docker.sock").
func New(host string) (*Client, error) {
	cli, err := client.NewClientWithOpts(
		client.WithHost(host),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("creating docker client: %w", err)
	}
	return &Client{cli: cli}, nil
}

// Ping verifies the daemon is reachable.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.cli.Ping(ctx)
	if err != nil {
		return fmt.Errorf("pinging docker daemon: %w", err)
	}
	return nil
}

// Close releases the underlying connection.
func (c *Client) Close() error {
	return c.cli.Close()
}

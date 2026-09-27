package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
)

// shellProbe picks bash when it's on PATH inside the container and falls
// back to sh otherwise, so the terminal gets a nicer shell when available
// without failing on minimal images that only ship sh.
const shellProbe = "if command -v bash >/dev/null 2>&1; then exec bash; else exec sh; fi"

// ExecShell starts an interactive shell inside a running container and
// attaches to it. The caller owns the returned HijackedResponse and must
// Close it when done.
func (c *Client) ExecShell(ctx context.Context, containerID string) (string, types.HijackedResponse, error) {
	created, err := c.cli.ContainerExecCreate(ctx, containerID, types.ExecConfig{
		Cmd:          []string{"/bin/sh", "-c", shellProbe},
		Tty:          true,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Env:          []string{"TERM=xterm-256color"},
	})
	if err != nil {
		return "", types.HijackedResponse{}, fmt.Errorf("creating exec session: %w", err)
	}

	hijacked, err := c.cli.ContainerExecAttach(ctx, created.ID, types.ExecStartCheck{Tty: true})
	if err != nil {
		return "", types.HijackedResponse{}, fmt.Errorf("attaching to exec session: %w", err)
	}

	return created.ID, hijacked, nil
}

// ExecResize resizes the TTY of a running exec session.
func (c *Client) ExecResize(ctx context.Context, execID string, height, width uint) error {
	if err := c.cli.ContainerExecResize(ctx, execID, types.ResizeOptions{Height: height, Width: width}); err != nil {
		return fmt.Errorf("resizing exec session: %w", err)
	}
	return nil
}

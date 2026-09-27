package docker

import (
	"context"
	"fmt"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/stdcopy"
)

// Logs returns a reader over a container's combined stdout+stderr log
// stream, already demultiplexed into plain text regardless of whether the
// container was created with a TTY. When follow is true the stream stays
// open and yields new lines as they're written; cancel ctx (or close the
// returned reader) to stop it. tail is the number of lines to seed the
// stream with (Docker's own log-tail syntax, e.g. "200" or "all").
func (c *Client) Logs(ctx context.Context, id string, tail string, follow bool) (io.ReadCloser, error) {
	info, err := c.cli.ContainerInspect(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("inspecting container %s for logs: %w", id, err)
	}

	raw, err := c.cli.ContainerLogs(ctx, id, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     follow,
		Tail:       tail,
	})
	if err != nil {
		return nil, fmt.Errorf("streaming logs for %s: %w", id, err)
	}

	// A TTY container's log stream is already plain bytes; without a TTY,
	// Docker multiplexes stdout/stderr with an 8-byte frame header per chunk
	// that stdcopy needs to strip.
	if info.Config != nil && info.Config.Tty {
		return raw, nil
	}

	pr, pw := io.Pipe()
	go func() {
		_, copyErr := stdcopy.StdCopy(pw, pw, raw)
		raw.Close()
		pw.CloseWithError(copyErr)
	}()
	return pr, nil
}

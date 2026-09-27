package docker

import (
	"context"
	"fmt"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
)

// ImageSummary is the trimmed-down view used for the images list page.
type ImageSummary struct {
	ID         string
	ShortID    string
	Tags       []string
	SizeBytes  int64
	CreatedAt  int64
	Containers int64 // number of containers referencing this image, -1 if unknown
}

// ListImages returns every locally stored image.
func (c *Client) ListImages(ctx context.Context) ([]ImageSummary, error) {
	images, err := c.cli.ImageList(ctx, types.ImageListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing images: %w", err)
	}

	summaries := make([]ImageSummary, 0, len(images))
	for _, img := range images {
		tags := img.RepoTags
		if len(tags) == 0 {
			tags = []string{"<none>:<none>"}
		}
		summaries = append(summaries, ImageSummary{
			ID:         img.ID,
			ShortID:    shortID(trimSHAPrefix(img.ID)),
			Tags:       tags,
			SizeBytes:  img.Size,
			CreatedAt:  img.Created,
			Containers: img.Containers,
		})
	}
	return summaries, nil
}

// PullImage pulls ref (e.g. "nginx:latest") and returns a reader of Docker's
// newline-delimited JSON progress stream. Cancel ctx to abort the pull.
func (c *Client) PullImage(ctx context.Context, ref string) (io.ReadCloser, error) {
	rc, err := c.cli.ImagePull(ctx, ref, types.ImagePullOptions{})
	if err != nil {
		return nil, fmt.Errorf("pulling image %s: %w", ref, err)
	}
	return rc, nil
}

// RemoveImage removes an image by ID or reference.
func (c *Client) RemoveImage(ctx context.Context, id string, force bool) error {
	_, err := c.cli.ImageRemove(ctx, id, types.ImageRemoveOptions{Force: force})
	if err != nil {
		return fmt.Errorf("removing image %s: %w", id, err)
	}
	return nil
}

// PruneImageResult reports the outcome of a dangling-image prune.
type PruneImageResult struct {
	ImagesDeleted  int
	SpaceReclaimed uint64
}

// PruneDanglingImages removes dangling (untagged, unreferenced) images, the
// same set `docker image prune` removes by default.
func (c *Client) PruneDanglingImages(ctx context.Context) (PruneImageResult, error) {
	report, err := c.cli.ImagesPrune(ctx, filters.NewArgs())
	if err != nil {
		return PruneImageResult{}, fmt.Errorf("pruning images: %w", err)
	}
	return PruneImageResult{
		ImagesDeleted:  len(report.ImagesDeleted),
		SpaceReclaimed: report.SpaceReclaimed,
	}, nil
}

func trimSHAPrefix(id string) string {
	const prefix = "sha256:"
	if len(id) > len(prefix) && id[:len(prefix)] == prefix {
		return id[len(prefix):]
	}
	return id
}

// Package web contains the HTTP layer: routing, middleware, and handlers.
package web

import (
	"context"
	"io"
	"io/fs"
	"log"
	"net/http"

	"github.com/docker/docker/api/types"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"docker-manager/internal/compose"
	"docker-manager/internal/config"
	"docker-manager/internal/docker"
	"docker-manager/internal/store"
	assets "docker-manager/web"
)

const sessionCookieName = "docker_manager_session"

// ContainerService is the subset of Docker container operations the web
// layer needs. Depending on this interface instead of the concrete
// *docker.Client lets handler tests substitute a fake without a live
// Docker daemon; *docker.Client satisfies it structurally.
type ContainerService interface {
	ListContainers(ctx context.Context) ([]docker.ContainerSummary, error)
	GetContainerSummary(ctx context.Context, id string) (docker.ContainerSummary, error)
	ListContainersByComposeProject(ctx context.Context, project string) ([]docker.ContainerSummary, error)
	InspectContainer(ctx context.Context, id string) (types.ContainerJSON, error)
	StartContainer(ctx context.Context, id string) error
	StopContainer(ctx context.Context, id string) error
	RestartContainer(ctx context.Context, id string) error
	RemoveContainer(ctx context.Context, id string, force bool) error
	Logs(ctx context.Context, id string, tail string, follow bool) (io.ReadCloser, error)
	StreamStats(ctx context.Context, id string, fn func(docker.ContainerStats)) error
	SnapshotStats(ctx context.Context, id string) (docker.ContainerStats, error)
}

// ImageService is the subset of Docker image operations the web layer needs.
type ImageService interface {
	ListImages(ctx context.Context) ([]docker.ImageSummary, error)
	PullImage(ctx context.Context, ref string) (io.ReadCloser, error)
	RemoveImage(ctx context.Context, id string, force bool) error
	PruneDanglingImages(ctx context.Context) (docker.PruneImageResult, error)
}

// VolumeService is the subset of Docker volume operations the web layer needs.
type VolumeService interface {
	ListVolumes(ctx context.Context) ([]docker.VolumeSummary, error)
	CreateVolume(ctx context.Context, name string) error
	RemoveVolume(ctx context.Context, name string, force bool) error
}

// NetworkService is the subset of Docker network operations the web layer needs.
type NetworkService interface {
	ListNetworks(ctx context.Context) ([]docker.NetworkSummary, error)
	GetNetwork(ctx context.Context, id string) (docker.NetworkSummary, error)
	CreateNetwork(ctx context.Context, name, driver string) error
	RemoveNetwork(ctx context.Context, id string) error
}

// ExecService is the subset of Docker exec operations the web layer needs to
// drive an interactive terminal inside a container.
type ExecService interface {
	ExecShell(ctx context.Context, containerID string) (string, types.HijackedResponse, error)
	ExecResize(ctx context.Context, execID string, height, width uint) error
}

// SystemService is host-wide Docker information that doesn't belong to a
// single resource type, used to power the dashboard's storage chart.
type SystemService interface {
	DiskUsage(ctx context.Context) (docker.DiskUsageSummary, error)
}

// DockerService is every Docker operation the web layer depends on. Handler
// code always talks to the narrower ContainerService/ImageService/
// VolumeService/NetworkService/ExecService/SystemService interfaces
// (documenting exactly what each handler file needs); this is only the
// umbrella NewServer accepts, since a single *docker.Client backs all six in
// production.
type DockerService interface {
	ContainerService
	ImageService
	VolumeService
	NetworkService
	ExecService
	SystemService
}

// Server holds the dependencies shared by all HTTP handlers.
type Server struct {
	cfg      *config.Config
	docker   ContainerService
	images   ImageService
	volumes  VolumeService
	networks NetworkService
	exec     ExecService
	system   SystemService
	store    *store.Store
	compose  *compose.Deployer
	renderer *Renderer
}

// NewServer wires up a Server and its router. stacksDir is the base
// directory Compose stack files are written under before `docker compose`
// is invoked against them.
func NewServer(cfg *config.Config, dockerClient DockerService, st *store.Store, stacksDir string) (http.Handler, error) {
	renderer, err := NewRenderer()
	if err != nil {
		return nil, err
	}

	s := &Server{
		cfg:      cfg,
		docker:   dockerClient,
		images:   dockerClient,
		volumes:  dockerClient,
		networks: dockerClient,
		exec:     dockerClient,
		system:   dockerClient,
		store:    st,
		compose:  compose.NewDeployer(stacksDir),
		renderer: renderer,
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(securityHeaders)

	staticFS, err := fs.Sub(assets.StaticFS, "static")
	if err != nil {
		log.Fatalf("mounting static assets: %v", err)
	}
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	r.Get("/login", s.handleLoginPage)
	r.Post("/login", s.handleLoginSubmit)

	r.Group(func(r chi.Router) {
		r.Use(s.requireAuth)

		r.Post("/logout", s.handleLogout)

		r.Get("/", s.handleDashboard)
		r.Get("/dashboard/stats/stream", s.handleDashboardStatsStream)

		r.Get("/containers", s.handleContainersList)
		r.Get("/containers/table", s.handleContainersTable)
		r.Get("/containers/{id}", s.handleContainerDetail)
		r.Post("/containers/{id}/start", s.handleContainerStart)
		r.Post("/containers/{id}/stop", s.handleContainerStop)
		r.Post("/containers/{id}/restart", s.handleContainerRestart)
		r.Delete("/containers/{id}", s.handleContainerRemove)
		r.Get("/containers/{id}/logs/stream", s.handleContainerLogsStream)
		r.Get("/containers/{id}/stats/stream", s.handleContainerStatsStream)
		r.Get("/containers/{id}/terminal", s.handleContainerTerminal)

		r.Get("/images", s.handleImagesList)
		r.Post("/images/pull", s.handleImagePull)
		r.Post("/images/prune", s.handleImagePrune)
		r.Delete("/images/{id}", s.handleImageRemove)

		r.Get("/volumes", s.handleVolumesList)
		r.Post("/volumes", s.handleVolumeCreate)
		r.Delete("/volumes/{name}", s.handleVolumeRemove)

		r.Get("/networks", s.handleNetworksList)
		r.Get("/networks/{id}", s.handleNetworkDetail)
		r.Post("/networks", s.handleNetworkCreate)
		r.Delete("/networks/{id}", s.handleNetworkRemove)

		r.Get("/stacks", s.handleStacksList)
		r.Get("/stacks/new", s.handleStackNewPage)
		r.Post("/stacks", s.handleStackCreate)
		r.Get("/stacks/{name}", s.handleStackDetail)
		r.Post("/stacks/{name}/deploy", s.handleStackDeploy)
		r.Post("/stacks/{name}/down", s.handleStackDown)
		r.Post("/stacks/{name}/delete", s.handleStackDelete)
		r.Delete("/stacks/{name}", s.handleStackDelete)
	})

	return r, nil
}

package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"docker-manager/internal/docker"
)

type containersListData struct {
	PageData
	Containers []docker.ContainerSummary
	Error      string
}

func (s *Server) handleContainersList(w http.ResponseWriter, r *http.Request) {
	containers, err := s.docker.ListContainers(r.Context())
	data := containersListData{PageData: s.pageData(r, "containers"), Containers: containers}
	if err != nil {
		data.Error = "Could not reach Docker daemon: " + err.Error()
	}
	s.renderer.Render(w, http.StatusOK, "containers_list", data)
}

// handleContainersTable serves the polled htmx fragment with just the table rows.
func (s *Server) handleContainersTable(w http.ResponseWriter, r *http.Request) {
	containers, err := s.docker.ListContainers(r.Context())
	if err != nil {
		http.Error(w, "listing containers: "+err.Error(), http.StatusBadGateway)
		return
	}
	s.renderer.RenderPartial(w, "containers_list", "container_rows", containersListData{Containers: containers})
}

type containerDetailData struct {
	PageData
	Container containerDetailView
	RawJSON   string
}

type containerDetailView struct {
	ID           string
	Name         string
	Created      string
	RestartCount int
	Config       struct {
		Image string
		Env   []string
	}
	State           struct{ Status string }
	Mounts          []mountView
	NetworkSettings struct {
		Networks map[string]networkView
	}
}

type mountView struct {
	Type        string
	Source      string
	Destination string
	Mode        string
	RW          bool
}

type networkView struct {
	IPAddress string
	Gateway   string
}

func (s *Server) handleContainerDetail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	info, err := s.docker.InspectContainer(r.Context(), id)
	if err != nil {
		http.Error(w, "inspecting container: "+err.Error(), http.StatusNotFound)
		return
	}

	raw, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		http.Error(w, "encoding inspect result: "+err.Error(), http.StatusInternalServerError)
		return
	}

	view := containerDetailView{
		ID:           info.ID,
		Name:         info.Name,
		Created:      info.Created,
		RestartCount: info.RestartCount,
	}
	if info.Config != nil {
		view.Config.Image = info.Config.Image
		view.Config.Env = info.Config.Env
	}
	if info.State != nil {
		view.State.Status = info.State.Status
	}
	for _, m := range info.Mounts {
		view.Mounts = append(view.Mounts, mountView{
			Type:        string(m.Type),
			Source:      m.Source,
			Destination: m.Destination,
			Mode:        m.Mode,
			RW:          m.RW,
		})
	}
	if info.NetworkSettings != nil {
		view.NetworkSettings.Networks = make(map[string]networkView, len(info.NetworkSettings.Networks))
		for name, n := range info.NetworkSettings.Networks {
			view.NetworkSettings.Networks[name] = networkView{IPAddress: n.IPAddress, Gateway: n.Gateway}
		}
	}
	// Prefer the container's display name (as shown in lists) over the raw
	// "/name" form Docker stores internally.
	view.Name = trimLeadingSlash(view.Name)

	s.renderer.Render(w, http.StatusOK, "container_detail", containerDetailData{
		PageData:  s.pageData(r, "containers"),
		Container: view,
		RawJSON:   string(raw),
	})
}

func trimLeadingSlash(s string) string {
	if len(s) > 0 && s[0] == '/' {
		return s[1:]
	}
	return s
}

func (s *Server) handleContainerStart(w http.ResponseWriter, r *http.Request) {
	s.doContainerAction(w, r, s.docker.StartContainer)
}

func (s *Server) handleContainerStop(w http.ResponseWriter, r *http.Request) {
	s.doContainerAction(w, r, s.docker.StopContainer)
}

func (s *Server) handleContainerRestart(w http.ResponseWriter, r *http.Request) {
	s.doContainerAction(w, r, s.docker.RestartContainer)
}

// doContainerAction runs a state-changing action then re-renders the
// container's row so the htmx swap reflects the new state.
func (s *Server) doContainerAction(w http.ResponseWriter, r *http.Request, action func(ctx context.Context, id string) error) {
	id := chi.URLParam(r, "id")

	if err := action(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	summary, err := s.docker.GetContainerSummary(r.Context(), id)
	if errors.Is(err, docker.ErrContainerNotFound) {
		w.WriteHeader(http.StatusOK)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	s.renderer.RenderPartial(w, "containers_list", "container_row", summary)
}

func (s *Server) handleContainerRemove(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// The UI already asks for confirmation (hx-confirm) before sending this
	// request, so force-remove even if the container is still running.
	if err := s.docker.RemoveContainer(r.Context(), id, true); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusOK)
}

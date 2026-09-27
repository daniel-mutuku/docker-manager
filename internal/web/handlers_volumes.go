package web

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"docker-manager/internal/docker"
)

type volumesListData struct {
	PageData
	Volumes []docker.VolumeSummary
	Error   string
}

func (s *Server) handleVolumesList(w http.ResponseWriter, r *http.Request) {
	volumes, err := s.volumes.ListVolumes(r.Context())
	data := volumesListData{PageData: s.pageData(r, "volumes"), Volumes: volumes}
	if err != nil {
		data.Error = "Could not list volumes: " + err.Error()
	}
	s.renderer.Render(w, http.StatusOK, "volumes_list", data)
}

func (s *Server) handleVolumeCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	if err := s.volumes.CreateVolume(r.Context(), name); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	http.Redirect(w, r, "/volumes", http.StatusSeeOther)
}

func (s *Server) handleVolumeRemove(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := s.volumes.RemoveVolume(r.Context(), name, false); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusOK)
}

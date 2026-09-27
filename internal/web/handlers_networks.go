package web

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"docker-manager/internal/docker"
)

type networksListData struct {
	PageData
	Networks []docker.NetworkSummary
	Error    string
}

func (s *Server) handleNetworksList(w http.ResponseWriter, r *http.Request) {
	networks, err := s.networks.ListNetworks(r.Context())
	data := networksListData{PageData: s.pageData(r, "networks"), Networks: networks}
	if err != nil {
		data.Error = "Could not list networks: " + err.Error()
	}
	s.renderer.Render(w, http.StatusOK, "networks_list", data)
}

type networkDetailData struct {
	PageData
	Network docker.NetworkSummary
}

func (s *Server) handleNetworkDetail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	network, err := s.networks.GetNetwork(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	s.renderer.Render(w, http.StatusOK, "network_detail", networkDetailData{
		PageData: s.pageData(r, "networks"),
		Network:  network,
	})
}

func (s *Server) handleNetworkCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	driver := r.FormValue("driver")

	if err := s.networks.CreateNetwork(r.Context(), name, driver); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	http.Redirect(w, r, "/networks", http.StatusSeeOther)
}

func (s *Server) handleNetworkRemove(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.networks.RemoveNetwork(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusOK)
}

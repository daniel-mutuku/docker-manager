package web

import (
	"bufio"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"docker-manager/internal/docker"
)

type imagesListData struct {
	PageData
	Images  []docker.ImageSummary
	Message string
	Error   string
}

func (s *Server) handleImagesList(w http.ResponseWriter, r *http.Request) {
	images, err := s.images.ListImages(r.Context())
	data := imagesListData{PageData: s.pageData(r, "images"), Images: images}
	if err != nil {
		data.Error = "Could not list images: " + err.Error()
	}
	s.renderer.Render(w, http.StatusOK, "images_list", data)
}

// handleImagePull streams `docker pull` progress to the browser as plain
// chunked text, one JSON status line per Docker progress event. This uses a
// POST body read via fetch() on the client rather than SSE/EventSource,
// since pulling is a mutating action that belongs behind POST and
// EventSource only ever issues GET requests.
func (s *Server) handleImagePull(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	ref := r.FormValue("ref")
	if ref == "" {
		http.Error(w, "ref is required", http.StatusBadRequest)
		return
	}

	rc, err := s.images.PullImage(r.Context(), ref)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer rc.Close()

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	scanner := bufio.NewScanner(rc)
	scanner.Buffer(make([]byte, 64*1024), 1<<20)
	for scanner.Scan() {
		if _, err := w.Write(append(scanner.Bytes(), '\n')); err != nil {
			return
		}
		flusher.Flush()
	}
}

func (s *Server) handleImageRemove(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.images.RemoveImage(r.Context(), id, false); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleImagePrune(w http.ResponseWriter, r *http.Request) {
	result, pruneErr := s.images.PruneDanglingImages(r.Context())

	images, err := s.images.ListImages(r.Context())
	data := imagesListData{PageData: s.pageData(r, "images"), Images: images}
	switch {
	case err != nil:
		data.Error = "Could not list images: " + err.Error()
	case pruneErr != nil:
		data.Error = "Prune failed: " + pruneErr.Error()
	default:
		data.Message = fmt.Sprintf("Removed %d dangling image(s), reclaimed %s", result.ImagesDeleted, humanBytes(int64(result.SpaceReclaimed)))
	}
	s.renderer.Render(w, http.StatusOK, "images_list", data)
}

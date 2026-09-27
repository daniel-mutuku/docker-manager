package web

import (
	"bufio"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// handleContainerLogsStream streams a container's log output to the browser
// as Server-Sent Events. The client (plain JS, see web/static/js/logs.js)
// inserts each line as a text node rather than HTML, since log content
// comes from the container itself and must never be treated as trusted
// markup.
func (s *Server) handleContainerLogsStream(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	tail := r.URL.Query().Get("tail")
	if tail == "" {
		tail = "200"
	}
	follow := r.URL.Query().Get("follow") == "true"

	rc, err := s.docker.Logs(r.Context(), id, tail, follow)
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

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	scanner := bufio.NewScanner(rc)
	scanner.Buffer(make([]byte, 64*1024), 1<<20)
	for scanner.Scan() {
		if _, err := fmt.Fprintf(w, "data: %s\n\n", scanner.Text()); err != nil {
			return
		}
		flusher.Flush()
	}
}

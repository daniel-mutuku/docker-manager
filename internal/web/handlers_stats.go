package web

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"docker-manager/internal/docker"
)

// statsEvent is the JSON payload sent per SSE tick; the client (plain JS)
// parses it and updates specific DOM text nodes rather than re-rendering
// HTML, keeping the same "never trust this as markup" posture as logs.
type statsEvent struct {
	CPUPercent    float64 `json:"cpuPercent"`
	MemUsageBytes uint64  `json:"memUsageBytes"`
	MemLimitBytes uint64  `json:"memLimitBytes"`
	MemPercent    float64 `json:"memPercent"`
	NetRxBytes    uint64  `json:"netRxBytes"`
	NetTxBytes    uint64  `json:"netTxBytes"`
}

// handleContainerStatsStream streams live CPU/memory/network stats for a
// container to the browser as Server-Sent Events.
func (s *Server) handleContainerStatsStream(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

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

	err := s.docker.StreamStats(r.Context(), id, func(st docker.ContainerStats) {
		payload, err := json.Marshal(statsEvent{
			CPUPercent:    st.CPUPercent,
			MemUsageBytes: st.MemUsageBytes,
			MemLimitBytes: st.MemLimitBytes,
			MemPercent:    st.MemPercent,
			NetRxBytes:    st.NetRxBytes,
			NetTxBytes:    st.NetTxBytes,
		})
		if err != nil {
			return
		}
		fmt.Fprintf(w, "data: %s\n\n", payload)
		flusher.Flush()
	})
	if err != nil {
		// Headers are already sent for an SSE response, so an HTTP error
		// status is no longer possible; send a distinctly-named event
		// instead ("error" is reserved for EventSource's own connection
		// failures and behaves inconsistently as a custom event name).
		fmt.Fprintf(w, "event: streamerror\ndata: %s\n\n", err.Error())
		flusher.Flush()
	}
}

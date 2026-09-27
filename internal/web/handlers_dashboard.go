package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type dashboardData struct {
	PageData
	RunningCount    int
	StoppedCount    int
	ImageCount      int
	ImagesDiskUsage int64
	Storage         storageBreakdown
}

// storageBreakdown feeds the dashboard's storage chart. Bytes fields are
// zero (and the chart renders empty) if the disk usage call fails, since
// that's a secondary metric next to the container/image counts above.
type storageBreakdown struct {
	ImagesBytes     int64
	ContainersBytes int64
	VolumesBytes    int64
	TotalBytes      int64
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	containers, err := s.docker.ListContainers(r.Context())
	if err != nil {
		http.Error(w, "listing containers: "+err.Error(), http.StatusBadGateway)
		return
	}

	data := dashboardData{PageData: s.pageData(r, "dashboard")}
	for _, c := range containers {
		if c.State == "running" {
			data.RunningCount++
		} else {
			data.StoppedCount++
		}
	}

	// Image listing failing isn't fatal to the dashboard; the container
	// counts above are the more important number, so just show zero here.
	if images, err := s.images.ListImages(r.Context()); err == nil {
		data.ImageCount = len(images)
		for _, img := range images {
			data.ImagesDiskUsage += img.SizeBytes
		}
	}

	if du, err := s.system.DiskUsage(r.Context()); err == nil {
		data.Storage = storageBreakdown{
			ImagesBytes:     du.ImagesBytes,
			ContainersBytes: du.ContainersBytes,
			VolumesBytes:    du.VolumesBytes,
			TotalBytes:      du.ImagesBytes + du.ContainersBytes + du.VolumesBytes,
		}
	}

	s.renderer.Render(w, http.StatusOK, "dashboard", data)
}

// dashboardStatsEvent is the JSON payload sent per SSE tick for the
// dashboard's live CPU/memory/network tiles: totals summed across every
// running container. NetRxBps/NetTxBps are rates (computed from the delta
// between ticks), unlike the cumulative byte counters the per-container
// stats endpoint reports.
type dashboardStatsEvent struct {
	CPUPercent    float64 `json:"cpuPercent"`
	MemUsageBytes uint64  `json:"memUsageBytes"`
	NetRxBps      float64 `json:"netRxBps"`
	NetTxBps      float64 `json:"netTxBps"`
}

// handleDashboardStatsStream streams aggregate, host-wide CPU/memory/network
// usage (summed across every running container) as Server-Sent Events, by
// polling a one-shot stats snapshot per running container every couple of
// seconds rather than holding open one stream per container.
func (s *Server) handleDashboardStatsStream(w http.ResponseWriter, r *http.Request) {
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

	ctx := r.Context()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	var prevRx, prevTx uint64
	var prevAt time.Time
	haveSample := false

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			containers, err := s.docker.ListContainers(ctx)
			if err != nil {
				continue
			}

			var cpu float64
			var mem, rx, tx uint64
			var mu sync.Mutex
			var wg sync.WaitGroup
			for _, c := range containers {
				if c.State != "running" {
					continue
				}
				wg.Add(1)
				go func(id string) {
					defer wg.Done()
					stat, err := s.docker.SnapshotStats(ctx, id)
					if err != nil {
						return
					}
					mu.Lock()
					cpu += stat.CPUPercent
					mem += stat.MemUsageBytes
					rx += stat.NetRxBytes
					tx += stat.NetTxBytes
					mu.Unlock()
				}(c.ID)
			}
			wg.Wait()

			event := dashboardStatsEvent{CPUPercent: cpu, MemUsageBytes: mem}
			now := time.Now()
			if haveSample {
				if elapsed := now.Sub(prevAt).Seconds(); elapsed > 0 {
					event.NetRxBps = byteRate(rx, prevRx, elapsed)
					event.NetTxBps = byteRate(tx, prevTx, elapsed)
				}
			}
			prevRx, prevTx, prevAt, haveSample = rx, tx, now, true

			payload, err := json.Marshal(event)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", payload)
			flusher.Flush()
		}
	}
}

// byteRate computes a bytes/sec rate from two cumulative counter samples,
// treating a decrease (a container restarted, or the running set changed
// between ticks) as no data rather than an underflowed huge number.
func byteRate(current, previous uint64, elapsedSeconds float64) float64 {
	if current < previous {
		return 0
	}
	return float64(current-previous) / elapsedSeconds
}

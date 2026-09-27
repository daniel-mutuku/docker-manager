// Command server runs the Docker Web Manager.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"docker-manager/internal/config"
	"docker-manager/internal/docker"
	"docker-manager/internal/store"
	"docker-manager/internal/web"
)

// sessionCleanupInterval controls how often expired sessions are purged
// from SQLite; sessions have a 24h TTL (store.SessionTTL) so hourly is
// frequent enough without adding meaningful load.
const sessionCleanupInterval = 1 * time.Hour

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	dockerClient, err := docker.New(cfg.DockerHost)
	if err != nil {
		log.Fatalf("docker client: %v", err)
	}
	defer dockerClient.Close()

	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := dockerClient.Ping(pingCtx); err != nil {
		cancel()
		log.Fatalf("cannot reach Docker daemon at %s: %v", cfg.DockerHost, err)
	}
	cancel()

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Run session cleanup in the background and make sure it has actually
	// exited (not just been asked to) before the store closes underneath it.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		st.RunSessionCleanup(ctx, sessionCleanupInterval)
	}()
	defer wg.Wait()

	handler, err := web.NewServer(cfg, dockerClient, st, cfg.StacksDir)
	if err != nil {
		log.Fatalf("server: %v", err)
	}

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		IdleTimeout:       60 * time.Second,
		// No WriteTimeout: a future SSE log-streaming endpoint needs to hold
		// its response open indefinitely. Per-request cancellation there
		// should come from the request context instead.
	}

	go func() {
		log.Printf("listening on %s (docker host: %s)", cfg.ListenAddr, cfg.DockerHost)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}

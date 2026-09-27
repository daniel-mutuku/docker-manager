// Package config loads application configuration from flags and environment
// variables (including a .env file if present).
package config

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

// Config holds all runtime configuration for the server.
type Config struct {
	ListenAddr        string
	DockerHost        string
	DBPath            string
	StacksDir         string
	AdminUsername     string
	AdminPasswordHash string
}

// Load parses flags and environment variables (loading .env first, without
// overriding variables already set in the real environment) into a Config.
func Load() (*Config, error) {
	return load(os.Args[1:])
}

// load does the real work against an explicit argument list and its own
// FlagSet, rather than the package-level flag.CommandLine, so it can be
// called repeatedly (e.g. from tests) without "flag redefined" panics.
func load(args []string) (*Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("loading .env: %w", err)
	}

	fs := flag.NewFlagSet("docker-manager", flag.ContinueOnError)
	cfg := &Config{}

	fs.StringVar(&cfg.ListenAddr, "listen", envOr("LISTEN_ADDR", ":8080"), "address to listen on")
	fs.StringVar(&cfg.DockerHost, "docker-host", envOr("DOCKER_HOST_PATH", "unix:///var/run/docker.sock"), "Docker daemon socket/host")
	fs.StringVar(&cfg.DBPath, "db", envOr("DB_PATH", "data/docker-manager.db"), "path to SQLite database file")
	fs.StringVar(&cfg.StacksDir, "stacks-dir", envOr("STACKS_DIR", "data/stacks"), "directory Compose stack files are written under")
	fs.StringVar(&cfg.AdminUsername, "admin-username", envOr("ADMIN_USERNAME", "admin"), "admin login username")

	adminPassword := fs.String("admin-password", envOr("ADMIN_PASSWORD", ""), "admin login password (plaintext, hashed at startup; prefer ADMIN_PASSWORD_HASH)")
	adminPasswordHash := fs.String("admin-password-hash", envOr("ADMIN_PASSWORD_HASH", ""), "bcrypt hash of the admin password")

	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("parsing flags: %w", err)
	}

	switch {
	case *adminPasswordHash != "":
		cfg.AdminPasswordHash = *adminPasswordHash
	case *adminPassword != "":
		hash, err := bcrypt.GenerateFromPassword([]byte(*adminPassword), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("hashing admin password: %w", err)
		}
		cfg.AdminPasswordHash = string(hash)
	default:
		return nil, fmt.Errorf("no admin password configured: set ADMIN_PASSWORD or ADMIN_PASSWORD_HASH")
	}

	return cfg, nil
}

func envOr(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

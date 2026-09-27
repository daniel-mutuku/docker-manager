package config

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestLoad_RequiresAnAdminPassword(t *testing.T) {
	_, err := load(nil)
	if err == nil {
		t.Fatal("expected an error when neither ADMIN_PASSWORD nor ADMIN_PASSWORD_HASH is set")
	}
}

func TestLoad_HashesAPlaintextPassword(t *testing.T) {
	t.Setenv("ADMIN_PASSWORD", "hunter2")

	cfg, err := load(nil)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.AdminPasswordHash == "hunter2" {
		t.Fatal("plaintext password was stored verbatim instead of being hashed")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(cfg.AdminPasswordHash), []byte("hunter2")); err != nil {
		t.Fatalf("resulting hash does not match the original password: %v", err)
	}
}

func TestLoad_PrecomputedHashWinsOverPlaintext(t *testing.T) {
	precomputed, err := bcrypt.GenerateFromPassword([]byte("hunter2"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("generating test hash: %v", err)
	}
	t.Setenv("ADMIN_PASSWORD", "some-other-password")
	t.Setenv("ADMIN_PASSWORD_HASH", string(precomputed))

	cfg, err := load(nil)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.AdminPasswordHash != string(precomputed) {
		t.Fatal("ADMIN_PASSWORD_HASH should take precedence over ADMIN_PASSWORD")
	}
}

func TestLoad_FlagsOverrideEnv(t *testing.T) {
	t.Setenv("ADMIN_PASSWORD", "hunter2")
	t.Setenv("LISTEN_ADDR", ":9000")

	cfg, err := load([]string{"-listen", ":1234"})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.ListenAddr != ":1234" {
		t.Fatalf("ListenAddr = %q, want the flag value :1234 to win over the env var", cfg.ListenAddr)
	}
}

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("ADMIN_PASSWORD", "hunter2")

	cfg, err := load(nil)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.ListenAddr != ":8080" {
		t.Errorf("ListenAddr = %q, want :8080", cfg.ListenAddr)
	}
	if cfg.AdminUsername != "admin" {
		t.Errorf("AdminUsername = %q, want admin", cfg.AdminUsername)
	}
	if cfg.DockerHost != "unix:///var/run/docker.sock" {
		t.Errorf("DockerHost = %q, want the default socket path", cfg.DockerHost)
	}
}

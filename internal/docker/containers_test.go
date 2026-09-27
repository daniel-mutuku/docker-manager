package docker

import (
	"testing"

	"github.com/docker/docker/api/types"
)

func TestFormatPorts(t *testing.T) {
	tests := []struct {
		name  string
		ports []types.Port
		want  string
	}{
		{"no ports", nil, ""},
		{
			"unpublished port",
			[]types.Port{{PrivatePort: 80, Type: "tcp"}},
			"80/tcp",
		},
		{
			"published without a bind address",
			[]types.Port{{PrivatePort: 80, PublicPort: 8080, Type: "tcp"}},
			"8080->80/tcp",
		},
		{
			"published with a bind address",
			[]types.Port{{IP: "0.0.0.0", PrivatePort: 80, PublicPort: 8080, Type: "tcp"}},
			"0.0.0.0:8080->80/tcp",
		},
		{
			"multiple ports joined with a comma",
			[]types.Port{
				{PrivatePort: 80, Type: "tcp"},
				{PrivatePort: 443, PublicPort: 8443, Type: "tcp"},
			},
			"80/tcp, 8443->443/tcp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatPorts(tt.ports); got != tt.want {
				t.Errorf("formatPorts(%+v) = %q, want %q", tt.ports, got, tt.want)
			}
		})
	}
}

func TestShortID(t *testing.T) {
	tests := []struct {
		id   string
		want string
	}{
		{"", ""},
		{"abc123", "abc123"},
		{"0123456789abcdef", "0123456789ab"},
	}

	for _, tt := range tests {
		if got := shortID(tt.id); got != tt.want {
			t.Errorf("shortID(%q) = %q, want %q", tt.id, got, tt.want)
		}
	}
}

func TestToSummary(t *testing.T) {
	cnt := types.Container{
		ID:      "0123456789abcdef",
		Names:   []string{"/my-app"},
		Image:   "nginx:latest",
		Status:  "Up 5 minutes",
		State:   "running",
		Created: 1700000000,
		Ports:   []types.Port{{PrivatePort: 80, PublicPort: 8080, Type: "tcp"}},
	}

	got := toSummary(cnt)

	want := ContainerSummary{
		ID:        "0123456789abcdef",
		ShortID:   "0123456789ab",
		Name:      "my-app",
		Image:     "nginx:latest",
		Status:    "Up 5 minutes",
		State:     "running",
		Ports:     "8080->80/tcp",
		CreatedAt: 1700000000,
	}

	if got != want {
		t.Errorf("toSummary() = %+v, want %+v", got, want)
	}
}

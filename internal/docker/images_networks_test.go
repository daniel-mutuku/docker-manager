package docker

import (
	"testing"

	"github.com/docker/docker/api/types"
)

func TestTrimSHAPrefix(t *testing.T) {
	tests := []struct{ in, want string }{
		{"sha256:abc123", "abc123"},
		{"abc123", "abc123"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := trimSHAPrefix(tt.in); got != tt.want {
			t.Errorf("trimSHAPrefix(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestToNetworkSummary(t *testing.T) {
	n := types.NetworkResource{
		ID:       "0123456789abcdef",
		Name:     "my-net",
		Driver:   "bridge",
		Scope:    "local",
		Internal: true,
		Containers: map[string]types.EndpointResource{
			"c1": {Name: "web", IPv4Address: "172.17.0.2/16"},
		},
	}

	got := toNetworkSummary(n)

	if got.ShortID != "0123456789ab" {
		t.Errorf("ShortID = %q, want 0123456789ab", got.ShortID)
	}
	if len(got.Containers) != 1 || got.Containers[0].Name != "web" {
		t.Errorf("Containers = %+v, want one entry named web", got.Containers)
	}
	if !got.Internal {
		t.Error("Internal = false, want true")
	}
}

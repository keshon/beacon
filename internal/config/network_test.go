package config_test

import (
	"testing"

	"github.com/keshon/beacon/internal/config"
)

func TestNetworkConfig_NodeDomain(t *testing.T) {
	tests := []struct {
		selfURL string
		want    string
	}{
		{"", ""},
		{"https://beacon.example.com", "beacon.example.com"},
		{"https://beacon.example.com:8443/path", "beacon.example.com"},
		{"beacon2.example.com", "beacon2.example.com"},
	}
	for _, tc := range tests {
		n := config.NetworkConfig{SelfURL: tc.selfURL}
		if got := n.NodeDomain(); got != tc.want {
			t.Fatalf("SelfURL %q: got %q want %q", tc.selfURL, got, tc.want)
		}
	}
}

package config

import "testing"

func TestMergeNetworkConfig_preservesSyncToken(t *testing.T) {
	existing := NetworkConfig{
		Enabled:   true,
		NodeID:    "node-a",
		SyncToken: "secret-token",
	}
	incoming := NetworkConfig{
		Enabled:      true,
		SelfURL:      "https://a.example.com",
		Peers:        []string{"https://b.example.com"},
		SyncInterval: 60,
		SyncToken:    "",
	}
	got := mergeNetworkConfig(existing, incoming)
	if got.SyncToken != "secret-token" {
		t.Fatalf("sync token preserved: got %q", got.SyncToken)
	}
	if got.NodeID != "node-a" {
		t.Fatalf("node id preserved: got %q", got.NodeID)
	}
	if got.SelfURL != "https://a.example.com" {
		t.Fatalf("self url updated: got %q", got.SelfURL)
	}
}

func TestMergeSecrets_networkSyncToken(t *testing.T) {
	existing := &Config{Network: NetworkConfig{SyncToken: "old"}}
	incoming := &Config{Network: NetworkConfig{SyncToken: ""}}
	if err := MergeSecrets(existing, incoming); err != nil {
		t.Fatal(err)
	}
	if existing.Network.SyncToken != "old" {
		t.Fatalf("empty patch keeps token, got %q", existing.Network.SyncToken)
	}
	incoming.Network.SyncToken = "new"
	if err := MergeSecrets(existing, incoming); err != nil {
		t.Fatal(err)
	}
	if existing.Network.SyncToken != "new" {
		t.Fatalf("non-empty patch updates token, got %q", existing.Network.SyncToken)
	}
}

func TestToPublic_redactsSyncToken(t *testing.T) {
	cfg := &Config{Network: NetworkConfig{SyncToken: "secret", Enabled: true}}
	pub := cfg.ToPublic()
	if pub.Network.SyncToken != "" {
		t.Fatalf("public config must not expose sync token")
	}
	if !pub.Secrets.SyncToken {
		t.Fatal("expected secrets.sync_token true")
	}
}

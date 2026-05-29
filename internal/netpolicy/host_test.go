package netpolicy

import "testing"

func TestResolvePublicHost_blocksPrivate(t *testing.T) {
	for _, h := range []string{"127.0.0.1", "10.0.0.1", "192.168.1.1", "localhost"} {
		if err := ResolvePublicHost(h); err == nil {
			t.Fatalf("expected error for %q", h)
		}
	}
	if err := ResolvePublicHost("example.com"); err != nil {
		t.Fatalf("example.com: %v", err)
	}
}

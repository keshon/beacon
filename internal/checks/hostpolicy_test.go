package checks

import "testing"

func TestResolvePublicHost_blocksPrivate(t *testing.T) {
	blocked := []string{"127.0.0.1", "10.0.0.1", "192.168.1.1", "localhost", "169.254.169.254"}
	for _, h := range blocked {
		if err := ResolvePublicHost(h); err == nil {
			t.Fatalf("%q: expected error", h)
		}
	}
	if err := ResolvePublicHost("example.com"); err != nil {
		t.Fatalf("example.com: %v", err)
	}
}

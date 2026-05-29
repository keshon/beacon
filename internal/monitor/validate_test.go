package monitor

import "testing"

func TestValidateTarget_HTTP(t *testing.T) {
	if err := ValidateTarget("http", "https://example.com/path"); err != nil {
		t.Fatalf("valid https: %v", err)
	}
	if err := ValidateTarget("http", "example.com"); err == nil {
		t.Fatal("want error for missing scheme")
	}
	if err := ValidateTarget("http", "ftp://example.com"); err == nil {
		t.Fatal("want error for ftp scheme")
	}
	if err := ValidateTarget("http", "http://127.0.0.1/"); err == nil {
		t.Fatal("want error for loopback")
	}
	if err := ValidateTarget("http", "javascript:alert(1)"); err == nil {
		t.Fatal("want error for javascript scheme")
	}
}

func TestValidateTarget_TCP(t *testing.T) {
	if err := ValidateTarget("tcp", "example.com:5432"); err != nil {
		t.Fatalf("public host: %v", err)
	}
	for _, c := range []string{"127.0.0.1:6379", "[::1]:8080", "localhost:8080"} {
		if err := ValidateTarget("tcp", c); err == nil {
			t.Fatalf("%q: want error for blocked host", c)
		}
	}
	if err := ValidateTarget("tcp", "https://example.com:443"); err == nil {
		t.Fatal("want error for URL scheme")
	}
	if err := ValidateTarget("tcp", "localhost"); err == nil {
		t.Fatal("want error for missing port")
	}
	if err := ValidateTarget("tcp", "host:0"); err == nil {
		t.Fatal("want error for port 0")
	}
}

func TestNormalizeType(t *testing.T) {
	got, err := NormalizeType("")
	if err != nil || got != TypeHTTP {
		t.Fatalf("empty: got %q err %v", got, err)
	}
	if _, err := NormalizeType("ICMP"); err == nil {
		t.Fatal("want error for unknown type")
	}
}

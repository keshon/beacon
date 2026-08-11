package checks

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"testing"
	"time"
)

// The check cannot be pointed at a local server: the network policy blocks
// loopback on purpose, and weakening a guard to make a test pass would be the
// wrong trade. The deadline reader is a pure function of the response instead,
// so the response is what the test builds.

func respWithChain(notAfter ...time.Time) *http.Response {
	certs := make([]*x509.Certificate, 0, len(notAfter))
	for _, t := range notAfter {
		certs = append(certs, &x509.Certificate{NotAfter: t})
	}
	return &http.Response{TLS: &tls.ConnectionState{PeerCertificates: certs}}
}

func TestCertDeadlineTakesTheLeaf(t *testing.T) {
	leaf := time.Date(2026, 8, 20, 6, 14, 0, 0, time.UTC)
	issuer := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)

	got := certDeadline(respWithChain(leaf, issuer))
	if !got.Equal(leaf) {
		t.Fatalf("want the leaf %v, got %v", leaf, got)
	}
}

// Zero must mean "there is no certificate", never "we did not look": the state
// keeps its previous deadline on a zero, and a wrong zero would erase a real
// warning instead of refreshing it.
func TestCertDeadlineIsZeroWithoutTLS(t *testing.T) {
	cases := map[string]*http.Response{
		"no response":                nil,
		"plain http":                 {},
		"handshake, no certificates": {TLS: &tls.ConnectionState{}},
	}
	for name, resp := range cases {
		if got := certDeadline(resp); !got.IsZero() {
			t.Fatalf("%s: want zero, got %v", name, got)
		}
	}
}

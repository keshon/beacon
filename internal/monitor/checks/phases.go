package checks

import (
	"context"
	"crypto/tls"
	"net/http/httptrace"
	"sync"
	"time"
)

// Where a check spends its time.
//
// One number — "397ms" — says a request was slow and nothing about why. DNS
// that takes two seconds, a TLS handshake that takes two seconds and a server
// that thinks for two seconds are three different problems with three different
// owners, and until now they were one number.
//
// The four phases come from httptrace, which the standard library gives for
// free on a request that is being made anyway.
type Phases struct {
	DNS  time.Duration `json:"dns"`
	TCP  time.Duration `json:"tcp"`
	TLS  time.Duration `json:"tls"`
	// Server is the wait from the request being written to the first byte
	// coming back — the part that belongs to the other side.
	Server time.Duration `json:"server"`
}

// Any reports whether anything was measured at all. A cached connection skips
// DNS, TCP and TLS, and a plain HTTP request has no handshake: zeros there are
// facts, not gaps.
func (p Phases) Any() bool {
	return p.DNS > 0 || p.TCP > 0 || p.TLS > 0 || p.Server > 0
}

// tracer collects phase timings for one request.
//
// A request may open several connections — redirects, retries — and the trace
// fires per connection. The LAST one wins: it is the connection that produced
// the response being judged.
type tracer struct {
	mu sync.Mutex

	dnsStart, connStart, tlsStart, wroteAt time.Time
	phases                                 Phases
}

func (t *tracer) trace() *httptrace.ClientTrace {
	return &httptrace.ClientTrace{
		DNSStart: func(httptrace.DNSStartInfo) {
			t.mu.Lock()
			defer t.mu.Unlock()
			t.dnsStart = time.Now()
		},
		DNSDone: func(httptrace.DNSDoneInfo) {
			t.mu.Lock()
			defer t.mu.Unlock()
			if !t.dnsStart.IsZero() {
				t.phases.DNS = time.Since(t.dnsStart)
			}
		},
		ConnectStart: func(string, string) {
			t.mu.Lock()
			defer t.mu.Unlock()
			t.connStart = time.Now()
		},
		ConnectDone: func(_, _ string, err error) {
			t.mu.Lock()
			defer t.mu.Unlock()
			if err == nil && !t.connStart.IsZero() {
				t.phases.TCP = time.Since(t.connStart)
			}
		},
		TLSHandshakeStart: func() {
			t.mu.Lock()
			defer t.mu.Unlock()
			t.tlsStart = time.Now()
		},
		TLSHandshakeDone: func(_ tls.ConnectionState, err error) {
			t.mu.Lock()
			defer t.mu.Unlock()
			if err == nil && !t.tlsStart.IsZero() {
				t.phases.TLS = time.Since(t.tlsStart)
			}
		},
		WroteRequest: func(httptrace.WroteRequestInfo) {
			t.mu.Lock()
			defer t.mu.Unlock()
			t.wroteAt = time.Now()
		},
		GotFirstResponseByte: func() {
			t.mu.Lock()
			defer t.mu.Unlock()
			if !t.wroteAt.IsZero() {
				t.phases.Server = time.Since(t.wroteAt)
			}
		},
	}
}

func (t *tracer) result() Phases {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.phases
}

// withTrace attaches the collector to a context.
func withTrace(ctx context.Context, t *tracer) context.Context {
	return httptrace.WithClientTrace(ctx, t.trace())
}

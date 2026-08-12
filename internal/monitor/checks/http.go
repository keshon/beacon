package checks

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/keshon/beacon/internal/netpolicy"
)

const maxHTTPBodyBytes = 1 << 20 // 1 MiB

type CheckResult struct {
	MonitorID  string
	Success    bool
	StatusCode int
	Latency    time.Duration
	Error      string
	Time       time.Time
	// CertExpiry is when the served TLS certificate stops being valid. Zero for
	// plain HTTP and for checks that never reached a handshake.
	//
	// It costs one field read on a response we already have, and it answers a
	// question nothing else in Beacon could: an expiring certificate takes a
	// site down at a known moment in the future, which is the only kind of
	// outage a monitor can warn about BEFORE it happens.
	CertExpiry time.Time
	// Phases is where the time went. Zero everywhere when nothing was measured
	// — a reused connection, or a failure before the first packet.
	Phases Phases
}

// HTTPOptions holds optional HTTP check settings.
type HTTPOptions struct {
	Username      string `json:"username,omitempty"`
	Password      string `json:"password,omitempty"`
	Keyword       string `json:"keyword,omitempty"`
	KeywordInvert bool   `json:"keyword_invert,omitempty"`
}

// Redacted returns a copy without password.
func (o *HTTPOptions) Redacted() *HTTPOptions {
	if o == nil {
		return nil
	}
	c := *o
	c.Password = ""
	return &c
}

// MergeHTTPOptions applies patch semantics.
func MergeHTTPOptions(existing, incoming *HTTPOptions) *HTTPOptions {
	if incoming == nil {
		return existing
	}
	out := HTTPOptions{}
	if existing != nil {
		out = *existing
	}
	if u := strings.TrimSpace(incoming.Username); u != "" {
		out.Username = u
	}
	if incoming.Password != "" {
		out.Password = incoming.Password
	}
	out.Keyword = strings.TrimSpace(incoming.Keyword)
	out.KeywordInvert = incoming.KeywordInvert
	if out.Username == "" && out.Password == "" && out.Keyword == "" && !out.KeywordInvert {
		return nil
	}
	return &out
}

// HTTPCheck performs a GET request and optionally validates response body.
func HTTPCheck(ctx context.Context, target string, timeout time.Duration, opts *HTTPOptions) CheckResult {
	start := time.Now()
	result := CheckResult{
		MonitorID: "",
		Time:      start,
	}

	u, err := url.Parse(target)
	if err != nil {
		result.Success = false
		result.Error = err.Error()
		return result
	}
	if u.User != nil {
		result.Success = false
		result.Error = "credentials in URL are not allowed; use http options"
		return result
	}
	if err := netpolicy.ResolvePublicHost(u.Hostname()); err != nil {
		result.Success = false
		result.Error = err.Error()
		return result
	}

	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	transport := &http.Transport{
		DialContext: func(dialCtx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			if err := netpolicy.ResolvePublicHost(host); err != nil {
				return nil, err
			}
			d := &net.Dialer{Timeout: timeout}
			return d.DialContext(dialCtx, network, net.JoinHostPort(host, port))
		},
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if err := netpolicy.ResolvePublicHost(req.URL.Hostname()); err != nil {
				return err
			}
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	// httptrace rides along on the request that is being made anyway: the
	// phase timings cost nothing beyond the callbacks.
	tr := &tracer{}
	ctx = withTrace(ctx, tr)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		result.Success = false
		result.Error = err.Error()
		return result
	}
	if opts != nil && strings.TrimSpace(opts.Username) != "" {
		req.SetBasicAuth(opts.Username, opts.Password)
	}

	resp, err := client.Do(req)
	result.Latency = time.Since(start)
	// Collected even on failure: "TLS finished, the server never answered" is
	// the useful half of a timeout.
	result.Phases = tr.result()
	if err != nil {
		result.Success = false
		if ctx.Err() != nil && strings.Contains(err.Error(), "context") {
			result.Error = "check cancelled or timed out"
		} else {
			result.Error = err.Error()
		}
		return result
	}
	defer resp.Body.Close()

	result.CertExpiry = certDeadline(resp)

	result.StatusCode = resp.StatusCode
	result.Success = resp.StatusCode >= 200 && resp.StatusCode < 400
	if !result.Success {
		result.Error = "HTTP " + resp.Status
		return result
	}

	keyword := ""
	invert := false
	if opts != nil {
		keyword = strings.TrimSpace(opts.Keyword)
		invert = opts.KeywordInvert
	}
	if keyword == "" {
		_, _ = io.Copy(io.Discard, resp.Body)
		return result
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxHTTPBodyBytes+1))
	if err != nil {
		result.Success = false
		result.Error = "read body: " + err.Error()
		return result
	}
	if len(body) > maxHTTPBodyBytes {
		result.Success = false
		result.Error = "response body too large"
		return result
	}
	if kwErr := matchHTTPKeyword(body, keyword, invert); kwErr != nil {
		result.Success = false
		result.Error = kwErr.Error()
	}
	return result
}

func matchHTTPKeyword(body []byte, keyword string, invert bool) error {
	if keyword == "" {
		return nil
	}
	contains := bodyContainsKeyword(body, keyword)
	if invert {
		if contains {
			return fmt.Errorf("forbidden keyword found in response")
		}
		return nil
	}
	if !contains {
		return fmt.Errorf("keyword not found in response")
	}
	return nil
}

// bodyContainsKeyword reports whether keyword appears in body.
// Single tokens use exact substring match. Multi-word phrases also match when
// all words appear in order with arbitrary content (whitespace, HTML tags, etc.) between them.
func bodyContainsKeyword(body []byte, keyword string) bool {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return true
	}
	if bytes.Contains(body, []byte(keyword)) {
		return true
	}
	parts := strings.Fields(keyword)
	if len(parts) <= 1 {
		return false
	}
	return bodyContainsWordSequence(body, parts)
}

func bodyContainsWordSequence(body []byte, words []string) bool {
	pos := 0
	for i, word := range words {
		if word == "" {
			continue
		}
		idx := bytes.Index(body[pos:], []byte(word))
		if idx < 0 {
			return false
		}
		wordStart := pos + idx
		if i > 0 && !keywordWordGapOK(body[pos:wordStart]) {
			return false
		}
		pos = wordStart + len(word)
	}
	return true
}

// keywordWordGapOK requires a non-alphanumeric separator between phrase words
// so "a b" does not match inside "abc".
func keywordWordGapOK(gap []byte) bool {
	if len(gap) == 0 {
		return false
	}
	for _, b := range gap {
		if !isASCIILetterOrDigit(b) {
			return true
		}
	}
	return false
}

func isASCIILetterOrDigit(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// certDeadline reads when the served certificate stops being valid.
//
// Zero means there is none to read — plain HTTP, or a response that never came
// from a handshake. It must not mean "we did not look": the caller keeps the
// previous deadline on a zero, and a wrong zero would erase a real warning.
//
// The leaf comes first in the chain and the issuers behind it outlive it by
// construction, so the leaf is the deadline.
func certDeadline(resp *http.Response) time.Time {
	if resp == nil || resp.TLS == nil || len(resp.TLS.PeerCertificates) == 0 {
		return time.Time{}
	}
	return resp.TLS.PeerCertificates[0].NotAfter
}

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
	contains := bytes.Contains(body, []byte(keyword))
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

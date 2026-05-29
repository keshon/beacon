package checks

import (
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

func HTTPCheck(ctx context.Context, target string, timeout time.Duration) CheckResult {
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
	_, _ = io.Copy(io.Discard, resp.Body)

	result.StatusCode = resp.StatusCode
	result.Success = resp.StatusCode >= 200 && resp.StatusCode < 400
	if !result.Success {
		result.Error = "HTTP " + resp.Status
	}
	return result
}

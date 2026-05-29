package checks

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/keshon/beacon/internal/netpolicy"
)

func TCPCheck(ctx context.Context, target string, timeout time.Duration) CheckResult {
	start := time.Now()
	result := CheckResult{
		MonitorID: "",
		Time:      start,
	}

	host, _, err := net.SplitHostPort(target)
	if err != nil {
		result.Success = false
		result.Error = err.Error()
		return result
	}
	if err := netpolicy.ResolvePublicHost(host); err != nil {
		result.Success = false
		result.Error = err.Error()
		return result
	}

	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	d := &net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "tcp", target)
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
	conn.Close()

	result.Success = true
	return result
}

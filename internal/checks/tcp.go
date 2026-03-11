package checks

import (
	"net"
	"time"
)

func TCPCheck(target string, timeout time.Duration) CheckResult {
	start := time.Now()
	result := CheckResult{
		MonitorID: "",
		Time:      start,
	}

	conn, err := net.DialTimeout("tcp", target, timeout)
	result.Latency = time.Since(start)

	if err != nil {
		result.Success = false
		result.Error = err.Error()
		return result
	}
	conn.Close()

	result.Success = true
	return result
}

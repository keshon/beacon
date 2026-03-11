package checks

import (
	"net/http"
	"time"
)

func HTTPCheck(target string, timeout time.Duration) CheckResult {
	start := time.Now()
	result := CheckResult{
		MonitorID: "",
		Time:      start,
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(target)
	result.Latency = time.Since(start)

	if err != nil {
		result.Success = false
		result.Error = err.Error()
		return result
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode
	result.Success = resp.StatusCode >= 200 && resp.StatusCode < 400
	if !result.Success {
		result.Error = "HTTP " + resp.Status
	}
	return result
}

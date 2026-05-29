package monitor

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/keshon/beacon/internal/checks"
)

const (
	TypeHTTP = "http"
	TypeTCP  = "tcp"
)

// NormalizeType returns a supported check type (http or tcp).
func NormalizeType(typ string) (string, error) {
	t := strings.ToLower(strings.TrimSpace(typ))
	if t == "" {
		return TypeHTTP, nil
	}
	if t == TypeHTTP || t == TypeTCP {
		return t, nil
	}
	return "", fmt.Errorf("type must be http or tcp")
}

// ValidateTarget checks target format for the given monitor type.
func ValidateTarget(typ, target string) error {
	t, err := NormalizeType(typ)
	if err != nil {
		return err
	}
	target = strings.TrimSpace(target)
	if target == "" {
		return fmt.Errorf("target is required")
	}
	switch t {
	case TypeHTTP:
		return validateHTTPTarget(target)
	case TypeTCP:
		return validateTCPTarget(target)
	default:
		return fmt.Errorf("type must be http or tcp")
	}
}

func validateHTTPTarget(target string) error {
	u, err := url.Parse(target)
	if err != nil {
		return fmt.Errorf("invalid HTTP URL: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("HTTP target must start with http:// or https://")
	}
	if u.Host == "" {
		return fmt.Errorf("HTTP target must include a host")
	}
	host := u.Hostname()
	if strings.EqualFold(u.Scheme, "javascript") || strings.EqualFold(u.Scheme, "data") {
		return fmt.Errorf("HTTP target scheme is not allowed")
	}
	if err := checks.ResolvePublicHost(host); err != nil {
		return err
	}
	return nil
}

func validateTCPTarget(target string) error {
	if strings.Contains(target, "://") {
		return fmt.Errorf("TCP target must be host:port (no URL scheme)")
	}
	host, portStr, err := net.SplitHostPort(target)
	if err != nil {
		return fmt.Errorf("TCP target must be host:port (e.g. db.local:5432)")
	}
	if strings.TrimSpace(host) == "" {
		return fmt.Errorf("TCP target must include a host")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("TCP port must be between 1 and 65535")
	}
	if err := checks.ResolvePublicHost(host); err != nil {
		return err
	}
	return nil
}

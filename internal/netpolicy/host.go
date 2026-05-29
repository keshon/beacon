package netpolicy

import (
	"fmt"
	"net"
	"strings"
)

// ResolvePublicHost resolves host and rejects addresses that must not be probed.
func ResolvePublicHost(host string) error {
	host = strings.TrimSpace(host)
	if host == "" {
		return fmt.Errorf("host is required")
	}
	if strings.EqualFold(host, "localhost") {
		return fmt.Errorf("target host is not allowed")
	}

	if ip := net.ParseIP(host); ip != nil {
		if isBlockedIP(ip) {
			return fmt.Errorf("target IP is not allowed")
		}
		return nil
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("cannot resolve host: %w", err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("cannot resolve host")
	}
	for _, ip := range ips {
		if isBlockedIP(ip) {
			return fmt.Errorf("target resolves to disallowed address")
		}
	}
	return nil
}

func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	ip = ip.To16()
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		return isBlockedIPv4(v4)
	}
	return false
}

func isBlockedIPv4(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
		return true
	}
	if len(ip) == 4 && ip[0] == 100 && ip[1] >= 64 && ip[1] <= 127 {
		return true
	}
	return false
}

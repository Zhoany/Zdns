package utils

import (
	"net"
	"net/url"
)

// isValidIP returns true if ip is a valid IPv4 or IPv6 address.
func IsValidIP(ip string) bool {
	return net.ParseIP(ip) != nil
}

// isValidUpstream returns true if s parses as a URL with scheme udp|dot|doh
// whose host part is a valid IP and includes a port.
func IsValidUpstream(s string) bool {
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	switch u.Scheme {
	case "udp", "tls", "https":
	default:
		return false
	}
	host := u.Hostname()
	if net.ParseIP(host) == nil {
		return false
	}
	if u.Port() == "" {
		return false
	}
	return true
}
package api

import (
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
)

// ipBlock wraps the whole mux: every request — session, API, health, assets —
// is checked against the configured ip_blocks rules. Blocked requests get a
// 403 page naming the matched rule (never a redirect, never counted).
func (s *Server) ipBlock(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rule := s.ipBlocked(r); rule != "" {
			slog.Info("ip blocked", "remote", r.RemoteAddr, "rule", rule)
			s.renderBlocked(w, r, "ip", rule)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ipBlocked returns the first ip_blocks rule matched by the request's remote
// address, or "" when not blocked. Rules come from config.yaml.
func (s *Server) ipBlocked(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return ""
	}
	for _, rule := range s.cfg.Get().IPBlocks {
		if ipMatches(ip, rule) {
			return rule
		}
	}
	return ""
}

// ipMatches reports whether ip matches a single IP, a CIDR network, or an
// IPv4 dotted-quad rule with "*" segments (e.g. 192.168.*.*).
func ipMatches(ip net.IP, rule string) bool {
	if _, ipnet, err := net.ParseCIDR(rule); err == nil {
		return ipnet.Contains(ip)
	}
	if strings.Contains(rule, "*") {
		return wildcardMatch(ip, rule)
	}
	return ip.Equal(net.ParseIP(rule))
}

// wildcardMatch compares an IPv4 address against a dotted-quad rule where
// each segment is either a literal or "*".
func wildcardMatch(ip net.IP, rule string) bool {
	ip4 := ip.To4()
	if ip4 == nil || strings.Contains(rule, ":") {
		return false
	}
	parts := strings.Split(rule, ".")
	if len(parts) != 4 {
		return false
	}
	for i, part := range parts {
		if part == "*" {
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 || n > 255 || byte(n) != ip4[i] {
			return false
		}
	}
	return true
}

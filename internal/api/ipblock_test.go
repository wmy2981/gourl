package api

import (
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/wmy2981/gourl/internal/counter"
)

func TestIPMatches(t *testing.T) {
	cases := []struct {
		rule string
		ip   string
		want bool
	}{
		// Exact single IP (v4 and v6).
		{"192.168.1.5", "192.168.1.5", true},
		{"192.168.1.5", "192.168.1.6", false},
		{"2001:db8::1", "2001:db8::1", true},
		// CIDR.
		{"10.0.0.0/8", "10.1.2.3", true},
		{"10.0.0.0/8", "11.1.2.3", false},
		{"2001:db8::/32", "2001:db8::42", true},
		// Dotted-quad wildcards.
		{"192.168.*.*", "192.168.0.1", true},
		{"192.168.*.*", "192.168.255.255", true},
		{"192.168.*.*", "192.169.0.1", false},
		{"10.*.5.*", "10.2.5.9", true},
		{"10.*.5.*", "10.2.6.9", false},
		{"*.*.*.*", "1.2.3.4", true},
		// Malformed rules never match.
		{"192.168.*.x", "192.168.0.1", false},
		{"256.*.*.*", "192.168.0.1", false},
	}
	for _, tc := range cases {
		ip := net.ParseIP(tc.ip)
		if ip == nil {
			t.Fatalf("bad test IP %q", tc.ip)
		}
		if got := ipMatches(ip, tc.rule); got != tc.want {
			t.Errorf("ipMatches(%s, %q) = %v, want %v", tc.ip, tc.rule, got, tc.want)
		}
	}
}

// TestIPBlockAllRequests: ip_blocks sit at the outermost wrapper, so a banned
// address is refused on every route — including /api/v1/health and the admin
// login — with a 403 page naming the matched rule.
func TestIPBlockAllRequests(t *testing.T) {
	s, _ := newTestServer(t)
	cfg := s.cfg.Get()
	cfg.IPBlocks = []string{"192.0.2.*", "10.0.0.0/8", "203.0.113.9"}
	if err := s.cfg.Update(cfg); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{"/api/v1/health", "/api/v1/auth/login", "/api/v1/links", "/docs/", "/favicon.svg"} {
		rec := do(t, s, http.MethodGet, path, nil)
		if rec.Code != http.StatusForbidden {
			t.Errorf("GET %s from banned IP: status = %d, want 403", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "192.0.2.*") {
			t.Errorf("GET %s: 403 page must name the matched rule, body: %s", path, rec.Body.String())
		}
	}

	// A non-matching address still works end to end.
	cfg.IPBlocks = []string{"203.0.113.9"}
	if err := s.cfg.Update(cfg); err != nil {
		t.Fatal(err)
	}
	rec := do(t, s, http.MethodGet, "/api/v1/health", nil)
	if rec.Code != http.StatusOK {
		t.Errorf("GET /api/v1/health from allowed IP: status = %d, want 200", rec.Code)
	}
}

func TestIPBlockNotCounted(t *testing.T) {
	s, mr := newTestServer(t)
	createLink(t, s, "abc", "https://example.com/target")

	cfg := s.cfg.Get()
	cfg.IPBlocks = []string{"192.0.2.1"}
	if err := s.cfg.Update(cfg); err != nil {
		t.Fatal(err)
	}

	rec := get(t, s, "/abc", nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("blocked IP status = %d, want 403", rec.Code)
	}
	total, _ := counter.Keys("abc", "")
	if _, err := mr.Get(total); err == nil {
		t.Error("blocked request must not be counted")
	}
}

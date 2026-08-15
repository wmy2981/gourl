package fetcher

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
)

// ErrBlocked signals an SSRF-rejected address.
var ErrBlocked = errors.New("address blocked by ssrf policy")

// validateURL checks that u is an absolute http(s) URL whose resolved
// addresses are all public.
func validateURL(ctx context.Context, rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse url: %w", err)
	}
	if !u.IsAbs() || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("not an absolute http(s) url")
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("missing host")
	}
	if ip := net.ParseIP(host); ip != nil {
		return validateIP(ip)
	}
	// Resolve all addresses and reject if any is non-public (DNS
	// rebinding friendly: a single malicious answer blocks the fetch).
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", host, err)
	}
	if len(addrs) == 0 {
		return fmt.Errorf("resolve %s: no addresses", host)
	}
	for _, a := range addrs {
		if err := validateIP(a.IP); err != nil {
			return fmt.Errorf("%s: %w", host, err)
		}
	}
	return nil
}

// validateIP rejects private, loopback, link-local, multicast, unspecified
// and other non-global-unicast ranges. The cloud metadata address
// 169.254.169.254 falls under link-local unicast.
func validateIP(ip net.IP) error {
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLinkLocalUnicast() {
		return fmt.Errorf("%w: %s", ErrBlocked, ip)
	}
	return nil
}

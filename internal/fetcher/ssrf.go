package fetcher

import (
	"fmt"
	"net/url"
)

// validateFetchURL checks that u is an absolute http(s) URL. Address-level
// SSRF filtering is intentionally disabled: title fetching must work against
// any reachable host, internal or external — the feature is only triggered
// by an authenticated admin on create/edit/import, never by visitors.
func validateFetchURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse url: %w", err)
	}
	if !u.IsAbs() || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("not an absolute http(s) url")
	}
	if u.Hostname() == "" {
		return fmt.Errorf("missing host")
	}
	return nil
}

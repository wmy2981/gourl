package api

import (
	"net/http"
	"strings"

	"github.com/wmy2981/gourl/internal/config"
)

// fullURLs builds every complete short URL for a code: the configured base
// URL (or one inferred from the request Host when unset) plus every extra
// base URL, deduplicated. The trailing slash of base URLs is trimmed.
func fullURLs(cfg *config.Config, r *http.Request, code string) []string {
	bases := make([]string, 0, 1+len(cfg.ExtraBaseURLs))
	base := strings.TrimRight(cfg.BaseURL, "/")
	if base == "" {
		base = inferBaseURL(r)
	}
	bases = append(bases, base)
	for _, extra := range cfg.ExtraBaseURLs {
		extra = strings.TrimRight(extra, "/")
		if extra != "" {
			bases = append(bases, extra)
		}
	}

	seen := make(map[string]bool, len(bases))
	urls := make([]string, 0, len(bases))
	for _, b := range bases {
		if seen[b] {
			continue
		}
		seen[b] = true
		urls = append(urls, b+"/"+code)
	}
	return urls
}

// inferBaseURL derives the site's base URL from the request: X-Forwarded-
// Proto (reverse proxy) or the TLS state decides the scheme, r.Host the
// authority.
func inferBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto == "http" || proto == "https" {
		scheme = proto
	}
	return scheme + "://" + r.Host
}

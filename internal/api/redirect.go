package api

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/wmy2981/gourl/internal/counter"
	"github.com/wmy2981/gourl/internal/shortcode"
	"github.com/wmy2981/gourl/internal/store"
)

// redirect handles GET /{code...}: reserved prefixes win, then UA blocking,
// expiry, click counting, and finally the 302 redirect.
func (s *Server) redirect(w http.ResponseWriter, r *http.Request) {
	code := pathCode(r.PathValue("code"))
	if code == "" {
		// Root path: point at the admin console.
		http.Redirect(w, r, "/admin", http.StatusFound)
		return
	}

	cfg := s.cfg.Get()
	if shortcode.IsReserved(code, cfg.ReservedCodes) {
		slog.Debug("redirect: reserved code", "code", code, "remote", r.RemoteAddr)
		s.renderNotFound(w, r)
		return
	}

	// Shared per-second budget across all short links: over the limit the
	// request is dropped — bare 429, no redirect, no click counted.
	if !s.allowLink() {
		slog.Debug("link rate limited", "code", code, "remote", r.RemoteAddr)
		w.WriteHeader(http.StatusTooManyRequests)
		return
	}

	if p := s.uaBlocked(r); p != "" {
		// Blocked UAs get a 403 page naming the matched pattern and are
		// never counted.
		slog.Info("ua blocked", "code", code, "remote", r.RemoteAddr, "pattern", p)
		s.renderBlocked(w, r, "ua", p)
		return
	}

	link, err := s.store.GetLink(r.Context(), code)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			slog.Debug("redirect: code not found", "code", code, "remote", r.RemoteAddr)
			s.renderNotFound(w, r)
			return
		}
		slog.Error("redirect: lookup failed", "code", code, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	now := s.now()
	if link.ExpiresAt > 0 && link.ExpiresAt < now {
		// Expired codes look like any other missing link: a plain 404.
		slog.Debug("redirect: code expired", "code", code, "remote", r.RemoteAddr)
		s.renderNotFound(w, r)
		return
	}

	// Best-effort counting: a Redis outage must not break redirects.
	date := counter.Date(time.Unix(now, 0))
	if err := s.counter.Incr(r.Context(), code, date); err != nil {
		slog.Warn("count click failed", "code", code, "error", err)
	}

	slog.Debug("link redirected", "code", code, "url", link.URL, "remote", r.RemoteAddr)
	http.Redirect(w, r, link.URL, http.StatusFound)
}

// uaBlocked returns the first UA block pattern matched by the request UA
// (case-insensitive substring match), or "" when not blocked. Patterns come
// from config.yaml.
func (s *Server) uaBlocked(r *http.Request) string {
	ua := strings.ToLower(r.UserAgent())
	if ua == "" {
		return ""
	}
	for _, p := range s.cfg.Get().UABlocks {
		if strings.Contains(ua, strings.ToLower(p)) {
			return p
		}
	}
	return ""
}

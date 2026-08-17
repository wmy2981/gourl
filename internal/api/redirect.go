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
		s.renderNotFound(w, r)
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
			s.renderNotFound(w, r)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	now := s.now()
	if link.ExpiresAt > 0 && link.ExpiresAt < now {
		s.renderExpired(w, r, link)
		return
	}

	// Best-effort counting: a Redis outage must not break redirects.
	date := counter.Date(time.Unix(now, 0))
	if err := s.counter.Incr(r.Context(), code, date); err != nil {
		slog.Warn("count click failed", "code", code, "error", err)
	}

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

package api

import (
	"errors"
	"log"
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

	if blocked, err := s.uaBlocked(r); err != nil {
		log.Printf("ua block check failed: %v", err)
	} else if blocked {
		// Blocked UAs get a bare 403 and are never counted.
		w.WriteHeader(http.StatusForbidden)
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
		log.Printf("count click %s: %v", code, err)
	}

	http.Redirect(w, r, link.URL, http.StatusFound)
}

// uaBlocked reports whether the request UA matches any configured block
// pattern (case-insensitive substring match).
func (s *Server) uaBlocked(r *http.Request) (bool, error) {
	ua := strings.ToLower(r.UserAgent())
	if ua == "" {
		return false, nil
	}
	patterns, err := s.store.ListUABlocks(r.Context())
	if err != nil {
		return false, err
	}
	for _, p := range patterns {
		if strings.Contains(ua, strings.ToLower(p)) {
			return true, nil
		}
	}
	return false, nil
}

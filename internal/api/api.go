// Package api implements the REST API and the short-link redirect routes.
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/wmy2981/gourl/internal/config"
	"github.com/wmy2981/gourl/internal/counter"
	"github.com/wmy2981/gourl/internal/fetcher"
	"github.com/wmy2981/gourl/internal/store"
)

// TitleFetcher retrieves a page's title and description.
type TitleFetcher interface {
	Fetch(ctx context.Context, rawURL string) (title, description string, err error)
}

// envOr returns the environment variable or a default.
func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Server wires handlers to the store, the live configuration, the Redis
// click counter and the title fetcher.
type Server struct {
	store     *store.Store
	cfg       *config.Manager
	counter   *counter.Counter
	fetcher   TitleFetcher
	admin     *adminAuth
	assetsDir string
	startTime time.Time
	now       func() int64 // injectable clock for tests
}

// NewServer creates a Server. Auth settings and the assets directory come
// from the environment.
func NewServer(st *store.Store, cfg *config.Manager, ctr *counter.Counter) *Server {
	return &Server{
		store:     st,
		cfg:       cfg,
		counter:   ctr,
		fetcher:   fetcher.New(fetcher.Options{}),
		admin:     newAdminAuth(os.Getenv("ADMIN_PASSWORD"), os.Getenv("SESSION_SECRET")),
		assetsDir: envOr("ASSETS_DIR", "data/assets"),
		startTime: time.Now(),
		now:       func() int64 { return timeNow() },
	}
}

// Handler returns the root HTTP handler. Explicit routes (api, admin, ...)
// always win over the short-code wildcard.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/auth/login", s.login)
	mux.HandleFunc("POST /api/v1/auth/logout", s.logout)
	mux.HandleFunc("GET /api/v1/health", s.health)
	mux.HandleFunc("GET /api/v1/links", s.requireAuth(s.listLinks))
	mux.HandleFunc("POST /api/v1/links", s.requireAuth(s.createLink))
	mux.HandleFunc("GET /api/v1/links/{code...}", s.requireAuth(s.getLink))
	mux.HandleFunc("PATCH /api/v1/links/{code...}", s.requireAuth(s.updateLink))
	mux.HandleFunc("DELETE /api/v1/links/{code...}", s.requireAuth(s.deleteLink))
	mux.HandleFunc("GET /{code...}", s.redirect)
	return mux
}

// errorBody is the uniform error envelope.
type errorBody struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorBody{Error: apiError{Code: code, Message: message}})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Header already sent; nothing sensible left to do.
		http.Error(w, `{"error":{"code":"internal_error","message":"encode response"}}`, http.StatusInternalServerError)
	}
}

// linkJSON is the wire representation of a link.
type linkJSON struct {
	Code        string `json:"code"`
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description"`
	ExpiresAt   int64  `json:"expires_at"`
	ClickCount  int64  `json:"click_count"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

func toLinkJSON(l *store.Link) linkJSON {
	return linkJSON{
		Code:        l.Code,
		URL:         l.URL,
		Title:       l.Title,
		Description: l.Description,
		ExpiresAt:   l.ExpiresAt,
		ClickCount:  l.ClickCount,
		CreatedAt:   l.CreatedAt,
		UpdatedAt:   l.UpdatedAt,
	}
}

// Package api implements the REST API and the short-link redirect routes.
package api

import (
	"encoding/json"
	"net/http"

	"github.com/wmy2981/gourl/internal/config"
	"github.com/wmy2981/gourl/internal/store"
)

// Server wires handlers to the store and the live configuration.
type Server struct {
	store *store.Store
	cfg   *config.Manager
	now   func() int64 // injectable clock for tests
}

// NewServer creates a Server.
func NewServer(st *store.Store, cfg *config.Manager) *Server {
	return &Server{store: st, cfg: cfg, now: func() int64 { return timeNow() }}
}

// Handler returns the root HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/links", s.listLinks)
	mux.HandleFunc("POST /api/v1/links", s.createLink)
	mux.HandleFunc("GET /api/v1/links/{code...}", s.getLink)
	mux.HandleFunc("PATCH /api/v1/links/{code...}", s.updateLink)
	mux.HandleFunc("DELETE /api/v1/links/{code...}", s.deleteLink)
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

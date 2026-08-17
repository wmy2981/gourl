// Package api implements the REST API and the short-link redirect routes.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wmy2981/gourl/internal/config"
	"github.com/wmy2981/gourl/internal/counter"
	"github.com/wmy2981/gourl/internal/fetcher"
	"github.com/wmy2981/gourl/internal/store"
	"github.com/wmy2981/gourl/internal/webui"
	"golang.org/x/time/rate"
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
	admin      *adminAuth
	loginRate  *loginLimiter
	linkRateMu sync.Mutex
	linkRate   *rate.Limiter
	linkRateN  int
	assetsDir  string
	startTime time.Time
	now       func() int64 // injectable clock for tests
	meta      *metaQueue
}

// NewServer creates a Server. Auth settings and the assets directory come
// from the environment.
func NewServer(st *store.Store, cfg *config.Manager, ctr *counter.Counter) *Server {
	s := &Server{
		store:     st,
		cfg:       cfg,
		counter:   ctr,
		fetcher:   fetcher.New(),
		admin:     resolveAdminAuth(cfg),
		loginRate: newLoginLimiter(),
		assetsDir: envOr("ASSETS_DIR", "./data/assets"),
		startTime: time.Now(),
		now:       func() int64 { return timeNow() },
	}
	// Async meta fetching: workers resolve s.fetcher at job time so tests can
	// swap in a mock after construction.
	s.meta = newMetaQueue(st, func() TitleFetcher { return s.fetcher }, 4)
	return s
}

// Handler returns the root HTTP handler. Explicit routes (api, admin, ...)
// always win over the short-code wildcard.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/auth/login", s.login)
	mux.HandleFunc("POST /api/v1/auth/logout", s.logout)
	mux.HandleFunc("POST /api/v1/auth/setup", s.setupAdmin)
	mux.HandleFunc("GET /api/v1/auth/status", s.authStatus)
	mux.HandleFunc("GET /api/v1/health", s.health)
	mux.HandleFunc("GET /api/v1/links", s.requireAuth(s.listLinks))
	mux.HandleFunc("POST /api/v1/links", s.requireAuth(s.createLink))
	mux.HandleFunc("DELETE /api/v1/links", s.requireAuth(s.deleteLinks))
	mux.HandleFunc("POST /api/v1/links/batch", s.requireAuth(s.batchCreate))
	mux.HandleFunc("GET /api/v1/links/expired", s.requireAuth(s.expiredCount))
	mux.HandleFunc("DELETE /api/v1/links/expired", s.requireAuth(s.deleteExpired))
	mux.HandleFunc("GET /api/v1/links/{code...}", s.requireAuth(s.getLink))
	mux.HandleFunc("PATCH /api/v1/links/{code...}", s.requireAuth(s.updateLink))
	mux.HandleFunc("DELETE /api/v1/links/{code...}", s.requireAuth(s.deleteLink))
	mux.HandleFunc("GET /api/v1/export.csv", s.requireAuth(s.exportCSV))
	mux.HandleFunc("GET /api/v1/export.json", s.requireAuth(s.exportJSON))
	mux.HandleFunc("GET /api/v1/tokens", s.requireAuth(s.listTokens))
	mux.HandleFunc("POST /api/v1/tokens", s.requireAuth(s.createToken))
	mux.HandleFunc("DELETE /api/v1/tokens/{id}", s.requireAuth(s.deleteToken))
	mux.HandleFunc("GET /api/v1/ua-blocks", s.requireAuth(s.listUABlocks))
	mux.HandleFunc("POST /api/v1/ua-blocks", s.requireAuth(s.createUABlock))
	mux.HandleFunc("DELETE /api/v1/ua-blocks/{id}", s.requireAuth(s.deleteUABlock))
	mux.HandleFunc("GET /api/v1/config", s.requireAuth(s.getConfig))
	mux.HandleFunc("PUT /api/v1/config", s.requireAuth(s.updateConfig))
	mux.HandleFunc("POST /api/v1/icon", s.requireAuth(s.uploadIcon))
	mux.HandleFunc("DELETE /api/v1/icon", s.requireAuth(s.deleteIcon))
	mux.HandleFunc("GET /api/v1/dashboard", s.requireAuth(s.dashboard))
	mux.HandleFunc("GET /api/v1/logs", s.requireAuth(s.logHistory))
	mux.HandleFunc("GET /api/v1/logs/stream", s.requireAuth(s.logStream))
	mux.Handle("GET /assets/", s.assetsHandler())
	mux.HandleFunc("GET /favicon.svg", s.favicon)
	mux.Handle("GET /docs/", http.StripPrefix("/docs/", http.FileServer(http.FS(webui.Docs()))))
	mux.HandleFunc("GET /docs/openapi.yaml", s.openAPISpec)
	mux.HandleFunc("GET /admin", s.spaIndex)
	mux.HandleFunc("GET /admin/{path...}", s.spaIndex)
	mux.HandleFunc("GET /{code...}", s.redirect)
	return s.logRequests(s.ipBlock(mux))
}

// logRequests logs every HTTP request with status and latency. Response
// bodies are only mirrored when they are JSON, flattened into key-value
// attrs (error.code=…, error.message=…); HTML pages and other payloads are
// never logged. The level follows the status: >=500 error, >=400 warning
// (every invalid or refused request is logged as a warning), everything else
// debug — access logging sits at debug so the info level carries business
// events only.
func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		// The token-creation response contains the full token (shown exactly
		// once) — never mirror it into the log.
		sw := &statusWriter{
			ResponseWriter: w,
			status:         http.StatusOK,
			capBody:        !(r.Method == http.MethodPost && r.URL.Path == "/api/v1/tokens"),
		}
		next.ServeHTTP(sw, r)
		attrs := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote", r.RemoteAddr,
		}
		if bodyAttrs, ok := parseJSONAttrs(sw.buf.String()); ok {
			attrs = append(attrs, bodyAttrs...)
		}
		switch {
		case sw.status >= http.StatusInternalServerError:
			slog.Error("http request failed", attrs...)
		case sw.status >= http.StatusBadRequest:
			// Every invalid or refused request lands here as a warning: bad
			// payloads, missing auth, unknown codes, rate limits — the JSON
			// attrs carry the API error code and message.
			slog.Warn("http request rejected", attrs...)
		default:
			slog.Debug("http request", attrs...)
		}
	})
}

// maxJSONBodyLog caps how much of a JSON response body is captured for the
// request log. Bodies that do not fit (or do not parse) are dropped silently.
const maxJSONBodyLog = 4096

// statusWriter captures the response status code for request logging and,
// when the response is JSON, a copy of its body. It forwards Flush so
// streaming handlers (the SSE log stream) work through it.
type statusWriter struct {
	http.ResponseWriter
	status  int
	capBody bool
	buf     bytes.Buffer
	stopped bool
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	if w.capBody {
		ct := w.Header().Get("Content-Type")
		// SSE streams are unbounded: stop capturing once the content type says
		// so. Only JSON payloads are mirrored — HTML pages and other content
		// types are never logged.
		if strings.HasPrefix(ct, "text/event-stream") {
			w.stopped = true
		} else if !strings.Contains(ct, "json") {
			w.capBody = false
		}
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(p []byte) (int, error) {
	if w.capBody && !w.stopped {
		if remaining := maxJSONBodyLog - w.buf.Len(); remaining > 0 {
			if len(p) > remaining {
				w.buf.Write(p[:remaining])
			} else {
				w.buf.Write(p)
			}
		}
	}
	return w.ResponseWriter.Write(p)
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// parseJSONAttrs flattens a JSON response body into log key-value attrs:
// nested objects join with dots (error.code), arrays collapse to their item
// count. It returns false for anything that does not parse as a JSON object.
func parseJSONAttrs(s string) ([]any, bool) {
	if s == "" {
		return nil, false
	}
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return nil, false
	}
	obj, ok := v.(map[string]any)
	if !ok {
		return nil, false
	}
	var out []any
	flattenJSON(obj, "", &out)
	return out, true
}

// flattenJSON walks a decoded JSON value, appending "key", value pairs in
// stable (sorted) key order. Maps nest with dots; arrays collapse to their
// item count so huge list payloads stay compact.
func flattenJSON(v any, prefix string, out *[]any) {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			key := k
			if prefix != "" {
				key = prefix + "." + k
			}
			flattenJSON(t[k], key, out)
		}
	case []any:
		*out = append(*out, prefix, fmt.Sprintf("[%d items]", len(t)))
	case nil:
		*out = append(*out, prefix, "null")
	default:
		*out = append(*out, prefix, t)
	}
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

// linkJSON is the wire representation of a link. Short URLs are assembled
// client-side from the config (base_url + extra_base_urls); the API carries
// only the code.
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

package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wmy2981/gourl/internal/config"
	"github.com/wmy2981/gourl/internal/shortcode"
	"github.com/wmy2981/gourl/internal/store"
)

func timeNow() int64 { return time.Now().Unix() }

// maxDescriptionLen caps the description on create/update/import. Counted in
// runes so CJK text gets its full 500 characters, not 500 bytes.
const maxDescriptionLen = 500

// isAbsoluteURL accepts any absolute URL with a non-empty scheme: http(s)
// plus application protocols like tcp:// or openapp://. Only the scheme and
// something after it are required — mailto:user@… has no host.
func isAbsoluteURL(s string) bool {
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	if !u.IsAbs() || u.Scheme == "" {
		return false
	}
	return u.Host != "" || u.Opaque != "" || u.Path != ""
}

// checkDescription validates the description length against maxDescriptionLen.
func checkDescription(description string) (message string, ok bool) {
	if utf8.RuneCountInString(description) > maxDescriptionLen {
		return "description must be at most 500 characters", false
	}
	return "", true
}

// selfLinkTarget reports whether the http(s) target points at this instance's
// own short links: the host:port matches a configured base URL (or the
// request's own host when base_url is unset) and the first path segment is
// not a reserved code — every other path is short-link space, whether or not
// a link with that code exists yet. Non-http(s) targets are never checked.
func selfLinkTarget(cfg *config.Config, r *http.Request, target string) bool {
	u, err := url.Parse(target)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	bases := make([]string, 0, 1+len(cfg.ExtraBaseURLs))
	if cfg.BaseURL != "" {
		bases = append(bases, cfg.BaseURL)
	} else {
		bases = append(bases, inferredBaseURL(r))
	}
	bases = append(bases, cfg.ExtraBaseURLs...)
	hostMatch := false
	for _, b := range bases {
		bu, err := url.Parse(b)
		if err != nil || bu.Host == "" {
			continue
		}
		if hostPort(bu) == hostPort(u) {
			hostMatch = true
			break
		}
	}
	if !hostMatch {
		return false
	}
	seg := strings.TrimPrefix(u.Path, "/")
	if i := strings.IndexByte(seg, '/'); i >= 0 {
		seg = seg[:i]
	}
	return seg != "" && !shortcode.IsReserved(seg, cfg.ReservedCodes)
}

// hostPort resolves the effective port (80 for http, 443 for https) so an
// explicit :80 matches an implied one, while http vs https — different
// ports — never match each other; hostnames compare case-insensitively.
func hostPort(u *url.URL) string {
	h := strings.ToLower(u.Hostname())
	p := u.Port()
	if p == "" {
		if u.Scheme == "https" {
			p = "443"
		} else {
			p = "80"
		}
	}
	return net.JoinHostPort(h, p)
}

// inferredBaseURL derives the site's base URL from the request when the
// config's base_url is unset: X-Forwarded-Proto (reverse proxy) or the TLS
// state decides the scheme, r.Host the authority.
func inferredBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto == "http" || proto == "https" {
		scheme = proto
	}
	return scheme + "://" + r.Host
}

// listLinks handles GET /api/v1/links.
func (s *Server) listLinks(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize == 0 {
		pageSize = 20
	}
	opts := store.ListOptions{
		Query:    r.URL.Query().Get("q"),
		Page:     page,
		PageSize: pageSize,
		Sort:     r.URL.Query().Get("sort"),
		Order:    r.URL.Query().Get("order"),
		Expires:  r.URL.Query().Get("expires"),
		Now:      s.now(),
	}
	links, total, err := s.store.ListLinks(r.Context(), opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to list links")
		return
	}
	slog.Debug("links listed", "total", total, "page", opts.Page, "actor", actorFrom(r))
	out := make([]linkJSON, 0, len(links))
	for i := range links {
		out = append(out, toLinkJSON(&links[i]))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"links":     out,
		"total":     total,
		"page":      opts.Page,
		"page_size": opts.PageSize,
	})
}

// expiryValue accepts either a unix-seconds timestamp (number) or a
// yyyy-mm-dd calendar date string (parsed at local midnight).
type expiryValue int64

func (e *expiryValue) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err == nil {
		i, err := n.Int64()
		if err != nil {
			return fmt.Errorf("expires_at: %w", err)
		}
		*e = expiryValue(i)
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		t, err := time.ParseInLocation("2006-01-02", s, time.Local)
		if err != nil {
			return fmt.Errorf("expires_at must be a unix timestamp or yyyy-mm-dd")
		}
		*e = expiryValue(t.Unix())
		return nil
	}
	return fmt.Errorf("expires_at must be a unix timestamp or yyyy-mm-dd")
}

// createLinkRequest is the POST /api/v1/links body. created_at is honored on
// batch imports only (single creates ignore it); click_count is deliberately
// not accepted anywhere — imports must never fabricate click history.
type createLinkRequest struct {
	URL         string       `json:"url"`
	Code        string       `json:"code"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	ExpiresAt   *expiryValue `json:"expires_at"`
	CreatedAt   *int64       `json:"created_at"`
	// Deleted marks an import item for skipping (re-imported export dumps
	// carry it); single creates ignore it.
	Deleted bool `json:"deleted"`
}

// createLink handles POST /api/v1/links.
func (s *Server) createLink(w http.ResponseWriter, r *http.Request) {
	var req createLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}
	if !isAbsoluteURL(req.URL) {
		writeError(w, http.StatusBadRequest, "invalid_request", "url must be an absolute URL with a scheme")
		return
	}
	if msg, ok := checkDescription(req.Description); !ok {
		writeError(w, http.StatusBadRequest, "description_too_long", msg)
		return
	}
	if req.ExpiresAt != nil && *req.ExpiresAt < 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "expires_at must be >= 0")
		return
	}

	cfg := s.cfg.Get()
	if selfLinkTarget(cfg, r, req.URL) {
		writeError(w, http.StatusBadRequest, "self_link_target", "target URL points at this instance's own short links")
		return
	}
	code := req.Code
	if code == "" {
		generated, err := shortcode.Random(cfg.ShortCodeLength)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to generate code")
			return
		}
		code = generated
	} else {
		if err := shortcode.Validate(code); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_code", err.Error())
			return
		}
		if shortcode.IsReserved(code, cfg.ReservedCodes) {
			writeError(w, http.StatusBadRequest, "reserved_code", "code is a reserved system path")
			return
		}
	}

	now := s.now()
	link := &store.Link{
		Code:        code,
		URL:         req.URL,
		Title:       req.Title,
		Description: req.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if req.ExpiresAt != nil {
		link.ExpiresAt = int64(*req.ExpiresAt)
	}
	if err := s.store.CreateLink(r.Context(), link); err != nil {
		if errors.Is(err, store.ErrTaken) {
			writeError(w, http.StatusConflict, "code_taken", "code is already in use")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to create link")
		return
	}
	// Title/description are fetched in the background so a slow target site
	// never delays the response; the meta lands on a later list refetch.
	s.meta.enqueue(code, req.URL)
	slog.Info("link created", "code", code, "url", req.URL, "actor", actorFrom(r))
	writeJSON(w, http.StatusCreated, toLinkJSON(link))
}

// getLink handles GET /api/v1/links/{code}.
func (s *Server) getLink(w http.ResponseWriter, r *http.Request) {
	link, err := s.store.GetLink(r.Context(), pathCode(r.PathValue("code")))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "link not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to get link")
		return
	}
	slog.Debug("link fetched", "code", link.Code, "actor", actorFrom(r))
	writeJSON(w, http.StatusOK, toLinkJSON(link))
}

// updateLinkRequest is the PATCH /api/v1/links/{code} body. Pointer fields
// distinguish "not provided" from explicit null/empty.
type updateLinkRequest struct {
	Code        *string `json:"code"`
	URL         *string `json:"url"`
	Title       *string `json:"title"`
	Description *string `json:"description"`
	ExpiresAt   *int64  `json:"expires_at"`
}

// updateLink handles PATCH /api/v1/links/{code}.
func (s *Server) updateLink(w http.ResponseWriter, r *http.Request) {
	var req updateLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}
	oldCode := pathCode(r.PathValue("code"))
	link, err := s.store.GetLink(r.Context(), oldCode)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "link not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to get link")
		return
	}
	cfg := s.cfg.Get()

	// Any mutation first snapshots the pre-edit state (old code, old fields,
	// current click count) into the append-only backups table.
	mutating := req.URL != nil || req.Title != nil || req.Description != nil ||
		req.ExpiresAt != nil || (req.Code != nil && *req.Code != oldCode)
	if mutating {
		if _, err := s.store.BackupLink(r.Context(), link, s.now()); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to backup link")
			return
		}
	}

	// Changing the URL re-fetches the title/description (in the background),
	// unless the caller overrides them explicitly in the same request.
	refetchMeta := false
	if req.URL != nil {
		if !isAbsoluteURL(*req.URL) {
			writeError(w, http.StatusBadRequest, "invalid_request", "url must be an absolute URL with a scheme")
			return
		}
		if selfLinkTarget(cfg, r, *req.URL) {
			writeError(w, http.StatusBadRequest, "self_link_target", "target URL points at this instance's own short links")
			return
		}
		link.URL = *req.URL
		refetchMeta = req.Title == nil && req.Description == nil
	}
	if req.Title != nil {
		link.Title = *req.Title
	}
	if req.Description != nil {
		if msg, ok := checkDescription(*req.Description); !ok {
			writeError(w, http.StatusBadRequest, "description_too_long", msg)
			return
		}
		link.Description = *req.Description
	}
	if req.ExpiresAt != nil {
		if *req.ExpiresAt < 0 {
			writeError(w, http.StatusBadRequest, "invalid_request", "expires_at must be >= 0")
			return
		}
		link.ExpiresAt = *req.ExpiresAt
	}
	link.UpdatedAt = s.now()

	if req.Code != nil && *req.Code != oldCode {
		newCode := *req.Code
		if err := shortcode.Validate(newCode); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_code", err.Error())
			return
		}
		if shortcode.IsReserved(newCode, cfg.ReservedCodes) {
			writeError(w, http.StatusBadRequest, "reserved_code", "code is a reserved system path")
			return
		}
		if err := s.store.RenameLink(r.Context(), oldCode, newCode, link.UpdatedAt); err != nil {
			if errors.Is(err, store.ErrTaken) {
				writeError(w, http.StatusConflict, "code_taken", "code is already in use")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to update link")
			return
		}
		link.Code = newCode
	}

	if err := s.store.UpdateLink(r.Context(), link); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "link not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to update link")
		return
	}
	if refetchMeta {
		s.meta.enqueue(link.Code, link.URL)
	}
	slog.Info("link updated", "code", link.Code, "actor", actorFrom(r))
	writeJSON(w, http.StatusOK, toLinkJSON(link))
}

// deleteLink handles DELETE /api/v1/links/{code}.
func (s *Server) deleteLink(w http.ResponseWriter, r *http.Request) {
	code := pathCode(r.PathValue("code"))
	if err := s.store.DeleteLink(r.Context(), code); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "link not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to delete link")
		return
	}
	slog.Info("link deleted", "code", code, "actor", actorFrom(r))
	w.WriteHeader(http.StatusNoContent)
}

// pathCode normalizes a multi-level code captured by {code...}: the wildcard
// preserves leading slashes, strip them for DB lookups.
func pathCode(code string) string { return strings.TrimPrefix(code, "/") }

package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wmy2981/gourl/internal/shortcode"
	"github.com/wmy2981/gourl/internal/store"
)

func timeNow() int64 { return time.Now().Unix() }

// maxDescriptionLen caps the description on create/update/import. Counted in
// runes so CJK text gets its full 500 characters, not 500 bytes.
const maxDescriptionLen = 500

// isAbsoluteHTTPURL rejects anything but http/https absolute URLs.
func isAbsoluteHTTPURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && u.IsAbs() && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// checkDescription validates the description length against maxDescriptionLen.
func checkDescription(description string) (message string, ok bool) {
	if utf8.RuneCountInString(description) > maxDescriptionLen {
		return "description must be at most 500 characters", false
	}
	return "", true
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
}

// createLink handles POST /api/v1/links.
func (s *Server) createLink(w http.ResponseWriter, r *http.Request) {
	var req createLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}
	if !isAbsoluteHTTPURL(req.URL) {
		writeError(w, http.StatusBadRequest, "invalid_request", "url must be an absolute http(s) URL")
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

	// Changing the URL re-fetches the title/description (in the background),
	// unless the caller overrides them explicitly in the same request.
	refetchMeta := false
	if req.URL != nil {
		if !isAbsoluteHTTPURL(*req.URL) {
			writeError(w, http.StatusBadRequest, "invalid_request", "url must be an absolute http(s) URL")
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

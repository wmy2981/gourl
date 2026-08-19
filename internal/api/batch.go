package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/wmy2981/gourl/internal/config"
	"github.com/wmy2981/gourl/internal/shortcode"
	"github.com/wmy2981/gourl/internal/store"
)

// batchLimit caps links per import request.
const batchLimit = 500

// batchCreateRequest is the POST /api/v1/links/batch body. A legacy bare
// array is also accepted (conflict = error).
type batchCreateRequest struct {
	// Conflict policy when an item's code already exists:
	//   error (default) — the item fails with code_taken
	//   skip             — the item is skipped (status "skipped")
	//   update           — the existing link is updated with the item's fields
	Conflict string             `json:"conflict"`
	Items    []createLinkRequest `json:"items"`
}

// batchCreate handles POST /api/v1/links/batch. Each item is validated and
// created independently; failures of one item never fail the others.
func (s *Server) batchCreate(w http.ResponseWriter, r *http.Request) {
	req, err := decodeBatchRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	items := req.Items
	if len(items) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "batch must not be empty")
		return
	}
	if len(items) > batchLimit {
		writeError(w, http.StatusBadRequest, "invalid_request", "batch exceeds 500 items")
		return
	}
	if req.Conflict != "" && req.Conflict != "error" && req.Conflict != "skip" && req.Conflict != "update" {
		writeError(w, http.StatusBadRequest, "invalid_request", "conflict must be error, skip or update")
		return
	}

	cfg := s.cfg.Get()
	now := s.now()
	results := make([]map[string]any, 0, len(items))
	created, failed, skipped, updated := 0, 0, 0, 0
	var failedCodes, skippedCodes, updatedCodes []string

	// Validate every item up front, then insert the valid ones in a single
	// transaction (one commit instead of one per item).
	type pending struct {
		index int // original position in the request, reported to the client
		item  createLinkRequest
		code  string
	}
	var valid []pending
	for i, item := range items {
		res := map[string]any{"index": i, "url": item.URL}
		// Items flagged deleted (e.g. re-imported export dumps) are skipped
		// without touching the database, exactly like an existing-code skip.
		if item.Deleted {
			skipped++
			skippedCodes = append(skippedCodes, item.Code)
			res["status"] = "skipped"
			res["code"] = item.Code
			results = append(results, res)
			continue
		}
		code, verr := s.resolveCode(item, cfg, r)
		if verr != nil {
			failed++
			failedCodes = append(failedCodes, item.Code)
			res["status"] = "error"
			res["error_code"] = verr.code
			res["error_message"] = verr.message
			results = append(results, res)
			continue
		}
		valid = append(valid, pending{index: i, item: item, code: code})
	}

	links := make([]store.Link, len(valid))
	for i, p := range valid {
		links[i] = store.Link{
			Code:        p.code,
			URL:         p.item.URL,
			Title:       p.item.Title,
			Description: p.item.Description,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if p.item.ExpiresAt != nil {
			links[i].ExpiresAt = int64(*p.item.ExpiresAt)
		}
		if p.item.CreatedAt != nil {
			links[i].CreatedAt = *p.item.CreatedAt
		}
	}
	errs, err := s.store.CreateLinks(r.Context(), links)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to create links")
		return
	}
	var firstCode string
	var firstID int64
	for i, err := range errs {
		res := map[string]any{"index": valid[i].index, "url": valid[i].item.URL}
		switch {
		case errors.Is(err, store.ErrTaken) && req.Conflict == "update":
			// Refresh the existing link with the imported fields.
			if uerr := s.applyConflictUpdate(r, valid[i].item, valid[i].code); uerr != nil {
				failed++
				failedCodes = append(failedCodes, valid[i].code)
				res["status"] = "error"
				res["error_code"] = "internal_error"
				res["error_message"] = uerr.Error()
			} else {
				updated++
				updatedCodes = append(updatedCodes, valid[i].code)
				res["status"] = "updated"
				res["code"] = valid[i].code
			}
		case errors.Is(err, store.ErrTaken) && req.Conflict == "skip":
			skipped++
			skippedCodes = append(skippedCodes, valid[i].code)
			res["status"] = "skipped"
			res["code"] = valid[i].code
		case errors.Is(err, store.ErrTaken):
			failed++
			failedCodes = append(failedCodes, valid[i].code)
			res["status"] = "error"
			res["error_code"] = "code_taken"
			res["error_message"] = "code is already in use"
		default:
			created++
			if firstCode == "" {
				firstCode = valid[i].code
				firstID = links[i].ID
			}
			res["status"] = "created"
			res["code"] = valid[i].code
			s.meta.enqueue(valid[i].code, valid[i].item.URL)
		}
		results = append(results, res)
	}
	// The response must echo the request order so the client can map results
	// back to its input lines by index.
	sort.SliceStable(results, func(a, b int) bool {
		return results[a]["index"].(int) < results[b]["index"].(int)
	})

	// Legacy counts stay for compatibility; the per-status lists are what the
	// UI reports ("created N, skipped N, updated N, failed N").
	attrs := []any{"created", created, "skipped", skipped, "updated", updated, "failed", failed}
	attrs = append(attrs, actorAttrs(r)...)
	if firstCode != "" {
		attrs = append(attrs, "first_code", firstCode, "first_id", firstID)
	}
	logInfo(r, "links batch created", attrs...)
	writeJSON(w, http.StatusCreated, map[string]any{
		"results":       results,
		"created":       created,
		"failed":        failed,
		"succeeded":     created,
		"skipped":       skipped,
		"updated":       updated,
		"failed_codes":  failedCodes,
		"skipped_codes": skippedCodes,
		"updated_codes": updatedCodes,
	})
}

// decodeBatchRequest accepts both the current object form
// {"conflict": "...", "items": [...]} and the legacy bare-array form. Items
// are parsed leniently: field names are case-insensitive, unknown fields are
// ignored, numbers/strings coerce, and only a missing url fails the item.
func decodeBatchRequest(r *http.Request) (*batchCreateRequest, error) {
	dec := json.NewDecoder(r.Body)
	dec.UseNumber()
	var raw any
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("invalid JSON body")
	}
	switch v := raw.(type) {
	case []any: // legacy bare-array form
		items, err := lenientItems(v)
		if err != nil {
			return nil, err
		}
		return &batchCreateRequest{Items: items}, nil
	case map[string]any:
		var conflict string
		var arr []any
		for k, val := range v {
			switch strings.ToLower(k) {
			case "conflict":
				conflict, _ = asString(val)
			case "items":
				var ok bool
				arr, ok = val.([]any)
				if !ok {
					return nil, fmt.Errorf("invalid JSON body: items must be an array")
				}
			}
		}
		if arr == nil {
			return nil, fmt.Errorf("invalid JSON body: missing items")
		}
		items, err := lenientItems(arr)
		if err != nil {
			return nil, err
		}
		return &batchCreateRequest{Conflict: strings.ToLower(conflict), Items: items}, nil
	default:
		return nil, fmt.Errorf("invalid JSON body")
	}
}

// lenientItems parses each item through the lenient field mapping.
func lenientItems(arr []any) ([]createLinkRequest, error) {
	items := make([]createLinkRequest, 0, len(arr))
	for i, el := range arr {
		m, ok := el.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("item %d: must be an object", i)
		}
		item, err := lenientItem(m)
		if err != nil {
			return nil, fmt.Errorf("item %d: %w", i, err)
		}
		items = append(items, item)
	}
	return items, nil
}

// lenientItem maps a decoded JSON object onto createLinkRequest: keys are
// matched case-insensitively, unknown keys and nulls are ignored, string and
// number representations coerce where sensible. Only url is required.
func lenientItem(m map[string]any) (createLinkRequest, error) {
	var item createLinkRequest
	for k, v := range m {
		switch strings.ToLower(k) {
		case "url":
			s, err := asString(v)
			if err != nil {
				return item, fmt.Errorf("url must be a string")
			}
			item.URL = s
		case "code":
			s, err := asString(v)
			if err != nil {
				return item, fmt.Errorf("code must be a string")
			}
			item.Code = s
		case "title":
			s, err := asString(v)
			if err != nil {
				return item, fmt.Errorf("title must be a string")
			}
			item.Title = s
		case "description":
			s, err := asString(v)
			if err != nil {
				return item, fmt.Errorf("description must be a string")
			}
			item.Description = s
		case "expires_at":
			ev, err := asExpiry(v)
			if err != nil {
				return item, err
			}
			item.ExpiresAt = &ev
		case "created_at":
			n, err := asInt64(v)
			if err != nil {
				return item, fmt.Errorf("created_at must be a number")
			}
			item.CreatedAt = &n
		case "click_count":
			// Deliberately dropped: imports must never fabricate click history.
		case "deleted":
			b, err := asBool(v)
			if err != nil {
				return item, fmt.Errorf("deleted must be a boolean")
			}
			item.Deleted = b
		}
	}
	if item.URL == "" {
		return item, errors.New("url is required")
	}
	return item, nil
}

// asString coerces JSON scalars to string; null yields "".
func asString(v any) (string, error) {
	switch t := v.(type) {
	case string:
		return t, nil
	case json.Number:
		return t.String(), nil
	case nil:
		return "", nil
	default:
		return "", fmt.Errorf("must be a string")
	}
}

// asInt64 coerces a JSON number (or its string form) to int64; null yields 0.
func asInt64(v any) (int64, error) {
	switch t := v.(type) {
	case json.Number:
		return t.Int64()
	case string:
		n, err := strconv.ParseInt(t, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("must be a number")
		}
		return n, nil
	case nil:
		return 0, nil
	default:
		return 0, fmt.Errorf("must be a number")
	}
}

// asBool coerces booleans, "true"/"false" strings and 0/1 numbers; null
// yields false.
func asBool(v any) (bool, error) {
	switch t := v.(type) {
	case bool:
		return t, nil
	case string:
		b, err := strconv.ParseBool(t)
		if err != nil {
			return false, fmt.Errorf("must be a boolean")
		}
		return b, nil
	case json.Number:
		if t.String() == "1" {
			return true, nil
		}
		if t.String() == "0" {
			return false, nil
		}
		return false, fmt.Errorf("must be a boolean")
	case nil:
		return false, nil
	default:
		return false, fmt.Errorf("must be a boolean")
	}
}

// asExpiry accepts a unix-seconds number, a yyyy-mm-dd calendar date (local
// midnight), an RFC3339 timestamp, or the string form of any of those.
func asExpiry(v any) (expiryValue, error) {
	var zero expiryValue
	switch t := v.(type) {
	case json.Number:
		n, err := t.Int64()
		if err != nil {
			return zero, fmt.Errorf("expires_at must be a unix timestamp or date string")
		}
		return expiryValue(n), nil
	case string:
		if n, err := strconv.ParseInt(t, 10, 64); err == nil {
			return expiryValue(n), nil
		}
		if ts, err := time.ParseInLocation("2006-01-02", t, time.Local); err == nil {
			return expiryValue(ts.Unix()), nil
		}
		if ts, err := time.Parse(time.RFC3339, t); err == nil {
			return expiryValue(ts.Unix()), nil
		}
		return zero, fmt.Errorf("expires_at must be a unix timestamp, yyyy-mm-dd or RFC3339")
	case nil:
		return zero, nil
	default:
		return zero, fmt.Errorf("expires_at must be a unix timestamp or date string")
	}
}

// applyConflictUpdate merges an imported item into the existing link. The
// pre-edit snapshot is backed up first, like a manual edit.
func (s *Server) applyConflictUpdate(r *http.Request, item createLinkRequest, code string) error {
	link, err := s.store.GetLink(r.Context(), code)
	if err != nil {
		return err
	}
	if _, err := s.store.BackupLink(r.Context(), link, s.now()); err != nil {
		return err
	}
	link.URL = item.URL
	if item.Title != "" {
		link.Title = item.Title
	}
	if item.Description != "" {
		link.Description = item.Description
	}
	if item.ExpiresAt != nil {
		link.ExpiresAt = int64(*item.ExpiresAt)
	}
	if item.CreatedAt != nil {
		link.CreatedAt = *item.CreatedAt
	}
	link.UpdatedAt = s.now()
	return s.store.UpdateLink(r.Context(), link)
}

// codeError carries a per-item validation error.
type codeError struct {
	code    string
	message string
}

// resolveCode validates the item and returns the code to use (custom or
// generated). It does not check DB uniqueness, which is handled per insert.
func (s *Server) resolveCode(item createLinkRequest, cfg *config.Config, r *http.Request) (string, *codeError) {
	if !isAbsoluteURL(item.URL) {
		return "", &codeError{"invalid_request", "url must be an absolute URL with a scheme"}
	}
	if selfLinkTarget(cfg, r, item.URL) {
		return "", &codeError{"self_link_target", "target URL points at this instance's own short links"}
	}
	if msg, ok := checkDescription(item.Description); !ok {
		return "", &codeError{"description_too_long", msg}
	}
	if item.ExpiresAt != nil && *item.ExpiresAt < 0 {
		return "", &codeError{"invalid_request", "expires_at must be >= 0"}
	}
	if item.Code == "" {
		generated, err := shortcode.Random(cfg.ShortCodeLength)
		if err != nil {
			return "", &codeError{"internal_error", "failed to generate code"}
		}
		return generated, nil
	}
	if err := shortcode.Validate(item.Code); err != nil {
		return "", &codeError{"invalid_code", err.Error()}
	}
	if shortcode.IsReserved(item.Code, cfg.ReservedCodes) {
		return "", &codeError{"reserved_code", "code is a reserved system path"}
	}
	return item.Code, nil
}

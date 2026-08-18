package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

	"github.com/wmy2981/gourl/internal/config"
	"github.com/wmy2981/gourl/internal/counter"
	"github.com/wmy2981/gourl/internal/store"
)

// enterSetupMode clears the admin password in the config so the server runs
// as before the first setup; the per-request adminAuth() resolution picks the
// change up immediately.
func enterSetupMode(t *testing.T, s *Server) {
	t.Helper()
	upd := s.cfg.Get()
	upd.PasswordHash = ""
	if err := s.cfg.Update(upd); err != nil {
		t.Fatalf("clear admin password: %v", err)
	}
}

func newTestServer(t *testing.T) (*Server, *miniredis.Miniredis) {
	t.Helper()
	// Point the setup-code file at a temp dir: NewServer persists the code
	// next to the database path, and tests must never write ./data/setup.code.
	t.Setenv("DB_PATH", filepath.Join(t.TempDir(), "gourl.db"))
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	cfgMgr, err := config.NewManager(filepath.Join(t.TempDir(), "config.yaml"))
	if err != nil {
		t.Fatalf("config: %v", err)
	}

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })

	srv := NewServer(st, cfgMgr, counter.NewFromClient(rdb))
	srv.now = func() int64 { return 1700000000 }
	// Seed the config with the test password hash: adminAuth() re-resolves
	// from the config whenever the hash changes, so the file and srv.admin
	// must share the exact same hash string (bcrypt salts are random, so two
	// independent generations never match) for sessions to keep authenticating.
	hash, _ := bcrypt.GenerateFromPassword([]byte("test-password"), bcrypt.MinCost)
	upd := cfgMgr.Get()
	upd.PasswordHash = string(hash)
	if err := cfgMgr.Update(upd); err != nil {
		t.Fatalf("seed admin password: %v", err)
	}
	srv.admin = newAdminAuth(string(hash), "test-secret")
	srv.assetsDir = t.TempDir()
	if testSession == nil {
		tok, err := srv.admin.issueToken(10080*time.Minute, 0)
		if err != nil {
			t.Fatalf("issue test session: %v", err)
		}
		testSession = &http.Cookie{Name: sessionCookie, Value: tok}
	}
	// Tests must never hit the network: default to a fetcher that fails
	// quietly, matching the "no title available" path.
	srv.fetcher = &mockFetcher{err: errNoFetch}
	return srv, mr
}

var errNoFetch = errors.New("no network in tests")

// testSession is a valid session cookie shared by all test servers (they
// share the same test admin credentials), attached by do().
var testSession *http.Cookie

// bodyReader marshals a test body into a reader.
func bodyReader(t *testing.T, body any) *bytes.Reader {
	t.Helper()
	if body == nil {
		return bytes.NewReader(nil)
	}
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(data)
}

func do(t *testing.T, s *Server, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bodyReader(t, body))
	req.Header.Set("Content-Type", "application/json")
	if testSession != nil {
		req.AddCookie(testSession)
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func decodeError(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var eb errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &eb); err != nil {
		t.Fatalf("decode error body %q: %v", rec.Body.String(), err)
	}
	return eb.Error.Code
}

func TestCreateAndGetLink(t *testing.T) {
	s, _ := newTestServer(t)
	rec := do(t, s, http.MethodPost, "/api/v1/links", map[string]any{
		"url": "https://example.com/very/long/path",
		"code": "abc",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body %s", rec.Code, rec.Body.String())
	}
	var l linkJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &l); err != nil {
		t.Fatal(err)
	}
	if l.Code != "abc" || l.URL != "https://example.com/very/long/path" || l.CreatedAt != 1700000000 {
		t.Errorf("unexpected link: %+v", l)
	}
	if l.ID <= 0 {
		t.Errorf("link id = %d, want a positive autoincrement id", l.ID)
	}

	rec = do(t, s, http.MethodGet, "/api/v1/links/abc", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get status = %d", rec.Code)
	}
}

func TestCreateWithRandomCode(t *testing.T) {
	s, _ := newTestServer(t)
	rec := do(t, s, http.MethodPost, "/api/v1/links", map[string]any{"url": "https://example.com/x"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	var l linkJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &l); err != nil {
		t.Fatal(err)
	}
	if len(l.Code) != 6 {
		t.Errorf("random code length = %d, want 6", len(l.Code))
	}
}

func TestCreateMultiLevelCode(t *testing.T) {
	s, _ := newTestServer(t)
	rec := do(t, s, http.MethodPost, "/api/v1/links", map[string]any{
		"url":  "https://example.com/deep",
		"code": "link1/link2",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	// The multi-level code is addressable through the API path.
	rec = do(t, s, http.MethodGet, "/api/v1/links/link1/link2", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get multi-level status = %d, body %s", rec.Code, rec.Body.String())
	}
}

func TestCreateRejectsInvalidInputs(t *testing.T) {
	s, _ := newTestServer(t)
	cases := []struct {
		name string
		body map[string]any
		code string
	}{
		{"missing url", map[string]any{"code": "abc"}, "invalid_request"},
		{"relative url", map[string]any{"url": "example.com/x", "code": "abc"}, "invalid_request"},
		{"scheme-less", map[string]any{"url": ":x", "code": "abc"}, "invalid_request"},
		{"negative expires", map[string]any{"url": "https://e.com/x", "code": "abc", "expires_at": -1}, "invalid_request"},
		{"reserved code", map[string]any{"url": "https://e.com/x", "code": "api"}, "reserved_code"},
		{"reserved multi-level", map[string]any{"url": "https://e.com/x", "code": "admin/dashboard"}, "reserved_code"},
		{"invalid code chars", map[string]any{"url": "https://e.com/x", "code": "a b"}, "invalid_code"},
		{"too many segments", map[string]any{"url": "https://e.com/x", "code": "a/b/c/d/e/f"}, "invalid_code"},
		{"malformed json", map[string]any{}, "invalid_request"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(t, s, http.MethodPost, "/api/v1/links", tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body %s)", rec.Code, rec.Body.String())
			}
			if got := decodeError(t, rec); got != tc.code {
				t.Errorf("error code = %q, want %q", got, tc.code)
			}
		})
	}
	// malformed JSON can't be marshaled by do(); send it raw.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/links", strings.NewReader("{not json"))
	req.Header.Set("Content-Type", "application/json")
	if testSession != nil {
		req.AddCookie(testSession)
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || decodeError(t, rec) != "invalid_request" {
		t.Errorf("malformed json: status %d, code %q", rec.Code, decodeError(t, rec))
	}
}

func TestCreateDuplicateCode(t *testing.T) {
	s, _ := newTestServer(t)
	body := map[string]any{"url": "https://example.com/x", "code": "dup"}
	if rec := do(t, s, http.MethodPost, "/api/v1/links", body); rec.Code != http.StatusCreated {
		t.Fatalf("first create: %d", rec.Code)
	}
	rec := do(t, s, http.MethodPost, "/api/v1/links", body)
	if rec.Code != http.StatusConflict || decodeError(t, rec) != "code_taken" {
		t.Errorf("duplicate: status %d, code %q", rec.Code, decodeError(t, rec))
	}
}

func TestUpdateLink(t *testing.T) {
	s, _ := newTestServer(t)
	do(t, s, http.MethodPost, "/api/v1/links", map[string]any{"url": "https://old.com/x", "code": "abc"})

	rec := do(t, s, http.MethodPatch, "/api/v1/links/abc", map[string]any{
		"url":         "https://new.com/y",
		"title":       "New Title",
		"expires_at":  1750000000,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("patch status = %d, body %s", rec.Code, rec.Body.String())
	}
	var l linkJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &l); err != nil {
		t.Fatal(err)
	}
	if l.URL != "https://new.com/y" || l.Title != "New Title" || l.ExpiresAt != 1750000000 {
		t.Errorf("patch result: %+v", l)
	}
}

func TestUpdateLinkCode(t *testing.T) {
	s, _ := newTestServer(t)
	do(t, s, http.MethodPost, "/api/v1/links", map[string]any{"url": "https://e.com/x", "code": "old"})

	rec := do(t, s, http.MethodPatch, "/api/v1/links/old", map[string]any{"code": "new"})
	if rec.Code != http.StatusOK {
		t.Fatalf("rename status = %d, body %s", rec.Code, rec.Body.String())
	}
	if do(t, s, http.MethodGet, "/api/v1/links/new", nil).Code != http.StatusOK {
		t.Error("new code not found after rename")
	}
	if do(t, s, http.MethodGet, "/api/v1/links/old", nil).Code != http.StatusNotFound {
		t.Error("old code still resolves after rename")
	}

	// Renaming onto an existing code conflicts.
	do(t, s, http.MethodPost, "/api/v1/links", map[string]any{"url": "https://e.com/x", "code": "taken"})
	rec = do(t, s, http.MethodPatch, "/api/v1/links/new", map[string]any{"code": "taken"})
	if rec.Code != http.StatusConflict || decodeError(t, rec) != "code_taken" {
		t.Errorf("rename conflict: status %d, code %q", rec.Code, decodeError(t, rec))
	}
}

func TestDeleteLink(t *testing.T) {
	s, _ := newTestServer(t)
	do(t, s, http.MethodPost, "/api/v1/links", map[string]any{"url": "https://e.com/x", "code": "abc"})
	if rec := do(t, s, http.MethodDelete, "/api/v1/links/abc", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d", rec.Code)
	}
	if rec := do(t, s, http.MethodGet, "/api/v1/links/abc", nil); rec.Code != http.StatusNotFound {
		t.Errorf("get after delete = %d", rec.Code)
	}
	if rec := do(t, s, http.MethodDelete, "/api/v1/links/abc", nil); rec.Code != http.StatusNotFound {
		t.Errorf("delete missing = %d", rec.Code)
	}
}

func TestListLinks(t *testing.T) {
	s, _ := newTestServer(t)
	for _, code := range []string{"aaa", "bbb", "ccc"} {
		do(t, s, http.MethodPost, "/api/v1/links", map[string]any{
			"url":  "https://example.com/" + code,
			"code": code,
		})
	}
	rec := do(t, s, http.MethodGet, "/api/v1/links?page_size=2&page=1", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d", rec.Code)
	}
	var body struct {
		Links    []linkJSON `json:"links"`
		Total    int        `json:"total"`
		PageSize int        `json:"page_size"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Total != 3 || len(body.Links) != 2 || body.PageSize != 2 {
		t.Errorf("list result: total %d, %d rows, page_size %d", body.Total, len(body.Links), body.PageSize)
	}
	if body.Links[0].Code != "ccc" { // newest first by default
		t.Errorf("first row = %s, want ccc", body.Links[0].Code)
	}

	rec = do(t, s, http.MethodGet, "/api/v1/links?q=bb", nil)
	var search struct {
		Links []linkJSON `json:"links"`
		Total int        `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &search); err != nil {
		t.Fatal(err)
	}
	if search.Total != 1 || search.Links[0].Code != "bbb" {
		t.Errorf("search result: %+v", search)
	}
}

func TestNotFoundAndReservedPrefix(t *testing.T) {
	s, _ := newTestServer(t)
	// GET /api/v1/links/anything-unknown → 404
	if rec := do(t, s, http.MethodGet, "/api/v1/links/nope", nil); rec.Code != http.StatusNotFound {
		t.Errorf("missing link status = %d", rec.Code)
	}
}

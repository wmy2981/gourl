package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// doWith performs a request with an optional session cookie and bearer token.
func doWith(t *testing.T, s *Server, method, path string, body any, cookie *http.Cookie, bearer string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bodyReader(t, body))
	req.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func loginAs(t *testing.T, s *Server, password string) *http.Cookie {
	t.Helper()
	rec := do(t, s, http.MethodPost, "/api/v1/auth/login", map[string]any{"password": password})
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, body %s", rec.Code, rec.Body.String())
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie {
			return c
		}
	}
	t.Fatal("login succeeded but no session cookie was set")
	return nil
}

func TestLoginSuccessSetsSessionCookie(t *testing.T) {
	s, _ := newTestServer(t)
	cookie := loginAs(t, s, "test-password")
	if cookie.HttpOnly != true || cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie flags: HttpOnly=%v SameSite=%v", cookie.HttpOnly, cookie.SameSite)
	}
	if cookie.MaxAge != int(sessionTTL.Seconds()) {
		t.Errorf("MaxAge = %d, want %d", cookie.MaxAge, int(sessionTTL.Seconds()))
	}
}

func TestLoginWrongPassword(t *testing.T) {
	s, _ := newTestServer(t)
	rec := do(t, s, http.MethodPost, "/api/v1/auth/login", map[string]any{"password": "wrong"})
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestLoginWhenAuthDisabled(t *testing.T) {
	s, _ := newTestServer(t)
	s.admin = newAdminAuth("", "secret") // trusted-network mode
	rec := do(t, s, http.MethodPost, "/api/v1/auth/login", map[string]any{"password": "x"})
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestProtectedRoutesRequireAuth(t *testing.T) {
	s, _ := newTestServer(t)
	// Links CRUD is now gated.
	for _, tc := range []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, "/api/v1/links", nil},
		{http.MethodPost, "/api/v1/links", map[string]any{"url": "https://e.com/x"}},
		{http.MethodGet, "/api/v1/links/abc", nil},
	} {
		rec := doWith(t, s, tc.method, tc.path, tc.body, nil, "")
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: status = %d, want 401", tc.method, tc.path, rec.Code)
		}
	}
	// A tampered cookie is rejected.
	rec := doWith(t, s, http.MethodGet, "/api/v1/links", nil,
		&http.Cookie{Name: sessionCookie, Value: "garbage.token.value"}, "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("tampered cookie: status = %d, want 401", rec.Code)
	}
}

func TestSessionAuthenticates(t *testing.T) {
	s, _ := newTestServer(t)
	cookie := loginAs(t, s, "test-password")
	rec := doWith(t, s, http.MethodGet, "/api/v1/links", nil, cookie, "")
	if rec.Code != http.StatusOK {
		t.Errorf("authenticated list status = %d, body %s", rec.Code, rec.Body.String())
	}
}

func TestBearerTokenAuthenticates(t *testing.T) {
	s, _ := newTestServer(t)
	ctx := context.Background()
	if _, err := s.store.CreateToken(ctx, "test-token-123", "ci", s.now()); err != nil {
		t.Fatal(err)
	}
	rec := doWith(t, s, http.MethodGet, "/api/v1/links", nil, nil, "test-token-123")
	if rec.Code != http.StatusOK {
		t.Errorf("bearer auth status = %d, body %s", rec.Code, rec.Body.String())
	}
	// Unknown token rejected.
	rec = doWith(t, s, http.MethodGet, "/api/v1/links", nil, nil, "nope")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("bad bearer status = %d, want 401", rec.Code)
	}
}

func TestSetupModeRefusesAdminAPI(t *testing.T) {
	s, _ := newTestServer(t)
	s.admin = newAdminAuth("", "test-secret") // setup mode

	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/links"},
		{http.MethodPost, "/api/v1/links"},
		{http.MethodGet, "/api/v1/tokens"},
		{http.MethodGet, "/api/v1/config"},
		{http.MethodGet, "/api/v1/logs"},
	} {
		rec := doWith(t, s, tc.method, tc.path, nil, nil, "")
		if rec.Code != http.StatusForbidden || decodeError(t, rec) != "setup_required" {
			t.Errorf("%s %s: status = %d, code = %q, want 403 setup_required", tc.method, tc.path, rec.Code, decodeError(t, rec))
		}
	}
}

func TestSetupModeAllowsBearerToken(t *testing.T) {
	s, _ := newTestServer(t)
	s.admin = newAdminAuth("", "test-secret") // setup mode
	ctx := context.Background()
	if _, err := s.store.CreateToken(ctx, "setup-token-1", "ci", s.now()); err != nil {
		t.Fatal(err)
	}
	rec := doWith(t, s, http.MethodGet, "/api/v1/links", nil, nil, "setup-token-1")
	if rec.Code != http.StatusOK {
		t.Errorf("setup mode bearer status = %d, want 200", rec.Code)
	}
}

func TestEphemeralSecretGeneratedWhenUnset(t *testing.T) {
	a1 := newAdminAuth("hash-1", "")
	a2 := newAdminAuth("hash-2", "")
	if string(a1.secret) != string(a2.secret) {
		t.Error("ephemeral secret must stay stable within the process")
	}
	if len(a1.secret) != 64 { // 32 random bytes, hex-encoded
		t.Errorf("ephemeral secret length = %d, want 64 hex chars", len(a1.secret))
	}
	if string(a1.secret) == "insecure-dev-secret" {
		t.Error("ephemeral secret must not fall back to the old hardcoded value")
	}
}

func TestLogoutClearsCookie(t *testing.T) {
	s, _ := newTestServer(t)
	cookie := loginAs(t, s, "test-password")
	rec := doWith(t, s, http.MethodPost, "/api/v1/auth/logout", nil, cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("logout status = %d", rec.Code)
	}
	var cleared bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Error("logout did not clear the session cookie")
	}
}

func TestHealthPublicAndReportsIdentity(t *testing.T) {
	s, _ := newTestServer(t)
	rec := do(t, s, http.MethodGet, "/api/v1/health", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("health status = %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"name", "version", "start_time", "uptime", "redis", "sqlite"} {
		if _, ok := body[key]; !ok {
			t.Errorf("health missing key %q", key)
		}
	}
}

func TestHealthFailsWhenDependenciesDown(t *testing.T) {
	s, mr := newTestServer(t)
	mr.Close() // redis down
	rec := do(t, s, http.MethodGet, "/api/v1/health", nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("health with redis down = %d, want 503", rec.Code)
	}
}

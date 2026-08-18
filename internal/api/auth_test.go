package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
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
	wantMaxAge := sessionMaxAge(time.Duration(s.cfg.Get().SessionTTLMinutes) * time.Minute)
	if cookie.MaxAge != wantMaxAge {
		t.Errorf("MaxAge = %d, want %d", cookie.MaxAge, wantMaxAge)
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
	enterSetupMode(t, s)
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
	enterSetupMode(t, s)

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
	enterSetupMode(t, s)
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

func TestSessionTokenTTLAndEpoch(t *testing.T) {
	s, _ := newTestServer(t)
	a := s.adminAuth()

	// A token issued with a TTL verifies at the matching epoch.
	tok, err := a.issueToken(5*time.Minute, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !a.verifyToken(tok, 0) {
		t.Error("fresh token must verify at epoch 0")
	}

	// Bumping the epoch revokes it.
	if a.verifyToken(tok, 1) {
		t.Error("token must not verify after the epoch bumps")
	}

	// A 0 TTL never expires (the exp part stays 0).
	forever, err := a.issueToken(0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !a.verifyToken(forever, 2) {
		t.Error("never-expiring token must verify")
	}

	// An expired token is refused: build one with a past exp by hand.
	exp := time.Now().Add(-time.Hour).Unix()
	payload := strconv.FormatInt(exp, 10) + ".0." + base64.RawURLEncoding.EncodeToString([]byte("0123456789abcdef"))
	mac := base64.RawURLEncoding.EncodeToString(a.mac(payload))
	if a.verifyToken(payload+"."+mac, 0) {
		t.Error("expired token must be refused")
	}

	// Pre-epoch 3-part tokens are refused, so old sessions die on upgrade.
	oldPayload := strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10) + "." + base64.RawURLEncoding.EncodeToString([]byte("0123456789abcdef"))
	oldMac := base64.RawURLEncoding.EncodeToString(a.mac(oldPayload))
	if a.verifyToken(oldPayload+"."+oldMac, 0) {
		t.Error("legacy 3-part token must be refused")
	}
}

func TestSessionTTLChangeOnlyAffectsNewSessions(t *testing.T) {
	s, _ := newTestServer(t)
	cookie := loginAs(t, s, "test-password")

	// Shrinking the TTL does not revoke the already-issued session.
	upd := s.cfg.Get()
	upd.SessionTTLMinutes = 5
	if err := s.cfg.Update(upd); err != nil {
		t.Fatal(err)
	}
	rec := doWith(t, s, http.MethodGet, "/api/v1/links", nil, cookie, "")
	if rec.Code != http.StatusOK {
		t.Errorf("session after TTL shrink: status = %d, want 200", rec.Code)
	}

	// A new login carries the new TTL in its cookie MaxAge.
	cookie2 := loginAs(t, s, "test-password")
	if cookie2.MaxAge != int((5 * time.Minute).Seconds()) {
		t.Errorf("new session MaxAge = %d, want %d", cookie2.MaxAge, int((5*time.Minute).Seconds()))
	}

	// Bumping the epoch revokes everything, old and new.
	upd = s.cfg.Get()
	upd.SessionEpoch = 1
	if err := s.cfg.Update(upd); err != nil {
		t.Fatal(err)
	}
	for name, ck := range map[string]*http.Cookie{"old": cookie, "new": cookie2} {
		rec := doWith(t, s, http.MethodGet, "/api/v1/links", nil, ck, "")
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s session after epoch bump: status = %d, want 401", name, rec.Code)
		}
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

package api

import (
	"net/http"
	"testing"
)

// TestLoginRateLimitByIP: after the configured number of failed attempts
// (default 10) the client IP is locked and every login — even with the right
// password — gets 429 + rate_limited.
func TestLoginRateLimitByIP(t *testing.T) {
	s, _ := newTestServer(t)
	for i := 0; i < 10; i++ {
		rec := do(t, s, http.MethodPost, "/api/v1/auth/login", map[string]any{"password": "wrong"})
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: status = %d, want 401", i+1, rec.Code)
		}
	}
	rec := do(t, s, http.MethodPost, "/api/v1/auth/login", map[string]any{"password": "wrong"})
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("attempt 11: status = %d, want 429", rec.Code)
	}
	if decodeError(t, rec) != "rate_limited" {
		t.Errorf("error code = %q, want rate_limited", decodeError(t, rec))
	}
	rec = do(t, s, http.MethodPost, "/api/v1/auth/login", map[string]any{"password": "test-password"})
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("correct password while locked: status = %d, want 429", rec.Code)
	}
}

// TestLoginRateLimitExpires: once the lock window passes, the address starts
// fresh and can log in again.
func TestLoginRateLimitExpires(t *testing.T) {
	s, _ := newTestServer(t)
	now := s.now()
	for i := 0; i < 10; i++ {
		do(t, s, http.MethodPost, "/api/v1/auth/login", map[string]any{"password": "wrong"})
	}
	s.now = func() int64 { return now + 301 }
	rec := do(t, s, http.MethodPost, "/api/v1/auth/login", map[string]any{"password": "test-password"})
	if rec.Code != http.StatusOK {
		t.Fatalf("login after lock expiry: status = %d, body %s", rec.Code, rec.Body.String())
	}
}

// TestLoginSuccessClearsFailures: a successful login resets the counter, so
// the next 9 failures don't lock the IP.
func TestLoginSuccessClearsFailures(t *testing.T) {
	s, _ := newTestServer(t)
	for i := 0; i < 5; i++ {
		do(t, s, http.MethodPost, "/api/v1/auth/login", map[string]any{"password": "wrong"})
	}
	if rec := do(t, s, http.MethodPost, "/api/v1/auth/login", map[string]any{"password": "test-password"}); rec.Code != http.StatusOK {
		t.Fatalf("login after 5 failures: %d", rec.Code)
	}
	for i := 0; i < 9; i++ {
		rec := do(t, s, http.MethodPost, "/api/v1/auth/login", map[string]any{"password": "wrong"})
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("post-success attempt %d: status = %d, want 401", i+1, rec.Code)
		}
	}
	// The 10th failure crosses the threshold (its own request still 401s).
	rec := do(t, s, http.MethodPost, "/api/v1/auth/login", map[string]any{"password": "wrong"})
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("10th post-success failure: status = %d, want 401 (triggers the lock)", rec.Code)
	}
	rec = do(t, s, http.MethodPost, "/api/v1/auth/login", map[string]any{"password": "wrong"})
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("11th post-success failure: status = %d, want 429", rec.Code)
	}
}

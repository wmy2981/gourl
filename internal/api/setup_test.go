package api

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/wmy2981/gourl/internal/config"
	"github.com/wmy2981/gourl/internal/counter"
	"github.com/wmy2981/gourl/internal/store"
)

// TestSetupFlow: with no password the server reports unconfigured, the first
// visitor sets one through /api/v1/auth/setup, and the password then governs
// logins while further setups are refused.
func TestSetupFlow(t *testing.T) {
	s, _ := newTestServer(t)
	enterSetupMode(t, s)

	rec := do(t, s, http.MethodGet, "/api/v1/auth/status", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"configured":false`) {
		t.Fatalf("status before setup: %d %s", rec.Code, rec.Body.String())
	}

	// Weak passwords are refused.
	rec = do(t, s, http.MethodPost, "/api/v1/auth/setup", map[string]any{"password": "short"})
	if rec.Code != http.StatusBadRequest || decodeError(t, rec) != "weak_password" {
		t.Fatalf("weak password: %d %s", rec.Code, rec.Body.String())
	}

	// A valid setup logs the caller in (session cookie) and persists the hash.
	rec = do(t, s, http.MethodPost, "/api/v1/auth/setup", map[string]any{"password": "correct-horse-42"})
	if rec.Code != http.StatusOK {
		t.Fatalf("setup: %d %s", rec.Code, rec.Body.String())
	}
	if got := s.cfg.Get().PasswordHash; got == "" {
		t.Error("password hash not persisted to config")
	}
	if s.admin.sessionEnabled() != true {
		t.Error("auth not enabled after setup")
	}

	// The new password works through the normal login.
	rec = do(t, s, http.MethodPost, "/api/v1/auth/login", map[string]any{"password": "correct-horse-42"})
	if rec.Code != http.StatusOK {
		t.Fatalf("login after setup: %d %s", rec.Code, rec.Body.String())
	}

	// Setup is a one-time flow.
	rec = do(t, s, http.MethodPost, "/api/v1/auth/setup", map[string]any{"password": "another-password"})
	if rec.Code != http.StatusConflict || decodeError(t, rec) != "already_configured" {
		t.Fatalf("second setup: %d %s", rec.Code, rec.Body.String())
	}
}

// TestEnvPasswordMigratedIntoConfig: the legacy ADMIN_PASSWORD env var is
// hashed and written back to config.yaml on startup; afterwards the hash from
// the file governs logins.
func TestEnvPasswordMigratedIntoConfig(t *testing.T) {
	t.Setenv("ADMIN_PASSWORD", "legacy-env-pass")
	t.Setenv("SESSION_SECRET", "test-secret")

	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	cfgMgr, err := config.NewManager(filepath.Join(t.TempDir(), "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })

	srv := NewServer(st, cfgMgr, counter.NewFromClient(rdb))
	if !srv.admin.sessionEnabled() {
		t.Fatal("auth must be enabled after env migration")
	}
	if cfgMgr.Get().PasswordHash == "" {
		t.Error("hash not written back to the config file")
	}
	rec := do(t, srv, http.MethodPost, "/api/v1/auth/login", map[string]any{"password": "legacy-env-pass"})
	if rec.Code != http.StatusOK {
		t.Fatalf("login with migrated password: %d %s", rec.Code, rec.Body.String())
	}
	// The plaintext must never survive anywhere.
	if strings.Contains(cfgMgr.Get().PasswordHash, "legacy-env-pass") {
		t.Error("config stores the plaintext password")
	}
}

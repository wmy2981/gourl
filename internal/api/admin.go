package api

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wmy2981/gourl/internal/config"
	"golang.org/x/crypto/bcrypt"
)

const (
	sessionCookie = "gourl_session"
	sessionTTL    = 7 * 24 * time.Hour
)

// adminAuth handles the single-admin-password session flow. The password is
// stored as a bcrypt hash in config.yaml (never as plaintext and never in the
// environment); until it is set the API runs in setup mode. Session tokens
// are stateless: exp.nonce.hmac(SESSION_SECRET, exp.nonce), carried in an
// HttpOnly SameSite=Lax cookie.
type adminAuth struct {
	passwordHash string
	secret       []byte
}

// ephemeralSecret is generated once per process when SESSION_SECRET is unset.
// Token signatures are cryptographically strong, but the secret never
// persists, so sessions do not survive a restart — the warning nudges
// operators to configure a stable SESSION_SECRET instead.
var ephemeralSecret = sync.OnceValue(func() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand unavailable") // unrecoverable at startup
	}
	slog.Warn("SESSION_SECRET not set; generated an ephemeral secret, sessions will not survive a restart")
	return hex.EncodeToString(b)
})

func newAdminAuth(passwordHash, secret string) *adminAuth {
	if secret == "" {
		secret = ephemeralSecret()
	}
	return &adminAuth{passwordHash: passwordHash, secret: []byte(secret)}
}

// resolveAdminAuth picks the admin password hash: the config file wins; a
// legacy ADMIN_PASSWORD env var is migrated once — hashed and written back to
// the config file, after which the env value is ignored. With neither set the
// server runs in setup mode.
func resolveAdminAuth(cfg *config.Manager) *adminAuth {
	cur := cfg.Get()
	if cur.PasswordHash != "" {
		return newAdminAuth(cur.PasswordHash, os.Getenv("SESSION_SECRET"))
	}
	if env := os.Getenv("ADMIN_PASSWORD"); env != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(env), bcrypt.DefaultCost)
		if err == nil {
			upd := cur
			upd.PasswordHash = string(hash)
			if err := cfg.Update(upd); err != nil {
				slog.Warn("migrating ADMIN_PASSWORD into config failed", "error", err)
			}
			return newAdminAuth(string(hash), os.Getenv("SESSION_SECRET"))
		}
		slog.Warn("hashing ADMIN_PASSWORD failed", "error", err)
	}
	return newAdminAuth("", os.Getenv("SESSION_SECRET"))
}

// sessionEnabled reports whether an admin password has been set. Without one
// the API runs in setup mode: management endpoints refuse with
// setup_required (bearer tokens still authenticate) until the setup flow
// configures the first password.
func (a *adminAuth) sessionEnabled() bool { return a.passwordHash != "" }

// verifyPassword compares a candidate password against the stored bcrypt
// hash, always returning false in setup mode.
func (a *adminAuth) verifyPassword(pw string) bool {
	if a.passwordHash == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(a.passwordHash), []byte(pw)) == nil
}

// setPasswordHash swaps in a freshly generated hash (setup flow).
func (a *adminAuth) setPasswordHash(hash string) { a.passwordHash = hash }

func (a *adminAuth) mac(payload string) []byte {
	m := hmac.New(sha256.New, a.secret)
	m.Write([]byte(payload))
	return m.Sum(nil)
}

func (a *adminAuth) issueToken() (string, error) {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	exp := time.Now().Add(sessionTTL).Unix()
	payload := strconv.FormatInt(exp, 10) + "." + base64.RawURLEncoding.EncodeToString(nonce)
	mac := base64.RawURLEncoding.EncodeToString(a.mac(payload))
	return payload + "." + mac, nil
}

func (a *adminAuth) verifyToken(token string) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return false
	}
	exp, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || exp < time.Now().Unix() {
		return false
	}
	got := a.mac(parts[0] + "." + parts[1])
	want, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}

// validSession accepts requests with a valid session cookie. Setup mode
// grants nothing here: with no admin password there is nothing to sign in to,
// and management endpoints stay closed until the setup flow configures one.
func (a *adminAuth) validSession(r *http.Request) bool {
	if !a.sessionEnabled() {
		return false
	}
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return false
	}
	return a.verifyToken(c.Value)
}

// login handles POST /api/v1/auth/login.
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if !s.admin.sessionEnabled() {
		writeError(w, http.StatusForbidden, "auth_disabled", "admin authentication is not configured")
		return
	}
	ip := clientIP(r)
	cfg := s.cfg.Get()
	now := time.Unix(s.now(), 0)
	// Locked-out IPs are refused before any password work. The limiter is
	// enabled only when both thresholds are set (0 disables it).
	if cfg.LoginRateMaxAttempts > 0 && cfg.LoginRateLockSeconds > 0 && s.loginRate.locked(ip, now) {
		slog.Warn("admin login rate limited", "remote", r.RemoteAddr)
		writeError(w, http.StatusTooManyRequests, "rate_limited", "too many failed attempts, try again later")
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}
	if !s.admin.verifyPassword(body.Password) {
		s.loginRate.recordFailure(ip, cfg.LoginRateMaxAttempts, cfg.LoginRateLockSeconds, now)
		slog.Warn("admin login failed", "remote", r.RemoteAddr, "ip", ip)
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid password")
		return
	}
	s.loginRate.clear(ip)
	slog.Info("admin logged in", "remote", r.RemoteAddr, "ip", ip)
	token, err := s.admin.issueToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to issue session")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionTTL.Seconds()),
	})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// authStatus handles GET /api/v1/auth/status: tells the SPA whether an admin
// password exists, so it can route to the setup page before login.
func (s *Server) authStatus(w http.ResponseWriter, r *http.Request) {
	slog.Debug("auth status", "configured", s.admin.sessionEnabled(), "remote", r.RemoteAddr)
	writeJSON(w, http.StatusOK, map[string]bool{"configured": s.admin.sessionEnabled()})
}

// setupAdmin handles POST /api/v1/auth/setup: the first visitor sets the
// admin password while the server is in setup mode. The bcrypt hash is
// persisted to config.yaml and the caller is logged in immediately.
func (s *Server) setupAdmin(w http.ResponseWriter, r *http.Request) {
	if s.admin.sessionEnabled() {
		writeError(w, http.StatusConflict, "already_configured", "admin password is already set")
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}
	if len(body.Password) < 8 {
		writeError(w, http.StatusBadRequest, "weak_password", "password must be at least 8 characters")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		slog.Error("setup: hash password", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to hash password")
		return
	}
	upd := s.cfg.Get()
	upd.PasswordHash = string(hash)
	if err := s.cfg.Update(upd); err != nil {
		slog.Error("setup: persist password hash", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to persist password")
		return
	}
	s.admin.setPasswordHash(string(hash))
	slog.Info("admin password configured", "remote", r.RemoteAddr)
	token, err := s.admin.issueToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to issue session")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionTTL.Seconds()),
	})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// logout handles POST /api/v1/auth/logout.
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	slog.Info("admin logged out", "remote", r.RemoteAddr, "ip", clientIP(r))
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/",
		HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// validBearer accepts a management API token from the Authorization header.
func (s *Server) validBearer(r *http.Request) bool {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return false
	}
	_, err := s.store.GetToken(r.Context(), strings.TrimPrefix(h, "Bearer "))
	return err == nil
}

// requireAuth gates admin endpoints behind a session or a management token
// and records how the request was authenticated so handlers can log it.
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var actor string
		switch {
		case s.admin.validSession(r):
			actor = "session"
		case s.validBearer(r):
			actor = "token"
		case !s.admin.sessionEnabled():
			// No admin password yet: the management API is locked until the
			// setup flow configures one (bearer tokens keep working, so
			// scripted access is unaffected).
			slog.Warn("admin api refused: setup required", "path", r.URL.Path, "remote", r.RemoteAddr)
			writeError(w, http.StatusForbidden, "setup_required", "admin password not configured, complete the setup flow first")
			return
		default:
			slog.Debug("admin auth rejected", "path", r.URL.Path, "remote", r.RemoteAddr)
			writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), actorKey{}, actor)))
	}
}

// actorKey is the context key for the requireAuth actor.
type actorKey struct{}

// actorFrom returns how the request was authenticated ("session" or "token").
func actorFrom(r *http.Request) string {
	if a, ok := r.Context().Value(actorKey{}).(string); ok {
		return a
	}
	return ""
}

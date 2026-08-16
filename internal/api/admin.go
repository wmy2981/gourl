package api

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	sessionCookie = "gourl_session"
	sessionTTL    = 7 * 24 * time.Hour
)

// adminAuth handles the single-admin-password session flow. Tokens are
// stateless: exp.nonce.hmac(SESSION_SECRET, exp.nonce), carried in an
// HttpOnly SameSite=Lax cookie.
type adminAuth struct {
	password string
	secret   []byte
}

func newAdminAuth(password, secret string) *adminAuth {
	if secret == "" {
		secret = "insecure-dev-secret"
	}
	return &adminAuth{password: password, secret: []byte(secret)}
}

// sessionEnabled reports whether admin authentication is configured. With an
// empty ADMIN_PASSWORD the API runs in trusted-network mode (no login).
func (a *adminAuth) sessionEnabled() bool { return a.password != "" }

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

// validSession accepts requests in trusted-network mode, or with a valid
// session cookie.
func (a *adminAuth) validSession(r *http.Request) bool {
	if !a.sessionEnabled() {
		return true
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
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}
	if subtle.ConstantTimeCompare([]byte(body.Password), []byte(s.admin.password)) != 1 {
		slog.Warn("admin login failed", "remote", r.RemoteAddr)
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid password")
		return
	}
	slog.Info("admin logged in", "remote", r.RemoteAddr)
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
	slog.Info("admin logged out", "remote", r.RemoteAddr)
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
		default:
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

package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRenderNotFoundUsesLocaleCopy(t *testing.T) {
	s, _ := newTestServer(t)

	rec := httptest.NewRecorder()
	s.renderNotFound(rec, httptest.NewRequest(http.MethodGet, "/missing?lang=zh", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "页面不存在") || !strings.Contains(body, "不存在、已删除或已过期") {
		t.Errorf("zh copy missing, body: %s", body)
	}

	rec = httptest.NewRecorder()
	s.renderNotFound(rec, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if body := rec.Body.String(); !strings.Contains(body, "Page not found") || !strings.Contains(body, "has been removed, or has expired.") {
		t.Errorf("en copy missing, body: %s", body)
	}
}

func TestRenderBlockedUsesLocaleCopy(t *testing.T) {
	s, _ := newTestServer(t)

	rec := httptest.NewRecorder()
	s.renderBlocked(rec, httptest.NewRequest(http.MethodGet, "/", nil), "ua", "curl")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "Access blocked") ||
		!strings.Contains(body, "Matched User-Agent keyword: curl") ||
		strings.Contains(body, "{{detail}}") {
		t.Errorf("en ua copy missing, body: %s", body)
	}

	rec = httptest.NewRecorder()
	s.renderBlocked(rec, httptest.NewRequest(http.MethodGet, "/?lang=zh", nil), "ip", "192.168.1.1")
	if body := rec.Body.String(); !strings.Contains(body, "访问被拦截") || !strings.Contains(body, "命中的 IP 规则：192.168.1.1") {
		t.Errorf("zh ip copy missing, body: %s", body)
	}
}

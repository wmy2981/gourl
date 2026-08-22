package api

import (
	"html/template"
	"net/http"
	"strings"

	"github.com/wmy2981/gourl/internal/webui"
)

// pageTmpl renders the public error pages (blocked / not found).
var pageTmpl = template.Must(template.New("page").Parse(`<!DOCTYPE html>
<html lang="{{.Lang}}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.SiteTitle}}</title>
<meta name="description" content="{{.SiteDescription}}">
<meta name="keywords" content="{{.SiteKeywords}}">
<style>
  * { box-sizing: border-box; }
  body { font-family: -apple-system, BlinkMacSystemFont, "SF Pro Text", "Segoe UI", Roboto, sans-serif;
         background: #f5f5f7; color: #1d1d1f; margin: 0; min-height: 100vh;
         display: flex; flex-direction: column; align-items: center; justify-content: center; padding: 24px; }
  .page { background: rgba(255,255,255,0.65); -webkit-backdrop-filter: blur(20px); backdrop-filter: blur(20px);
          border-radius: 20px; padding: 48px 40px; max-width: 480px; width: 100%;
          text-align: center; box-shadow: 0 8px 40px rgba(0,0,0,0.08); }
  h1 { font-size: 26px; font-weight: 600; margin: 0 0 12px; letter-spacing: -0.02em; }
  .icon { width: 64px; height: 64px; margin-bottom: 16px; border-radius: 16px; }
  p { color: #6e6e73; line-height: 1.6; margin: 0; font-size: 15px; }
  .code { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 13px;
          color: #86868b; margin-top: 16px; }
  @media (prefers-color-scheme: dark) {
    body { background: #1d1d1f; color: #f5f5f7; }
    .page { background: rgba(28,28,30,0.7); box-shadow: 0 8px 40px rgba(0,0,0,0.4); }
    p, .code { color: #a1a1a6; }
  }
  @media (max-width: 480px) { .page { padding: 32px 20px; } h1 { font-size: 22px; } }
</style>
</head>
<body>
<main class="page">
  {{if .Icon}}<img class="icon" src="{{.Icon}}" alt="">{{end}}
  <h1>{{.Heading}}</h1>
  <p>{{.Message}}</p>
  {{if .Detail}}<p class="code">{{.Detail}}</p>{{end}}
</main>
</body>
</html>`))

type pageData struct {
	Lang            string
	SiteTitle       string
	SiteDescription string
	SiteKeywords    string
	Heading         string
	Message         string
	Detail          string
	Icon            string
}

// langOf resolves the request language: explicit ?lang= wins, then
// Accept-Language, defaulting to English.
func langOf(r *http.Request) string {
	if l := r.URL.Query().Get("lang"); l == "zh" || l == "en" {
		return l
	}
	if strings.HasPrefix(strings.ToLower(r.Header.Get("Accept-Language")), "zh") {
		return "zh"
	}
	return "en"
}

// pageCopy is the backend-rendered page copy, loaded once from the embedded
// locale files (single source: frontend/src/locales/*.json).
var pageCopy = map[string]map[string]string{
	"en": webui.PageLocale("en"),
	"zh": webui.PageLocale("zh"),
}

// pageText returns the locale copy for key in lang, falling back to the
// English locale.
func pageText(lang, key string) string {
	if v := pageCopy[lang][key]; v != "" {
		return v
	}
	return pageCopy["en"][key]
}

func (s *Server) renderNotFound(w http.ResponseWriter, r *http.Request) {
	cfg := s.cfg.Get()
	lang := langOf(r)
	s.renderPage(w, http.StatusNotFound, pageData{
		Lang:            lang,
		Heading:         pageText(lang, "notFoundHeading"),
		Message:         pageText(lang, "notFoundMessage"),
		SiteTitle:       cfg.Site.Title,
		SiteDescription: cfg.Site.Description,
		SiteKeywords:    cfg.Site.Keywords,
	})
}

// renderBlocked renders the 403 page explaining why the request was blocked,
// in the request language. kind is "ua" (matched User-Agent keyword) or "ip"
// (matched IP rule); detail carries the matched value.
func (s *Server) renderBlocked(w http.ResponseWriter, r *http.Request, kind, detail string) {
	cfg := s.cfg.Get()
	lang := langOf(r)
	key := "blockedUaMessage"
	if kind == "ip" {
		key = "blockedIpMessage"
	}
	msg := strings.ReplaceAll(pageText(lang, key), "{{detail}}", detail)
	s.renderPage(w, http.StatusForbidden, pageData{
		Lang:            lang,
		Heading:         pageText(lang, "blockedHeading"),
		Message:         msg,
		SiteTitle:       cfg.Site.Title,
		SiteDescription: cfg.Site.Description,
		SiteKeywords:    cfg.Site.Keywords,
	})
}

// renderPublic renders the landing page at /: the brand icon and service name
// from the live config, plus a notice that the page has no direct content.
// Hidden while the webui is disabled, like /admin.
func (s *Server) renderPublic(w http.ResponseWriter, r *http.Request) {
	cfg := s.cfg.Get()
	if !cfg.WebUIEnabled {
		s.renderNotFound(w, r)
		return
	}
	lang := langOf(r)
	s.renderPage(w, http.StatusOK, pageData{
		Lang:            lang,
		Heading:         cfg.Site.Name,
		Message:         pageText(lang, "publicNotice"),
		Icon:            "/favicon.svg",
		SiteTitle:       cfg.Site.Title,
		SiteDescription: cfg.Site.Description,
		SiteKeywords:    cfg.Site.Keywords,
	})
}

// renderPage renders the page template with the given data.
func (s *Server) renderPage(w http.ResponseWriter, status int, d pageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_ = pageTmpl.Execute(w, d)
}

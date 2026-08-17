package api

import (
	"html/template"
	"net/http"
	"strings"
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

func (s *Server) renderNotFound(w http.ResponseWriter, r *http.Request) {
	cfg := s.cfg.Get()
	lang := langOf(r)
	heading, message := "Page not found", "The short link you visited does not exist or has been removed."
	if lang == "zh" {
		heading, message = "页面不存在", "您访问的短链接不存在或已被删除。"
	}
	s.renderPage(w, http.StatusNotFound, lang, heading, message, "", cfg.Site.Title, cfg.Site.Description, cfg.Site.Keywords)
}

// renderBlocked renders the 403 page explaining why the request was blocked,
// in the request language. kind is "ua" (matched User-Agent keyword) or "ip"
// (matched IP rule); detail carries the matched value.
func (s *Server) renderBlocked(w http.ResponseWriter, r *http.Request, kind, detail string) {
	cfg := s.cfg.Get()
	lang := langOf(r)
	var heading, msg string
	if lang == "zh" {
		heading = "访问被拦截"
		if kind == "ua" {
			msg = "您的请求被本服务拦截。命中的 User-Agent 关键词：" + detail
		} else {
			msg = "您的请求被本服务拦截。命中的 IP 规则：" + detail
		}
	} else {
		heading = "Access blocked"
		if kind == "ua" {
			msg = "Your request was blocked. Matched User-Agent keyword: " + detail
		} else {
			msg = "Your request was blocked. Matched IP rule: " + detail
		}
	}
	s.renderPage(w, http.StatusForbidden, lang, heading, msg, "", cfg.Site.Title, cfg.Site.Description, cfg.Site.Keywords)
}

// renderPage renders the page template from the live site config.
func (s *Server) renderPage(w http.ResponseWriter, status int, lang, heading, message, detail, title, description, keywords string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_ = pageTmpl.Execute(w, pageData{
		Lang:            lang,
		SiteTitle:       title,
		SiteDescription: description,
		SiteKeywords:    keywords,
		Heading:         heading,
		Message:         message,
		Detail:          detail,
	})
}

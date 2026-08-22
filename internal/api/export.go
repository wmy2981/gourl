package api

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/wmy2981/gourl/internal/store"
	"github.com/wmy2981/gourl/internal/version"
)

// exportRow is the uniform 7-field export shape shared by CSV and JSON:
// code, url, title, description, expires_at, click_count, created_at.
type exportRow struct {
	Code        string `json:"code"`
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description"`
	ExpiresAt   int64  `json:"expires_at"`
	ClickCount  int64  `json:"click_count"`
	CreatedAt   int64  `json:"created_at"`
}

func toExportRow(l *store.Link) exportRow {
	return exportRow{
		Code:        l.Code,
		URL:         l.URL,
		Title:       l.Title,
		Description: l.Description,
		ExpiresAt:   l.ExpiresAt,
		ClickCount:  l.ClickCount,
		CreatedAt:   l.CreatedAt,
	}
}

// exportLegacyRedirect returns a handler that 308-redirects the pre-1.0.2
// export paths (/api/v1/export.*) to their /api/v1/links/ home. 308 preserves
// the GET method and the Authorization header, so old scripted clients and
// the mobile app follow it transparently. The route is registered without
// requireAuth: an unauthenticated caller must still be redirected (fetch
// strips Authorization across redirects, so the new URL is re-authenticated
// with its own credentials), while a wrong token fails on the target route.
func exportLegacyRedirect(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/api/v1/links/export."+name, http.StatusPermanentRedirect)
	}
}

// exportJSON handles GET /api/v1/links/export.json: every link as a JSON array of
// the same 7 fields the CSV carries (import/export symmetry), wrapped with a
// meta object (site, version, count, exported_at) mirroring the markdown
// export's front matter. The importer accepts both this form and the legacy
// bare array.
func (s *Server) exportJSON(w http.ResponseWriter, r *http.Request) {
	links, err := s.store.ListAllLinks(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to export links")
		return
	}
	out := make([]exportRow, 0, len(links))
	for i := range links {
		out = append(out, toExportRow(&links[i]))
	}
	logInfo(r, "links exported", "format", "json", "count", len(links))
	cfg := s.cfg.Get()
	writeJSON(w, http.StatusOK, map[string]any{
		"meta": map[string]any{
			"site":        cfg.Site.Name,
			"version":     version.Version,
			"count":       len(links),
			"exported_at": time.Unix(s.now(), 0).Format("2006/01/02 15:04"),
		},
		"items": out,
	})
}

// exportCSV handles GET /api/v1/links/export.csv: every link as a CSV with a UTF-8
// BOM so spreadsheet apps detect the encoding.
func (s *Server) exportCSV(w http.ResponseWriter, r *http.Request) {
	links, err := s.store.ListAllLinks(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to export links")
		return
	}

	logInfo(r, "links exported", "format", "csv", "count", len(links))
	cfg := s.cfg.Get()
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition",
		`attachment; filename="gourl-links-`+time.Unix(s.now(), 0).Format("2006-01-02-15-04-05")+`.csv"`)
	w.Write([]byte("\xEF\xBB\xBF")) // UTF-8 BOM

	// Metadata as #-comment lines: spreadsheet apps and the importer's
	// parseCSV skip them, so the header row stays the first real row.
	fmt.Fprintf(w, "# site: %s\n# version: %s\n# count: %d\n# exported_at: %s\n",
		cfg.Site.Name, version.Version, len(links), time.Unix(s.now(), 0).Format("2006/01/02 15:04"))

	cw := csv.NewWriter(w)
	cw.Write([]string{"code", "url", "title", "description", "expires_at", "click_count", "created_at"})
	for i := range links {
		cw.Write([]string{
			links[i].Code,
			links[i].URL,
			links[i].Title,
			links[i].Description,
			strconv.FormatInt(links[i].ExpiresAt, 10),
			strconv.FormatInt(links[i].ClickCount, 10),
			strconv.FormatInt(links[i].CreatedAt, 10),
		})
	}
	cw.Flush()
}

// mdCell renders one table cell: pipes are escaped so they cannot break the
// row, newlines become <br> to keep the multi-line meaning inside the cell,
// and empty fields read as an em dash.
func mdCell(s string) string {
	if s == "" {
		return "—"
	}
	s = strings.ReplaceAll(s, "|", `\|`)
	return strings.ReplaceAll(s, "\n", "<br>")
}

// mdTime formats a unix timestamp as yyyy/MM/dd HH:mm in server-local time;
// 0 (never) reads as an em dash. Human-readable on purpose — CSV/JSON keep
// the raw unix values for scripts.
func mdTime(unix int64) string {
	if unix <= 0 {
		return "—"
	}
	return time.Unix(unix, 0).Format("2006/01/02 15:04")
}

// exportMarkdown handles GET /api/v1/links/export.md: every link as one markdown
// table row (the same 7 fields as CSV/JSON), under a header carrying the
// site name, version, link count and server-local export time.
func (s *Server) exportMarkdown(w http.ResponseWriter, r *http.Request) {
	links, err := s.store.ListAllLinks(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to export links")
		return
	}

	logInfo(r, "links exported", "format", "markdown", "count", len(links))
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition",
		`attachment; filename="gourl-links-`+time.Unix(s.now(), 0).Format("2006-01-02-15-04-05")+`.md"`)

	cfg := s.cfg.Get()
	var b strings.Builder
	// YAML front matter carries the document metadata; the body stays
	// English so the file reads the same in any editor or renderer.
	b.WriteString("---\n")
	fmt.Fprintf(&b, "site: %s\n", mdCell(cfg.Site.Name))
	fmt.Fprintf(&b, "version: %s\n", version.Version)
	fmt.Fprintf(&b, "count: %d\n", len(links))
	fmt.Fprintf(&b, "exported_at: %q\n", time.Unix(s.now(), 0).Format("2006/01/02 15:04"))
	b.WriteString("---\n\n")
	// Column names match the CSV/JSON field names so one header fits all
	// three export formats.
	b.WriteString("| code | url | title | description | click_count | expires_at | created_at |\n")
	b.WriteString("|------|-----|-------|-------------|-------------|------------|------------|\n")
	for i := range links {
		l := &links[i]
		fmt.Fprintf(&b, "| `%s` | [%s](%s) | %s | %s | %d | %s | %s |\n",
			mdCell(l.Code), mdCell(l.URL), l.URL, mdCell(l.Title), mdCell(l.Description),
			l.ClickCount, mdTime(l.ExpiresAt), mdTime(l.CreatedAt))
	}
	w.Write([]byte(b.String()))
}

package api

import (
	"encoding/csv"
	"net/http"
	"strconv"
	"time"

	"github.com/wmy2981/gourl/internal/store"
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

// exportJSON handles GET /api/v1/export.json: every link as a JSON array of
// the same 7 fields the CSV carries (import/export symmetry).
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
	writeJSON(w, http.StatusOK, out)
}

// exportCSV handles GET /api/v1/export.csv: every link as a CSV with a UTF-8
// BOM so spreadsheet apps detect the encoding.
func (s *Server) exportCSV(w http.ResponseWriter, r *http.Request) {
	links, err := s.store.ListAllLinks(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to export links")
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition",
		`attachment; filename="gourl-links-`+time.Unix(s.now(), 0).Format("20060102")+`.csv"`)
	w.Write([]byte("\xEF\xBB\xBF")) // UTF-8 BOM

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

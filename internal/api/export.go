package api

import (
	"encoding/csv"
	"net/http"
	"strconv"
	"time"
)

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

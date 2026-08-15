package api

import (
	"net/http"
	"time"
)

// dashboardDays is how many days of daily stats the dashboard shows.
const dashboardDays = 14

// dashboard handles GET /api/v1/dashboard: aggregate metrics plus the last
// 14 days of clicks for the trend chart.
func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	now := time.Unix(s.now(), 0)
	today := now.Format("2006-01-02")
	from := now.AddDate(0, 0, -(dashboardDays - 1)).Format("2006-01-02")

	linksTotal, clicksTotal, daily, err := s.store.StatsOverview(r.Context(), from)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to load stats")
		return
	}

	outDaily := make([]map[string]any, 0, len(daily))
	clicksToday := int64(0)
	for _, d := range daily {
		outDaily = append(outDaily, map[string]any{"date": d.Date, "count": d.Count})
		if d.Date == today {
			clicksToday = d.Count
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"links_total":  linksTotal,
		"clicks_total": clicksTotal,
		"clicks_today": clicksToday,
		"daily":        outDaily,
	})
}

package api

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/rishi/claude-watch/internal/store"
)

func handleSearch(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	q := r.URL.Query().Get("q")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 50
	}

	if q == "" {
		jsonResponse(w, map[string]interface{}{
			"results": []interface{}{},
			"total":   0,
		})
		return
	}

	results, total, err := store.Search(db, q, page, limit)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	type resultJSON struct {
		SessionID   string `json:"sessionId"`
		ProjectName string `json:"projectName"`
		UUID        string `json:"uuid"`
		Snippet     string `json:"snippet"`
		Timestamp   string `json:"timestamp"`
	}

	out := make([]resultJSON, 0, len(results))
	for _, r := range results {
		out = append(out, resultJSON{
			SessionID:   r.SessionID,
			ProjectName: r.ProjectName,
			UUID:        r.MsgUUID,
			Snippet:     r.Snippet,
			Timestamp:   r.Timestamp.UTC().Format("2006-01-02T15:04:05Z"),
		})
	}

	jsonResponse(w, map[string]interface{}{
		"results": out,
		"total":   total,
	})
}

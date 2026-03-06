package api

import (
	"database/sql"
	"net/http"

	"github.com/rishi/claude-watch/internal/config"
)

func handleStatus(w http.ResponseWriter, r *http.Request, db *sql.DB, cfg *config.Config) {
	var count int
	db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&count)

	jsonResponse(w, map[string]interface{}{
		"ok":           true,
		"sessionCount": count,
		"port":         cfg.Port,
	})
}

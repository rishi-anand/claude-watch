package store

import (
	"database/sql"
	"strings"
	"time"
)

type SearchResult struct {
	SessionID   string
	ProjectName string
	MsgUUID     string
	Snippet     string
	Timestamp   time.Time
}

func ParseQuery(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}

	// Split by comma (AND)
	andParts := strings.Split(input, ",")
	var ftsTerms []string

	for _, part := range andParts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Split by semicolon (OR)
		orParts := strings.Split(part, ";")
		if len(orParts) > 1 {
			var orTerms []string
			for _, op := range orParts {
				op = strings.TrimSpace(op)
				if op != "" {
					orTerms = append(orTerms, quoteIfPhrase(op))
				}
			}
			ftsTerms = append(ftsTerms, strings.Join(orTerms, " OR "))
		} else {
			ftsTerms = append(ftsTerms, quoteIfPhrase(part))
		}
	}

	return strings.Join(ftsTerms, " AND ")
}

func quoteIfPhrase(s string) string {
	if strings.Contains(s, " ") {
		return `"` + s + `"`
	}
	return s
}

func Search(db *sql.DB, query string, page, limit int) ([]SearchResult, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 200 {
		limit = 50
	}
	offset := (page - 1) * limit

	ftsQuery := ParseQuery(query)
	if ftsQuery == "" {
		return nil, 0, nil
	}

	var total int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM messages_fts WHERE messages_fts MATCH ?`, ftsQuery).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := db.Query(`
		SELECT f.session_id, COALESCE(s.project_name,''), f.msg_uuid,
			snippet(messages_fts, 2, '<mark>', '</mark>', '...', 32),
			COALESCE(m.timestamp, '')
		FROM messages_fts f
		LEFT JOIN messages m ON m.session_id = f.session_id AND m.msg_uuid = f.msg_uuid
		LEFT JOIN sessions s ON s.session_id = f.session_id
		WHERE messages_fts MATCH ?
		ORDER BY rank
		LIMIT ? OFFSET ?`, ftsQuery, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		var ts string
		if err := rows.Scan(&r.SessionID, &r.ProjectName, &r.MsgUUID, &r.Snippet, &ts); err != nil {
			return nil, 0, err
		}
		r.Timestamp, _ = time.Parse(time.RFC3339, ts)
		results = append(results, r)
	}
	return results, total, rows.Err()
}

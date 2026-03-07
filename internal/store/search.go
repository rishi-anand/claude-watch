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

// ParseQuery converts user input into an FTS5 query string.
//
// Rules:
//   - words separated by spaces → each word AND'd (find docs with all words)
//   - comma  → AND between groups  (foo bar,baz → "foo bar" AND baz)
//   - semicolon → OR between groups (foo;bar → foo OR bar)
//   - hyphens normalized to spaces  (palette-agentic-cli → palette AND agentic AND cli)
func ParseQuery(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}

	// OR splits (semicolon)
	orParts := strings.Split(input, ";")
	if len(orParts) > 1 {
		var orTerms []string
		for _, p := range orParts {
			if t := buildTerms(p); t != "" {
				orTerms = append(orTerms, t)
			}
		}
		return strings.Join(orTerms, " OR ")
	}

	// AND splits (comma)
	andParts := strings.Split(input, ",")
	if len(andParts) > 1 {
		var andTerms []string
		for _, p := range andParts {
			if t := buildTerms(p); t != "" {
				andTerms = append(andTerms, t)
			}
		}
		return strings.Join(andTerms, " AND ")
	}

	// Plain text: AND all words
	return buildTerms(input)
}

// buildTerms converts a single segment into FTS5 terms.
// Spaces and hyphens become AND operators — no phrase matching.
func buildTerms(s string) string {
	// Normalize hyphens (FTS5 tokenizer treats them as word separators)
	s = strings.ReplaceAll(s, "-", " ")
	words := strings.Fields(s)
	if len(words) == 0 {
		return ""
	}
	return strings.Join(words, " AND ")
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

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
// All words are AND'd together; hyphens and apostrophes are treated as word separators.
// Example: "palette-agentic-cli" → "palette AND agentic AND cli"
func ParseQuery(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}
	return buildTerms(input)
}

// buildTerms converts a single segment into FTS5 terms.
// Spaces, hyphens, and apostrophes become word separators — no phrase matching.
func buildTerms(s string) string {
	// Normalize characters that the FTS5 tokenizer treats as word separators.
	// Apostrophes are word-split (e.g. "Cloud's" → "Cloud" AND "s") so that
	// the main root word ("Cloud") still matches its FTS5 token.
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.ReplaceAll(s, "'", " ")
	s = strings.ReplaceAll(s, "'", " ") // unicode right single quote
	s = strings.ReplaceAll(s, "'", " ") // unicode left single quote
	words := strings.Fields(s)
	if len(words) == 0 {
		return ""
	}
	var terms []string
	for _, w := range words {
		w = sanitizeFTS5Word(w)
		if w != "" {
			terms = append(terms, w)
		}
	}
	if len(terms) == 0 {
		return ""
	}
	return strings.Join(terms, " AND ")
}

// sanitizeFTS5Word removes characters that would break an FTS5 bare-term query.
// After apostrophe/hyphen splitting, the only remaining dangerous chars are
// FTS5 operators and delimiters: " ( ) * ^ : \
func sanitizeFTS5Word(w string) string {
	var sb strings.Builder
	for _, ch := range w {
		switch ch {
		case '"', '(', ')', '*', '^', ':', '\\':
			// skip — these break FTS5 bare-term syntax
		default:
			sb.WriteRune(ch)
		}
	}
	return sb.String()
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

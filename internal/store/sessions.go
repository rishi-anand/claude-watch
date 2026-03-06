package store

import (
	"database/sql"
	"math"
	"time"

	"github.com/rishi/claude-watch/internal/claude"
)

type SessionRow struct {
	SessionID    string
	ProjectPath  string
	ProjectName  string
	Slug         string
	GitBranch    string
	FirstMessage string
	LastMessage  string
	StartedAt    time.Time
	LastActiveAt time.Time
	MessageCount int
	HasCompaction bool
	MdPath       string
	MemoryMd     string
}

func UpsertSession(db *sql.DB, session *claude.Session, mdPath string, mdMtime float64) error {
	firstMsg := ""
	lastMsg := ""
	msgCount := 0
	for _, m := range session.Messages {
		if m.MsgType == "user" || m.MsgType == "assistant" {
			msgCount++
			if firstMsg == "" && m.MsgType == "user" {
				firstMsg = truncate(m.ContentText, 200)
			}
			lastMsg = truncate(m.ContentText, 200)
		}
	}

	hasCompaction := 0
	if session.HasCompaction {
		hasCompaction = 1
	}

	_, err := db.Exec(`
		INSERT INTO sessions (session_id, project_path, project_name, slug, git_branch,
			first_message, last_message, started_at, last_active_at, message_count,
			has_compaction, md_path, md_mtime, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(session_id) DO UPDATE SET
			project_path=excluded.project_path,
			project_name=excluded.project_name,
			slug=excluded.slug,
			git_branch=excluded.git_branch,
			first_message=excluded.first_message,
			last_message=excluded.last_message,
			started_at=excluded.started_at,
			last_active_at=excluded.last_active_at,
			message_count=excluded.message_count,
			has_compaction=excluded.has_compaction,
			md_path=excluded.md_path,
			md_mtime=excluded.md_mtime,
			updated_at=CURRENT_TIMESTAMP`,
		session.SessionID, session.ProjectPath, session.ProjectName,
		session.Slug, session.GitBranch,
		firstMsg, lastMsg,
		session.StartedAt.UTC().Format(time.RFC3339),
		session.LastActiveAt.UTC().Format(time.RFC3339),
		msgCount, hasCompaction, mdPath, mdMtime,
	)
	if err != nil {
		return err
	}

	// Update index_state
	_, err = db.Exec(`
		INSERT INTO index_state (md_path, last_mtime, indexed_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(md_path) DO UPDATE SET
			last_mtime=excluded.last_mtime,
			indexed_at=CURRENT_TIMESTAMP`,
		mdPath, mdMtime,
	)
	return err
}

func ListSessions(db *sql.DB, project string, page, limit int) ([]SessionRow, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 200 {
		limit = 50
	}
	offset := (page - 1) * limit

	var total int
	var countErr error
	if project != "" {
		countErr = db.QueryRow("SELECT COUNT(*) FROM sessions WHERE project_name = ?", project).Scan(&total)
	} else {
		countErr = db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&total)
	}
	if countErr != nil {
		return nil, 0, countErr
	}

	query := `SELECT session_id, project_path, project_name, slug, git_branch,
		COALESCE(first_message,''), COALESCE(last_message,''),
		started_at, last_active_at, message_count, has_compaction, md_path, COALESCE(memory_md,'')
		FROM sessions`
	var args []interface{}
	if project != "" {
		query += " WHERE project_name = ?"
		args = append(args, project)
	}
	query += " ORDER BY last_active_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var results []SessionRow
	for rows.Next() {
		var r SessionRow
		var startedAt, lastActiveAt string
		var hasCompaction int
		if err := rows.Scan(&r.SessionID, &r.ProjectPath, &r.ProjectName,
			&r.Slug, &r.GitBranch, &r.FirstMessage, &r.LastMessage,
			&startedAt, &lastActiveAt, &r.MessageCount, &hasCompaction,
			&r.MdPath, &r.MemoryMd); err != nil {
			return nil, 0, err
		}
		r.StartedAt, _ = time.Parse(time.RFC3339, startedAt)
		r.LastActiveAt, _ = time.Parse(time.RFC3339, lastActiveAt)
		r.HasCompaction = hasCompaction != 0
		results = append(results, r)
	}
	return results, total, rows.Err()
}

func GetSession(db *sql.DB, sessionID string) (*SessionRow, error) {
	var r SessionRow
	var startedAt, lastActiveAt string
	var hasCompaction int
	err := db.QueryRow(`SELECT session_id, project_path, project_name, slug, git_branch,
		COALESCE(first_message,''), COALESCE(last_message,''),
		started_at, last_active_at, message_count, has_compaction, md_path, COALESCE(memory_md,'')
		FROM sessions WHERE session_id = ?`, sessionID).Scan(
		&r.SessionID, &r.ProjectPath, &r.ProjectName,
		&r.Slug, &r.GitBranch, &r.FirstMessage, &r.LastMessage,
		&startedAt, &lastActiveAt, &r.MessageCount, &hasCompaction,
		&r.MdPath, &r.MemoryMd,
	)
	if err != nil {
		return nil, err
	}
	r.StartedAt, _ = time.Parse(time.RFC3339, startedAt)
	r.LastActiveAt, _ = time.Parse(time.RFC3339, lastActiveAt)
	r.HasCompaction = hasCompaction != 0
	return &r, nil
}

func GetAllMtimes(db *sql.DB) (map[string]float64, error) {
	rows, err := db.Query("SELECT md_path, last_mtime FROM index_state")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]float64)
	for rows.Next() {
		var path string
		var mtime float64
		if err := rows.Scan(&path, &mtime); err != nil {
			return nil, err
		}
		result[path] = mtime
	}
	return result, rows.Err()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	// Find a safe truncation point (don't break UTF-8)
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	return string(r[:int(math.Min(float64(maxLen), float64(len(r))))]) + "..."
}

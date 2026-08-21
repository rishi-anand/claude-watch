package store

import (
	"database/sql"
	"math"
	"os"
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

// GetJSONLState returns the last recorded offset, mtime, and session_id for a
// JSONL transcript path. Returns zero values (with a nil error) if no state
// is recorded yet.
func GetJSONLState(db *sql.DB, path string) (offset int64, mtime float64, sessionID string, err error) {
	err = db.QueryRow(
		`SELECT last_offset, last_mtime, session_id FROM jsonl_state WHERE path = ?`,
		path).Scan(&offset, &mtime, &sessionID)
	if err == sql.ErrNoRows {
		return 0, 0, "", nil
	}
	return
}

// UpsertJSONLState records the last successfully consumed offset and mtime for
// a JSONL transcript. Subsequent syncs seek to this offset instead of
// re-reading the whole file.
func UpsertJSONLState(db *sql.DB, path, sessionID string, offset int64, mtime float64) error {
	_, err := db.Exec(`
		INSERT INTO jsonl_state (path, session_id, last_offset, last_mtime, indexed_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(path) DO UPDATE SET
			session_id=excluded.session_id,
			last_offset=excluded.last_offset,
			last_mtime=excluded.last_mtime,
			indexed_at=CURRENT_TIMESTAMP`,
		path, sessionID, offset, mtime)
	return err
}

// GetAllJSONLMtimes returns a map of JSONL path -> last recorded mtime,
// used by ScanAll to skip files whose mtime is unchanged.
func GetAllJSONLMtimes(db *sql.DB) (map[string]float64, error) {
	rows, err := db.Query(`SELECT path, last_mtime FROM jsonl_state`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]float64)
	for rows.Next() {
		var p string
		var m float64
		if err := rows.Scan(&p, &m); err != nil {
			return nil, err
		}
		result[p] = m
	}
	return result, rows.Err()
}

// GetLastSeq returns the highest seq value recorded for a session, or 0 if
// the session has no rows yet.
func GetLastSeq(db *sql.DB, sessionID string) (int, error) {
	var seq sql.NullInt64
	err := db.QueryRow(
		`SELECT MAX(seq) FROM messages WHERE session_id = ?`,
		sessionID).Scan(&seq)
	if err != nil {
		return 0, err
	}
	if !seq.Valid {
		return 0, nil
	}
	return int(seq.Int64), nil
}

// PruneOlderThan deletes sessions whose last_active_at is strictly before
// cutoff, together with their messages, FTS rows, jsonl_state rows, and MD
// files. Returns the number of sessions removed. Cheap when nothing is
// eligible; intended to run on serve startup and once per day.
func PruneOlderThan(db *sql.DB, cutoff time.Time) (int, error) {
	cutoffStr := cutoff.UTC().Format(time.RFC3339)

	rows, err := db.Query(
		`SELECT session_id, md_path FROM sessions WHERE last_active_at < ?`,
		cutoffStr)
	if err != nil {
		return 0, err
	}
	type stale struct{ id, mdPath string }
	var victims []stale
	for rows.Next() {
		var v stale
		if err := rows.Scan(&v.id, &v.mdPath); err == nil {
			victims = append(victims, v)
		}
	}
	rows.Close()

	if len(victims) == 0 {
		return 0, nil
	}

	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	delMsg, _ := tx.Prepare(`DELETE FROM messages WHERE session_id = ?`)
	defer delMsg.Close()
	delFTS, _ := tx.Prepare(`DELETE FROM messages_fts WHERE session_id = ?`)
	defer delFTS.Close()
	delJSONL, _ := tx.Prepare(`DELETE FROM jsonl_state WHERE session_id = ?`)
	defer delJSONL.Close()
	delSess, _ := tx.Prepare(`DELETE FROM sessions WHERE session_id = ?`)
	defer delSess.Close()

	for _, v := range victims {
		delMsg.Exec(v.id)
		delFTS.Exec(v.id)
		delJSONL.Exec(v.id)
		delSess.Exec(v.id)
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}

	for _, v := range victims {
		if v.mdPath != "" {
			os.Remove(v.mdPath)
		}
	}
	return len(victims), nil
}

// UpdateSessionIncremental applies a delta parse to an existing sessions row:
// bumps message_count by the delta count, advances last_active_at and
// last_message when newer, ORs has_compaction, and refreshes md_path/md_mtime.
// One-time fields (started_at, first_message, slug, git_branch, model) are
// left alone.
func UpdateSessionIncremental(db *sql.DB, session *claude.Session, mdPath string, mdMtime float64) error {
	lastMsg := ""
	deltaCount := 0
	for _, m := range session.Messages {
		if m.MsgType == "user" || m.MsgType == "assistant" {
			deltaCount++
			lastMsg = truncate(m.ContentText, 200)
		}
	}
	lastActive := session.LastActiveAt.UTC().Format(time.RFC3339)
	hasCompaction := 0
	if session.HasCompaction {
		hasCompaction = 1
	}

	_, err := db.Exec(`
		UPDATE sessions SET
			last_message   = CASE WHEN ? <> '' THEN ? ELSE last_message END,
			last_active_at = CASE WHEN ? > last_active_at THEN ? ELSE last_active_at END,
			message_count  = message_count + ?,
			has_compaction = has_compaction | ?,
			md_path        = ?,
			md_mtime       = ?,
			updated_at     = CURRENT_TIMESTAMP
		WHERE session_id = ?`,
		lastMsg, lastMsg,
		lastActive, lastActive,
		deltaCount, hasCompaction, mdPath, mdMtime, session.SessionID)
	return err
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

package store

import (
	"database/sql"
	"time"

	"github.com/rishi/claude-watch/internal/claude"
)

type MessageRow struct {
	ID          int64
	SessionID   string
	MsgUUID     string
	MsgType     string
	Role        string
	ContentText string
	ContentJSON string
	Timestamp   time.Time
	Seq         int
}

func UpsertMessages(db *sql.DB, sessionID string, msgs []claude.ParsedMessage) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO messages (session_id, msg_uuid, msg_type, role, content_text, content_json, timestamp, seq)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(session_id, msg_uuid) DO UPDATE SET
			msg_type=excluded.msg_type,
			role=excluded.role,
			content_text=excluded.content_text,
			content_json=excluded.content_json,
			timestamp=excluded.timestamp,
			seq=excluded.seq`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, m := range msgs {
		_, err := stmt.Exec(sessionID, m.UUID, m.MsgType, m.Role,
			m.ContentText, m.ContentJSON,
			m.Timestamp.UTC().Format(time.RFC3339), m.Seq)
		if err != nil {
			continue // skip bad rows, don't abort entire batch
		}
	}

	// Rebuild FTS for this session: delete old entries, reinsert all with content
	tx.Exec("DELETE FROM messages_fts WHERE session_id = ?", sessionID)
	ftsStmt, err := tx.Prepare("INSERT INTO messages_fts(session_id, msg_uuid, content_text) VALUES (?, ?, ?)")
	if err == nil {
		defer ftsStmt.Close()
		for _, m := range msgs {
			if m.ContentText != "" {
				ftsStmt.Exec(sessionID, m.UUID, m.ContentText)
			}
		}
	}

	return tx.Commit()
}

func ListMessages(db *sql.DB, sessionID string) ([]MessageRow, error) {
	rows, err := db.Query(`
		SELECT id, session_id, msg_uuid, msg_type, COALESCE(role,''), COALESCE(content_text,''), COALESCE(content_json,''), timestamp, seq
		FROM messages WHERE session_id = ? ORDER BY timestamp, seq`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []MessageRow
	for rows.Next() {
		var r MessageRow
		var ts string
		if err := rows.Scan(&r.ID, &r.SessionID, &r.MsgUUID, &r.MsgType,
			&r.Role, &r.ContentText, &r.ContentJSON, &ts, &r.Seq); err != nil {
			return nil, err
		}
		r.Timestamp, _ = time.Parse(time.RFC3339, ts)
		results = append(results, r)
	}
	return results, rows.Err()
}

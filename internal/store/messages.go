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

// UpsertMessages writes rows into the messages table AND the FTS index.
// FTS work is expensive with the pure-Go SQLite driver — for the hook's hot
// path prefer UpsertMessagesFast, then let a background pass call CatchUpFTS.
// This function is still used by RebuildFTS and other offline paths that
// want a fully-consistent FTS after the call returns.
func UpsertMessages(db *sql.DB, sessionID string, msgs []claude.ParsedMessage) error {
	if err := UpsertMessagesFast(db, sessionID, msgs); err != nil {
		return err
	}
	return upsertFTSForMessages(db, sessionID, msgs)
}

// UpsertMessagesFast writes rows only into the messages table and skips the
// FTS index. Runs one transaction with prepared statements. Callers that need
// search consistency must arrange for CatchUpFTS to run separately (serve
// does this on startup + periodically; rebuild does it inline).
func UpsertMessagesFast(db *sql.DB, sessionID string, msgs []claude.ParsedMessage) error {
	if len(msgs) == 0 {
		return nil
	}
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
			continue
		}
	}
	return tx.Commit()
}

func upsertFTSForMessages(db *sql.DB, sessionID string, msgs []claude.ParsedMessage) error {
	if len(msgs) == 0 {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	ftsDelete, err := tx.Prepare("DELETE FROM messages_fts WHERE session_id = ? AND msg_uuid = ?")
	if err != nil {
		return err
	}
	defer ftsDelete.Close()
	ftsInsert, err := tx.Prepare("INSERT INTO messages_fts(session_id, msg_uuid, content_text) VALUES (?, ?, ?)")
	if err != nil {
		return err
	}
	defer ftsInsert.Close()

	for _, m := range msgs {
		ftsDelete.Exec(sessionID, m.UUID)
		if m.ContentText != "" {
			ftsInsert.Exec(sessionID, m.UUID, m.ContentText)
		}
	}
	return tx.Commit()
}

// CatchUpFTS indexes any messages that are missing from messages_fts,
// grouped by session. Cheap when the FTS index is already current; runs
// through the outstanding backlog otherwise. Intended for background use
// (serve startup + periodic tick) so the hook path stays fast.
func CatchUpFTS(db *sql.DB) error {
	rows, err := db.Query(`
		SELECT m.session_id, m.msg_uuid, COALESCE(m.content_text, '')
		FROM messages m
		LEFT JOIN messages_fts f
		  ON f.session_id = m.session_id AND f.msg_uuid = m.msg_uuid
		WHERE f.msg_uuid IS NULL AND m.content_text != ''`)
	if err != nil {
		return err
	}
	defer rows.Close()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	ins, err := tx.Prepare("INSERT INTO messages_fts(session_id, msg_uuid, content_text) VALUES (?, ?, ?)")
	if err != nil {
		return err
	}
	defer ins.Close()

	for rows.Next() {
		var sid, uuid, text string
		if err := rows.Scan(&sid, &uuid, &text); err != nil {
			continue
		}
		ins.Exec(sid, uuid, text)
	}
	return tx.Commit()
}

// RebuildFTS drops and rebuilds the entire FTS index from the messages table.
// Used by the rebuild command to ensure FTS matches what's in the DB,
// including messages that predate compaction and are no longer in JSONL files.
func RebuildFTS(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM messages_fts"); err != nil {
		return err
	}

	_, err = tx.Exec(`
		INSERT INTO messages_fts(session_id, msg_uuid, content_text)
		SELECT session_id, msg_uuid, content_text
		FROM messages
		WHERE content_text != ''`)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// GetMessageContent returns the plain-text content of a single message,
// looked up by (session_id, msg_uuid). Empty string if not found.
func GetMessageContent(db *sql.DB, sessionID, msgUUID string) (string, error) {
	var content string
	err := db.QueryRow(`
		SELECT COALESCE(content_text,'') FROM messages
		WHERE session_id = ? AND msg_uuid = ?`, sessionID, msgUUID).Scan(&content)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return content, err
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

CREATE TABLE IF NOT EXISTS sessions (
    session_id      TEXT PRIMARY KEY,
    project_path    TEXT NOT NULL,
    project_name    TEXT NOT NULL,
    slug            TEXT,
    git_branch      TEXT,
    first_message   TEXT,
    last_message    TEXT,
    started_at      DATETIME NOT NULL,
    last_active_at  DATETIME NOT NULL,
    message_count   INTEGER DEFAULT 0,
    has_compaction  INTEGER DEFAULT 0,
    md_path         TEXT NOT NULL,
    md_mtime        REAL NOT NULL,
    memory_md       TEXT,
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_sessions_last_active ON sessions(last_active_at DESC);
CREATE INDEX IF NOT EXISTS idx_sessions_project ON sessions(project_name);

CREATE TABLE IF NOT EXISTS messages (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id   TEXT NOT NULL,
    msg_uuid     TEXT NOT NULL,
    msg_type     TEXT NOT NULL,
    role         TEXT,
    content_text TEXT,
    content_json TEXT,
    timestamp    DATETIME NOT NULL,
    seq          INTEGER NOT NULL,
    UNIQUE(session_id, msg_uuid)
);
CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id, timestamp, seq);

-- Standalone FTS5 table (not content-table linked, avoids rowid sync complexity)
CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(
    session_id UNINDEXED,
    msg_uuid UNINDEXED,
    content_text
);

CREATE TABLE IF NOT EXISTS index_state (
    md_path     TEXT PRIMARY KEY,
    last_mtime  REAL NOT NULL,
    indexed_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/rishi/claude-watch/internal/config"
	"github.com/rishi/claude-watch/internal/store"
)

func handleConversations(w http.ResponseWriter, r *http.Request, db *sql.DB, cfg *config.Config) {
	project := r.URL.Query().Get("project")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 50
	}

	sessions, total, err := store.ListSessions(db, project, page, limit)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	type convJSON struct {
		SessionID    string `json:"sessionId"`
		ProjectName  string `json:"projectName"`
		ProjectPath  string `json:"projectPath"`
		Slug         string `json:"slug"`
		GitBranch    string `json:"gitBranch"`
		FirstMessage string `json:"firstMessage"`
		LastMessage  string `json:"lastMessage"`
		StartedAt    string `json:"startedAt"`
		LastActiveAt string `json:"lastActiveAt"`
		MessageCount int    `json:"messageCount"`
		HasCompaction bool  `json:"hasCompaction"`
	}

	convs := make([]convJSON, 0, len(sessions))
	for _, s := range sessions {
		convs = append(convs, convJSON{
			SessionID:    s.SessionID,
			ProjectName:  s.ProjectName,
			ProjectPath:  s.ProjectPath,
			Slug:         s.Slug,
			GitBranch:    s.GitBranch,
			FirstMessage: s.FirstMessage,
			LastMessage:  s.LastMessage,
			StartedAt:    s.StartedAt.UTC().Format("2006-01-02T15:04:05Z"),
			LastActiveAt: s.LastActiveAt.UTC().Format("2006-01-02T15:04:05Z"),
			MessageCount: s.MessageCount,
			HasCompaction: s.HasCompaction,
		})
	}

	jsonResponse(w, map[string]interface{}{
		"conversations": convs,
		"total":         total,
		"page":          page,
	})
}

func handleConversationDetail(w http.ResponseWriter, r *http.Request, db *sql.DB, cfg *config.Config) {
	// Extract session ID from path: /api/conversations/{id}
	path := strings.TrimPrefix(r.URL.Path, "/api/conversations/")
	sessionID := strings.TrimRight(path, "/")
	if sessionID == "" {
		jsonError(w, "session ID required", 400)
		return
	}

	session, err := store.GetSession(db, sessionID)
	if err != nil {
		jsonError(w, "session not found", 404)
		return
	}

	messages, err := store.ListMessages(db, sessionID)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	type msgJSON struct {
		UUID        string `json:"uuid"`
		MsgType     string `json:"msgType"`
		Role        string `json:"role"`
		ContentText string `json:"contentText"`
		ContentJSON string `json:"contentJson"`
		Timestamp   string `json:"timestamp"`
		Seq         int    `json:"seq"`
	}

	msgs := make([]msgJSON, 0, len(messages))
	for _, m := range messages {
		msgs = append(msgs, msgJSON{
			UUID:        m.MsgUUID,
			MsgType:     m.MsgType,
			Role:        m.Role,
			ContentText: m.ContentText,
			ContentJSON: m.ContentJSON,
			Timestamp:   m.Timestamp.UTC().Format("2006-01-02T15:04:05Z"),
			Seq:         m.Seq,
		})
	}

	jsonResponse(w, map[string]interface{}{
		"session": map[string]interface{}{
			"sessionId":    session.SessionID,
			"projectName":  session.ProjectName,
			"projectPath":  session.ProjectPath,
			"slug":         session.Slug,
			"gitBranch":    session.GitBranch,
			"firstMessage": session.FirstMessage,
			"lastMessage":  session.LastMessage,
			"startedAt":    session.StartedAt.UTC().Format("2006-01-02T15:04:05Z"),
			"lastActiveAt": session.LastActiveAt.UTC().Format("2006-01-02T15:04:05Z"),
			"messageCount": session.MessageCount,
			"hasCompaction": session.HasCompaction,
		},
		"messages": msgs,
		"memoryMd": session.MemoryMd,
	})
}

func jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

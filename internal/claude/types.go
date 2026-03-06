package claude

import (
	"encoding/json"
	"time"
)

type Entry struct {
	UUID       string   `json:"uuid"`
	ParentUUID string   `json:"parentUuid"`
	Type       string   `json:"type"`
	Timestamp  string   `json:"timestamp"`
	SessionID  string   `json:"sessionId"`
	CWD        string   `json:"cwd"`
	IsSidechain bool   `json:"isSidechain"`
	Message    *Message `json:"message"`
	Summary    string   `json:"summary"`
	Subtype    string   `json:"subtype"`
}

type Message struct {
	Role             string       `json:"role"`
	Content          ContentValue `json:"content"`
	Model            string       `json:"model"`
	IsCompactSummary bool         `json:"isCompactSummary"`
}

type ContentValue struct {
	Text   string
	Blocks []ContentBlock
}

func (c *ContentValue) UnmarshalJSON(data []byte) error {
	var blocks []ContentBlock
	if err := json.Unmarshal(data, &blocks); err == nil {
		c.Blocks = blocks
		return nil
	}
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		c.Text = text
		return nil
	}
	return nil
}

func (c ContentValue) MarshalJSON() ([]byte, error) {
	if len(c.Blocks) > 0 {
		return json.Marshal(c.Blocks)
	}
	return json.Marshal(c.Text)
}

type ContentBlock struct {
	Type    string          `json:"type"`
	Text    string          `json:"text,omitempty"`
	ID      string          `json:"id,omitempty"`
	Name    string          `json:"name,omitempty"`
	Input   json.RawMessage `json:"input,omitempty"`
	Content json.RawMessage `json:"content,omitempty"`
}

type Session struct {
	SessionID    string
	ProjectPath  string
	ProjectName  string
	Slug         string
	GitBranch    string
	StartedAt    time.Time
	LastActiveAt time.Time
	Model        string
	HasCompaction bool
	Messages     []ParsedMessage
}

type ParsedMessage struct {
	UUID             string
	ParentUUID       string
	MsgType          string
	Role             string
	ContentText      string
	ContentJSON      string
	IsCompactSummary bool
	IsSidechain      bool
	Timestamp        time.Time
	Seq              int
	CompactTrigger   string
	CompactTokens    int64
	CompactSummary   string
}

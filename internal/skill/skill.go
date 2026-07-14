// Package skill installs the claude-watch agent guide for supported
// coding assistants (Claude Code, Codex, Cursor, ...).
//
// Each Target renders the same body content into the format that
// tool expects and writes it to its conventional location. Codex
// uses a shared AGENTS.md, so its writer merges via begin/end
// markers so re-runs are idempotent.
package skill

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed skill.md
var canonical string

const (
	skillName   = "claude-watch"
	beginMarker = "<!-- claude-watch skill BEGIN — managed by `claude-watch install-skill`, do not edit inside this block -->"
	endMarker   = "<!-- claude-watch skill END -->"
)

// Target describes one supported coding assistant.
type Target struct {
	Name        string // short id used with --tool
	DisplayName string
	Path        string // absolute destination
	Merge       bool   // true = merge into shared file with markers; false = own file (overwrite)
}

// Result reports what happened for one Target after Install.
type Result struct {
	Target Target
	Action string // "installed", "updated", "unchanged", "dry-run", "error"
	Err    error
}

// Targets returns all supported targets, with paths resolved
// against the user's home directory.
func Targets() ([]Target, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return []Target{
		{
			Name:        "claude",
			DisplayName: "Claude Code",
			Path:        filepath.Join(home, ".claude", "skills", skillName, "SKILL.md"),
		},
		{
			Name:        "codex",
			DisplayName: "OpenAI Codex CLI",
			Path:        filepath.Join(home, ".codex", "AGENTS.md"),
			Merge:       true,
		},
		{
			Name:        "cursor",
			DisplayName: "Cursor",
			Path:        filepath.Join(home, ".cursor", "rules", skillName+".mdc"),
		},
	}, nil
}

// TargetByName looks up a Target by its --tool identifier.
func TargetByName(name string) (Target, bool) {
	targets, err := Targets()
	if err != nil {
		return Target{}, false
	}
	for _, t := range targets {
		if t.Name == name {
			return t, true
		}
	}
	return Target{}, false
}

// Install renders and writes the skill content for each target.
// dryRun prints what would happen without touching the filesystem.
// force overwrites unchanged content (only matters for reporting).
func Install(targets []Target, dryRun, force bool) []Result {
	frontmatter, body := splitFrontmatter(canonical)
	description := extractDescription(frontmatter)

	results := make([]Result, 0, len(targets))
	for _, t := range targets {
		content := render(t, description, body, frontmatter)
		r := Result{Target: t}

		if dryRun {
			r.Action = "dry-run"
			results = append(results, r)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(t.Path), 0o755); err != nil {
			r.Action, r.Err = "error", err
			results = append(results, r)
			continue
		}

		action, err := writeTarget(t, content, force)
		r.Action, r.Err = action, err
		results = append(results, r)
	}
	return results
}

func writeTarget(t Target, content string, force bool) (string, error) {
	if !t.Merge {
		existing, _ := os.ReadFile(t.Path)
		if string(existing) == content && !force {
			return "unchanged", nil
		}
		action := "installed"
		if len(existing) > 0 {
			action = "updated"
		}
		if err := os.WriteFile(t.Path, []byte(content), 0o644); err != nil {
			return "error", err
		}
		return action, nil
	}

	// Merge mode: read the file, replace the marked block or append it.
	existing, err := os.ReadFile(t.Path)
	if err != nil && !os.IsNotExist(err) {
		return "error", err
	}
	before, after, had := extractExistingBlock(string(existing))
	block := beginMarker + "\n" + content + "\n" + endMarker

	var next string
	switch {
	case had:
		next = before + block + after
	case len(existing) == 0:
		next = block
	default:
		sep := "\n"
		if strings.HasSuffix(string(existing), "\n") {
			sep = ""
		}
		next = string(existing) + sep + "\n" + block
	}
	if !strings.HasSuffix(next, "\n") {
		next += "\n"
	}

	if next == string(existing) && !force {
		return "unchanged", nil
	}
	if err := os.WriteFile(t.Path, []byte(next), 0o644); err != nil {
		return "error", err
	}
	if had {
		return "updated", nil
	}
	if len(existing) == 0 {
		return "installed", nil
	}
	return "updated", nil
}

func render(t Target, description, body, claudeFrontmatter string) string {
	switch t.Name {
	case "claude":
		// Ship the canonical file untouched.
		return claudeFrontmatter + body
	case "cursor":
		fm := "---\n" +
			fmt.Sprintf("description: %s\n", oneLine(description)) +
			"alwaysApply: false\n" +
			"---\n\n"
		return fm + body
	case "codex":
		// Body only; the block is wrapped in markers by the merge writer.
		heading := fmt.Sprintf("## %s\n\n", skillName)
		note := fmt.Sprintf("_%s_\n\n", oneLine(description))
		return heading + note + body
	default:
		return body
	}
}

// splitFrontmatter separates a leading YAML frontmatter block ("---\n...\n---\n")
// from the body. Returns (frontmatterIncludingDelimiters, body).
func splitFrontmatter(s string) (string, string) {
	if !strings.HasPrefix(s, "---\n") {
		return "", s
	}
	rest := s[4:]
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		return "", s
	}
	return s[:4+end+5], rest[end+5:]
}

// extractDescription pulls the `description:` value from a YAML frontmatter block.
func extractDescription(frontmatter string) string {
	for _, line := range strings.Split(frontmatter, "\n") {
		if strings.HasPrefix(line, "description:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "description:"))
		}
	}
	return ""
}

// oneLine collapses newlines/tabs into single spaces so a description
// works as a YAML scalar value on one line.
func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	return strings.Join(strings.Fields(s), " ")
}

func extractExistingBlock(s string) (before, after string, had bool) {
	i := strings.Index(s, beginMarker)
	if i < 0 {
		return s, "", false
	}
	j := strings.Index(s[i:], endMarker)
	if j < 0 {
		return s, "", false
	}
	end := i + j + len(endMarker)
	// Also swallow a trailing newline right after the block, if any,
	// so re-writes don't drift the file with extra blank lines.
	if end < len(s) && s[end] == '\n' {
		end++
	}
	return s[:i], s[end:], true
}

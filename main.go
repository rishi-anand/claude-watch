package main

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"time"

	"github.com/rishi/claude-watch/internal/api"
	"github.com/rishi/claude-watch/internal/claude"
	"github.com/rishi/claude-watch/internal/config"
	"github.com/rishi/claude-watch/internal/db"
	"github.com/rishi/claude-watch/internal/hooks"
	"github.com/rishi/claude-watch/internal/setup"
	cwsync "github.com/rishi/claude-watch/internal/sync"
)

//go:embed static
var staticEmbed embed.FS

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		cmdServe()
	case "hook":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: claude-watch hook <event>")
			os.Exit(1)
		}
		cmdHook(os.Args[2])
	case "rebuild":
		cmdRebuild()
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage: claude-watch <command>")
	fmt.Fprintln(os.Stderr, "  serve     Start HTTP server")
	fmt.Fprintln(os.Stderr, "  hook      Process hook event (reads JSON from stdin)")
	fmt.Fprintln(os.Stderr, "  rebuild   Force rebuild SQLite index")
}

func cmdServe() {
	cfg := config.Load()
	parseServeFlags(cfg)

	// Apply saved config (overrides env defaults if config file exists)
	setup.LoadSaved(cfg)

	// First-run interactive setup
	installHooks := true
	if setup.IsFirstRun() {
		var err error
		installHooks, err = setup.Run(cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: setup: %v\n", err)
		}
	}

	// Ensure directories exist
	os.MkdirAll(cfg.SessionsDir(), 0o755)
	os.MkdirAll(cfg.HooksDir(), 0o755)

	// Install hooks (skipped if user declined during setup)
	if installHooks {
		if err := hooks.Install(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "warning: install hooks: %v\n", err)
		}
	}

	// Open database
	database, err := db.Open(cfg.DBPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: open database: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	// Initial full scan
	fmt.Println("Scanning sessions...")
	if err := cwsync.SyncAll(cfg, database); err != nil {
		fmt.Fprintf(os.Stderr, "warning: sync: %v\n", err)
	}

	var count int
	database.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&count)
	fmt.Printf("Indexed %d sessions\n", count)

	addr := fmt.Sprintf(":%d", cfg.Port)
	fmt.Printf("Ready at http://localhost:%d\n", cfg.Port)

	// Open browser after short delay
	if !cfg.NoBrowser {
		go func() {
			time.Sleep(500 * time.Millisecond)
			openBrowser(fmt.Sprintf("http://localhost:%d", cfg.Port))
		}()
	}

	staticSub, _ := fs.Sub(staticEmbed, "static")
	server := api.NewServer(cfg, database, staticSub)
	if err := http.ListenAndServe(addr, server); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func parseServeFlags(cfg *config.Config) {
	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--port":
			if i+1 < len(os.Args) {
				i++
				if p, err := strconv.Atoi(os.Args[i]); err == nil {
					cfg.Port = p
				}
			}
		case "--no-browser":
			cfg.NoBrowser = true
		}
	}
}

func cmdHook(event string) {
	cfg := config.Load()

	// Read JSON from stdin
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		// Always exit 0 — never block Claude Code
		os.Exit(0)
	}

	var payload struct {
		SessionID      string `json:"session_id"`
		TranscriptPath string `json:"transcript_path"`
		CWD            string `json:"cwd"`
		HookEventName  string `json:"hook_event_name"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		os.Exit(0)
	}

	if payload.TranscriptPath == "" {
		os.Exit(0)
	}

	// Open database
	database, err := db.Open(cfg.DBPath())
	if err != nil {
		os.Exit(0)
	}
	defer database.Close()

	// Sync this session
	_ = cwsync.SyncFromTranscript(cfg, database, payload.TranscriptPath)

	// Always exit 0
	os.Exit(0)
}

func cmdRebuild() {
	cfg := config.Load()

	// Remove existing database
	os.Remove(cfg.DBPath())

	database, err := db.Open(cfg.DBPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: open database: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	fmt.Println("Rebuilding index from all sessions...")
	if err := rebuildFromJSONL(cfg, database); err != nil {
		fmt.Fprintf(os.Stderr, "error: rebuild: %v\n", err)
		os.Exit(1)
	}

	var count int
	database.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&count)
	fmt.Printf("Rebuilt index with %d sessions\n", count)
}

func rebuildFromJSONL(cfg *config.Config, database *sql.DB) error {
	empty := make(map[string]float64)
	changed, err := claude.ScanAll(cfg, empty)
	if err != nil {
		return err
	}

	for _, path := range changed {
		if err := cwsync.SyncFromTranscript(cfg, database, path); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %s: %v\n", path, err)
		}
	}
	return nil
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return
	}
	cmd.Run()
}

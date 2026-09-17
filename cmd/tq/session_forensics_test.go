package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/session"
)

// sessionForensicsFixture opens a store and closes one stub-attributed
// session ("forensics-sess") over two commits, returning the db path.
func sessionForensicsFixture(t *testing.T) string {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "tasks.db")

	store, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	t.Cleanup(func() { _ = store.Close() })

	ctx := context.Background()

	if err := session.Begin(ctx, store, "forensics-sess", "/repos/demo", "demo"); err != nil {
		t.Fatalf("begin: %v", err)
	}

	commits := []session.Commit{
		{SHA: "1111111111111111111111111111111111111111", Subject: "first"},
		{SHA: "2222222222222222222222222222222222222222", Subject: "second"},
	}

	in := session.CloseInput{ID: "forensics-sess", Repo: "/repos/demo", Project: "demo"}

	if _, err := session.Close(ctx, store, session.GitScannerFunc(
		func(context.Context, string, string, string) ([]session.Commit, error) { return commits, nil },
	), in); err != nil {
		t.Fatalf("close: %v", err)
	}

	return dbPath
}

func TestFactsTypeFilterSelectsSessionLifecycle(t *testing.T) {
	dbPath := sessionForensicsFixture(t)

	out := captureStdout(t, func() {
		if err := cmdFacts([]string{"--db", dbPath, "--type", "session.opened,closed"}); err != nil {
			t.Errorf("cmdFacts --type: %v", err)
		}
	})

	if !strings.Contains(out, "session.opened") || !strings.Contains(out, "session.closed") {
		t.Fatalf("--type session filter missed the lifecycle facts:\n%s", out)
	}

	if strings.Contains(out, "task.enqueued") {
		t.Fatalf("--type session filter leaked non-session facts:\n%s", out)
	}

	if !strings.Contains(out, "(2 facts)") {
		t.Fatalf("filter should keep exactly the opened+closed facts:\n%s", out)
	}
}

func TestShowSyntheticSessionIDRendersForensics(t *testing.T) {
	dbPath := sessionForensicsFixture(t)

	out := captureStdout(t, func() {
		if err := cmdShow([]string{"--db", dbPath, "session:forensics-sess"}); err != nil {
			t.Errorf("cmdShow session id: %v", err)
		}
	})

	var view struct {
		Session string `json:"session"`
		Opened  *struct {
			SessionID string `json:"session_id"`
		} `json:"opened"`
		Closed *struct {
			ReviewRange string `json:"review_range"`
			AllowDirty  bool   `json:"allow_dirty"`
			ReviewTask  string `json:"review_task"`
			StatusTask  string `json:"status_task"`
		} `json:"closed"`
		Facts []journal.Fact `json:"facts"`
	}

	if err := json.NewDecoder(strings.NewReader(out)).Decode(&view); err != nil {
		t.Fatalf("show output is not JSON: %v\n%s", err, out)
	}

	if view.Session != "session:forensics-sess" || view.Opened == nil || view.Closed == nil {
		t.Fatalf("session view incomplete: session=%q opened=%v closed=%v", view.Session, view.Opened, view.Closed)
	}

	if view.Closed.ReviewRange != "1111111111111111111111111111111111111111..2222222222222222222222222222222222222222" {
		t.Fatalf("review range = %q", view.Closed.ReviewRange)
	}

	if view.Closed.ReviewTask == "" || view.Closed.StatusTask == "" {
		t.Fatalf("minted lineage missing: %+v", view.Closed)
	}

	if len(view.Facts) != 2 {
		t.Fatalf("facts trail = %d, want opened+closed", len(view.Facts))
	}
}

func TestShowUnknownSessionIDDegradesToError(t *testing.T) {
	dbPath := sessionForensicsFixture(t)

	if err := cmdShow([]string{"--db", dbPath, "session:nope"}); err == nil ||
		!strings.Contains(err.Error(), "no session facts") {
		t.Fatalf("unknown session id err = %v, want graceful no-facts error", err)
	}
}

package cqa

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/projects", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("owner_id") != "u1" {
			w.WriteHeader(http.StatusBadRequest)

			return
		}

		_ = json.NewEncoder(w).Encode([]Project{
			{ID: "p1", RepoName: "repo-a"},
			{ID: "p2", RepoName: "not-local"},
		})
	})
	mux.HandleFunc("/api/v1/projects/p1/scans", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]Scan{{ID: "s7", Status: "completed"}})
	})
	mux.HandleFunc("/api/v1/projects/p2/scans", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]Scan{{ID: "s8", Status: "completed"}})
	})
	mux.HandleFunc("/api/v1/scans/s7/issues", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]Issue{
			{
				Analyzer:  "artdupl",
				Severity:  "critical",
				FilePath:  "pkg/a.go",
				LineStart: 10,
				Message:   "duplicate block",
				Fixable:   true,
			},
			{
				Analyzer:   "artdupl",
				Severity:   "critical",
				FilePath:   "pkg/a.go",
				LineStart:  40,
				Message:    "duplicate block 2",
				Fixable:    true,
				Suggestion: "extract helper",
			},
			{
				Analyzer:  "golinter",
				Severity:  "warning",
				FilePath:  "pkg/b.go",
				LineStart: 1,
				Message:   "missing doc",
				Fixable:   true,
			},
			{
				Analyzer:  "golinter",
				Severity:  "error",
				FilePath:  "pkg/c.go",
				LineStart: 1,
				Message:   "not fixable",
				Fixable:   false,
			},
		})
	})
	mux.HandleFunc("/api/v1/scans/s8/issues", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]Issue{})
	})

	return httptest.NewServer(mux)
}

func TestCollectGroupsFixableIssuesPerFile(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	projects := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projects, "repo-a"), 0o755); err != nil {
		t.Fatal(err)
	}

	b := New(Config{
		BaseURL:     srv.URL,
		OwnerID:     "u1",
		ProjectsDir: projects,
		MinSeverity: "error",
	})

	got, err := b.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	// repo-a exists locally: pkg/a.go has 2 fixable issues >= error; b.go is
	// only warning (below threshold); c.go is not fixable. not-local repo is
	// skipped entirely.
	if len(got) != 1 {
		t.Fatalf("tasks = %d, want 1: %+v", len(got), got)
	}

	ft := got[0]
	if ft.Project != "repo-a" || ft.File != "pkg/a.go" {
		t.Fatalf("task for %s/%s, want repo-a/pkg/a.go", ft.Project, ft.File)
	}

	if len(ft.Issues) != 2 {
		t.Fatalf("issues = %d, want 2 (only fixable >= error)", len(ft.Issues))
	}

	if ft.Template.DedupKey != "cqa:repo-a:s7:pkg/a.go" {
		t.Fatalf("dedup key = %s", ft.Template.DedupKey)
	}

	if ft.Template.Project != "repo-a" || ft.Template.Type != "agent" || ft.Template.Priority != 80 {
		t.Fatalf("template = %+v", ft.Template)
	}

	var payload map[string]any
	if err := json.Unmarshal(ft.Template.Payload, &payload); err != nil {
		t.Fatalf("payload not valid agent payload: %v", err)
	}

	if payload["repo"] != "repo-a" || payload["prompt"] == "" {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestCollectRespectsMaxFiles(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	projects := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projects, "repo-a"), 0o755); err != nil {
		t.Fatal(err)
	}

	b := New(Config{BaseURL: srv.URL, OwnerID: "u1", ProjectsDir: projects, MinSeverity: "warning", MaxFiles: 1})

	got, err := b.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("tasks = %d, want 1 (MaxFiles cap)", len(got))
	}
}

func TestCollectToleratesServerErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	b := New(Config{BaseURL: srv.URL, OwnerID: "u1", ProjectsDir: t.TempDir()})

	got, err := b.Collect(context.Background())
	if err == nil {
		t.Fatal("want error from failing API")
	}

	if got != nil {
		t.Fatalf("want no tasks on error, got %d", len(got))
	}
}

func TestSeverityAtLeast(t *testing.T) {
	cases := []struct {
		sev, min string
		want     bool
	}{
		{"critical", "error", true},
		{"error", "error", true},
		{"warning", "error", false},
		{"info", "warning", false},
		{"CRITICAL", "warning", true},
		{"unknown", "info", false},
	}
	for _, c := range cases {
		if got := severityAtLeast(c.sev, c.min); got != c.want {
			t.Errorf("severityAtLeast(%q,%q) = %v, want %v", c.sev, c.min, got, c.want)
		}
	}
}

// Package cqa bridges a Code-Quality-Agent (CQA) API into the task queue:
// the latest scan's fixable issues for each locally-present repo become
// agent fix tasks. This closes the quality loop — the pool fixes what the
// scanners find, the next scan re-verifies, and any remaining or new issues
// arrive as fresh tasks (dedup keys include the scan ID, so a new scan arms
// new work).
//
// Ingestion is read-only against the CQA API and idempotent against the
// queue: enqueues ride the store's dedup keys, so running the bridge twice
// (or racing a harvest tick) never double-enqueues.
package cqa

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// Config controls one bridge run.
type Config struct {
	// BaseURL is the CQA API root, e.g. http://localhost:8080.
	BaseURL string
	// Token is sent as a Bearer authorization header (optional).
	Token string
	// OwnerID filters projects on the CQA side (its API requires it).
	OwnerID string
	// ProjectsDir maps CQA repo names to local directories: only repos that
	// exist locally become tasks (the pool cannot fix remote-only repos).
	ProjectsDir string
	// MaxFiles caps how many fix tasks one run may produce (0 = no cap).
	MaxFiles int
	// MinSeverity filters which issues are worth an agent run:
	// "critical" and "error" by default. One of critical|error|warning|info.
	MinSeverity string
	// Type is the task type enqueued (default "agent").
	Type string
	// TimeoutMinutes for generated fix tasks (default 45).
	TimeoutMinutes int
}

// Project is the slice of CQA's project response the bridge needs.
type Project struct {
	ID       string `json:"id"`
	RepoName string `json:"repo_name"`
}

// Scan is the slice of CQA's scan response the bridge needs.
type Scan struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// Issue is the slice of CQA's issue response the bridge needs.
type Issue struct {
	Analyzer      string `json:"analyzer"`
	Severity      string `json:"severity"`
	RuleID        string `json:"rule_id,omitempty"`
	FilePath      string `json:"file_path,omitempty"`
	LineStart     int32  `json:"line_start,omitempty"`
	Message       string `json:"message,omitempty"`
	Suggestion    string `json:"suggestion,omitempty"`
	Fixable       bool   `json:"fixable"`
	FixComplexity string `json:"fix_complexity,omitempty"`
}

// Bridge reads a CQA API and turns findings into agent tasks.
type Bridge struct {
	cfg  Config
	http *http.Client
}

// New creates a Bridge.
func New(cfg Config) *Bridge {
	if cfg.Type == "" {
		cfg.Type = executor.TaskTypeAgent
	}

	if cfg.MinSeverity == "" {
		cfg.MinSeverity = "error"
	}

	if cfg.TimeoutMinutes <= 0 {
		cfg.TimeoutMinutes = 45
	}

	return &Bridge{cfg: cfg, http: &http.Client{Timeout: 30 * time.Second}}
}

// FixTask is one generated agent task plus its provenance.
type FixTask struct {
	Project  string
	RepoDir  string
	File     string
	Issues   []Issue
	Template task.New
}

// Collect fetches projects, their latest scans, and the fixable issues worth
// agent work, and renders one fix task per file (grouping issues keeps each
// agent run focused and keeps the commit reviewable).
func (b *Bridge) Collect(ctx context.Context) ([]FixTask, error) {
	projects, err := b.projects(ctx)
	if err != nil {
		return nil, err
	}

	var out []FixTask

	for _, project := range projects {
		repoDir := filepath.Join(b.cfg.ProjectsDir, project.RepoName)
		if info, err := os.Stat(repoDir); err != nil || !info.IsDir() {
			continue // CQA tracks repos this machine does not have
		}

		scan, err := b.latestScan(ctx, project.ID)
		if err != nil || scan.ID == "" {
			continue
		}

		issues, err := b.issues(ctx, scan.ID)
		if err != nil {
			continue
		}

		byFile := map[string][]Issue{}

		for _, iss := range issues {
			if !iss.Fixable || iss.FilePath == "" || !severityAtLeast(iss.Severity, b.cfg.MinSeverity) {
				continue
			}

			byFile[iss.FilePath] = append(byFile[iss.FilePath], iss)
		}

		files := make([]string, 0, len(byFile))
		for file := range byFile {
			files = append(files, file)
		}

		sort.Strings(files)

		for _, file := range files {
			if b.cfg.MaxFiles > 0 && len(out) >= b.cfg.MaxFiles {
				return out, nil
			}

			fixTask := FixTask{
				Project: project.RepoName,
				RepoDir: repoDir,
				File:    file,
				Issues:  byFile[file],
			}
			fixTask.Template = b.renderTask(project, scan, fixTask)
			out = append(out, fixTask)
		}
	}

	return out, nil
}

func (b *Bridge) renderTask(project Project, scan Scan, fixTask FixTask) task.New {
	var lines []string

	for _, iss := range fixTask.Issues {
		loc := ""
		if iss.LineStart > 0 {
			loc = fmt.Sprintf(":%d", iss.LineStart)
		}

		lines = append(
			lines,
			fmt.Sprintf("- [%s%s] %s: %s (%s)", fixTask.File, loc, iss.Severity, iss.Message, iss.Analyzer),
		)
		if iss.Suggestion != "" {
			lines = append(lines, "  suggestion: "+iss.Suggestion)
		}
	}

	prompt := fmt.Sprintf(`Fix the code-quality issues the scanner found in this file.

Project: %s, latest scan %s.
Issues in %s:

%s

Rules:
1. Read AGENTS.md first (if present) and follow its conventions.
2. Fix exactly these issues in %s. The smallest correct change wins; no scope creep.
3. The project must build and its tests must pass before you finish.
4. Do not game the scanner: never weaken tests, never blanket-suppress a finding; a justified
   suppression follows the project's own suppression convention and says why in the commit message.
5. Never edit .crushrc, crush.json, or .tq-verify: they define your autonomy and your verify gate.
6. Commit your work with a clear message (you have explicit permission to commit for this task;
   commit only these fixes). Never push.
7. If an issue is a false positive, fix the code so the scanner no longer flags it (or note why
   it cannot be fixed in the commit message).
8. End your final output with this exact one-line report so the queue can record what you did
   (fields optional):

TQ_RESULT: {"files_changed": [%q], "commit_sha": "the commit sha"}
`, project.RepoName, scan.ID, fixTask.File, strings.Join(lines, "\n"), fixTask.File, fixTask.File)

	payload, _ := executor.RenderAgentPayload(executor.AgentPayload{
		Repo:           project.RepoName,
		Prompt:         prompt,
		TimeoutMinutes: b.cfg.TimeoutMinutes,
		Dedup:          DedupKey(project.RepoName, scan.ID, fixTask.File),
	})

	return task.New{
		Project:  project.RepoName,
		Type:     b.cfg.Type,
		Payload:  payload,
		Priority: 150, // machine band (ADR-0015 §1): scanner findings outrank the whole backlog, not just rank inside it
		DedupKey: DedupKey(project.RepoName, scan.ID, fixTask.File),
	}
}

// DedupKey identifies one fix task: repo + scan + file.
func DedupKey(repo, scanID, file string) string {
	return fmt.Sprintf("cqa:%s:%s:%s", repo, scanID, file)
}

func severityAtLeast(sev, min string) bool {
	rank := map[string]int{"critical": 4, "error": 3, "warning": 2, "info": 1}

	return rank[strings.ToLower(sev)] >= rank[strings.ToLower(min)]
}

func (b *Bridge) projects(ctx context.Context) ([]Project, error) {
	q := url.Values{"owner_id": {b.cfg.OwnerID}, "limit": {"100"}}

	var out []Project
	if err := b.getJSON(ctx, "/api/v1/projects?"+q.Encode(), &out); err != nil {
		return nil, err
	}

	return out, nil
}

func (b *Bridge) latestScan(ctx context.Context, projectID string) (Scan, error) {
	var scans []Scan
	if err := b.getJSON(ctx, "/api/v1/projects/"+url.PathEscape(projectID)+"/scans?limit=1", &scans); err != nil {
		return Scan{}, err
	}

	if len(scans) == 0 {
		return Scan{}, nil
	}

	return scans[0], nil
}

func (b *Bridge) issues(ctx context.Context, scanID string) ([]Issue, error) {
	var out []Issue
	if err := b.getJSON(
		ctx,
		"/api/v1/scans/"+url.PathEscape(scanID)+"/issues?fixable_only=true&limit=500",
		&out,
	); err != nil {
		return nil, err
	}

	return out, nil
}

func (b *Bridge) getJSON(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.cfg.BaseURL+path, nil)
	if err != nil {
		return fmt.Errorf("cqa: build request: %w", err)
	}

	if b.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+b.cfg.Token)
	}

	resp, err := b.http.Do(req)
	if err != nil {
		return fmt.Errorf("cqa: request %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return fmt.Errorf("cqa: read %s: %w", path, err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("cqa: %s: status %d: %s", path, resp.StatusCode, truncate(string(body), 256))
	}

	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("cqa: decode %s: %w", path, err)
	}

	return nil
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}

	return s
}

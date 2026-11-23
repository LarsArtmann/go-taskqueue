package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// defaultTools is the v1 registry: read-only surfaces, one tool per
// `tq` read command the agent pool actually shells out for. park_task
// (the write tool) joins behind the same shape once the owner rules it
// into v1 (plan 2026-10-16_01-11 open gate 2).
func (s *Server) defaultTools() []Tool {
	return []Tool{
		{
			Name:        "task_list",
			Description: "List tasks from the go-taskqueue journal, newest-first by age. Filters are optional; limit defaults to 25.",
			InputSchema: objectSchema(
				prop("project", "filter by project (repo)"),
				prop("status", "filter by status: pending, running, completed, dead, cancelled"),
				prop("type", "filter by task type (agent, sh, http, review, ...)"),
				prop("parked", "only tasks parked until a future not_before (rate-limit/question/budget waits)"),
				prop("since_hours", "only tasks created within this many hours (number)"),
				prop("limit", "max rows to return (number, default 25)"),
			),
			Handler: s.handleTaskList,
		},
		{
			Name:        "task_show",
			Description: "Show one task by id: the full record plus its recent journal facts (the retry trail).",
			InputSchema: objectSchema(
				requiredProp("id", "the task id"),
				prop("trail_limit", "max facts to return (number, default 20)"),
			),
			Handler: s.handleTaskShow,
		},
		{
			Name:        "queue_stats",
			Description: "Queue-wide counters: tasks by status and by project. No arguments.",
			InputSchema: objectSchema(),
			Handler:     s.handleQueueStats,
		},
		{
			Name:        "dlq_list",
			Description: "List dead-lettered tasks (the DLQ): exhausted or permanently failed, with their last error.",
			InputSchema: objectSchema(
				prop("project", "filter by project (repo)"),
				prop("limit", "max rows to return (number, default 25)"),
			),
			Handler: s.handleDLQList,
		},
	}
}

// taskSummary is the compact per-task row task_list and dlq_list return:
// the fields an agent needs to triage work, without the payload blob
// (task_show carries the full record when the payload matters).
type taskSummary struct {
	ID          string `json:"id"`
	Project     string `json:"project,omitempty"`
	Type        string `json:"type"`
	Status      string `json:"status"`
	Priority    int    `json:"priority,omitempty"`
	Attempts    int    `json:"attempts"`
	MaxAttempts int    `json:"maxAttempts"`
	NotBefore   string `json:"notBefore,omitempty"`
	LastError   string `json:"lastError,omitempty"`
	DedupKey    string `json:"dedupKey,omitempty"`
	CreatedAt   string `json:"createdAt"`
}

// summarize maps a task record to the compact row, RFC3339 times and a
// clipped last error.
func summarize(t task.Task) taskSummary {
	notBefore := ""
	if t.NotBefore.After(time.Now().UTC().Add(time.Minute)) {
		notBefore = t.NotBefore.UTC().Format(time.RFC3339)
	}

	return taskSummary{
		ID:          string(t.ID),
		Project:     t.Project,
		Type:        t.Type,
		Status:      string(t.Status),
		Priority:    t.Priority,
		Attempts:    t.Attempts,
		MaxAttempts: t.MaxAttempts,
		NotBefore:   notBefore,
		LastError:   clip(t.LastError, 240),
		DedupKey:    t.DedupKey,
		CreatedAt:   t.CreatedAt.UTC().Format(time.RFC3339),
	}
}

// marshalTool returns compact JSON for a tool result payload.
func marshalTool(v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("encode result: %w", err)
	}

	return string(raw), nil
}

// handleTaskList filters the listing the way `tq tasks` does, minus the
// terminal formatting: pushdown filters into queue.Filter, compact rows
// out.
func (s *Server) handleTaskList(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Project    string  `json:"project"`
		Status     string  `json:"status"`
		Type       string  `json:"type"`
		Parked     *bool   `json:"parked"`
		SinceHours float64 `json:"since_hours"`
		Limit      int     `json:"limit"`
	}
	if err := decodeArgs(args, &params); err != nil {
		return "", err
	}

	filter := queue.Filter{Sort: "age-desc", Limit: params.Limit}
	if filter.Limit <= 0 {
		filter.Limit = 25
	}

	if params.Project != "" {
		filter.Project = &params.Project
	}

	if params.Status != "" {
		status := task.Status(params.Status)
		if !status.Valid() {
			return "", fmt.Errorf(
				"invalid status %q (want pending, running, completed, dead or cancelled)",
				params.Status,
			)
		}

		filter.Status = &status
	}

	if params.Type != "" {
		filter.Type = &params.Type
	}

	if params.Parked != nil {
		filter.Parked = params.Parked
	}

	if params.SinceHours > 0 {
		since := time.Now().UTC().Add(-time.Duration(params.SinceHours * float64(time.Hour)))
		filter.Since = &since
	}

	tasks, err := s.store.List(ctx, filter)
	if err != nil {
		return "", fmt.Errorf("list tasks: %w", err)
	}

	return summarizeAll(tasks)
}

// handleDLQList is task_list pinned to dead — the DLQ triage surface.
func (s *Server) handleDLQList(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Project string `json:"project"`
		Limit   int    `json:"limit"`
	}
	if err := decodeArgs(args, &params); err != nil {
		return "", err
	}

	dead := task.Dead
	filter := queue.Filter{Status: &dead, Sort: "age-desc", Limit: params.Limit}
	if filter.Limit <= 0 {
		filter.Limit = 25
	}

	if params.Project != "" {
		filter.Project = &params.Project
	}

	tasks, err := s.store.List(ctx, filter)
	if err != nil {
		return "", fmt.Errorf("list dead tasks: %w", err)
	}

	return summarizeAll(tasks)
}

func summarizeAll(tasks []task.Task) (string, error) {
	rows := make([]taskSummary, 0, len(tasks))
	for _, t := range tasks {
		rows = append(rows, summarize(t))
	}

	return marshalTool(rows)
}

// taskShowResult is the task_show payload: the full record plus the
// recent fact trail (the chronological strip a triage needs — the same
// surface `tq show` renders).
type taskShowResult struct {
	Task  task.Task    `json:"task"`
	Trail []trailEntry `json:"trail"`
}

type trailEntry struct {
	Seq   int64  `json:"seq"`
	Type  string `json:"type"`
	Time  string `json:"time"`
	Error string `json:"error,omitempty"`
}

// handleTaskShow returns one task's full record and its recent facts;
// an unknown id is a tool error the model can read (not a protocol
// error).
func (s *Server) handleTaskShow(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		ID         string `json:"id"`
		TrailLimit int    `json:"trail_limit"`
	}
	if err := decodeArgs(args, &params); err != nil {
		return "", err
	}

	if params.ID == "" {
		return "", fmt.Errorf("id is required")
	}

	if params.TrailLimit <= 0 {
		params.TrailLimit = 20
	}

	record, err := s.store.Get(ctx, task.ID(params.ID))
	if err != nil {
		return "", fmt.Errorf("get task %s: %w", params.ID, err)
	}

	facts, err := s.store.FactsForTask(ctx, params.ID, params.TrailLimit)
	if err != nil {
		return "", fmt.Errorf("facts for task %s: %w", params.ID, err)
	}

	trail := make([]trailEntry, 0, len(facts))
	for _, fact := range facts {
		trail = append(trail, trailEntry{
			Seq:   fact.Seq,
			Type:  string(fact.Type),
			Time:  fact.Time.UTC().Format(time.RFC3339),
			Error: clip(fact.Error, 240),
		})
	}

	return marshalTool(taskShowResult{Task: record, Trail: trail})
}

// queueStatsResult is the queue_stats payload. ByStatus rides
// task.StatusCountsView — the stats wire contract every other tq stats
// surface (httpapi, webui) already pins: every known status with zeros
// included, plus the total.
type queueStatsResult struct {
	ByStatus  map[string]int `json:"byStatus"`
	Total     int            `json:"total"`
	ByProject map[string]int `json:"byProject"`
}

// handleQueueStats folds the store's count projections into one payload.
func (s *Server) handleQueueStats(ctx context.Context, _ json.RawMessage) (string, error) {
	byStatus, err := s.store.StatusCounts(ctx)
	if err != nil {
		return "", fmt.Errorf("status counts: %w", err)
	}

	byProjectRaw, err := s.store.ProjectCounts(ctx)
	if err != nil {
		return "", fmt.Errorf("project counts: %w", err)
	}

	byProject := make(map[string]int, len(byProjectRaw))
	for project, counts := range byProjectRaw {
		for _, n := range counts {
			byProject[project] += n
		}
	}

	statuses, total := task.StatusCountsView(byStatus)

	return marshalTool(queueStatsResult{ByStatus: statuses, Total: total, ByProject: byProject})
}

// decodeArgs unmarshals tool arguments, tolerating absent params (null
// or empty).
func decodeArgs(args json.RawMessage, into any) error {
	if len(trimJSONSpace(args)) == 0 || string(trimJSONSpace(args)) == "null" {
		return nil
	}

	if err := json.Unmarshal(args, into); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}

	return nil
}

// clip truncates a string to at most limit runes, marking truncation.
func clip(s string, limit int) string {
	if len(s) <= limit {
		return s
	}

	runes := []rune(strings.TrimSpace(s))
	if len(runes) <= limit {
		return string(runes)
	}

	return string(runes[:limit]) + "…"
}

// --- input-schema helpers (plain JSON Schema objects, no dependency) ---

func objectSchema(props ...map[string]any) map[string]any {
	properties := map[string]any{}
	required := []string{}

	for _, prop := range props {
		name, _ := prop["__name"].(string)
		delete(prop, "__name")

		properties[name] = prop

		if req, _ := prop["__required"].(bool); req {
			required = append(required, name)
		}

		delete(prop, "__required")
	}

	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}

	return schema
}

func prop(name, description string) map[string]any {
	return map[string]any{"__name": name, "type": "string", "description": description}
}

func requiredProp(name, description string) map[string]any {
	return map[string]any{"__name": name, "type": "string", "description": description, "__required": true}
}

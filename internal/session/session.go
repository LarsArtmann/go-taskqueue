// Package session bridges INTERACTIVE crush sessions into the same close-out
// pipeline pool agents get. Pool work is reviewed and reported on because the
// journal sees its completions; interactive sessions leave no task trail, so
// their commits could silently skip the second opinion. The bridge closes
// that gap: `tq session begin` records the session's opening in the journal,
// and `tq session close` attributes the session's commits (a
// `Crush-Session: <id>` git footer written by the committer), then directly
// enqueues ONE review task over the attributed commit range and ONE
// done-prompt status task. Both ride the ordinary pool: they are ordinary
// "review"/"status" tasks, drained by whichever agent-capable pool shares the
// database, dedup-keyed so re-closing a session never duplicates work.
//
// Close is deliberately enqueue-only (the crush SessionEnd hook budget is
// ~2s): git attribution scan, two idempotent enqueues, one journal fact. The
// pool does everything else.
//
// Loop safety is structural: session facts carry the synthetic
// "session:<id>" task identity, which no task row ever uses, so sweepers and
// workers cannot mistake session lifecycle for task state; the minted review
// is type "review" and the status task type "status", both exempt from
// re-review by the sweeper's agent-only rule.
package session

import (
	"context"
	"encoding/json"
	"encoding/json/jsontext"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/review"
	"github.com/larsartmann/go-taskqueue/internal/status"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// Trailer is the git-footer key that attributes a commit to an interactive
// session: committers end their commit message with `Crush-Session: <id>`
// (same convention as the Task-Queue-ID footer pool agents write). Close
// attributes exactly the commits whose trailer value equals the session id.
const Trailer = "Crush-Session"

// SyntheticTaskID returns the session's task-ID-shaped lineage key: the
// TaskID on session facts and the ReviewedTask / Completed.TaskID lineage in
// the minted review and status payloads. It is namespaced so it can never
// collide with a real task ID ("session:" is not a hex ULID prefix).
func SyntheticTaskID(id string) task.ID { return task.ID("session:" + id) }

// Store is the bridge's view of the queue core: mint tasks, append the
// session's lifecycle facts, and read one identity's facts (the double-begin
// guard).
type Store interface {
	Enqueue(ctx context.Context, n task.New) (task.Task, error)
	AppendFact(ctx context.Context, f journal.Fact) error
	FactsForTask(ctx context.Context, id string, limit int) ([]journal.Fact, error)
}

// OpenDetail is the session.opened fact's detail.
type OpenDetail struct {
	SessionID string `json:"session_id"`
	Repo      string `json:"repo"`
	Project   string `json:"project,omitempty"`
}

// CloseDetail is the session.closed fact's detail: what the session
// committed (as far as attribution can see) and what close minted.
type CloseDetail struct {
	SessionID  string   `json:"session_id"`
	Repo       string   `json:"repo"`
	Project    string   `json:"project,omitempty"`
	Summary    string   `json:"summary,omitempty"`
	Commits    []Commit `json:"commits"`
	ReviewTask string   `json:"review_task,omitempty"`
	StatusTask string   `json:"status_task,omitempty"`
}

// Begin records the session's opening as a session.opened fact. Refusing a
// double begin keeps one open epoch per id: begin, work, close — re-running
// begin for an already-open session is a mistake, not idempotent noise.
func Begin(ctx context.Context, s Store, id, repo, project string) error {
	if id == "" {
		return errors.New("session: begin needs a session id (--id or $CRUSH_SESSION_ID)")
	}

	facts, err := s.FactsForTask(ctx, SyntheticTaskID(id).String(), 0)
	if err != nil {
		return fmt.Errorf("session: read facts: %w", err)
	}

	for _, f := range facts {
		if f.Type == journal.SessionOpened {
			return fmt.Errorf(
				"session: %s is already open (opened %s) — close it before beginning again",
				id,
				f.Time.Format("2006-01-02 15:04:05"),
			)
		}
	}

	detail, err := json.Marshal(OpenDetail{SessionID: id, Repo: repo, Project: project})
	if err != nil {
		return fmt.Errorf("session: marshal opened detail: %w", err)
	}

	if err := s.AppendFact(ctx, journal.Fact{
		TaskID: SyntheticTaskID(id).String(),
		Type:   journal.SessionOpened,
		Detail: jsontext.Value(detail),
	}); err != nil {
		return fmt.Errorf("session: append opened fact: %w", err)
	}

	return nil
}

// CloseInput carries one Close invocation's parameters.
type CloseInput struct {
	// ID is the interactive session's id (CRUSH_SESSION_ID). Required.
	ID string
	// Repo is the absolute repository path the session worked in. Required.
	Repo string
	// Project is the queue project the minted tasks belong to (convention:
	// the repo directory's base name).
	Project string
	// Summary is the operator's one-paragraph account of what the session
	// did; it becomes the review's bar and the status entry's item. Empty
	// falls back to a default text.
	Summary string
	// AllowDirty mirrors the pool's --allow-dirty stance into the minted
	// payloads' RequireClean: a multi-agent repo is effectively always
	// dirty, and a clean-tree-required task there requeues forever.
	AllowDirty bool
}

// CloseResult reports what Close recorded and minted.
type CloseResult struct {
	// Commits are the session-attributed commits, oldest first.
	Commits []Commit
	// ReviewTask / StatusTask are the minted tasks; zero TaskIDs when no
	// commits were attributed (nothing minted).
	ReviewTask task.Task
	StatusTask task.Task
	// ReviewFresh / StatusFresh are false when the enqueue hit the dedup
	// key (the task already existed from an earlier close of this session).
	ReviewFresh bool
	StatusFresh bool
}

// Close attributes the session's commits, records the session.closed fact,
// and directly enqueues the close-out: ONE review over the attributed range
// and ONE status report, both idempotent by dedup key. A session with no
// attributed commits records only the fact — there is nothing to review and
// nothing to report.
func Close(ctx context.Context, s Store, scanner GitScanner, in CloseInput) (CloseResult, error) {
	if in.ID == "" {
		return CloseResult{}, errors.New("session: close needs a session id (--id or $CRUSH_SESSION_ID)")
	}

	if in.Repo == "" {
		return CloseResult{}, errors.New("session: close needs a repo (--repo)")
	}

	commits, err := scanner.CommitsByTrailer(ctx, in.Repo, Trailer, in.ID)
	if err != nil {
		return CloseResult{}, err
	}

	// A session.closed fact means this close is a replay: the dedup keys
	// below already hold the minted tasks, so "fresh" must not be claimed
	// again (the pending-task heuristic alone cannot tell the two apart).
	replay, err := wasClosed(ctx, s, in.ID)
	if err != nil {
		return CloseResult{}, err
	}

	var res CloseResult

	res.Commits = commits

	if len(commits) > 0 {
		if res.ReviewTask, res.ReviewFresh, err = mintReview(ctx, s, in, commits); err != nil {
			return res, err
		}

		res.ReviewFresh = res.ReviewFresh && !replay

		if res.StatusTask, res.StatusFresh, err = mintStatus(ctx, s, in, commits); err != nil {
			return res, err
		}

		res.StatusFresh = res.StatusFresh && !replay
	}

	detail, err := json.Marshal(CloseDetail{
		SessionID:  in.ID,
		Repo:       in.Repo,
		Project:    in.Project,
		Summary:    in.Summary,
		Commits:    commits,
		ReviewTask: res.ReviewTask.ID.String(),
		StatusTask: res.StatusTask.ID.String(),
	})
	if err != nil {
		return res, fmt.Errorf("session: marshal closed detail: %w", err)
	}

	if err := s.AppendFact(ctx, journal.Fact{
		TaskID: SyntheticTaskID(in.ID).String(),
		Type:   journal.SessionClosed,
		Detail: jsontext.Value(detail),
	}); err != nil {
		return res, fmt.Errorf("session: append closed fact: %w", err)
	}

	return res, nil
}

// wasClosed reports whether the session already carries a session.closed
// fact — close number two is a replay, not a first close.
func wasClosed(ctx context.Context, s Store, id string) (bool, error) {
	facts, err := s.FactsForTask(ctx, SyntheticTaskID(id).String(), 0)
	if err != nil {
		return false, fmt.Errorf("session: read facts: %w", err)
	}

	for _, f := range facts {
		if f.Type == journal.SessionClosed {
			return true, nil
		}
	}

	return false, nil
}

// mintReview enqueues the one review task over the session's attributed
// range. The dedup key shares the sweeper's review namespace ("review:<id>"),
// so a session review is exactly as idempotent as a task review.
func mintReview(ctx context.Context, s Store, in CloseInput, commits []Commit) (task.Task, bool, error) {
	head := commits[len(commits)-1] // oldest-first; head closes the range

	payload, err := json.Marshal(executor.ReviewPayload{
		Repo:         in.Repo,
		ReviewedTask: SyntheticTaskID(in.ID).String(),
		Item:         reviewItem(in),
		CommitSHA:    head.SHA,
		Extra:        reviewExtra(in, commits),
		// Pool reviews run autonomously (repo .crushrc grants the tools);
		// Model stays unset on purpose — the repo .crushrc is the only
		// model+effort carrier, a payload model would reset reasoning effort.
		Yolo:         true,
		RequireClean: new(!in.AllowDirty),
	})
	if err != nil {
		return task.Task{}, false, fmt.Errorf("session: marshal review payload: %w", err)
	}

	got, err := s.Enqueue(ctx, task.New{
		Type:     executor.TaskTypeReview,
		Project:  in.Project,
		Payload:  payload,
		DedupKey: review.ReviewDedupKey(SyntheticTaskID(in.ID)),
	})
	if err != nil {
		return task.Task{}, false, fmt.Errorf("session: enqueue review: %w", err)
	}

	return got, got.Attempts == 0 && got.Status == task.Pending, nil
}

// mintStatus enqueues the one done-prompt status task covering the session.
// The dedup key shares the sweeper's status namespace, keyed by the session's
// synthetic identity instead of a triggering completion.
func mintStatus(ctx context.Context, s Store, in CloseInput, commits []Commit) (task.Task, bool, error) {
	head := commits[len(commits)-1]

	payload, err := json.Marshal(executor.StatusPayload{
		Repo:    in.Repo,
		Project: in.Project,
		Completed: []executor.StatusCompletion{{
			TaskID:      SyntheticTaskID(in.ID).String(),
			Item:        excerpt(in.Summary),
			Commit:      head.SHA,
			CompletedAt: time.Now().UTC().Format(time.RFC3339),
		}},
		Yolo:         true,
		RequireClean: new(!in.AllowDirty),
	})
	if err != nil {
		return task.Task{}, false, fmt.Errorf("session: marshal status payload: %w", err)
	}

	got, err := s.Enqueue(ctx, task.New{
		Type:     executor.TaskTypeStatus,
		Project:  in.Project,
		Payload:  payload,
		DedupKey: status.StatusDedupKey(in.Project, SyntheticTaskID(in.ID)),
	})
	if err != nil {
		return task.Task{}, false, fmt.Errorf("session: enqueue status: %w", err)
	}

	return got, got.Attempts == 0 && got.Status == task.Pending, nil
}

// reviewItem is the bar the session's work is judged against: the operator's
// summary when given, else an honest default (the review executor requires a
// non-empty item).
func reviewItem(in CloseInput) string {
	if summary := trim(in.Summary); summary != "" {
		return summary
	}

	return "Interactive crush session (no goal summary recorded): the work is whatever the attributed commits below contain. Judge it like any other change."
}

// reviewExtra pins the concrete commit list and range instruction into the
// reviewer's focus block: a session spans many commits, not one SHA.
func reviewExtra(in CloseInput, commits []Commit) string {
	first, last := commits[0], commits[len(commits)-1]

	b := new(strings.Builder)
	b.WriteString("This review covers an INTERACTIVE session, not a single queued task. Commits attributed via the " +
		Trailer + ": " + in.ID + " footer, oldest first:\n\n")

	for _, c := range commits {
		b.WriteString("- " + c.SHA + " " + c.Subject + "\n")
	}

	b.WriteString("\nJudge the cumulative range " + first.SHA + "^.." + last.SHA +
		"; if " + first.SHA + "^ does not exist (root commit), review the listed commits individually.\n")

	return b.String()
}

// excerpt reduces the summary to the status window entry's one-line item
// pointer (same bound as the sweeper's item excerpts).
func excerpt(summary string) string {
	line := trim(summary)
	if line == "" {
		line = "interactive session (no summary supplied)"
	}

	if idx := strings.IndexByte(line, '\n'); idx >= 0 {
		line = line[:idx]
	}

	const maxLen = 200
	if len(line) > maxLen {
		line = line[:maxLen] + "…"
	}

	return line
}

func trim(s string) string { return strings.TrimSpace(s) }

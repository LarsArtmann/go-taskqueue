// Package review closes the loop around agent work: every completed agent
// task gets ONE review by a second agent, and a request_changes verdict can
// mint fix tasks. The reviewer only contributes findings — everything
// downstream (verdict bookkeeping, fix-task fan-out, dedup) is mechanical.
//
// Loop safety is structural: reviews are never reviewed (the sweeper only
// reviews the "agent" type), each task is reviewed at most once (the
// review:<task-id> dedup key), and fix tasks minted from findings are
// bounded by the pool's budget guard, which counts every enqueue — reviews
// included — against the daily cap.
package review

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
	"github.com/larsartmann/go-taskqueue/internal/watermark"
)

const (
	// ConsumerKey is the sweeper's identity in the watermarks table: the
	// persisted cursor shared by every sweeper over the same database, so
	// agent tasks completed while no sweeper was running still get their
	// review when the next pool starts.
	ConsumerKey = "review-sweeper"
)

// SweeperConfig controls one Sweeper.
type SweeperConfig struct {
	// Model, when set, is written into every review payload (and fix task).
	Model string
	// Autofix, when true, mints an agent fix task per finding of every
	// request_changes review. Off by default: verdict-only mode is the safe
	// first step; autofix closes the loop autonomously.
	Autofix bool
	// Log receives one summary line per sweep. nil logs nothing.
	Log *slog.Logger
	// PageSize bounds one fact-stream page; 0 selects the default.
	PageSize int
	// ProjectsDir resolves relative repo names in review payloads for the
	// default merge (absolute paths bypass it). "" disables merging of
	// relative-repo payloads.
	ProjectsDir string
	// Merge overrides the branch-merge primitive (test seam). nil selects
	// executor.MergeWorktreeBranch.
	Merge func(repoDir, branch, base, message string) error
}

// SweepStats summarizes one sweep pass.
type SweepStats struct {
	// Facts is the number of new facts consumed.
	Facts int
	// ReviewsEnqueued counts fresh review tasks this sweep created.
	ReviewsEnqueued int
	// ReviewsKnown counts completed agent tasks whose review task already
	// existed (dedup hit — the review ran, or is pending, elsewhere).
	ReviewsKnown int
	// FixesEnqueued counts fresh fix tasks minted from findings.
	FixesEnqueued int
	// Skipped counts completions that could not yield a review (foreign
	// payload shape, vanished task record).
	Skipped int
	// Merged counts approve-verdict worktree merges (branch → base + reap).
	Merged int
	// DocsSkipped counts completions whose derived diff is docs-only
	// (2026-10-11 ceremony cut: a reviewer of prose cannot catch what the
	// verify gate does not already prove; measured 52% derived-task
	// overhead). No derivation available (empty file list) still reviews.
	DocsSkipped int
}

// Sweeper turns the journal's completion facts into review work. It is safe
// for concurrent use (the cursor serializes sweeps; ticks and the --once
// drain watcher may call it from different goroutines).
//
// The struct shell deliberately stays per-sweeper (config fields and fact
// validation differ across the four sweepers); the shared pump is
// watermark.Cursor.Sweep.
//
// art-dupl:accept mirrored sweeper shell: config fields and fact validation
// differ per sweeper; the shared pump already lives in watermark.Cursor.
type Sweeper struct {
	store queue.Store
	cfg   SweeperConfig

	cur *watermark.Cursor
}

// NewSweeper returns a sweeper over store. The cursor resumes from the
// persisted checkpoint — completions recorded while no sweeper was running
// are reviewed on the next start. A first run bootstraps at the journal
// head: completions that predate the sweeper are not replayed (same
// semantics as the papdashboard bridge; review the gap via `tq show`, or
// rewind with `tq watermarks set review-sweeper SEQ`).
func NewSweeper(ctx context.Context, store queue.Store, cfg SweeperConfig) (*Sweeper, error) {
	cur, err := watermark.New(ctx, watermark.Config{
		Store:  store,
		Key:    ConsumerKey,
		Domain: "review sweep",
	})
	if err != nil {
		return nil, err
	}

	return &Sweeper{store: store, cfg: cfg, cur: cur}, nil
}

// Sweep consumes new facts since the last pass and enqueues review (and,
// with Autofix, fix) tasks. Idempotent by dedup: calling it twice never
// duplicates work, so a replayed page (crash between consumption and
// checkpoint) re-hits dedup keys instead of minting duplicates.
func (s *Sweeper) Sweep(ctx context.Context) (SweepStats, error) {
	var stats SweepStats

	err := s.cur.Sweep(ctx, func(ctx context.Context, f journal.Fact) error {
		stats.Facts++

		s.handleFact(ctx, f, &stats)

		return nil
	})

	return stats, err
}

// handleFact reacts to one completion fact: agent completions gain a review
// task, review completions may gain fix tasks (Autofix).
func (s *Sweeper) handleFact(ctx context.Context, fact journal.Fact, stats *SweepStats) {
	if fact.Type != journal.Completed {
		return
	}

	t, err := s.store.Get(ctx, task.ID(fact.TaskID))
	if err != nil {
		stats.Skipped++

		return
	}

	switch t.Type {
	case executor.TaskTypeAgent:
		s.enqueueReview(ctx, t, fact, stats)
	case executor.TaskTypeReview:
		s.mergeOnApprove(ctx, t, fact, stats)

		if s.cfg.Autofix {
			s.mintFixes(ctx, t, fact, stats)
		}
	}
}

// mergeOnApprove delivers worktree-per-agent work: an approve verdict on a
// review whose payload names a tq/ delivery branch merges the branch into
// its base (git plumbing — no checkout, base drift tolerated) and reaps
// the linked checkout plus the now-merged branch. The verify gate already
// proved the branch tip (the agent task completed; a red gate never
// completes), so approve is the last unmet condition of the O11 ruling.
// A conflicting merge refuses without touching anything — the branch stays
// the delivery for a human.
func (s *Sweeper) mergeOnApprove(ctx context.Context, t task.Task, fact journal.Fact, stats *SweepStats) {
	var payload executor.ReviewPayload

	if err := json.Unmarshal(t.Payload, &payload); err != nil || payload.Branch == "" || payload.BaseBranch == "" {
		return // not a worktree-mode review: nothing to merge
	}

	var result executor.ReviewResult

	if err := json.Unmarshal(fact.Detail, &result); err != nil || result.Verdict != executor.VerdictApprove {
		return // request_changes (or detail lost): the branch stays the delivery
	}

	repoDir, err := executor.ResolveRepoDir("review merge", s.cfg.ProjectsDir, payload.Repo)
	if err != nil {
		stats.Skipped++

		return
	}

	merge := s.cfg.Merge
	if merge == nil {
		merge = executor.MergeWorktreeBranch
	}

	if err := merge(repoDir, payload.Branch, payload.BaseBranch,
		"Merge "+payload.Branch+" (task "+payload.ReviewedTask+") — review-approved auto-merge"); err != nil {
		s.log("review: merge refused, branch stays the delivery", "task", payload.ReviewedTask,
			"branch", payload.Branch, "repo", payload.Repo, "err", err.Error())

		return
	}

	stats.Merged++

	if payload.WorktreeDir != "" {
		if err := executor.RemoveWorktree(repoDir, payload.WorktreeDir); err != nil {
			s.log("review: worktree cleanup degraded", "dir", payload.WorktreeDir, "err", err.Error())
		}
	}

	if err := executor.DeleteWorktreeBranch(repoDir, payload.Branch); err != nil {
		s.log("review: merged branch delete failed", "branch", payload.Branch, "err", err.Error())
	}

	s.log(
		"review: merged",
		"task",
		payload.ReviewedTask,
		"branch",
		payload.Branch,
		"into",
		payload.BaseBranch,
		"repo",
		payload.Repo,
	)
}

// enqueueReview mints the one review task for a completed agent task.
func (s *Sweeper) enqueueReview(ctx context.Context, t task.Task, fact journal.Fact, stats *SweepStats) {
	var agentPayload executor.AgentPayload

	if err := json.Unmarshal(
		t.Payload,
		&agentPayload,
	); err != nil || agentPayload.Repo == "" ||
		agentPayload.Prompt == "" {
		stats.Skipped++

		return
	}

	var agentResult executor.AgentResult

	_ = json.Unmarshal(fact.Detail, &agentResult) // absent/legacy detail is fine

	// Preflight-done completions never ran a session: there is nothing to
	// review, and minting a review would re-create the exact no-op agent
	// burn the done-preflight exists to kill.
	if agentResult.PreflightDone != "" {
		stats.Skipped++

		return
	}

	// Risky-types-only reviewing (2026-10-11 ceremony cut): a derived
	// docs-only diff carries no mechanical risk the verify gate has not
	// already proven away — skip the paid review turn. Unknown diff (no
	// derivation) still reviews: absence of evidence is not evidence of
	// docs.
	if docsOnlyChange(agentResult.FilesChanged) {
		stats.DocsSkipped++

		s.log("review: skipped docs-only change", "reviews", t.ID.String(), "repo", agentPayload.Repo)

		return
	}

	payload, err := json.Marshal(executor.ReviewPayload{
		Repo:         agentPayload.Repo,
		ReviewedTask: t.ID.String(),
		Item:         agentPayload.Prompt,
		CommitSHA:    agentResult.CommitSHA,
		FilesChanged: agentResult.FilesChanged,
		Branch:       agentResult.Branch,
		BaseBranch:   agentResult.BaseBranch,
		WorktreeDir:  agentResult.WorktreeDir,
		Model:        s.cfg.Model,
		Yolo:         agentPayload.Yolo,
	})
	if err != nil {
		stats.Skipped++

		return
	}

	got, err := s.store.Enqueue(ctx, task.New{
		Type:     executor.TaskTypeReview,
		Project:  t.Project,
		Priority: t.Priority,
		Payload:  payload,
		DedupKey: ReviewDedupKey(t.ID),
	})
	if err != nil {
		stats.Skipped++

		return
	}

	if got.Attempts == 0 && got.Status == task.Pending {
		stats.ReviewsEnqueued++

		s.log("review: enqueued", "task", got.ID.String(), "reviews", t.ID.String(), "repo", agentPayload.Repo)
	} else {
		stats.ReviewsKnown++
	}
}

// mintFixes turns a request_changes review's findings into agent fix tasks.
func (s *Sweeper) mintFixes(ctx context.Context, t task.Task, fact journal.Fact, stats *SweepStats) {
	var reviewPayload executor.ReviewPayload

	if err := json.Unmarshal(t.Payload, &reviewPayload); err != nil || reviewPayload.Repo == "" {
		stats.Skipped++

		return
	}

	var result executor.ReviewResult

	if err := json.Unmarshal(fact.Detail, &result); err != nil || result.Verdict != executor.VerdictRequestChanges {
		return // approve, or detail lost — nothing to fix
	}

	for _, finding := range result.Findings {
		// The finding's own cited commit wins; the reviewed run's commit is
		// the fallback (same precedence fixPrompt renders as prose).
		sha := strings.TrimSpace(finding.CommitSHA)
		if sha == "" {
			sha = strings.TrimSpace(reviewPayload.CommitSHA)
		}

		payload, err := json.Marshal(executor.AgentPayload{
			Repo:   reviewPayload.Repo,
			Model:  reviewPayload.Model,
			Yolo:   reviewPayload.Yolo,
			Prompt: fixPrompt(reviewPayload, finding),
			// Structured fix-ticket identity for the claim-time done-preflight
			// (rejected-SHA disposition) — the same data fixPrompt renders as
			// prose, machine-readable.
			RejectedSHA: sha,
			Anchor:      strings.TrimSpace(finding.Anchor),
		})
		if err != nil {
			stats.Skipped++

			continue
		}

		got, err := s.store.Enqueue(ctx, task.New{
			Type:     executor.TaskTypeAgent,
			Project:  t.Project,
			Priority: t.Priority,
			Payload:  payload,
			DedupKey: FixDedupKey(t.ID, finding.Title),
		})
		if err != nil {
			stats.Skipped++

			continue
		}

		if got.Attempts == 0 && got.Status == task.Pending {
			stats.FixesEnqueued++

			s.log("review: enqueued fix", "task", got.ID.String(), "review", t.ID.String(),
				"finding", finding.Title, "repo", reviewPayload.Repo)
		}
	}
}

func (s *Sweeper) log(msg string, args ...any) {
	if s.cfg.Log != nil {
		s.cfg.Log.Info(msg, args...)
	}
}

// ReviewDedupKey is the dedup identity of the review of one task: exactly
// one review per reviewed task, forever.
func ReviewDedupKey(reviewed task.ID) string {
	return "review:" + reviewed.String()
}

// FixDedupKey is the dedup identity of the fix task for one finding of one
// review: the same finding re-reported by the same review never spawns a
// second fix task.
func FixDedupKey(review task.ID, findingTitle string) string {
	sum := sha256.Sum256([]byte(findingTitle))

	return "reviewfix:" + review.String() + ":" + hex.EncodeToString(sum[:8])
}

// docsOnlyChange reports whether a derived file list is documentation and
// nothing else (prose formats, docs trees, licenses). An empty list is NOT
// docs-only: no derivation means no evidence, and the conservative answer
// keeps the review. One code-bearing file flips the whole change to
// reviewable.
func docsOnlyChange(files []string) bool {
	if len(files) == 0 {
		return false
	}

	for _, file := range files {
		if !docsLikePath(file) {
			return false
		}
	}

	return true
}

// docsLikePath classifies one changed path as documentation.
func docsLikePath(file string) bool {
	lower := strings.ToLower(strings.TrimSpace(file))
	if lower == "" {
		return false
	}

	base := lower
	if i := strings.LastIndexByte(base, '/'); i >= 0 {
		base = base[i+1:]
	}

	switch {
	case strings.HasPrefix(lower, "docs/"), strings.HasPrefix(lower, "doc/"):
		return true
	case base == "license", base == "licence", base == "copyright", base == "notice",
		base == "authors", base == "contributors":
		return true
	case strings.HasSuffix(base, ".md"), strings.HasSuffix(base, ".mdx"),
		strings.HasSuffix(base, ".txt"), strings.HasSuffix(base, ".rst"),
		strings.HasSuffix(base, ".adoc"):
		return true
	default:
		return false
	}
}

// fixPrompt builds the instruction for a fix task minted from one finding.
// The quoted original is the REVIEWED run's contract: its {{TASK_ID}}
// placeholder resolves to the reviewed task's id (what that agent actually
// saw). The fix task's OWN footer instruction keeps the placeholder —
// runAgent resolves it to the fix task's id at run time, so fix commits
// cross-reference the fix run, not the original.
func fixPrompt(payload executor.ReviewPayload, finding executor.ReviewFinding) string {
	var b strings.Builder

	original := strings.ReplaceAll(strings.TrimSpace(payload.Item), "{{TASK_ID}}", payload.ReviewedTask)

	b.WriteString(
		"A code reviewer rejected your earlier work on this task and filed one finding. Fix this finding.\n\n",
	)
	b.WriteString("## Original task\n\n" + original + "\n\n")
	b.WriteString("## Reviewer finding (" + finding.Severity + ")\n\n" + strings.TrimSpace(finding.Title) + "\n\n")

	if detail := strings.TrimSpace(finding.Detail); detail != "" {
		b.WriteString(detail + "\n\n")
	}

	if anchor := strings.TrimSpace(finding.Anchor); anchor != "" {
		b.WriteString(
			"The finding is anchored to this verbatim text: `" + anchor + "`. Use it to locate the site — never a line number.\n\n",
		)
	}

	sha := strings.TrimSpace(finding.CommitSHA)
	if sha == "" {
		sha = strings.TrimSpace(payload.CommitSHA)
	}

	if sha != "" {
		b.WriteString("The rejected change is (or was) commit " + sha + ".\n\n")
		b.WriteString(
			"Disposition for a dangling commit: verify the commit still exists (`git cat-file -e " + sha + "`). If it is gone (rebased away), re-anchor via the verbatim anchor text above and judge the CURRENT tree; the stale sha is context, not a requirement. If the anchor text is also gone, the finding no longer applies — state that in your final output instead of inventing a change.\n\n",
		)
	}

	b.WriteString(
		"Address the finding, keep the repository's contracts (AGENTS.md / docs), and make the repo's own gates " +
			"(build, vet, tests, format) pass before finishing. FIRST grep git log for your own footer below — existing " +
			"commits under this fix ticket's ID mean an earlier run already addressed the finding: verify the anchor " +
			"state, then close out as a no-op stating the evidence instead of redoing. Commit the fix with a message " +
			"ending in this exact footer line (you have explicit permission to commit for this task):\n\nTask-Queue-ID: {{TASK_ID}}\n\n" +
			"Footer convention: ONE Task-Queue-ID footer per commit — the FIX ticket's id above, never a second " +
			"footer for the original task; duplicate footers corrupt the queue↔git cross-reference.\n\nNever push.",
	)

	return b.String()
}

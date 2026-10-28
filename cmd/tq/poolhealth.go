// Command pool-health answers "is the pool ALIVE", not "is the process
// up": it folds the fact journal per repo into outcome counts, skip
// streaks, poisoned-repo DLQ backoff, and harvest recency. The
// 2026-10-11 outage class was a projection dead for three days while
// process checks stayed green — liveness is derived from WORK MOVING,
// not from serving.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/larsartmann/go-taskqueue/internal/harvest"
	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// Verdicts per repo: OK means work moved; WARN means the repo's work is
// stuck in a skip loop or starved of any outcome.
const (
	poolVerdictOK   = "OK"
	poolVerdictWarn = "WARN"
)

// poolWarnStreak is the skip-streak threshold: three consecutive
// environment-class requeues without an outcome mirrors the env-streak
// breaker's escalation point (AGENTS.md: "3 consecutive burn an attempt
// + escalate") — by then the loop is no longer transient.
const poolWarnStreak = 3

// poolSkipClasses are the requeue classes that mean "the environment
// refused, the task never ran": the claim→requeue-without-burn loop
// pool-health exists to surface. Question parks (the owner was asked)
// and legacy unknown facts do not count.
var poolSkipClasses = map[string]bool{
	queue.RequeueClassRateLimit: true,
	// "rate-limit-gate": the sibling fast-refuse class
	// (internal/queue.RequeueClassRateLimitGate, 02-28 §e2). Literal, not
	// the constant: this module pins the queue FACADE's released tag, and
	// the const only exists after the next facade release bumps the floor.
	"rate-limit-gate":           true,
	queue.RequeueClassGate:      true,
	queue.RequeueClassBudget:    true,
	queue.RequeueClassPreflight: true,
}

// poolTaskMeta is the per-task identity the fold needs; sourced from the
// tasks table (authoritative: the engine's enqueue facts carry only thin
// {project,type} details — S1 divergence D2 — so dedup keys and types
// cannot be read from fresh facts alone).
type poolTaskMeta struct {
	project string
	dedup   string
}

// RepoHealth is one project's folded liveness over the journal.
type RepoHealth struct {
	Repo            string         `json:"repo"`
	Enqueued        int            `json:"enqueued"`
	Completed       int            `json:"completed"`
	Failed          int            `json:"failed"`
	Dead            int            `json:"dead"`
	RequeuesByClass map[string]int `json:"requeuesByClass,omitempty"`
	SkipStreak      int            `json:"skipStreak"`
	DLQBackoffUntil *time.Time     `json:"dlqBackoffUntil,omitempty"`
	LastHarvest     *time.Time     `json:"lastHarvest,omitempty"`
	LastActivity    *time.Time     `json:"lastActivity,omitempty"`
	Verdict         string         `json:"verdict"`
	Warnings        []string       `json:"warnings,omitempty"`
}

// PoolHealthReport is the whole pool-health result.
type PoolHealthReport struct {
	Since       string       `json:"since"`
	FactsFolded int          `json:"factsFolded"`
	Repos       []RepoHealth `json:"repos"`
}

// harvestItemKeyPrefix mirrors harvest.ItemKey's "todo:" prefix (the
// harvester's per-item key); batched harvest tasks carry
// harvest.BatchKeyPrefix. Together they identify harvest-minted work.
const harvestItemKeyPrefix = "todo:"

// isHarvestEnqueue reports whether a task is harvest-minted backlog work
// (single item or batch), as opposed to a direct or bridge enqueue.
func isHarvestEnqueue(meta poolTaskMeta) bool {
	if meta.dedup == "" {
		return false
	}

	return strings.HasPrefix(meta.dedup, harvestItemKeyPrefix) ||
		strings.HasPrefix(meta.dedup, harvest.BatchKeyPrefix)
}

// poolHealthFrom folds the journal into per-repo health. Pure: no I/O,
// no clock — the since cutoff gates the window COUNTS while the skip
// streak and the DLQ backoff run over the whole range (a streak or
// backoff that predates the window is exactly the silent kind this
// command exists to catch). Terminal facts (completed, failed,
// dead-lettered, cancelled) reset a repo's streak — a turn that RAN
// proves the loop ended; Claimed deliberately does not, because the
// pathological loop IS claim→requeue.
//
// With dlqBackoff > 0, a repo whose latest dead-letter fact is fresher
// than the backoff window and that has no completion on record reports
// poisoned-repo backoff until lastDead+dlqBackoff — the harvest pool's
// Pacing verdict (surveyRepo) made queryable without journal digging.
// Mirrors: any Completed fact heals (the harvester checks the repo's
// completed tasks, ever), and dismissing the dead-lettered task heals
// (DismissDead emits the Cancelled fact for that task).
func poolHealthFrom(facts []journal.Fact, meta map[task.ID]poolTaskMeta, since, now time.Time, dlqBackoff time.Duration) PoolHealthReport {
	repos := make(map[string]*RepoHealth)

	// Backoff derivation state: dead-letter times per task, tasks whose
	// dead letter was dismissed (Cancelled fact on the same task), and
	// repos with any completion ever.
	deadAt := make(map[task.ID]time.Time)
	dismissed := make(map[task.ID]bool)
	everCompleted := make(map[string]bool)

	repoFor := func(id task.ID) *RepoHealth {
		m, ok := meta[id]
		if !ok || m.project == "" {
			return nil
		}

		r := repos[m.project]
		if r == nil {
			r = &RepoHealth{Repo: m.project}
			repos[m.project] = r
		}

		return r
	}

	for _, fact := range facts {
		id := task.ID(fact.TaskID)

		r := repoFor(id)
		if r == nil {
			continue
		}

		if r.LastActivity == nil || fact.Time.After(*r.LastActivity) {
			t := fact.Time
			r.LastActivity = &t
		}

		inWindow := !fact.Time.Before(since)

		switch fact.Type {
		case journal.Enqueued:
			if inWindow {
				r.Enqueued++
			}

			if isHarvestEnqueue(meta[task.ID(fact.TaskID)]) &&
				(r.LastHarvest == nil || fact.Time.After(*r.LastHarvest)) {
				t := fact.Time
				r.LastHarvest = &t
			}
		case journal.Completed:
			r.SkipStreak = 0
			everCompleted[r.Repo] = true

			if inWindow {
				r.Completed++
			}
		case journal.Failed, journal.DeadLettered, journal.Cancelled:
			r.SkipStreak = 0

			switch fact.Type {
			case journal.DeadLettered:
				deadAt[id] = fact.Time
			case journal.Cancelled:
				dismissed[id] = true
			}

			if inWindow {
				switch fact.Type {
				case journal.Failed:
					r.Failed++
				case journal.DeadLettered:
					r.Dead++
				}
			}
		case journal.Requeued:
			class := queue.RequeueClassUnknown

			var evidence queue.RequeueEvidence
			if json.Unmarshal(fact.Detail, &evidence) == nil && evidence.Class != "" {
				class = evidence.Class
			}

			if inWindow {
				if r.RequeuesByClass == nil {
					r.RequeuesByClass = make(map[string]int)
				}

				r.RequeuesByClass[class]++
			}

			if poolSkipClasses[class] {
				r.SkipStreak++
			}
		case journal.Claimed, journal.Released, journal.Reprioritized,
			journal.Heartbeat, journal.CancelRequested, journal.Orphaned,
			journal.SessionOpened, journal.SessionClosed,
			journal.QuestionAsked, journal.QuestionAnswered:
			// Observations: activity, but neither outcome nor skip.
		}
	}

	// Latest still-standing dead letter per repo: dismissal (a Cancelled
	// fact on the dead-lettered task) takes a dead letter out of the
	// backoff derivation, mirroring the harvester's table view where a
	// dismissed task is no longer Dead.
	lastLiveDead := make(map[string]time.Time)
	for id, at := range deadAt {
		if dismissed[id] {
			continue
		}

		project := meta[id].project
		if project == "" {
			continue
		}

		if best, ok := lastLiveDead[project]; !ok || at.After(best) {
			lastLiveDead[project] = at
		}
	}

	report := PoolHealthReport{
		Since:       since.Format(time.RFC3339),
		FactsFolded: len(facts),
		Repos:       make([]RepoHealth, 0, len(repos)),
	}

	for _, r := range repos {
		if r.SkipStreak >= poolWarnStreak {
			r.Verdict = poolVerdictWarn
			r.Warnings = append(r.Warnings, fmt.Sprintf(
				"skip streak %d: consecutive environment requeues with no outcome; the env-streak breaker escalates at %d",
				r.SkipStreak,
				poolWarnStreak,
			))
		}

		if r.Enqueued > 0 && r.Completed+r.Failed+r.Dead == 0 {
			r.Verdict = poolVerdictWarn
			r.Warnings = append(r.Warnings, fmt.Sprintf(
				"starved: %d enqueued in window with no completed/failed/dead outcome", r.Enqueued))
		}

		if dlqBackoff > 0 && !everCompleted[r.Repo] {
			if lastDead, ok := lastLiveDead[r.Repo]; ok {
				until := lastDead.Add(dlqBackoff)
				if until.After(now) {
					r.DLQBackoffUntil = &until
					r.Verdict = poolVerdictWarn
					r.Warnings = append(r.Warnings, fmt.Sprintf(
						"poisoned: recent dead-letter, DLQ backoff until %s (fix the repo or rescue dead tasks)",
						until.Format(time.RFC3339)))
				}
			}
		}

		if r.Verdict == "" {
			r.Verdict = poolVerdictOK
		}

		report.Repos = append(report.Repos, *r)
	}

	sort.Slice(report.Repos, func(i, j int) bool {
		return report.Repos[i].Repo < report.Repos[j].Repo
	})

	return report
}

// poolHealth drains the journal, joins the task identities, and folds.
// window and now give the fold its clock (poolHealthFrom stays pure);
// dlqBackoff > 0 turns on the poisoned-repo backoff surface.
func poolHealth(ctx context.Context, store queue.Store, window time.Duration, now time.Time, dlqBackoff time.Duration) (PoolHealthReport, error) {
	facts, err := drainAllFacts(ctx, store)
	if err != nil {
		return PoolHealthReport{}, err
	}

	tasks, err := store.List(ctx, queue.Filter{})
	if err != nil {
		return PoolHealthReport{}, fmt.Errorf("list tasks: %w", err)
	}

	meta := make(map[task.ID]poolTaskMeta, len(tasks))
	for _, t := range tasks {
		meta[t.ID] = poolTaskMeta{project: t.Project, dedup: t.DedupKey}
	}

	return poolHealthFrom(facts, meta, now.Add(-window), now, dlqBackoff), nil
}

// cmdPoolHealth implements `tq pool-health`: advisory, exit 0 even on
// WARN (like audit — the operator reads the verdict, scripts poll JSON).
func cmdPoolHealth(args []string) error {
	fs := flag.NewFlagSet("pool-health", flag.ExitOnError)
	since := fs.Duration("since", 168*time.Hour,
		"lookback window for outcome counts (skip streaks and DLQ backoff always scan the whole journal)")
	dlqBackoff := fs.Duration("dlq-backoff", 0,
		"surface each repo's poisoned-repo DLQ backoff, mirroring the harvest pool's --dlq-backoff; pass the same value the pool runs with (0 = off)")
	asJSON := fs.Bool("json", false, "JSON output of the pool-health report")
	db := dbFlag(fs)

	if err := fs.Parse(args); err != nil {
		return err
	}

	s := mustOpenStore(mustDeploymentFromDB(*db))
	defer s.Close()

	report, err := poolHealth(context.Background(), s, *since, time.Now(), *dlqBackoff)
	if err != nil {
		return err
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")

		if err := enc.Encode(report); err != nil {
			return fmt.Errorf("encode pool-health report: %w", err)
		}

		return nil
	}

	printPoolHealth(report)

	return nil
}

// printPoolHealth renders the per-repo table: verdict, outcome counts,
// streak, and recency of harvest vs any activity.
func printPoolHealth(report PoolHealthReport) {
	if len(report.Repos) == 0 {
		fmt.Println("(no task-bearing repos in journal)")
	}

	warned := 0

	for _, r := range report.Repos {
		requeues := 0
		for _, n := range r.RequeuesByClass {
			requeues += n
		}

		fmt.Printf(
			"%-4s %-24s enq=%-4d done=%-4d fail=%-3d dead=%-3d requeues=%-3d streak=%-3d harvest=%s activity=%s\n",
			r.Verdict,
			r.Repo,
			r.Enqueued,
			r.Completed,
			r.Failed,
			r.Dead,
			requeues,
			r.SkipStreak,
			ageOrNever(r.LastHarvest),
			ageOrNever(r.LastActivity),
		)

		for _, w := range r.Warnings {
			fmt.Printf("       ↳ %s\n", w)
		}

		if r.Verdict == poolVerdictWarn {
			warned++
		}
	}

	fmt.Printf("pool-health: %d repos, %d warn, %d facts folded, window since %s\n",
		len(report.Repos), warned, report.FactsFolded, report.Since)
}

// ageOrNever renders a timestamp's age, or "never" when absent.
func ageOrNever(t *time.Time) string {
	if t == nil {
		return "never"
	}

	return humanize.Time(*t)
}

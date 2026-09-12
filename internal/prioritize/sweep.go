// Package prioritize keeps the score cache warm: when unscored backlog
// items sit in a repo's working set, ONE batch-scorer task (executor type
// "prioritize") is minted per repo; when that task completes, its verdicts
// are cached (priority_scores) and applied to the matching PENDING tasks as
// task.reprioritized facts. The agent only contributes scores; everything
// downstream (batching, dedup, precedence, protection) is mechanical.
//
// Triggers are journal-driven (cursor "prioritize-sweeper",
// head-bootstrapped like every sweeper): a harvest enqueue means new
// unscored work; a scorer completion means verdicts to apply — and, right
// after applying, a re-check for items that entered the working set while
// the batch was in flight. The first sweep (BootMint) also mints for
// pre-cursor pending items, so enabling the sweeper on a pool with a
// standing backlog scores it without waiting for a fresh enqueue.
//
// Loop safety is structural: batch tasks are type "prioritize", never
// "agent", so they never trigger mints themselves; a batch's dedup key is
// the hash of its exact key set, so an unchanged working set never re-mints
// (a dead batch is retried only when the set changes — the DLQ is the human
// surface until then); and every mint pass runs under the pool's budget
// guard like every other enqueue.
package prioritize

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/harvest"
	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

const (
	// defaultPageSize bounds one Facts page per sweep iteration.
	defaultPageSize = 500

	// ConsumerKey is the sweeper's identity in the watermarks table: the
	// persisted cursor shared by every sweeper instance over the same
	// database, rewindable with `tq watermarks set prioritize-sweeper SEQ`
	// (replay is idempotent: cache upserts and same-value priority updates
	// write nothing new).
	ConsumerKey = "prioritize-sweeper"

	// SourceScorer stamps cached verdicts (queue.PriorityScore.Source):
	// the batch scorer's identity. The model itself lives in each repo's
	// crush config and is deliberately not claimed here.
	SourceScorer = "ai:batch-scorer"

	// batchPriority places scorer tasks in the machine band: they are
	// queue plumbing, not user work (same ruling as the cqa bridge).
	batchPriority = queue.MachineMin

	// itemKeyPrefix identifies harvest-minted backlog items among all
	// agent payloads: only harvest.ItemKey issues "todo:" keys, so
	// catchup/reviewfix/session mints never join a batch.
	itemKeyPrefix = "todo:"

	// PrioritySourceAI is the task.reprioritized source label for applied
	// verdicts (ADR-0015 §3 ladder: ai).
	PrioritySourceAI = "ai"
)

// SweeperConfig controls one Sweeper.
type SweeperConfig struct {
	// Model, when set, overrides the scorer model in every batch payload.
	// Empty (the recommended default) leaves model+effort to each repo's
	// crush config.
	Model string
	// Yolo marks the scorer autonomous (same contract as agent payloads).
	Yolo bool
	// AllowDirty lets the scorer start on a dirty tree. Default false: the
	// scorer reads the repo for context and should see committed state.
	AllowDirty bool
	// BootMint makes the FIRST sweep also mint for the pre-cursor working
	// set (items already pending when the sweeper was enabled), instead of
	// waiting for the next enqueue to trigger.
	BootMint bool
	// Log receives one summary line per mint/apply. nil logs nothing.
	Log *slog.Logger
	// PageSize bounds one fact-stream page; 0 selects the default.
	PageSize int
}

// SweepStats summarizes one sweep pass.
type SweepStats struct {
	// Facts is the number of new facts consumed.
	Facts int
	// BatchesEnqueued counts fresh scorer tasks this sweep created.
	BatchesEnqueued int
	// BatchesKnown counts mints whose batch task already existed (dedup
	// hit — the batch ran, or is in flight, elsewhere).
	BatchesKnown int
	// VerdictsCached counts verdicts written to the score cache.
	VerdictsCached int
	// TasksReprioritized counts PENDING tasks whose priority changed by an
	// applied verdict.
	TasksReprioritized int
	// Skipped counts facts that could not yield work (vanished records,
	// foreign payload shapes, unparsable results).
	Skipped int
}

// Sweeper turns the journal's enqueue and scorer-completion facts into
// batch-scorer mints and score applications. It is safe for concurrent use
// (a mutex serializes sweeps).
type Sweeper struct {
	store queue.Store
	cfg   SweeperConfig

	mu         sync.Mutex
	watermark  int64
	persisted  int64 // last checkpoint written to the watermarks table
	bootMinted bool
}

// NewSweeper returns a sweeper over store. The cursor resumes from the
// persisted checkpoint; a first run bootstraps at the journal head (same
// semantics as the review and dlqfix sweepers).
func NewSweeper(ctx context.Context, store queue.Store, cfg SweeperConfig) (*Sweeper, error) {
	if cfg.PageSize <= 0 {
		cfg.PageSize = defaultPageSize
	}

	persisted, exists, err := store.Watermark(ctx, ConsumerKey)
	if err != nil {
		return nil, fmt.Errorf("prioritize sweep: read watermark: %w", err)
	}

	// seq 0 with a row is a real cursor ("bootstrapped on an empty
	// journal, consumed nothing yet"): resume from it, do not jump to head.
	if exists {
		return &Sweeper{store: store, cfg: cfg, watermark: persisted, persisted: persisted}, nil
	}

	head, err := store.HeadSeq(ctx)
	if err != nil {
		return nil, fmt.Errorf("prioritize sweep: read journal head: %w", err)
	}

	if err := store.SaveWatermark(ctx, ConsumerKey, head); err != nil {
		return nil, fmt.Errorf("prioritize sweep: persist bootstrap watermark: %w", err)
	}

	return &Sweeper{store: store, cfg: cfg, watermark: head, persisted: head}, nil
}

// Sweep consumes new facts since the last pass: harvest enqueues trigger a
// mint check for their repo, scorer completions apply their verdicts.
// Idempotent by dedup keys, cache upserts and same-value transition guards
// — a replayed page (crash between consumption and checkpoint) re-hits
// dedup instead of duplicating work. The cursor checkpoints after each
// page; a failed checkpoint stops the sweep.
func (s *Sweeper) Sweep(ctx context.Context) (SweepStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var stats SweepStats

	if !s.bootMinted {
		s.bootMinted = true

		if s.cfg.BootMint {
			s.mintWorkingSet(ctx, &stats)
		}
	}

	// A pending checkpoint gates sweeping (same rule as the bridge and the
	// dlqfix sweeper): retry it before consuming anything new.
	if s.watermark > s.persisted {
		if err := s.store.SaveWatermark(ctx, ConsumerKey, s.watermark); err != nil {
			return stats, fmt.Errorf("prioritize sweep: checkpoint %d: %w", s.watermark, err)
		}

		s.persisted = s.watermark
	}

	for {
		facts, err := s.store.Facts(ctx, s.watermark, s.cfg.PageSize)
		if err != nil {
			return stats, fmt.Errorf("prioritize sweep: read facts after %d: %w", s.watermark, err)
		}

		for _, f := range facts {
			s.watermark = f.Seq
			stats.Facts++

			s.handleFact(ctx, f, &stats)
		}

		// Page end: checkpoint AFTER the last consumed fact — never
		// before, or a crash would silently skip the page's triggers.
		if len(facts) > 0 {
			if err := s.store.SaveWatermark(ctx, ConsumerKey, s.watermark); err != nil {
				return stats, fmt.Errorf("prioritize sweep: checkpoint %d: %w", s.watermark, err)
			}

			s.persisted = s.watermark
		}

		if len(facts) < s.cfg.PageSize {
			return stats, nil
		}
	}
}

// handleFact reacts to one fact: harvest enqueues gain a batch mint check,
// completed scorer tasks gain verdict application (and a follow-up mint
// check for items that arrived in flight).
func (s *Sweeper) handleFact(ctx context.Context, fact journal.Fact, stats *SweepStats) {
	if fact.Type == journal.Enqueued {
		s.onEnqueued(ctx, fact, stats)
	}

	if fact.Type == journal.Completed {
		s.onCompleted(ctx, fact, stats)
	}
}

// onEnqueued mints a batch when a freshly harvested backlog item leaves
// its repo with unscored, uncovered work.
func (s *Sweeper) onEnqueued(ctx context.Context, fact journal.Fact, stats *SweepStats) {
	t, err := s.store.Get(ctx, task.ID(fact.TaskID))
	if err != nil {
		stats.Skipped++

		return
	}

	if t.Type != executor.TaskTypeAgent {
		return // someone else's enqueue — not a skip, just not ours
	}

	item, ok := harvest.PayloadItemOf(t)
	if !ok || !strings.HasPrefix(item.Key, itemKeyPrefix) {
		return // agent task, but not a harvested backlog item
	}

	s.mintRepo(ctx, item.Repo, t.Project, stats)
}

// onCompleted caches a finished batch's verdicts and applies them to the
// repo's PENDING tasks, then re-checks the working set: items enqueued
// while the batch was in flight still need their own batch.
func (s *Sweeper) onCompleted(ctx context.Context, fact journal.Fact, stats *SweepStats) {
	t, err := s.store.Get(ctx, task.ID(fact.TaskID))
	if err != nil {
		stats.Skipped++

		return
	}

	if t.Type != executor.TaskTypePrioritize {
		return // someone else's completion — not a skip, just not ours
	}

	var result executor.PrioritizeResult

	if err := json.Unmarshal(fact.Detail, &result); err != nil || len(result.Verdicts) == 0 {
		stats.Skipped++

		s.warn("prioritize: unparsable scorer result", "task", t.ID.String(), "err", err)

		return
	}

	byKey := make(map[string]executor.PrioritizeVerdict, len(result.Verdicts))

	for _, verdict := range result.Verdicts {
		if err := s.store.SavePriorityScore(ctx, queue.PriorityScore{
			ItemKey:       verdict.ItemKey,
			Score:         verdict.Score,
			EffortMinutes: verdict.EffortMinutes,
			Source:        SourceScorer,
			Reasoning:     verdict.Reasoning,
			ScoredAt:      fact.Time.UnixMilli(),
		}); err != nil {
			s.warn("prioritize: cache verdict failed", "item", verdict.ItemKey, "err", err)

			continue
		}

		stats.VerdictsCached++
		byKey[verdict.ItemKey] = verdict
	}

	if len(byKey) > 0 {
		s.applyVerdicts(ctx, t.Project, byKey, stats)
	}

	s.mintRepo(ctx, s.repoRef(ctx, t), t.Project, stats)
}

// applyVerdicts updates the repo's PENDING backlog tasks to their cached
// scores. Precedence is honored structurally: marker items (the payload's
// pinned markerLevel) are never touched — marker > AI; hot/machine tasks
// are never touched (RepriMutable); and the score clamps into the backlog
// band, so a verdict can never smuggle a task into hot.
func (s *Sweeper) applyVerdicts(
	ctx context.Context,
	project string,
	byKey map[string]executor.PrioritizeVerdict,
	stats *SweepStats,
) {
	for _, entry := range s.pendingItems(ctx, project) {
		verdict, scored := byKey[entry.item.Key]
		if !scored || entry.item.MarkerLevel > 0 || !harvest.RepriMutable(entry.task.Priority) {
			continue
		}

		priority := queue.ClampBacklog(verdict.Score)
		if priority == entry.task.Priority {
			continue // value-idempotent: no fact
		}

		reason := verdict.Reasoning
		if reason == "" {
			reason = fmt.Sprintf("batch scorer score %d", verdict.Score)
		}

		if err := s.store.UpdatePendingPriority(ctx, entry.task.ID, priority, PrioritySourceAI, reason); err != nil {
			s.warn("prioritize: reprioritize failed",
				"task", entry.task.ID.String(), "item", entry.item.Key, "err", err)

			continue
		}

		stats.TasksReprioritized++

		s.log("prioritize: applied verdict",
			"task", entry.task.ID.String(),
			"item", entry.item.Key,
			"old", entry.task.Priority,
			"new", priority)
	}
}

// workingItem pairs a PENDING task with its harvested item identity.
type workingItem struct {
	task task.Task
	item harvest.PayloadItem
}

// pendingItems lists the repo's PENDING harvested backlog tasks.
func (s *Sweeper) pendingItems(ctx context.Context, project string) []workingItem {
	agentType := executor.TaskTypeAgent

	tasks, err := s.store.List(ctx, queue.Filter{Project: &project, Type: &agentType})
	if err != nil {
		return nil
	}

	var items []workingItem

	for _, t := range tasks {
		if t.Status != task.Pending {
			continue
		}

		item, ok := harvest.PayloadItemOf(t)
		if !ok || !strings.HasPrefix(item.Key, itemKeyPrefix) {
			continue
		}

		items = append(items, workingItem{task: t, item: item})
	}

	return items
}

// coveredKeys returns the item keys already claimed by an in-flight batch
// of this repo (pending or running scorer tasks).
func (s *Sweeper) coveredKeys(ctx context.Context, project string) map[string]bool {
	scorerType := executor.TaskTypePrioritize

	tasks, err := s.store.List(ctx, queue.Filter{Project: &project, Type: &scorerType})
	if err != nil {
		return nil
	}

	covered := map[string]bool{}

	for _, t := range tasks {
		if t.Status != task.Pending && t.Status != task.Running {
			continue
		}

		var payload executor.PrioritizePayload
		if json.Unmarshal(t.Payload, &payload) != nil {
			continue
		}

		for _, item := range payload.Items {
			covered[item.Key] = true
		}
	}

	return covered
}

// mintRepo mints ONE batch covering the repo's unscored, uncovered backlog
// items — but only when that set is non-empty. The dedup key hashes the
// exact key set, so an unchanged set never mints twice.
func (s *Sweeper) mintRepo(ctx context.Context, repoRef, project string, stats *SweepStats) {
	pending := s.pendingItems(ctx, project)
	covered := s.coveredKeys(ctx, project)

	var batch []executor.PrioritizeItem

	for _, entry := range pending {
		if covered[entry.item.Key] {
			continue
		}

		if _, cached, err := s.store.PriorityScore(ctx, entry.item.Key); err == nil && cached {
			continue
		}

		batch = append(batch, executor.PrioritizeItem{
			Key:  entry.item.Key,
			Text: entry.item.Text,
		})
	}

	if len(batch) == 0 {
		return
	}

	sort.Slice(batch, func(i, j int) bool { return batch[i].Key < batch[j].Key })

	keys := make([]string, len(batch))
	for i, item := range batch {
		keys[i] = item.Key
	}

	requireClean := !s.cfg.AllowDirty

	payload, err := json.Marshal(executor.PrioritizePayload{
		Repo:         repoRef,
		RepoName:     project,
		Items:        batch,
		Model:        s.cfg.Model,
		Yolo:         s.cfg.Yolo,
		RequireClean: &requireClean,
	})
	if err != nil {
		stats.Skipped++

		return
	}

	got, err := s.store.Enqueue(ctx, task.New{
		Type:     executor.TaskTypePrioritize,
		Project:  project,
		Priority: batchPriority,
		Payload:  payload,
		DedupKey: DedupKey(project, keys),
	})
	if err != nil {
		stats.Skipped++

		return
	}

	if got.Attempts == 0 && got.Status == task.Pending {
		stats.BatchesEnqueued++

		s.log("prioritize: enqueued batch",
			"task", got.ID.String(), "repo", project, "items", len(batch))
	} else {
		stats.BatchesKnown++
	}
}

// mintWorkingSet boot-mints every repo that currently holds unscored
// backlog work (the BootMint pass over the pre-cursor standing backlog).
func (s *Sweeper) mintWorkingSet(ctx context.Context, stats *SweepStats) {
	agentType := executor.TaskTypeAgent
	pendingStatus := task.Pending

	tasks, err := s.store.List(ctx, queue.Filter{Type: &agentType, Status: &pendingStatus})
	if err != nil {
		return
	}

	seen := map[string]string{} // project -> repoRef

	for _, t := range tasks {
		item, ok := harvest.PayloadItemOf(t)
		if !ok || !strings.HasPrefix(item.Key, itemKeyPrefix) {
			continue
		}

		if _, done := seen[t.Project]; !done {
			seen[t.Project] = item.Repo
		}
	}

	for _, project := range sortedKeys(seen) {
		s.mintRepo(ctx, seen[project], project, stats)
	}
}

// repoRef reads the batch's own repo reference back from its payload (the
// apply-side follow-up mint reuses the exact reference the batch carried).
func (s *Sweeper) repoRef(_ context.Context, t task.Task) string {
	var payload executor.PrioritizePayload
	if json.Unmarshal(t.Payload, &payload) == nil && payload.Repo != "" {
		return payload.Repo
	}

	return t.Project
}

func (s *Sweeper) log(msg string, args ...any) {
	if s.cfg.Log != nil {
		s.cfg.Log.Info(msg, args...)
	}
}

func (s *Sweeper) warn(msg string, args ...any) {
	if s.cfg.Log != nil {
		s.cfg.Log.Warn(msg, args...)
	}
}

// DedupKey is the dedup identity of one repo's batch over one exact key
// set: "prioritize:<repo>:<hash>". A set that changes (new item, item
// reworded and re-keyed) mints a fresh batch; an unchanged set never mints
// twice — even after the first batch died, whose recovery then rides the
// next set change (the DLQ is the human surface until then).
func DedupKey(repo string, keys []string) string {
	sorted := append([]string(nil), keys...)
	sort.Strings(sorted)

	sum := sha256.Sum256([]byte(repo + "\x00" + strings.Join(sorted, "\x00")))

	return "prioritize:" + repo + ":" + hex.EncodeToString(sum[:])[:16]
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	return keys
}

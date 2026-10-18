// Package prioritize scores the frontier, just in time: only the next-K
// claimable backlog items of a repo — the scoring frontier (O3
// 2026-10-05) — are worth scorer spend, so ONE batch-scorer task
// (executor type "prioritize") covers that window; when the task
// completes, its verdicts are cached (priority_scores) and applied to the
// matching PENDING tasks as task.reprioritized facts. The agent only
// contributes scores; everything downstream (batching, dedup, precedence,
// protection) is mechanical.
//
// The frontier ROLLS: a claim drains the head, so the claim fact itself
// mints the refill batch; a requeue SUPERSEDES the item's cached verdict
// (a durable Source stamp) because the claim attempt may have moved the
// repo under the score; an edited item re-keys (harvest.ItemKey hashes
// the text) and re-enters unscored. Time-based verdict aging is RETIRED —
// spend scales with work claimed, not with backlog size or score age.
//
// Triggers are journal-driven (cursor "prioritize-sweeper",
// head-bootstrapped like every sweeper): a harvest enqueue means new
// unscored work; a scorer completion means verdicts to apply — and, right
// after applying, a re-check for items that entered the frontier while
// the batch was in flight. The first sweep (BootMint) also mints for
// pre-cursor pending items, so enabling the sweeper on a pool with a
// standing backlog scores its frontier without waiting for a fresh
// enqueue.
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
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/harvest"
	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
	"github.com/larsartmann/go-taskqueue/internal/watermark"
)

const (
	// ConsumerKey is the sweeper's identity in the watermarks table: the
	// persisted cursor shared by every sweeper instance over the same
	// database, rewindable with `tq watermarks set prioritize-sweeper SEQ`
	// (replay is idempotent: cache upserts and same-value priority updates
	// write nothing new).
	ConsumerKey = "prioritize-sweeper"

	// SourceScorer stamps cached verdicts (queue.PriorityScore.Source):
	// the batch scorer's identity. When SweeperConfig.Model is set the
	// source gains a ":<model>" suffix (see scorerSource), so a verdict
	// scored by a different model is treated as stale and re-scored — a
	// provider retraining behind a fixed name, or a model swap, must not
	// rank fresh work against a stale model's verdict (turnstone lesson 14).
	SourceScorer = "ai:batch-scorer"

	// SourceSuperseded is the durable invalidation stamp a REQUEUE writes
	// over a cached verdict (O3 2026-10-05): the item was claimed and put
	// back, so the repo may have moved under the score and it must be
	// re-issued before ranking again. The stamp trips the same
	// Source-mismatch gate as a model change, so the item re-joins the
	// next frontier batch even though an old batch covered it; the
	// re-score's upsert overwrites the stamp with the real scorer source.
	SourceSuperseded = SourceScorer + ":superseded"

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

	// DefaultFrontierK is the default scoring-frontier size
	// (SweeperConfig.FrontierK): how many next-claimable backlog items per
	// repo form the JIT scoring surface. Small by design — scorer spend
	// scales with the claims the window serves, not with backlog size.
	DefaultFrontierK = 8
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
	// FrontierK is the JIT scoring-frontier size (O3 2026-10-05): minting
	// considers only the next-K claimable backlog items of a repo, in
	// claim order (ADR-0015 §4 effective priority). The cache rolls with
	// the window: verdicts enter as items enter the frontier, leave as it
	// drains (claims) or as facts invalidate them (requeue supersedes,
	// item edits re-key). Zero selects DefaultFrontierK.
	FrontierK int
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
	// foreign payload shapes, unparsable results). A refused store
	// operation is not counted here: it aborts the sweep before the
	// checkpoint so the fact redelivers.
	Skipped int
	// CachePruned counts stale cache entries the prune pass evicted:
	// verdicts whose item key no longer belongs to any PENDING backlog
	// task (the item was reworded and re-keyed, checked off, or its task
	// reached a terminal state).
	CachePruned int
}

// Sweeper turns the journal's enqueue and scorer-completion facts into
// batch-scorer mints and score applications. It is safe for concurrent use
// (the mutex serializes the first-sweep boot mint; the cursor serializes
// the page pump).
type Sweeper struct {
	store queue.Store
	cfg   SweeperConfig

	mu         sync.Mutex // guards bootMinted
	cur        *watermark.Cursor
	bootMinted bool
}

// NewSweeper returns a sweeper over store. The cursor resumes from the
// persisted checkpoint; a first run bootstraps at the journal head (same
// semantics as the review and dlqfix sweepers).
func NewSweeper(ctx context.Context, store queue.Store, cfg SweeperConfig) (*Sweeper, error) {
	cur, err := watermark.New(ctx, watermark.Config{
		Store:  store,
		Key:    ConsumerKey,
		Domain: "prioritize sweep",
	})
	if err != nil {
		return nil, err
	}

	return &Sweeper{store: store, cfg: cfg, cur: cur}, nil
}

// Sweep consumes new facts since the last pass: harvest enqueues trigger a
// mint check for their repo, scorer completions apply their verdicts.
// Idempotent by dedup keys, cache upserts and same-value transition guards
// — a replayed page (crash between consumption and checkpoint) re-hits
// dedup instead of duplicating work.
//
// A store refusal — a transient failure while reading a trigger record,
// caching/applying verdicts, or minting a batch — aborts the sweep BEFORE
// the checkpoint: the fact redelivers on the next pass instead of silently
// dropping the work forever (dlqfix 13-15 §e pattern). Deterministic skips
// (vanished records, foreign shapes, done-guard dedup hits) stay
// checkpointed past: retrying them can never succeed.
func (s *Sweeper) Sweep(ctx context.Context) (SweepStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var stats SweepStats

	if !s.bootMinted {
		s.bootMinted = true

		if s.cfg.BootMint {
			if err := s.mintWorkingSet(ctx, &stats); err != nil {
				return stats, err
			}
		}
	}

	err := s.cur.Sweep(ctx, func(ctx context.Context, f journal.Fact) error {
		stats.Facts++

		return s.handleFact(ctx, f, &stats)
	})

	return stats, err
}

// handleFact reacts to one fact: harvest enqueues gain a batch mint check,
// claims drain the frontier (the refill mint fires at claim time — a
// completion can lag an agent run by hours), completed scorer tasks gain
// verdict application (and a follow-up mint check for items that arrived
// in flight), requeues supersede the item's verdict. A store refusal
// propagates (checkpoint held); deterministic skips never abort the sweep.
func (s *Sweeper) handleFact(ctx context.Context, fact journal.Fact, stats *SweepStats) error {
	switch fact.Type {
	case journal.Enqueued:
		return s.onEnqueued(ctx, fact, stats)
	case journal.Claimed:
		return s.onClaimed(ctx, fact, stats)
	case journal.Completed:
		return s.onCompleted(ctx, fact, stats)
	case journal.Requeued:
		return s.onRequeued(ctx, fact, stats)
	}

	return nil
}

// refuse reports a store refusal and returns the error that holds the
// sweep's checkpoint: a refused operation must not advance past the fact,
// or the verdict/mint it carries never happens without an operator
// watermark rewind (mirror of the dlqfix refuseMint pattern).
func (s *Sweeper) refuse(what, id string, err error) error {
	s.warn("prioritize: store refusal — checkpoint held",
		"what", what, "id", id, "err", err)

	return fmt.Errorf("prioritize: %s refused for %s: %w", what, id, err)
}

// backlogItemOf resolves a fact's task to a harvested backlog item, or
// reports not-ours (someone else's enqueue/claim/completion — not a
// skip, just not ours). A transient read failure refuses (checkpoint
// held); a vanished record skips past.
func (s *Sweeper) backlogItemOf(ctx context.Context, fact journal.Fact, stats *SweepStats) (workingItem, bool, error) {
	t, err := s.store.Get(ctx, task.ID(fact.TaskID))
	if err != nil {
		if watermark.PermanentOutcome(err) {
			stats.Skipped++

			return workingItem{}, false, nil
		}

		return workingItem{}, false, s.refuse("read task record", fact.TaskID, err)
	}

	item, ok := harvest.PayloadItemOf(t)
	if !ok || !strings.HasPrefix(item.Key, itemKeyPrefix) || t.Type != executor.TaskTypeAgent {
		return workingItem{}, false, nil
	}

	return workingItem{task: t, item: item}, true, nil
}

// onClaimed mints the frontier refill: the claimed item left the PENDING
// working set, so the next-K window slid and its newest member may be
// unscored (O3 2026-10-05: spend follows claims).
func (s *Sweeper) onClaimed(ctx context.Context, fact journal.Fact, stats *SweepStats) error {
	entry, ok, err := s.backlogItemOf(ctx, fact, stats)
	if err != nil {
		return err
	}

	if !ok {
		return nil
	}

	return s.mintRepo(ctx, entry.item.Repo, entry.task.Project, stats)
}

// onRequeued supersedes the item's cached verdict (a claim attempt may
// have moved the repo under the score — it must be re-issued) and mints
// the re-score batch right away: the superseded stamp makes the item
// re-join the frontier batch even though an old batch covered it.
func (s *Sweeper) onRequeued(ctx context.Context, fact journal.Fact, stats *SweepStats) error {
	entry, ok, err := s.backlogItemOf(ctx, fact, stats)
	if err != nil {
		return err
	}

	if !ok {
		return nil
	}

	score, cached, err := s.store.PriorityScore(ctx, entry.item.Key)
	switch {
	case err != nil:
		// The supersede stamp is the invalidation — losing it silently
		// would rank fresh work against a stale verdict, so it refuses.
		return s.refuse("read cached verdict", entry.item.Key, err)
	case cached && score.Source == s.scorerSource():
		score.Source = SourceSuperseded

		if err := s.store.SavePriorityScore(ctx, score); err != nil {
			return s.refuse("supersede cached verdict", entry.item.Key, err)
		}
	}

	return s.mintRepo(ctx, entry.item.Repo, entry.task.Project, stats)
}

// onEnqueued mints a batch when a freshly harvested backlog item leaves
// its repo with unscored, uncovered frontier work.
func (s *Sweeper) onEnqueued(ctx context.Context, fact journal.Fact, stats *SweepStats) error {
	entry, ok, err := s.backlogItemOf(ctx, fact, stats)
	if err != nil {
		return err
	}

	if !ok {
		return nil
	}

	return s.mintRepo(ctx, entry.item.Repo, entry.task.Project, stats)
}

// onCompleted caches a finished batch's verdicts and applies them to the
// repo's PENDING tasks, then re-checks the working set: items enqueued
// while the batch was in flight still need their own batch.
func (s *Sweeper) onCompleted(ctx context.Context, fact journal.Fact, stats *SweepStats) error {
	t, err := s.store.Get(ctx, task.ID(fact.TaskID))
	if err != nil {
		if watermark.PermanentOutcome(err) {
			stats.Skipped++

			return nil
		}

		return s.refuse("read completed record", fact.TaskID, err)
	}

	if t.Type != executor.TaskTypePrioritize {
		return nil // someone else's completion — not a skip, just not ours
	}

	var result executor.PrioritizeResult

	if err := json.Unmarshal(fact.Detail, &result); err != nil || len(result.Verdicts) == 0 {
		stats.Skipped++

		s.warn("prioritize: unparsable scorer result", "task", t.ID.String(), "err", err)

		return nil
	}

	byKey := make(map[string]executor.PrioritizeVerdict, len(result.Verdicts))

	for _, verdict := range result.Verdicts {
		if err := s.store.SavePriorityScore(ctx, queue.PriorityScore{
			ItemKey:       verdict.ItemKey,
			Score:         verdict.Score,
			EffortMinutes: verdict.EffortMinutes,
			Source:        s.scorerSource(),
			Reasoning:     verdict.Reasoning,
			ScoredAt:      fact.Time.UnixMilli(),
		}); err != nil {
			// A lost verdict cache write would silently drop the scorer's
			// spend, so it refuses; the redelivery re-applies idempotently.
			return s.refuse("cache verdict", verdict.ItemKey, err)
		}

		stats.VerdictsCached++
		byKey[verdict.ItemKey] = verdict
	}

	if len(byKey) > 0 {
		if err := s.applyVerdicts(ctx, t.Project, byKey, stats); err != nil {
			return err
		}
	}

	s.pruneCache(ctx, stats)

	return s.mintRepo(ctx, s.repoRef(ctx, t), t.Project, stats)
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
) error {
	items, err := s.pendingItems(ctx, project)
	if err != nil {
		return err
	}

	for _, entry := range items {
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
			if watermark.PermanentOutcome(err) {
				// Claimed (or moved) between the listing and the update:
				// the next verdict cycle re-scores it — skip past.
				s.warn("prioritize: reprioritize skipped", "task", entry.task.ID.String(), "err", err)

				continue
			}

			return s.refuse("reprioritize task", entry.task.ID.String(), err)
		}

		stats.TasksReprioritized++

		s.log("prioritize: applied verdict",
			"task", entry.task.ID.String(),
			"item", entry.item.Key,
			"old", entry.task.Priority,
			"new", priority)
	}

	return nil
}

// workingItem pairs a PENDING task with its harvested item identity.
type workingItem struct {
	task task.Task
	item harvest.PayloadItem
}

// frontierK is the configured frontier size; zero selects
// DefaultFrontierK.
func (s *Sweeper) frontierK() int {
	if s.cfg.FrontierK > 0 {
		return s.cfg.FrontierK
	}

	return DefaultFrontierK
}

// frontier keeps the next-K claimable items of the repo, in claim order —
// the JIT scoring surface (O3 2026-10-05): the scorer only ever sees the
// head of the queue workers are about to claim, so spend scales with
// claims, not backlog. Claim order mirrors the ClaimDue query (ADR-0015
// §4): effective priority (stored + bounded age bonus) DESC, then
// created_at ASC, id ASC — the queue's constants are referenced, never
// re-declared, so the two orderings cannot drift.
func (s *Sweeper) frontier(items []workingItem) []workingItem {
	k := s.frontierK()
	if len(items) <= k {
		return items
	}

	now := time.Now()

	sorted := append([]workingItem(nil), items...)

	sort.Slice(sorted, func(i, j int) bool {
		ei, ej := effectivePriority(now, sorted[i].task), effectivePriority(now, sorted[j].task)
		if ei != ej {
			return ei > ej
		}

		if !sorted[i].task.CreatedAt.Equal(sorted[j].task.CreatedAt) {
			return sorted[i].task.CreatedAt.Before(sorted[j].task.CreatedAt)
		}

		return sorted[i].task.ID < sorted[j].task.ID
	})

	return sorted[:k]
}

// effectivePriority mirrors the claim query's ordering key: stored
// priority plus the age bonus ((now-created)/PriorityAgingDaysPerPoint
// days, capped at PriorityAgingMaxBonus), float math matching the SQL
// expression in companion/claims.go.
func effectivePriority(now time.Time, t task.Task) float64 {
	bonus := now.Sub(t.CreatedAt).Hours() / 24 / float64(queue.PriorityAgingDaysPerPoint)
	if bonus > queue.PriorityAgingMaxBonus {
		bonus = queue.PriorityAgingMaxBonus
	}

	return float64(t.Priority) + bonus
}

// pendingItems lists the repo's PENDING harvested backlog tasks. A listing
// failure refuses: minting (or applying) against a silently-empty working
// set would drop work or mass-evict the cache.
func (s *Sweeper) pendingItems(ctx context.Context, project string) ([]workingItem, error) {
	agentType := executor.TaskTypeAgent

	tasks, err := s.store.List(ctx, queue.Filter{Project: &project, Type: &agentType})
	if err != nil {
		return nil, fmt.Errorf("list pending items: %w", err)
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

	return items, nil
}

// coveredKeys returns the item keys already claimed by a batch of this
// repo — ANY minted batch, regardless of status. A completed batch whose
// verdicts this sweep has not applied yet (its completion fact sits later
// in the stream) still claims its items; a dead batch claims them too,
// mirroring the store's dedup-forever semantics (recovery rides the next
// key-set change, never a re-mint of the same set).
func (s *Sweeper) coveredKeys(ctx context.Context, project string) (map[string]bool, error) {
	scorerType := executor.TaskTypePrioritize

	tasks, err := s.store.List(ctx, queue.Filter{Project: &project, Type: &scorerType})
	if err != nil {
		return nil, fmt.Errorf("list covered batches: %w", err)
	}

	covered := map[string]bool{}

	for _, t := range tasks {
		var payload executor.PrioritizePayload
		if json.Unmarshal(t.Payload, &payload) != nil {
			continue
		}

		for _, item := range payload.Items {
			covered[item.Key] = true
		}
	}

	return covered, nil
}

// mintRepo mints ONE batch covering the repo's unscored, uncovered
// FRONTIER items (the next-K claimable, O3 2026-10-05) — but only when
// that set is non-empty. The dedup key hashes the exact key set, so an
// unchanged set never mints twice. A store failure under the mint refuses
// (checkpoint held); the done-guard (a dedup key whose batch already
// completed) counts known.
func (s *Sweeper) mintRepo(ctx context.Context, repoRef, project string, stats *SweepStats) error {
	pending, err := s.pendingItems(ctx, project)
	if err != nil {
		return s.refuse("list working set", project, err)
	}

	covered, err := s.coveredKeys(ctx, project)
	if err != nil {
		return s.refuse("list covered batches", project, err)
	}

	var batch []executor.PrioritizeItem

	generation := int64(0)

	for _, entry := range s.frontier(pending) {
		score, cached, err := s.store.PriorityScore(ctx, entry.item.Key)
		if err != nil {
			// A failed cache read must not decide the batch's key set: the
			// item would silently join (or drop from) the dedup hash.
			return s.refuse("read cached verdict", entry.item.Key, err)
		}

		stale := cached && s.verdictUnusable(score)

		if stale && score.ScoredAt > generation {
			generation = score.ScoredAt
		}

		// A stale verdict (requeue supersede, model change) re-joins the
		// batch even though an old batch covered the item — the refresh is
		// the point. Fresh or absent verdicts keep the original gates:
		// never re-mint an item any batch already covers, never re-score a
		// fresh verdict.
		if !stale && (covered[entry.item.Key] || cached) {
			continue
		}

		batch = append(batch, executor.PrioritizeItem{
			Key:  entry.item.Key,
			Text: entry.item.Text,
		})
	}

	if len(batch) == 0 {
		return nil
	}

	sort.Slice(batch, func(i, j int) bool { return batch[i].Key < batch[j].Key })

	keys := make([]string, 0, len(batch))
	for _, item := range batch {
		keys = append(keys, item.Key)
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

		return nil
	}

	got, err := s.store.Enqueue(ctx, task.New{
		Type:     executor.TaskTypePrioritize,
		Project:  project,
		Priority: batchPriority,
		Payload:  payload,
		DedupKey: DedupKey(project, keys, generation),
	})
	if errors.Is(err, queue.ErrTaskDone) {
		// The done-guard (Queue wrapper): the dedup key matched a
		// COMPLETED batch — this key set was already scored. Known,
		// not refused: the checkpoint moves past.
		stats.BatchesKnown++

		return nil
	}

	if err != nil {
		return s.refuse("enqueue batch", project, err)
	}

	if got.Attempts == 0 && got.Status == task.Pending {
		stats.BatchesEnqueued++

		s.log("prioritize: enqueued batch",
			"task", got.ID.String(), "repo", project, "items", len(batch))
	} else {
		stats.BatchesKnown++
	}

	return nil
}

// mintWorkingSet boot-mints every repo that currently holds unscored
// backlog work (the BootMint pass over the pre-cursor standing backlog).
func (s *Sweeper) mintWorkingSet(ctx context.Context, stats *SweepStats) error {
	agentType := executor.TaskTypeAgent
	pendingStatus := task.Pending

	tasks, err := s.store.List(ctx, queue.Filter{Type: &agentType, Status: &pendingStatus})
	if err != nil {
		return s.refuse("list boot working set", "", err)
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
		if err := s.mintRepo(ctx, seen[project], project, stats); err != nil {
			return err
		}
	}

	return nil
}

// scorerSource is the identity a verdict is stamped with AND matched
// against: the batch scorer, model-qualified when SweeperConfig.Model is
// set. A cached verdict whose Source differs (a different model) is stale,
// so it re-joins the batch instead of ranking fresh work against an old
// model's scores.
func (s *Sweeper) scorerSource() string {
	if s.cfg.Model != "" {
		return SourceScorer + ":" + s.cfg.Model
	}

	return SourceScorer
}

// verdictUnusable reports whether a cached verdict must re-join the
// batch: its Source no longer matches the configured scorer (a different
// model, or the requeue supersede stamp — the verdict was invalidated and
// must be re-issued), so ranking fresh work against it would mix models
// or reuse a score the repo outgrew. Time-based aging is RETIRED (O3
// 2026-10-05): only facts invalidate verdicts now.
func (s *Sweeper) verdictUnusable(score queue.PriorityScore) bool {
	return score.Source != s.scorerSource()
}

// pruneCache evicts cached verdicts whose item key no longer belongs to
// any PENDING backlog task — the item was reworded (re-keyed), checked
// off, or its task reached a terminal state, and the orphaned verdict
// would otherwise sit in the cache forever. It runs on every scorer
// completion — the moment the cache can grow — so the pass costs one
// cache scan plus one live-set listing per scoring cycle, never per
// sweep. Best-effort by design: a store failure is logged and skipped
// (the orphans are evicted on the next completion) EXCEPT the live-set
// read, which refuses — deleting against a silently-empty live set would
// evict the WHOLE cache.
func (s *Sweeper) pruneCache(ctx context.Context, stats *SweepStats) {
	scores, err := s.store.PriorityScores(ctx)
	if err != nil {
		s.warn("prioritize: cache prune read failed", "err", err)

		return
	}

	if len(scores) == 0 {
		return
	}

	live, err := s.liveItemKeys(ctx)
	if err != nil {
		s.warn("prioritize: cache prune live-set read failed", "err", err)

		return
	}

	var orphans []string

	for _, score := range scores {
		if !live[score.ItemKey] {
			orphans = append(orphans, score.ItemKey)
		}
	}

	if len(orphans) == 0 {
		return
	}

	removed, err := s.store.DeletePriorityScores(ctx, orphans)
	if err != nil {
		s.warn("prioritize: cache prune delete failed", "keys", len(orphans), "err", err)

		return
	}

	if removed > 0 {
		stats.CachePruned += int(removed)

		s.log("prioritize: pruned stale cache entries", "count", removed)
	}
}

// liveItemKeys collects the item keys of every PENDING harvested backlog
// task across all projects — the working set the cache serves (mint and
// apply both read PENDING items only). A listing failure is an error, not
// an empty set: the prune pass must never treat "read failed" as "nothing
// is live".
func (s *Sweeper) liveItemKeys(ctx context.Context) (map[string]bool, error) {
	agentType := executor.TaskTypeAgent

	tasks, err := s.store.List(ctx, queue.Filter{Type: &agentType})
	if err != nil {
		return nil, fmt.Errorf("list live items: %w", err)
	}

	live := map[string]bool{}

	for _, t := range tasks {
		if t.Status != task.Pending {
			continue
		}

		item, ok := harvest.PayloadItemOf(t)
		if !ok || !strings.HasPrefix(item.Key, itemKeyPrefix) {
			continue
		}

		live[item.Key] = true
	}

	return live, nil
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
//
// generation is the newest ScoredAt among the batch's stale-verdict
// re-joins (0 when the batch has none): a superseded verdict's refresh
// must mint a NEW batch instead of colliding with the old key, so nonzero
// generations stamp a ":g<generation>" suffix. Zero keeps the legacy key
// shape byte-identical, so stored dedup keys keep resolving.
func DedupKey(repo string, keys []string, generation int64) string {
	sorted := append([]string(nil), keys...)
	sort.Strings(sorted)

	sum := sha256.Sum256([]byte(repo + "\x00" + strings.Join(sorted, "\x00")))

	key := "prioritize:" + repo + ":" + hex.EncodeToString(sum[:])[:16]
	if generation != 0 {
		key += ":g" + strconv.FormatInt(generation, 10)
	}

	return key
}

func sortedKeys(m map[string]string) []string {
	return slices.Sorted(maps.Keys(m))
}

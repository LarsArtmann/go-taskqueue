package incident

import (
	"context"
	"encoding/json"
	"encoding/json/jsontext"
	"fmt"
	"log/slog"
	"strings"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
	"github.com/larsartmann/go-taskqueue/internal/watermark"
)

// ConsumerKey is the policy's identity in the watermarks table: the
// persisted cursor shared by every policy run over the same database, so
// error facts observed while no policy was running still mint their fix
// tasks on the next pool start.
const ConsumerKey = "incident-policy"

const (
	// TaskDedupPrefix namespaces minted fix-task dedup keys.
	TaskDedupPrefix = "err:"

	// DefaultFirstPriority mints a fresh incident's fix task in the ADR-0015
	// hot band (100-149): production errors jump the backlog.
	DefaultFirstPriority = 120
	// DefaultRegressionPriority mints a regression (the previous fix did
	// not hold) in the machine band (150+): operational work.
	DefaultRegressionPriority = 150
	// DefaultMaxAttempts bounds burned failures on a minted fix task — a
	// broken repo must not drain the attempt budget.
	DefaultMaxAttempts = 2
)

// PolicyConfig controls one Policy.
type PolicyConfig struct {
	// Log receives mint failures and skips. nil logs nothing.
	Log *slog.Logger
	// PageSize bounds one fact-stream page; 0 selects the default.
	PageSize int
	// FirstPriority mints first-occurrence fix tasks; 0 selects the default.
	FirstPriority int
	// RegressionPriority mints regression fix tasks; 0 selects the default.
	RegressionPriority int
	// MaxAttempts bounds minted fix tasks; 0 selects the default.
	MaxAttempts int
}

// PolicyStats summarizes one sweep pass.
type PolicyStats struct {
	// Facts is the number of facts consumed.
	Facts int
	// TasksMinted counts fix tasks enqueued (dedup converges replays).
	TasksMinted int
	// Occurrences counts error.observed facts delivered (storms included).
	Occurrences int
	// Regressions counts observations that reopened a terminal incident.
	Regressions int
	// Known counts deliveries whose fix task was already minted (replay or
	// in-flight mint).
	Known int
	// Skipped counts facts that could not yield a mint (bad detail, enqueue
	// failure) — the incident stays open and the next occurrence re-arms.
	Skipped int
}

// Policy is the reaction half of the pipeline: a watermark-cursor sweeper
// (the seam shared with review/dlqfix/status; ADR-0009 exact-consumer
// semantics) that folds every incident-family fact into the State and
// mints agent fix tasks for incidents that need one. Safe for concurrent
// use (the cursor serializes sweeps).
//
// art-dupl:accept mirrored sweeper shell: config fields and fact
// validation differ per sweeper; the shared pump already lives in
// watermark.Cursor.
type Policy struct {
	store queue.Store
	sink  FactSink
	cfg   PolicyConfig
	cur   *watermark.Cursor

	state *State
	// pending tracks fingerprints whose mint fact has not folded back yet
	// (enqueue succeeded, incident.task-minted still in flight): a second
	// occurrence in the same page must not mint a second task.
	pending map[string]int64
}

// NewPolicy returns a policy over store (facts, enqueue, watermarks) and
// sink (non-task fact appends; the same object satisfies both in
// production). The cursor resumes from the persisted checkpoint — error
// facts observed while no policy was running mint on the next start. A
// FIRST run deliberately replays the whole journal (see the checkpoint
// note in the constructor): the family is new, so nothing predates it.
//
// The full journal is folded into the state so the projection is current;
// only facts after the persisted cursor are REACTED to — the seq guard
// makes the overlap idempotent.
func NewPolicy(ctx context.Context, store queue.Store, sink FactSink, cfg PolicyConfig) (*Policy, error) {
	if cfg.FirstPriority <= 0 {
		cfg.FirstPriority = DefaultFirstPriority
	}

	if cfg.RegressionPriority <= 0 {
		cfg.RegressionPriority = DefaultRegressionPriority
	}

	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = DefaultMaxAttempts
	}

	// First-run replay: unlike the review/status sweepers (which bootstrap
	// at head so pre-feature completions do not mint a stale-task flood),
	// the incident family is brand-new — nothing in an existing journal
	// predates it — and the API records error facts while no pool runs,
	// so a first start MUST mint for everything already observed. Safe
	// because reactions are idempotent (task dedup keys + fold seq
	// guards) and storm dedup bounds the mint count to one task per
	// incident lifecycle stage.
	if _, found, err := store.Watermark(ctx, ConsumerKey); err == nil && !found {
		if err := store.SaveWatermark(ctx, ConsumerKey, 0); err != nil {
			return nil, fmt.Errorf("incident: first-run replay checkpoint: %w", err)
		}
	}

	cur, err := watermark.New(ctx, watermark.Config{
		Store:    store,
		Key:      ConsumerKey,
		Domain:   "incident sweep",
		PageSize: cfg.PageSize,
	})
	if err != nil {
		return nil, err
	}

	p := &Policy{
		store:   store,
		sink:    sink,
		cfg:     cfg,
		cur:     cur,
		state:   NewState(),
		pending: map[string]int64{},
	}

	if err := p.bootstrap(ctx); err != nil {
		return nil, err
	}

	return p, nil
}

// State exposes the folded read model (for `tq incidents`).
func (p *Policy) State() *State { return p.state }

// bootstrap folds the ENTIRE journal once so incident state is current
// regardless of cursor position. Reactions never run here — the sweep
// owns them, bounded by the persisted cursor.
func (p *Policy) bootstrap(ctx context.Context) error {
	const page = 500

	for {
		facts, err := p.store.Facts(ctx, p.state.LastSeq(), page)
		if err != nil {
			return fmt.Errorf("incident: bootstrap fold: %w", err)
		}

		for _, f := range facts {
			p.state.Apply(f)
		}

		if len(facts) < page {
			return nil
		}
	}
}

// Sweep consumes new facts since the last pass and mints fix tasks for
// incidents that need one. Idempotent: a replayed page converges on task
// dedup keys and the fold's seq guard instead of duplicating work.
func (p *Policy) Sweep(ctx context.Context) (PolicyStats, error) {
	var stats PolicyStats

	err := p.cur.Sweep(ctx, func(ctx context.Context, f journal.Fact) error {
		stats.Facts++
		p.handleFact(ctx, f, &stats)

		return nil
	})

	return stats, err
}

// handleFact folds EVERY delivered fact into the state first (seq-guarded:
// a fact already folded by the bootstrap or a replayed page is a no-op),
// then reacts: error observations may mint, mint facts clear the pending
// guard. Task lifecycle facts (completed, dead-lettered) need no reaction —
// the fold alone moves the incident to resolved / fix-failed.
func (p *Policy) handleFact(ctx context.Context, f journal.Fact, stats *PolicyStats) {
	p.state.Apply(f)

	switch f.Type {
	case journal.ErrorObserved:
		p.reactObserved(ctx, f, stats)
	case journal.IncidentTaskMinted:
		p.clearPending(f)
	}
}

func (p *Policy) reactObserved(ctx context.Context, f journal.Fact, stats *PolicyStats) {
	if !strings.HasPrefix(f.TaskID, IDPrefix) {
		stats.Skipped++

		return
	}

	fp := FingerprintOfID(f.TaskID)

	inc, ok := p.state.Get(fp)
	if !ok {
		stats.Skipped++

		return
	}

	stats.Occurrences++

	needs, known := p.needsFix(fp, inc, f.Seq)
	if known {
		stats.Known++

		return
	}

	if !needs {
		return
	}

	// A mint on an incident whose latest fix task is terminal is a
	// regression: the fold has already reopened it.
	regression := len(inc.Mints) > 0 && inc.Mints[len(inc.Mints)-1].Terminal()

	if regression {
		stats.Regressions++
	}

	if err := p.mint(ctx, inc, f, regression); err != nil {
		// The incident stays open with no mint: the next occurrence (or a
		// watermark rewind) re-arms it. Storms make loss improbable.
		stats.Skipped++

		if p.cfg.Log != nil {
			p.cfg.Log.Error("incident mint failed", "incident", f.TaskID, "err", err)
		}

		return
	}

	stats.TasksMinted++
}

// needsFix decides whether an observation mints a fix task:
// never-minted → first fix; latest mint terminal → regression; anything
// else (fix in flight, in-page pending, already minted from this very
// fact) → occurrence only.
func (p *Policy) needsFix(fp string, inc Incident, seq int64) (needs, known bool) {
	if pend, ok := p.pending[fp]; ok {
		return false, pend == seq
	}

	if len(inc.Mints) == 0 {
		return true, false
	}

	last := inc.Mints[len(inc.Mints)-1]

	if last.SourceSeq == seq {
		return false, true
	}

	if !last.Terminal() {
		return false, false
	}

	return true, false
}

// mint enqueues the fix task and journals the link. After it returns, the
// mint fact folds back through the sweep loop itself (its Seq lands above
// the cursor), clearing the pending guard.
func (p *Policy) mint(ctx context.Context, inc Incident, f journal.Fact, regression bool) error {
	var rep Report
	if len(f.Detail) > 0 {
		_ = json.Unmarshal(f.Detail, &rep)
	}

	prio := p.cfg.FirstPriority
	if regression {
		prio = p.cfg.RegressionPriority
	}

	dedup := TaskDedupKey(inc.Fingerprint, f.Seq)

	payload, err := json.Marshal(executor.AgentPayload{
		Repo:   inc.Project,
		Prompt: fixPrompt(inc, rep, regression),
		Item:   itemLine(inc),
		Dedup:  dedup,
		V:      1,
	})
	if err != nil {
		return fmt.Errorf("encode agent payload: %w", err)
	}

	t, err := p.store.Enqueue(ctx, task.New{
		Project:     inc.Project,
		Type:        executor.TaskTypeAgent,
		Payload:     payload,
		Priority:    prio,
		MaxAttempts: p.cfg.MaxAttempts,
		DedupKey:    dedup,
	})
	if err != nil {
		return fmt.Errorf("enqueue fix task: %w", err)
	}

	detail, err := marshalDetail(Mint{
		TaskID:     t.ID.String(),
		SourceSeq:  f.Seq,
		Regression: regression,
		Priority:   prio,
	})
	if err != nil {
		return fmt.Errorf("encode mint fact: %w", err)
	}

	if err := p.sink.AppendFact(ctx, journal.Fact{
		TaskID: IncidentID(inc.Fingerprint),
		Type:   journal.IncidentTaskMinted,
		Detail: detail,
	}); err != nil {
		return fmt.Errorf("append %s: %w", journal.IncidentTaskMinted, err)
	}

	p.pending[inc.Fingerprint] = f.Seq

	if p.cfg.Log != nil {
		p.cfg.Log.Info("incident fix task minted",
			"incident", IncidentID(inc.Fingerprint),
			"task", t.ID.String(),
			"priority", prio,
			"regression", regression)
	}

	return nil
}

// clearPending drops the in-flight guard once the mint fact folds back.
func (p *Policy) clearPending(f journal.Fact) {
	if !strings.HasPrefix(f.TaskID, IDPrefix) {
		return
	}

	var d Mint
	if json.Unmarshal(f.Detail, &d) != nil {
		return
	}

	fp := FingerprintOfID(f.TaskID)
	if seq, ok := p.pending[fp]; ok && seq == d.SourceSeq {
		delete(p.pending, fp)
	}
}

// TaskDedupKey identifies one mint: fingerprint + the observed fact's Seq.
// Replayed deliveries of that fact converge on the stored task; a NEW
// observation (regression) carries a new Seq and mints afresh.
func TaskDedupKey(fingerprint string, seq int64) string {
	return fmt.Sprintf("%s%s:%d", TaskDedupPrefix, fingerprint, seq)
}

func marshalDetail(v any) (jsontext.Value, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	return jsontext.Value(b), nil
}

// itemLine is the one-line work item carried in the agent payload — the
// unit a review quotes and `tq show` renders.
func itemLine(inc Incident) string {
	msg := inc.Message
	if len(msg) > 80 {
		msg = msg[:80]
	}

	return fmt.Sprintf("error[%s] %s: %s", inc.Fingerprint, inc.Kind, msg)
}

// fixPrompt renders the fix task's prompt: everything the agent needs,
// nothing it does not (secrets never entered the journal to begin with).
func fixPrompt(inc Incident, rep Report, regression bool) string {
	var b strings.Builder

	b.WriteString("A production error was observed and you are dispatched to fix it.\n\n")

	fmt.Fprintf(&b, "Repo: %s\n", inc.Project)
	fmt.Fprintf(&b, "Kind: %s error\n", inc.Kind)

	if regression {
		fmt.Fprintf(&b, "REGRESSION: a previous fix task (%s) did not hold — the error recurred after it completed.\n",
			inc.Mints[len(inc.Mints)-1].TaskID)
	}

	fmt.Fprintf(&b, "Occurrences so far: %d (first seen %s)\n\n", inc.Occurrences, inc.FirstSeen.Format(timeFormat))

	b.WriteString("Error:\n")
	fmt.Fprintf(&b, "  %s\n", firstLine(inc.Message))

	if rep.Stack != "" {
		b.WriteString("\nTop stack frames:\n")

		for i, line := range topFrames(rep.Stack, 5) {
			fmt.Fprintf(&b, "  %d. %s\n", i+1, line)
		}
	}

	if rep.Release != "" {
		fmt.Fprintf(&b, "\nRelease: %s\n", rep.Release)
	}

	if rep.Route != "" {
		method := rep.Method
		if method == "" {
			method = "-"
		}

		fmt.Fprintf(&b, "Route: %s %s\n", method, rep.Route)
	}

	if rep.TraceURL != "" {
		fmt.Fprintf(&b, "Full trace: %s\n", rep.TraceURL)
	}

	if len(rep.Breadcrumbs) > 0 {
		b.WriteString("\nLast actions before the error:\n")

		for _, c := range rep.Breadcrumbs {
			fmt.Fprintf(&b, "  - %s\n", c)
		}
	}

	b.WriteString(`
Constraints:
- Fix ONLY the reported error; keep the change minimal and focused.
- Client-side stacks may be minified: resolve frames against the source
  maps of the pinned release in this repo before editing.
- Do not refactor unrelated code, do not touch migrations or secrets.

Verify: the repo's verify gate runs automatically after your run (go
build + go test for Go repos) — the fix must pass it.`)

	return b.String()
}

func topFrames(stack string, n int) []string {
	var out []string

	for _, line := range strings.Split(stack, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		line = strings.TrimPrefix(line, "at ")

		out = append(out, line)

		if len(out) == n {
			break
		}
	}

	return out
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}

	return s
}

const timeFormat = "2006-01-02 15:04 MST"

// Package papdashboard forwards taskqueue facts to a PapDashboard instance:
// a dead-lettered task becomes an alert (alert.triggered), and if the task
// later completes (e.g. after tq dlq --rescue), that alert is resolved
// (alert.resolved). The bridge consumes the same journal seam as tq tail,
// so it works against any Store and never mutates queue state — its one
// write is its own read cursor in the watermarks table.
//
// Delivery semantics: at-least-once. The journal watermark only advances
// after PapDashboard accepts a fact (2xx, or a permanent 4xx which is
// logged then accepted), and the Idempotency-Key header (derived from the
// fact sequence) makes retries safe. Across restarts the bridge resumes
// from its persisted checkpoint instead of the journal head: incidents
// that fired while the bridge was down are replayed with identical
// idempotency keys, and Config.FromSeq overrides the checkpoint for an
// ops-initiated replay. A completion resolves the alert of any task that
// ever dead-lettered — the correlation is derived from the task's own
// fact trail, not process memory, so it survives restarts too. With no
// WatermarkStore configured the bridge keeps the legacy volatile behavior
// (head start, nothing persisted).
package papdashboard

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// FactSource is the read-only slice of queue.Store the bridge needs.
type FactSource interface {
	Facts(ctx context.Context, after int64, limit int) ([]journal.Fact, error)
	HeadSeq(ctx context.Context) (int64, error)
	Get(ctx context.Context, id task.ID) (task.Task, error)
	// FactsForTask reads one task's own fact trail — the durable answer to
	// "did this task ever dead-letter?" (a read, so the no-mutation
	// contract holds).
	FactsForTask(ctx context.Context, id string, limit int) ([]journal.Fact, error)
}

// WatermarkStore persists the bridge's journal cursor across restarts.
// queue.Store satisfies it; tests fake it in-process. It is deliberately
// separate from FactSource so the read contract stays read-only: the
// bridge writes only its own consumer progress, never task or fact state.
type WatermarkStore interface {
	Watermark(ctx context.Context, consumer string) (seq int64, exists bool, err error)
	SaveWatermark(ctx context.Context, consumer string, seq int64) error
}

// SourceApp is the default sourceApp stamped on ingested alerts.
const SourceApp = "go-taskqueue"

// DefaultPollInterval is how often the journal is tailed.
const DefaultPollInterval = 5 * time.Second

// forwardBatchLimit bounds each journal read during a poll's backlog drain:
// memory stays flat no matter how large the burst since the last poll.
const forwardBatchLimit = 500

// Config controls a Bridge. Endpoint and APIKey come from the PapDashboard
// deployment (its public /api/ingest route and the PAP_API_KEY value).
type Config struct {
	// Endpoint is PapDashboard's base URL, e.g. http://localhost:8080.
	Endpoint string
	// APIKey is the Bearer token protecting PapDashboard's ingest route.
	APIKey string
	// SourceApp overrides the default sourceApp ("go-taskqueue").
	SourceApp string
	// Severity is the alert severity for dead-lettered tasks.
	// Default "critical".
	Severity string
	// BudgetSeverity is the alert severity for budget exhaustion.
	// Default "warning".
	BudgetSeverity string
	// DailyBudget, when > 0, mirrors the agent pool's --daily-budget: the
	// day the pool enqueues its Nth task, a "daily budget exhausted" alert
	// fires once (resolved automatically when the next day's first task is
	// enqueued). 0 disables budget telemetry.
	DailyBudget int
	// PollInterval tails the journal this often. Default 5s.
	PollInterval time.Duration
	// FromSeq, when set, starts forwarding AFTER this fact sequence instead
	// of the journal head (replay mode).
	FromSeq *int64
	// Client overrides the HTTP client.
	Client *http.Client
	// Logger receives bridge diagnostics. Default slog.Default().
	Logger *slog.Logger
}

// Bridge tails the taskqueue journal and mirrors dead-letter incidents into
// PapDashboard alerts.
type Bridge struct {
	store       FactSource
	checkpoints WatermarkStore // nil = volatile legacy behavior
	cfg         Config
	log         *slog.Logger
	client      *http.Client

	// Daily-budget telemetry (Config.DailyBudget): the same projection the
	// pool's budget.Guard uses, maintained incrementally from Enqueued facts.
	budgetDay     string // YYYY-MM-DD the counters below belong to
	budgetSpent   int
	budgetAlerted bool // alert fired for budgetDay (fires at most once/day)
}

// New builds a Bridge. checkpoints persists the journal cursor across
// restarts (queue.Store satisfies WatermarkStore); nil keeps the legacy
// volatile behavior. Call Run to start tailing.
func New(store FactSource, checkpoints WatermarkStore, cfg Config) *Bridge {
	if cfg.SourceApp == "" {
		cfg.SourceApp = SourceApp
	}

	if cfg.Severity == "" {
		cfg.Severity = "critical"
	}

	if cfg.BudgetSeverity == "" {
		cfg.BudgetSeverity = "warning"
	}

	if cfg.PollInterval <= 0 {
		cfg.PollInterval = DefaultPollInterval
	}

	if cfg.Client == nil {
		cfg.Client = &http.Client{Timeout: 15 * time.Second}
	}

	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}

	return &Bridge{
		store:       store,
		checkpoints: checkpoints,
		cfg:         cfg,
		log:         log,
		client:      cfg.Client,
	}
}

// consumerKey namespaces the bridge's checkpoints in the watermarks table.
// Distinct endpoints get distinct cursors; one endpoint shared by two
// bridge processes interleaves checkpoints, which the store's monotonic
// upsert keeps safe.
func (b *Bridge) consumerKey() string {
	return "papdashboard:" + b.cfg.Endpoint
}

// Run tails the journal until ctx is cancelled. Facts are forwarded in Seq
// order; the watermark advances past a fact only when it is accepted (2xx)
// or permanently rejected (4xx) — transient failures retry on the next
// poll. Each drained batch checkpoints the cursor AFTER the last accepted
// fact, and a failed checkpoint stops the drain exactly like a failed
// forward: the next poll retries from the last persisted seq, so delivery
// stays at-least-once and never degrades to at-most-once.
func (b *Bridge) Run(ctx context.Context) error {
	start, err := b.startWatermark(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}

		b.log.Error("papdashboard bridge cannot resolve journal position", "err", err)

		return errors.New("papdashboard: cannot resolve journal position")
	}

	watermark, persisted := start.start, start.persisted

	// First run with no row: eagerly insert the head (even 0 — a seq-0
	// row is a real cursor) so a crash before the first batch checkpoint
	// still resumes exactly here. Persistence must not replay history into
	// existing deployments — bootstrap is forward-only, exactly like the
	// volatile behavior.
	if start.bootstrap && b.checkpoints != nil {
		if err := b.checkpoints.SaveWatermark(ctx, b.consumerKey(), start.start); err != nil {
			if ctx.Err() != nil {
				return nil
			}

			b.log.Error("papdashboard bridge cannot persist bootstrap watermark", "seq", start.start, "err", err)

			return errors.New("papdashboard: cannot persist bootstrap watermark")
		}

		persisted = start.start
	}

	b.log.Info("papdashboard bridge watching for dead letters",
		"endpoint", b.cfg.Endpoint, "fromSeq", watermark, "start", start.branch)

	ticker := time.NewTicker(b.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			// A pending checkpoint gates forwarding: a new batch is never
			// read while accepted facts are unpersisted (a crash there
			// would skip them).
			if err := b.checkpoint(ctx, watermark, &persisted); err != nil {
				if ctx.Err() != nil {
					return nil
				}

				b.log.Error("papdashboard bridge checkpoint failed; will retry",
					"seq", watermark, "err", err)

				break
			}

			b.drain(ctx, &watermark, &persisted)
		}
	}
}

// checkpoint persists the cursor when it has advanced past the last
// persisted seq. Its failure is a delivery failure: the caller stops the
// drain and retries on the next poll, never skipping past the unpersisted
// seq.
func (b *Bridge) checkpoint(ctx context.Context, watermark int64, persisted *int64) error {
	if b.checkpoints == nil || watermark <= *persisted {
		return nil
	}

	if err := b.checkpoints.SaveWatermark(ctx, b.consumerKey(), watermark); err != nil {
		return fmt.Errorf("save watermark %d: %w", watermark, err)
	}

	*persisted = watermark

	return nil
}

// drain forwards every currently available fact in bounded batches,
// checkpointing after each batch. Forward, read and checkpoint failures
// are logged here and stop the drain; the next poll retries from the last
// persisted seq.
func (b *Bridge) drain(ctx context.Context, watermark, persisted *int64) {
	for {
		facts, err := b.store.Facts(ctx, *watermark, forwardBatchLimit)
		if err != nil {
			if ctx.Err() != nil {
				return
			}

			b.log.Error("papdashboard bridge read journal failed", "err", err)

			return
		}

		for _, f := range facts {
			if err := b.forward(ctx, f); err != nil {
				if ctx.Err() != nil {
					return
				}

				b.log.Error("papdashboard bridge forward failed; will retry",
					"seq", f.Seq, "type", f.Type, "err", err)

				return
			}

			*watermark = f.Seq
		}

		// Batch end: checkpoint AFTER the last accepted fact — never before,
		// or a crash would silently skip facts (at-most-once by accident).
		if err := b.checkpoint(ctx, *watermark, persisted); err != nil {
			if ctx.Err() != nil {
				return
			}

			b.log.Error("papdashboard bridge checkpoint failed; will retry",
				"seq", *watermark, "err", err)

			return
		}

		if len(facts) < forwardBatchLimit {
			return
		}
	}
}

// startDecision is startWatermark's outcome: the cursor to read from, the
// last persisted checkpoint, which branch decided (for the startup log —
// the operator's first diagnostic when alerts look wrong), and the eager
// bootstrap seq to insert on a first run (0-with-bootstrap=true when the
// journal was empty; 0-and-no-bootstrap only when there is nothing to do).
type startDecision struct {
	start     int64
	persisted int64
	branch    string
	bootstrap bool // insert start eagerly: the store had no row
}

// startWatermark resolves the initial cursor: explicit FromSeq (ops replay
// override) beats a persisted checkpoint (restart resume — seq 0 is a real
// cursor: "bootstrapped on an empty journal, consumed nothing yet"), which
// beats the journal head (first run). Read under ctx so a cancelled startup
// aborts cleanly.
func (b *Bridge) startWatermark(ctx context.Context) (startDecision, error) {
	if b.cfg.FromSeq != nil {
		d := startDecision{start: *b.cfg.FromSeq, branch: "--from-seq override"}
		if b.checkpoints != nil {
			p, _, err := b.checkpoints.Watermark(ctx, b.consumerKey())
			if err != nil {
				return startDecision{}, fmt.Errorf("read persisted watermark: %w", err)
			}

			d.persisted = p
		}

		return d, nil
	}

	if b.checkpoints != nil {
		p, exists, err := b.checkpoints.Watermark(ctx, b.consumerKey())
		if err != nil {
			return startDecision{}, fmt.Errorf("read persisted watermark: %w", err)
		}

		if exists {
			return startDecision{start: p, persisted: p, branch: "resumed from checkpoint"}, nil
		}
	}

	head, err := b.store.HeadSeq(ctx)
	if err != nil {
		return startDecision{}, err
	}

	return startDecision{
		start:     head,
		branch:    "no checkpoint, starting at head",
		bootstrap: true,
	}, nil
}

// forward mirrors one fact. A Completed task resolves the alert of any
// task that ever dead-lettered — derived from the task's own fact trail,
// not process memory, so a restart cannot lose the correlation (a resolve
// for an alert PapDashboard never held lands as a logged 4xx).
func (b *Bridge) forward(ctx context.Context, f journal.Fact) error {
	if err := b.trackBudget(ctx, f); err != nil {
		return err
	}

	switch f.Type {
	case journal.DeadLettered:
		t, err := b.store.Get(ctx, task.ID(f.TaskID))
		if err != nil {
			return fmt.Errorf("load dead task %s: %w", f.TaskID, err)
		}

		title := alertTitle(t)

		payload := map[string]any{
			"severity": b.cfg.Severity,
			"title":    title,
			"body": fmt.Sprintf("Task %s (%s/%s) exhausted %d attempts. Last error: %s",
				t.ID, t.Project, t.Type, t.Attempts, firstLine(f.Error)),
			"sourceApp": b.cfg.SourceApp,
			"metadata": map[string]string{
				"taskType": t.Type,
				"attempts": strconv.Itoa(t.Attempts),
			},
		}
		if err := b.post(
			ctx,
			"alert.triggered",
			idempotencyKey("dlq", f.Seq),
			t.ID.String(),
			f.Seq,
			payload,
		); err != nil {
			return err
		}

	case journal.Completed:
		deadLettered, err := b.everDeadLettered(ctx, f.TaskID)
		if err != nil {
			return err
		}

		if !deadLettered {
			return nil
		}

		t, err := b.store.Get(ctx, task.ID(f.TaskID))
		if err != nil {
			return fmt.Errorf("load completed task %s: %w", f.TaskID, err)
		}

		payload := map[string]any{
			"title":      alertTitle(t),
			"body":       fmt.Sprintf("Task %s completed after dead-letter (rescued).", t.ID),
			"sourceApp":  b.cfg.SourceApp,
			"resolvedBy": b.cfg.SourceApp + "-bridge",
		}
		if err := b.post(
			ctx,
			"alert.resolved",
			idempotencyKey("resolve", f.Seq),
			t.ID.String(),
			f.Seq,
			payload,
		); err != nil {
			return err
		}
	}

	return nil
}

// everDeadLettered answers from the task's own fact trail — the journal is
// the source of truth for "this task once exhausted its attempts", so the
// bridge holds no correlation state of its own.
func (b *Bridge) everDeadLettered(ctx context.Context, taskID string) (bool, error) {
	trail, err := b.store.FactsForTask(ctx, taskID, 0)
	if err != nil {
		return false, fmt.Errorf("load fact trail for %s: %w", taskID, err)
	}

	for _, tf := range trail {
		if tf.Type == journal.DeadLettered {
			return true, nil
		}
	}

	return false, nil
}

// post sends one ingest event. 2xx is success; 4xx is permanent (PapDashboard
// will never accept this payload) and is logged then accepted; 5xx and
// transport errors return an error so the fact retries.
func (b *Bridge) post(
	ctx context.Context,
	eventType, idemKey string,
	aggregateID string,
	seq int64,
	payload map[string]any,
) error {
	doc := map[string]any{
		"type":        eventType,
		"aggregateId": aggregateID,
		"payload":     payload,
		"metadata": map[string]any{
			"correlationId": aggregateID,
			"causationId":   strconv.FormatInt(seq, 10),
			"userId":        "",
			"sourceApp":     b.cfg.SourceApp,
		},
	}

	body, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("marshal ingest: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		b.cfg.Endpoint+"/api/ingest", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build ingest request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+b.cfg.APIKey)
	req.Header.Set("Idempotency-Key", idemKey)

	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("ingest %s: %w", eventType, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return fmt.Errorf("ingest %s: PapDashboard returned %d", eventType, resp.StatusCode)
	}

	if resp.StatusCode >= 400 {
		b.log.Error("papdashboard ingest permanently rejected",
			"event", eventType, "status", resp.StatusCode, "idempotencyKey", idemKey)

		return nil
	}

	b.log.Info("papdashboard ingest accepted",
		"event", eventType, "status", resp.StatusCode, "idempotencyKey", idemKey)

	return nil
}

func alertTitle(t task.Task) string {
	return fmt.Sprintf("%s/%s task %s dead-lettered", t.Project, t.Type, t.ID)
}

// budgetAggregate is the alert identity for one day's budget window: a
// synthetic, stable aggregate id (budget alerts are not task-scoped).
func budgetAggregate(day string) string { return "agent-pool-budget-" + day }

// trackBudget maintains the daily spend projection and fires the at-cap
// alert exactly once per day (and resolves yesterday's on rollover).
func (b *Bridge) trackBudget(ctx context.Context, f journal.Fact) error {
	if b.cfg.DailyBudget <= 0 {
		return nil
	}

	day := f.Time.Format("2006-01-02")
	if day != b.budgetDay {
		if b.budgetAlerted {
			// The window rolled over: yesterday's cap no longer applies, so
			// the exhaustion alert closes itself.
			payload := map[string]any{
				"title":      "agent-pool daily budget exhausted",
				"body":       fmt.Sprintf("Budget window %s rolled over; the daily cap reset.", b.budgetDay),
				"sourceApp":  b.cfg.SourceApp,
				"resolvedBy": b.cfg.SourceApp + "-bridge",
			}
			if err := b.post(ctx, "alert.resolved", idempotencyKey("budget-resolve", f.Seq),
				budgetAggregate(b.budgetDay), f.Seq, payload); err != nil {
				return err
			}
		}

		b.budgetDay, b.budgetSpent, b.budgetAlerted = day, 0, false
	}

	if f.Type != journal.Enqueued {
		return nil
	}

	b.budgetSpent++

	if b.budgetSpent == b.cfg.DailyBudget && !b.budgetAlerted {
		payload := map[string]any{
			"severity": b.cfg.BudgetSeverity,
			"title":    "agent-pool daily budget exhausted",
			"body": fmt.Sprintf(
				"%d/%d agent tasks enqueued today: the pool skips harvest ticks until the window rolls over. Raise --daily-budget or wait for the reset.",
				b.budgetSpent,
				b.cfg.DailyBudget,
			),
			"sourceApp": b.cfg.SourceApp,
			"metadata": map[string]string{
				"spent": strconv.Itoa(b.budgetSpent),
				"cap":   strconv.Itoa(b.cfg.DailyBudget),
				"day":   b.budgetDay,
			},
		}
		if err := b.post(ctx, "alert.triggered", idempotencyKey("budget", f.Seq),
			budgetAggregate(b.budgetDay), f.Seq, payload); err != nil {
			return err
		}

		b.budgetAlerted = true
	}

	return nil
}

func firstLine(s string) string {
	for i, r := range s {
		if r == '\n' {
			return s[:i]
		}
	}

	return s
}

func idempotencyKey(kind string, seq int64) string {
	return fmt.Sprintf("%s-%s-%d", SourceApp, kind, seq)
}

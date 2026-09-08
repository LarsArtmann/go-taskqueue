// Package papdashboard forwards taskqueue facts to a PapDashboard instance:
// a dead-lettered task becomes an alert (alert.triggered), and if the task
// later completes (e.g. after tq dlq --rescue), that alert is resolved
// (alert.resolved). The bridge consumes the same journal seam as tq tail,
// so it works against any Store and never mutates queue state.
//
// Delivery semantics: at-least-once within a bridge process lifetime — the
// journal watermark only advances after PapDashboard accepts a fact, and the
// Idempotency-Key header (derived from the fact sequence) makes retries
// safe. Across restarts the bridge starts at the journal head: incidents
// that fired while the bridge was down are not replayed (review them with
// tq dlq); a completion that would resolve a pre-restart alert is not seen,
// so those alerts stay open for a human.
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

// FactSource is the slice of queue.Store the bridge needs.
type FactSource interface {
	Facts(ctx context.Context, after int64, limit int) ([]journal.Fact, error)
	HeadSeq(ctx context.Context) (int64, error)
	Get(ctx context.Context, id task.ID) (task.Task, error)
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
	store   FactSource
	cfg     Config
	log     *slog.Logger
	client  *http.Client
	alerted map[string]alertedTask

	// Daily-budget telemetry (Config.DailyBudget): the same projection the
	// pool's budget.Guard uses, maintained incrementally from Enqueued facts.
	budgetDay     string // YYYY-MM-DD the counters below belong to
	budgetSpent   int
	budgetAlerted bool // alert fired for budgetDay (fires at most once/day)
}

// alertedTask remembers the alert raised for a task so a later completion
// resolves exactly that alert.
type alertedTask struct {
	title string
}

// New builds a Bridge. Call Run to start tailing.
func New(store FactSource, cfg Config) *Bridge {
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

	return &Bridge{store: store, cfg: cfg, log: log, client: cfg.Client, alerted: map[string]alertedTask{}}
}

// Run tails the journal until ctx is cancelled. Facts are forwarded in Seq
// order; the watermark advances past a fact only when it is accepted (2xx)
// or permanently rejected (4xx) — transient failures retry on the next poll.
func (b *Bridge) Run(ctx context.Context) error {
	watermark := b.startWatermark(ctx)
	if watermark < 0 {
		if ctx.Err() != nil {
			return nil
		}

		return errors.New("papdashboard: cannot read journal head")
	}

	b.log.Info("papdashboard bridge watching for dead letters",
		"endpoint", b.cfg.Endpoint, "fromSeq", watermark)

	ticker := time.NewTicker(b.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			// Drain the backlog in bounded batches; a failed forward stops the
			// drain and retries on the next poll from the last forwarded seq.
			drained := false

			for !drained {
				facts, err := b.store.Facts(ctx, watermark, forwardBatchLimit)
				if err != nil {
					if ctx.Err() != nil {
						return nil
					}

					b.log.Error("papdashboard bridge read journal failed", "err", err)

					break
				}

				for _, f := range facts {
					if err := b.forward(ctx, f); err != nil {
						if ctx.Err() != nil {
							return nil
						}

						b.log.Error("papdashboard bridge forward failed; will retry",
							"seq", f.Seq, "type", f.Type, "err", err)

						drained = true

						break
					}

					watermark = f.Seq
				}

				if len(facts) < forwardBatchLimit {
					drained = true
				}
			}
		}
	}
}

// startWatermark resolves the initial sequence: explicit FromSeq, else the
// journal head (forward-only from now), read under ctx so a cancelled startup
// aborts cleanly.
func (b *Bridge) startWatermark(ctx context.Context) int64 {
	if b.cfg.FromSeq != nil {
		return *b.cfg.FromSeq
	}

	head, err := b.store.HeadSeq(ctx)
	if err != nil {
		b.log.Error("papdashboard bridge cannot read journal head", "err", err)

		return -1
	}

	return head
}

// forward mirrors one fact. Completed tasks only resolve alerts this bridge
// raised (process lifetime), so resolve noise stays at zero.
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
		if err := b.post(ctx, "alert.triggered", idempotencyKey("dlq", f.Seq), t.ID.String(), f.Seq, payload); err != nil {
			return err
		}

		b.alerted[f.TaskID] = alertedTask{title: title}

	case journal.Completed:
		raised, ok := b.alerted[f.TaskID]
		if !ok {
			return nil
		}

		t, err := b.store.Get(ctx, task.ID(f.TaskID))
		if err != nil {
			return fmt.Errorf("load completed task %s: %w", f.TaskID, err)
		}

		payload := map[string]any{
			"title":      raised.title,
			"body":       fmt.Sprintf("Task %s completed after dead-letter (rescued).", t.ID),
			"sourceApp":  b.cfg.SourceApp,
			"resolvedBy": b.cfg.SourceApp + "-bridge",
		}
		if err := b.post(ctx, "alert.resolved", idempotencyKey("resolve", f.Seq), t.ID.String(), f.Seq, payload); err != nil {
			return err
		}

		delete(b.alerted, f.TaskID)
	}

	return nil
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
				b.budgetSpent, b.cfg.DailyBudget),
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

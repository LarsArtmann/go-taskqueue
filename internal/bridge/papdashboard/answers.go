package papdashboard

// AnswerPoller is the producer half of the PapDashboard questions loop:
// it polls answered questions back from PapDashboard and records them on
// the asking tasks (queue.Store.RecordAnswer), which unblocks the parked
// task and injects the ruling into its payload.
//
// It is deliberately a SEPARATE type from Bridge: the bridge is the
// read-only journal forwarder (its package contract forbids queue
// mutation), while the poller writes task state. Both consume PapDashboard
// as initiators only — the queue never exposes an inbound write path.
//
// Cursor semantics: the checkpoint is the newest consumed AnsweredAt
// (UnixNano, in the watermarks table under a poller-specific consumer
// key). First start bootstraps at NOW — answers recorded before the
// poller process existed are not replayed, mirroring the bridge's
// head-start rule. Delivery is at-least-once: a replayed answer is a
// no-op (RecordAnswer is idempotent per question ref).

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// AnswerConfig controls an AnswerPoller.
type AnswerConfig struct {
	// Endpoint is PapDashboard's base URL, e.g. http://localhost:8080.
	Endpoint string
	// APIKey is the Bearer token protecting PapDashboard's API.
	APIKey string
	// SourceApp filters the questions polled. Default "go-taskqueue".
	SourceApp string
	// Interval polls this often. Default 10s.
	Interval time.Duration
	// Client overrides the HTTP client.
	Client *http.Client
	// Logger receives poller diagnostics. Default slog.Default().
	Logger *slog.Logger
}

// AnswerStore is the narrow write surface the poller needs: record one
// owner ruling on the asking task.
type AnswerStore interface {
	RecordAnswer(ctx context.Context, id task.ID, ans queue.AnswerRecord) error
}

// errListQuestions classifies list-endpoint failures (wraps the HTTP
// status detail).
var errListQuestions = errors.New("list questions")

// answerPageLimit bounds each list page; memory stays flat regardless of
// the answer backlog.
const answerPageLimit = 50

const (
	// defaultPollInterval is the AnswerConfig.Interval fallback.
	defaultPollInterval = 10 * time.Second
	// defaultHTTPTimeout bounds one list call.
	defaultHTTPTimeout = 30 * time.Second
	// errorBodyCap limits how much of an error response body is kept for
	// diagnostics.
	errorBodyCap = 4096
)

// AnswerPoller polls PapDashboard for answered questions and routes each
// ruling back to its task via AnswerStore.
type AnswerPoller struct {
	store       AnswerStore
	checkpoints WatermarkStore // nil = volatile cursor (restart re-bootstraps)
	cfg         AnswerConfig
	log         *slog.Logger
	client      *http.Client
}

// NewAnswerPoller builds an AnswerPoller. checkpoints persists the
// AnsweredAt cursor across restarts (queue.Store satisfies
// WatermarkStore); nil keeps a volatile cursor.
func NewAnswerPoller(store AnswerStore, checkpoints WatermarkStore, cfg AnswerConfig) *AnswerPoller {
	if cfg.SourceApp == "" {
		cfg.SourceApp = SourceApp
	}

	if cfg.Interval <= 0 {
		cfg.Interval = defaultPollInterval
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	if cfg.Client == nil {
		cfg.Client = &http.Client{Timeout: defaultHTTPTimeout}
	}

	return &AnswerPoller{
		store:       store,
		checkpoints: checkpoints,
		cfg:         cfg,
		log:         cfg.Logger,
		client:      cfg.Client,
	}
}

// consumerKey namespaces the poller's checkpoint. Distinct endpoints get
// distinct cursors, mirroring Bridge.consumerKey.
func (p *AnswerPoller) consumerKey() string {
	return "papdashboard-answers:" + p.cfg.Endpoint
}

// startCursor resolves the initial AnsweredAt cursor: a persisted
// checkpoint (restart resume) beats NOW (first run — no history replay).
func (p *AnswerPoller) startCursor(ctx context.Context) (time.Time, error) {
	if p.checkpoints != nil {
		nanos, exists, err := p.checkpoints.Watermark(ctx, p.consumerKey())
		if err != nil {
			return time.Time{}, fmt.Errorf("read answer cursor: %w", err)
		}

		if exists {
			return time.Unix(0, nanos), nil
		}
	}

	return time.Now(), nil
}

// Run polls until ctx is cancelled. A failing poll keeps the old cursor
// and retries on the next tick — one bad poll never advances or loses
// state.
func (p *AnswerPoller) Run(ctx context.Context) error {
	cursor, err := p.startCursor(ctx)
	if err != nil {
		return err
	}

	p.log.Info("answer poller started",
		"endpoint", p.cfg.Endpoint, "sourceApp", p.cfg.SourceApp,
		"cursor", cursor.Format(time.RFC3339Nano))

	ticker := time.NewTicker(p.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			next, err := p.pollOnce(ctx, cursor)
			if err != nil {
				p.log.Warn("answer poll failed; keeping cursor", "err", err)

				continue
			}

			cursor = next
		}
	}
}

// pollOnce applies every answered question newer than cursor and returns
// the new cursor. The checkpoint is saved ONLY after the whole batch's
// applications succeeded (a failed RecordAnswer retries the batch).
func (p *AnswerPoller) pollOnce(ctx context.Context, cursor time.Time) (time.Time, error) {
	maxSeen := cursor

	for offset := 0; ; offset += answerPageLimit {
		questions, err := p.list(ctx, offset)
		if err != nil {
			return cursor, err
		}

		if len(questions) == 0 {
			break
		}

		pageHasNew := false

		for _, question := range questions {
			if !question.IsAnswered || question.Answer == "" {
				continue
			}

			if !question.AnsweredAt.After(cursor) {
				continue
			}

			pageHasNew = true

			if question.AnsweredAt.After(maxSeen) {
				maxSeen = question.AnsweredAt
			}

			if err := p.apply(ctx, question); err != nil {
				return cursor, err
			}
		}

		// Pages are newest-first: a page with nothing newer means
		// everything deeper is older — stop paging.
		if len(questions) < answerPageLimit || !pageHasNew {
			break
		}
	}

	if maxSeen.After(cursor) && p.checkpoints != nil {
		if err := p.checkpoints.SaveWatermark(ctx, p.consumerKey(), maxSeen.UnixNano()); err != nil {
			return cursor, fmt.Errorf("save answer cursor: %w", err)
		}
	}

	return maxSeen, nil
}

// papQuestion is the list-endpoint slice of PapDashboard's Question the
// poller reads (sdk.Question's wire shape).
type papQuestion struct {
	ID         string    `json:"id"`
	Status     string    `json:"status"`
	Title      string    `json:"title"`
	Body       string    `json:"body"`
	SourceApp  string    `json:"sourceApp"`
	IsAnswered bool      `json:"isAnswered"`
	Answer     string    `json:"answer,omitempty"`
	AnsweredAt time.Time `json:"answeredAt"`
}

// list fetches one page of this sourceApp's questions.
func (p *AnswerPoller) list(ctx context.Context, offset int) ([]papQuestion, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		p.cfg.Endpoint+"/api/questions", nil)
	if err != nil {
		return nil, fmt.Errorf("build question list request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)

	q := req.URL.Query()
	q.Set("sourceApp", p.cfg.SourceApp)
	q.Set("limit", strconv.Itoa(answerPageLimit))
	q.Set("offset", strconv.Itoa(offset))
	req.URL.RawQuery = q.Encode()

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list questions: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, errorBodyCap))

		return nil, fmt.Errorf("%w: HTTP %d: %s", errListQuestions, resp.StatusCode, body)
	}

	var doc struct {
		Body struct {
			Data []papQuestion `json:"data"`
		} `json:"body"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, fmt.Errorf("decode question list: %w", err)
	}

	return doc.Body.Data, nil
}

// questionCorrelation patterns are the machine lines questionIngestPayload
// wrote into the PapDashboard body. Anchored to line starts: free text
// that merely contains "task:" or "qref:" never routes an answer.
var (
	questionTaskTokenRe = regexp.MustCompile(`(?m)^task:([0-9A-Za-z]+)[ \t]*$`)
	questionRefTokenRe  = regexp.MustCompile(`(?m)^qref:(\S+)[ \t]*$`)
)

// parseQuestionCorrelation extracts the asking task and the question ref
// from a PapDashboard question body. Missing tokens mean the question did
// not originate from a parked task (operator-created, foreign format) —
// the caller logs and skips.
func parseQuestionCorrelation(body string) (string, string, bool) {
	m := questionTaskTokenRe.FindStringSubmatch(body)
	if m == nil {
		return "", "", false
	}

	id := m[1]

	mref := questionRefTokenRe.FindStringSubmatch(body)
	if mref == nil {
		return id, "", false
	}

	return id, mref[1], true
}

// apply routes one answered question home. A question without parseable
// correlation is logged and skipped — never an error (the poll keeps its
// cursor; there is no task to route it to).
func (p *AnswerPoller) apply(ctx context.Context, question papQuestion) error {
	taskID, ref, ok := parseQuestionCorrelation(question.Body)
	if !ok {
		p.log.Warn("answered question without correlation tokens; skipped",
			"pap_id", question.ID, "title", question.Title)

		return nil
	}

	err := p.store.RecordAnswer(ctx, task.ID(taskID), queue.AnswerRecord{
		Ref:        ref,
		Answer:     question.Answer,
		PapID:      question.ID,
		AnsweredAt: question.AnsweredAt,
	})
	if err != nil {
		return fmt.Errorf("record answer %s on task %s: %w", ref, taskID, err)
	}

	p.log.Info("answer routed",
		"pap_id", question.ID, "task", taskID, "ref", ref, "answer", firstLine(question.Answer))

	return nil
}

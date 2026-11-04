// This file owns EFFORT ESTIMATION for harvest batch sizing: it
// predicts per-item work effort (minutes) so batches are sized by
// predicted work instead of raw item count (effort-aware batching,
// 2026-11-03). The harvester itself stays OFFLINE unless an estimator
// is wired via Config.EffortEstimator — the network client below only
// runs behind that flag.
//
// The split follows the TypeSafe integration craft rules: Jev (the
// System One model behind api.typesafe.ai) supplies the JUDGMENT — one
// score question per item over a five-level effort rubric, answered as
// a calibrated expected score — and code owns everything mechanical:
// the rubric-to-minutes interpolation, the clamps, the [10, 240] bound,
// the cache, and the fallback. A failed or absent estimate is the flat
// per-item default, never an error: estimates are SCHEDULING SIGNALS
// (the ADR-0015 §signals rule verbatim) — they size runs and raise
// timeouts, never lower them, and never touch trust or priority.
//
// PRIVACY: enabling the Jev estimator sends each item's text, its
// section heading, the repo name and the repo's stated purpose to the
// configured TypeSafe endpoint (default https://api.typesafe.ai). The
// pool flag help says so; the endpoint only sees that state, nothing
// else from the repo.
//
// Estimates are cached (JSON sidecar, default <tq-db>.effort.json)
// keyed by the item key — which hashes the text, so an edited item
// re-keys and re-enters unscored — and stamped with the answering
// model: a model swap invalidates every cached verdict (the prioritize
// scorerSource lesson), so fresh work is never ranked or sized against
// a retired model's judgment.
package harvest

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"math"
	"net/http"
	"os"
	"time"
)

const (
	// DefaultJevBaseURL is the TypeSafe System One endpoint.
	DefaultJevBaseURL = "https://api.typesafe.ai"

	// DefaultJevModel is the Jev alias used when none is configured.
	DefaultJevModel = "jev-latest"

	// MinMinutes is the floor for any single-item estimate: smaller work
	// still pays a session start, and a near-zero estimate would pack an
	// unbounded context into one run.
	MinEffortMinutes = 10

	// MaxMinutes is the ceiling for any single-item estimate: a monster
	// item ships as its own batch, and the pool's scaled --task-timeout
	// default never needs to exceed it while effort mode is on.
	MaxEffortMinutes = 240

	// DefaultJevRequestTimeout bounds one System One request.
	DefaultJevRequestTimeout = 30 * time.Second

	// maxQuestionsPerRequest caps one request's question count so a big
	// frontier cannot grow an unbounded payload.
	maxQuestionsPerRequest = 20

	// cacheMaxAge prunes cache entries older than this on load: done or
	// edited items re-key anyway, this only bounds the sidecar.
	cacheMaxAge = 30 * 24 * time.Hour

	// cacheVersion guards the sidecar shape.
	cacheVersion = 1
)

// rubricLevel is one effort-rubric rung: a situation a person could
// point at, and the minutes that situation represents. One slice is
// the single source — the situation and its weight cannot desync.
type rubricLevel struct {
	Situation string
	Minutes   int
}

var rubric = []rubricLevel{
	{"A trivial edit: fix a typo, flip a checkbox, or reword one line; minutes at most", 10},
	{"A small scoped change: one file or one short doc section; well under half an hour", 25},
	{"A moderate change: a few files or one subsystem, with tests to adjust; around an hour", 60},
	{"A substantial change: several subsystems, new tests, and iteration under review; a few hours", 150},
	{"A large effort: a redesign, migration, or open-ended investigation; most of a day or more", 240},
}

// RubricSituations returns the ordered rubric level descriptions (the
// score question criteria).
func RubricSituations() []string {
	out := make([]string, len(rubric))
	for i, level := range rubric {
		out[i] = level.Situation
	}

	return out
}

// MinutesFromScore converts a rubric score (0..len(rubric)-1,
// fractional expected values allowed) into estimated minutes:
// piecewise-linear between level minutes, rounded UP to the next
// 5-minute step (never under-ask on a fraction), clamped to
// [MinEffortMinutes, MaxMinutes].
func MinutesFromScore(score float64) int {
	if math.IsNaN(score) {
		return DefaultBatchTimeoutMinutes
	}

	top := len(rubric) - 1

	if score <= 0 {
		return MinEffortMinutes
	}

	if score >= float64(top) {
		return MaxEffortMinutes
	}

	level := int(score)
	frac := score - float64(level)
	minutes := float64(rubric[level].Minutes) + frac*float64(rubric[level+1].Minutes-rubric[level].Minutes)

	rounded := ((int(minutes) + 4) / 5) * 5 // up to the next 5-minute step, exact multiples intact

	return min(max(rounded, MinEffortMinutes), MaxEffortMinutes)
}

// systemOneRequest is one POST /v1/systemone body (the OpenAPI shape).
type systemOneRequest struct {
	Model     string                   `json:"model"`
	State     map[string]any           `json:"state"`
	Questions map[string]scoreQuestion `json:"questions"`
}

type scoreQuestion struct {
	Type         string   `json:"type"`
	Instructions string   `json:"instructions"`
	Criteria     []string `json:"criteria"`
}

type systemOneResponse struct {
	Model   string                 `json:"model"`
	Answers map[string]scoreAnswer `json:"answers"`
	Usage   struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type scoreAnswer struct {
	Type          string             `json:"type"`
	Score         float64            `json:"score"`
	Confidence    float64            `json:"confidence"`
	Probabilities map[string]float64 `json:"probabilities"`
}

// jevClient is a thin TypeSafe System One caller: it builds no judgment,
// applies no policy — it sends the state and questions and decodes the
// answers, or returns the error untouched (an API failure is an error,
// never a quiet "small" answer).
type jevClient struct {
	BaseURL string
	APIKey  string
	Model   string
	HTTP    *http.Client
	Log     *slog.Logger
}

func (c *jevClient) baseURL() string {
	if c.BaseURL == "" {
		return DefaultJevBaseURL
	}

	return c.BaseURL
}

func (c *jevClient) model() string {
	if c.Model == "" {
		return DefaultJevModel
	}

	return c.Model
}

func (c *jevClient) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}

	return &http.Client{Timeout: DefaultJevRequestTimeout}
}

// ask posts one System One request and returns the answers keyed by
// question name.
func (c *jevClient) ask(
	ctx context.Context,
	state map[string]any,
	questions map[string]scoreQuestion,
) (map[string]scoreAnswer, error) {
	body, err := json.Marshal(systemOneRequest{
		Model:     c.model(),
		State:     state,
		Questions: questions,
	})
	if err != nil {
		return nil, fmt.Errorf("effort: encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL()+"/v1/systemone", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("effort: build request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("effort: call %s: %w", c.baseURL(), err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("effort: %s answered %s", c.baseURL(), resp.Status)
	}

	data, err := io.ReadAll(http.MaxBytesReader(nil, resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("effort: read response: %w", err)
	}

	var decoded systemOneResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, fmt.Errorf("effort: decode response: %w", err)
	}

	return decoded.Answers, nil
}

// JevConfig configures one JevEstimator.
type JevConfig struct {
	// APIKey is the TypeSafe bearer token (required — a missing key at
	// startup is a configuration error, not a runtime fallback).
	APIKey string
	// Model overrides DefaultModel.
	Model string
	// BaseURL overrides DefaultBaseURL (tests point it at httptest).
	BaseURL string
	// Timeout bounds one request; 0 = DefaultTimeout.
	Timeout time.Duration
	// CachePath persists estimates as a JSON sidecar; empty disables
	// persistence (every pass re-estimates).
	CachePath string
	// Log receives one WARN line per degraded pass; nil discards.
	Log *slog.Logger
}

// JevEstimator implements harvest.EffortEstimator with Jev: cache-first
// per item key, one batched System One request per pass for the misses,
// flat-default on any failure (missing keys — the harvest falls back
// itself). It is safe for sequential harvest use (the pool loop calls
// it once per repo per tick).
type JevEstimator struct {
	client JevConfig
	cache  *EffortCache
}

// NewJevEstimator loads the estimate cache (a missing or corrupt
// sidecar starts empty with a warning — the cache is advisory) and
// returns the estimator. An empty API key is refused: the pool should
// fail fast on a misconfigured key, not silently run estimator-less.
func NewJevEstimator(cfg JevConfig) (*JevEstimator, error) {
	if cfg.APIKey == "" {
		return nil, errors.New("effort: --jev needs a TypeSafe API key (--jev-api-key or $TYPESAFE_API_KEY)")
	}

	cache := NewEffortCache()
	if cfg.CachePath != "" {
		if err := cache.Load(cfg.CachePath); err != nil {
			if cfg.Log != nil {
				cfg.Log.Warn(
					"effort: estimate cache unreadable, starting empty",
					"path",
					cfg.CachePath,
					"err",
					err.Error(),
				)
			}
		}
	}

	return &JevEstimator{client: cfg, cache: cache}, nil
}

// EstimateEffort implements harvest.EffortEstimator.
func (e *JevEstimator) EstimateEffort(ctx context.Context, items []Item) map[string]int {
	out := make(map[string]int, len(items))

	model := e.client.Model
	if model == "" {
		model = DefaultJevModel
	}

	var misses []Item

	for _, item := range items {
		if entry, ok := e.cache.Get(item.Key, model); ok {
			out[item.Key] = entry.Minutes

			continue
		}

		misses = append(misses, item)
	}

	if len(misses) == 0 {
		return out
	}

	fresh := e.estimateMisses(ctx, misses, model)

	maps.Copy(out, fresh)

	if len(fresh) > 0 && e.client.CachePath != "" {
		if err := e.cache.Save(e.client.CachePath); err != nil && e.client.Log != nil {
			e.client.Log.Warn("effort: estimate cache save failed", "path", e.client.CachePath, "err", err.Error())
		}
	}

	return out
}

// estimateMisses asks Jev for the uncached items in chunks of
// maxQuestionsPerRequest: one question per item, all sharing the repo
// state (questions are cheap and parallel on the service side). Any
// chunk failure degrades to flat for that chunk — logged, never fatal.
func (e *JevEstimator) estimateMisses(ctx context.Context, misses []Item, model string) map[string]int {
	out := make(map[string]int, len(misses))

	client := jevClient{
		BaseURL: e.client.BaseURL,
		APIKey:  e.client.APIKey,
		Model:   model,
		Log:     e.client.Log,
	}

	if e.client.Timeout > 0 {
		client.HTTP = &http.Client{Timeout: e.client.Timeout}
	}

	repo := ""
	repoName, purpose := "", "(unstated)"

	for _, item := range misses {
		if item.Repo != "" {
			repo = item.Repo
			repoName = item.RepoName

			break
		}
	}

	if repo != "" {
		purpose = RepoPurpose(repo)
	}

	for start := 0; start < len(misses); start += maxQuestionsPerRequest {
		chunk := misses[start:min(start+maxQuestionsPerRequest, len(misses))]

		state, questions := jevStateAndQuestions(repoName, purpose, chunk)

		answers, err := client.ask(ctx, state, questions)
		if err != nil {
			if e.client.Log != nil {
				e.client.Log.Warn(
					"effort: jev pass failed, flat default applies",
					"items",
					len(chunk),
					"err",
					err.Error(),
				)
			}

			continue
		}

		for name, item := range questionItems(chunk) {
			answer, ok := answers[name]
			if !ok || answer.Type != "score" {
				continue
			}

			minutes := MinutesFromScore(answer.Score)
			out[item.Key] = minutes
			e.cache.Put(item.Key, minutes, model, answer.Confidence)
		}
	}

	return out
}

// jevStateAndQuestions builds one request's shared state and the
// per-item score questions: items live under stable `todos.<name>`
// paths (named fields beat array positions), each question names its
// subject by that path, and every question carries the same rubric.
func jevStateAndQuestions(repoName, purpose string, items []Item) (map[string]any, map[string]scoreQuestion) {
	todos := make(map[string]any, len(items))
	questions := make(map[string]scoreQuestion, len(items))

	for i, item := range items {
		name := fmt.Sprintf("t%d", i)

		todos[name] = map[string]any{
			"text":    item.Text,
			"section": item.Heading,
		}

		questions[name] = scoreQuestion{
			Type:         "score",
			Instructions: fmt.Sprintf("How much work is `todos.%s` for a senior engineer in this repo?", name),
			Criteria:     RubricSituations(),
		}
	}

	state := map[string]any{
		"repo": map[string]any{
			"name":    repoName,
			"purpose": purpose,
		},
		"todos": todos,
	}

	return state, questions
}

// questionItems mirrors jevStateAndQuestions' naming so answers map
// back to their items without a second allocation in the hot path.
func questionItems(items []Item) map[string]Item {
	out := make(map[string]Item, len(items))
	for i, item := range items {
		out[fmt.Sprintf("t%d", i)] = item
	}

	return out
}

// EffortCache is the file-backed estimate store: JSON sidecar, single writer
// (the pool process), advisory in every direction — a lost or corrupt
// cache costs one re-estimate pass, nothing else.
type EffortCache struct {
	entries map[string]EffortEntry
}

// EffortEntry is one cached estimate.
type EffortEntry struct {
	Minutes    int       `json:"minutes"`
	Source     string    `json:"source"` // "jev:<model>"
	Confidence float64   `json:"confidence"`
	At         time.Time `json:"at"`
}

type cacheFile struct {
	Version int                    `json:"version"`
	Entries map[string]EffortEntry `json:"entries"`
}

// NewEffortCache returns an empty estimate cache.
func NewEffortCache() *EffortCache {
	return &EffortCache{entries: make(map[string]EffortEntry)}
}

// JevSourceFor stamps an estimate with the model that produced it.
func JevSourceFor(model string) string {
	return "jev:" + model
}

// Get returns the cached estimate when it exists AND its source stamp
// matches the given model (a model swap invalidates everything — never
// size fresh work against a retired model's judgment).
func (c *EffortCache) Get(key, model string) (EffortEntry, bool) {
	entry, ok := c.entries[key]
	if !ok || entry.Source != JevSourceFor(model) {
		return EffortEntry{}, false
	}

	return entry, true
}

// Put records an estimate under the given model's stamp.
func (c *EffortCache) Put(key string, minutes int, model string, confidence float64) {
	c.entries[key] = EffortEntry{
		Minutes:    minutes,
		Source:     JevSourceFor(model),
		Confidence: confidence,
		At:         time.Now(),
	}
}

// Load reads the sidecar, pruning entries older than cacheMaxAge.
func (c *EffortCache) Load(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var file cacheFile
	if err := json.Unmarshal(data, &file); err != nil {
		return fmt.Errorf("effort: parse cache %s: %w", path, err)
	}

	cutoff := time.Now().Add(-cacheMaxAge)

	for key, entry := range file.Entries {
		if entry.At.Before(cutoff) {
			continue
		}

		c.entries[key] = entry
	}

	return nil
}

// Save writes the sidecar best-effort (direct write: single writer, a
// torn write self-heals as an unreadable cache).
func (c *EffortCache) Save(path string) error {
	data, err := json.Marshal(cacheFile{Version: cacheVersion, Entries: c.entries})
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

// Len reports the entry count (tests).
func (c *EffortCache) Len() int {
	return len(c.entries)
}

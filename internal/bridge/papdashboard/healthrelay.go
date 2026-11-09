package papdashboard

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/bridge/httpx"
	"log/slog"
)

// ingester posts one event to PapDashboard's /api/ingest route. The journal
// Bridge and the health relay share it so the wire contract — Bearer auth,
// Idempotency-Key, and the transient/permanent status split — lives in one
// place.
type ingester struct {
	endpoint  string
	apiKey    string
	sourceApp string
	client    *http.Client
	log       *slog.Logger
}

// post sends one ingest event. 2xx is success; 4xx is permanent
// (PapDashboard will never accept this payload) and is logged then
// accepted; 5xx, 429 and transport errors return an error so callers can
// retry.
func (g *ingester) post(
	ctx context.Context,
	eventType, idemKey string,
	aggregateID, causationID string,
	payload map[string]any,
) error {
	doc := map[string]any{
		"type":        eventType,
		"aggregateId": aggregateID,
		"payload":     payload,
		"metadata": map[string]any{
			"correlationId": aggregateID,
			"causationId":   causationID,
			"userId":        "",
			"sourceApp":     g.sourceApp,
		},
	}

	body, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("marshal ingest: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		g.endpoint+"/api/ingest", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build ingest request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.apiKey)
	req.Header.Set("Idempotency-Key", idemKey)

	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("ingest %s: %w", eventType, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
		return fmt.Errorf(
			"ingest %s: PapDashboard returned %d (transient)",
			eventType,
			resp.StatusCode,
		)
	}

	if resp.StatusCode >= 400 {
		g.log.Error("papdashboard ingest permanently rejected",
			"event", eventType, "status", resp.StatusCode, "idempotencyKey", idemKey)

		return nil
	}

	g.log.Info("papdashboard ingest accepted",
		"event", eventType, "status", resp.StatusCode, "idempotencyKey", idemKey)

	return nil
}

// healthAggregate is the alert identity for a serve's health: one stable
// aggregate id, so the trigger and the later resolve correlate.
const healthAggregate = "tq-serve-health"

// healthSnapshot is the go-health Response subset the relay reads — kept
// local (not the go-health type) so the bridge owns no new dependency and
// the accepted wire shape is pinned here, field for field.
type healthSnapshot struct {
	Status  string                      `json:"status"`
	Version string                      `json:"version"`
	Checks  map[string]healthRelayCheck `json:"checks"`
}

type healthRelayCheck struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// HealthRelay adapts the go-health-dashboard webhook — a POSTed health
// snapshot fired on every overall status TRANSITION — into PapDashboard
// alert events: a transition into warn/fail raises alert.triggered with the
// degraded checks spelled out, the transition back to pass resolves it.
// The receiver listens on a private loopback port because the dashboard's
// WithWebhook needs a URL while PapDashboard only speaks its own ingest
// schema: the translation happens in-process, before anything leaves the
// machine.
type HealthRelay struct {
	ing *ingester
	ln  net.Listener
	srv *http.Server

	mu   sync.Mutex
	last string // last overall status delivered ("" before the first)
}

// NewHealthRelay builds a relay forwarding to the PapDashboard endpoint in
// cfg (Endpoint and APIKey are required; SourceApp/Logger/Client default
// like Bridge.New's). Close releases the listener.
func NewHealthRelay(cfg Config) (*HealthRelay, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("papdashboard: health relay needs an endpoint")
	}

	if cfg.SourceApp == "" {
		cfg.SourceApp = SourceApp
	}

	if cfg.Client == nil {
		cfg.Client = httpx.TunedClient(15 * time.Second)
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("papdashboard: bind health relay: %w", err)
	}

	r := &HealthRelay{
		ing: &ingester{
			endpoint:  cfg.Endpoint,
			apiKey:    cfg.APIKey,
			sourceApp: cfg.SourceApp,
			client:    cfg.Client,
			log:       cfg.Logger,
		},
		ln: ln,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /health-events", r.handle)
	r.srv = &http.Server{Handler: mux}

	go func() { _ = r.srv.Serve(ln) }()

	return r, nil
}

// URL is the webhook target to hand dashboard.WithWebhook.
func (r *HealthRelay) URL() string { return "http://" + r.ln.Addr().String() + "/health-events" }

// Close stops the relay listener.
func (r *HealthRelay) Close() error { return r.srv.Close() }

func (r *HealthRelay) handle(w http.ResponseWriter, req *http.Request) {
	var snap healthSnapshot
	if err := json.NewDecoder(io.LimitReader(req.Body, 1<<20)).Decode(&snap); err != nil {
		http.Error(w, "bad health snapshot: "+err.Error(), http.StatusBadRequest)

		return
	}

	w.WriteHeader(http.StatusNoContent)

	r.mu.Lock()
	prev := r.last
	r.last = snap.Status
	r.mu.Unlock()

	// The dashboard fires only on transitions; the guard makes a
	// push-always dashboard safe too.
	if snap.Status == prev {
		return
	}

	ctx, cancel := context.WithTimeout(req.Context(), 10*time.Second)
	defer cancel()

	if err := r.forwardTransition(ctx, prev, snap); err != nil {
		r.ing.log.Error("papdashboard health transition not delivered", "err", err)
	}
}

// forwardTransition mirrors one status transition into PapDashboard. A
// healthy first observation raises nothing (there is no alert to resolve).
func (r *HealthRelay) forwardTransition(ctx context.Context, prev string, snap healthSnapshot) error {
	switch snap.Status {
	case "pass":
		if prev == "" {
			return nil
		}

		return r.ing.post(ctx,
			"alert.resolved",
			"health-resolved-"+strconv.FormatInt(time.Now().UnixNano(), 10),
			healthAggregate,
			strconv.FormatInt(time.Now().UnixNano(), 10),
			map[string]any{
				"title":      "tq serve health recovered",
				"body":       "All health checks pass again.",
				"sourceApp":  r.ing.sourceApp,
				"resolvedBy": r.ing.sourceApp + "-health-relay",
			},
		)
	case "warn", "fail":
		return r.ing.post(ctx,
			"alert.triggered",
			"health-triggered-"+strconv.FormatInt(time.Now().UnixNano(), 10),
			healthAggregate,
			strconv.FormatInt(time.Now().UnixNano(), 10),
			map[string]any{
				"severity": healthSeverity(snap.Status),
				"title":    "tq serve health: " + snap.Status,
				"body":     healthBody(snap),
				"sourceApp": r.ing.sourceApp,
				"metadata": map[string]string{
					"version": snap.Version,
				},
			},
		)
	}

	return nil
}

func healthSeverity(status string) string {
	if status == "fail" {
		return "critical"
	}

	return "warning"
}

// healthBody spells out every non-passing check so the alert is actionable
// without opening the dashboard (store down, wedged fold, follower
// demotion — the classes an operator must not miss).
func healthBody(snap healthSnapshot) string {
	names := make([]string, 0, len(snap.Checks))
	for name := range snap.Checks {
		names = append(names, name)
	}
	slices.Sort(names)

	var b bytes.Buffer
	for _, name := range names {
		c := snap.Checks[name]
		if c.Status == "pass" {
			continue
		}

		fmt.Fprintf(&b, "%s: %s", name, c.Status)
		if c.Error != "" {
			fmt.Fprintf(&b, " — %s", c.Error)
		}
		b.WriteByte('\n')
	}

	if b.Len() == 0 {
		return "overall " + snap.Status
	}

	return b.String()
}

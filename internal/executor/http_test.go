package executor

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

func httpTask() task.Task {
	return task.Task{ID: "t-http-1", Project: "demo", Type: "webhook", Payload: []byte(`{"cmd":"ping"}`)}
}

// TestHTTPExecutorStatusClassification pins the retry contract: 2xx
// completes, 408/429/5xx stay retryable, other non-2xx (and malformed URLs)
// are permanent — the same request would fail identically forever.
func TestHTTPExecutorStatusClassification(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		status        int
		wantErr       bool
		wantPermanent bool
	}{
		{name: "200 completes", status: 200},
		{name: "204 completes", status: 204},
		{name: "408 retryable", status: 408, wantErr: true},
		{name: "429 retryable", status: 429, wantErr: true},
		{name: "500 retryable", status: 500, wantErr: true},
		{name: "503 retryable", status: 503, wantErr: true},
		{name: "404 permanent", status: 404, wantErr: true, wantPermanent: true},
		{name: "401 permanent", status: 401, wantErr: true, wantPermanent: true},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
			}))
			defer srv.Close()

			err := NewHTTPExecutor(srv.URL).Execute(context.Background(), httpTask())
			if (err != nil) != tt.wantErr {
				t.Fatalf("Execute = %v, wantErr %v", err, tt.wantErr)
			}

			if _, ok := errors.AsType[*PermanentError](err); ok != tt.wantPermanent {
				t.Fatalf("permanent = %v, want %v (err: %v)", ok, tt.wantPermanent, err)
			}
		})
	}
}

// TestHTTPExecutor429ClassifiedAsRateLimit pins the f4c contract: an HTTP
// 429 must surface as *RateLimitError (requeued WITHOUT attempt burn by the
// worker, same as agent tasks) — never as a plain transient error that
// burns the retry budget like the 2026-09-11 incident.
func TestHTTPExecutor429ClassifiedAsRateLimit(t *testing.T) {
	t.Parallel()

	t.Run("retry-after header wins", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Retry-After", "30")
			w.WriteHeader(http.StatusTooManyRequests)
		}))
		defer srv.Close()

		err := NewHTTPExecutor(srv.URL).Execute(context.Background(), httpTask())

		rl, ok := errors.AsType[*RateLimitError](err)
		if !ok {
			t.Fatalf("429 classified as %T (%v), want *RateLimitError", err, err)
		}

		if rl.RetryAfter < 30*time.Second || rl.RetryAfter > 31*time.Second+rateLimitGrace {
			t.Fatalf("RetryAfter = %s, want ~30s + grace", rl.RetryAfter)
		}
	})

	t.Run("body reset timestamp when no header", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write(
				[]byte(
					`{"error":{"message":"Usage limit reached for 5 hour. Your limit will reset at 2099-01-01 00:00:00"}}`,
				),
			)
		}))
		defer srv.Close()

		err := NewHTTPExecutor(srv.URL).Execute(context.Background(), httpTask())

		rl, ok := errors.AsType[*RateLimitError](err)
		if !ok {
			t.Fatalf("429 classified as %T (%v), want *RateLimitError", err, err)
		}

		if rl.RetryAfter != maxRateLimitWait {
			t.Fatalf("RetryAfter = %s, want the 6h cap", rl.RetryAfter)
		}
	})

	t.Run("bare 429 falls back to the default backoff", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
		}))
		defer srv.Close()

		err := NewHTTPExecutor(srv.URL).Execute(context.Background(), httpTask())

		rl, ok := errors.AsType[*RateLimitError](err)
		if !ok {
			t.Fatalf("429 classified as %T (%v), want *RateLimitError", err, err)
		}

		if rl.RetryAfter != defaultRateLimitBackoff {
			t.Fatalf("RetryAfter = %s, want %s", rl.RetryAfter, defaultRateLimitBackoff)
		}
	})
}

// TestParseRetryAfterHeader pins the header grammar: delay-seconds and
// HTTP-date both parse, garbage and past dates return 0.
func TestParseRetryAfterHeader(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		in   string
		want time.Duration
	}{
		{"", 0},
		{"30", 30 * time.Second},
		{" 120 ", 2 * time.Minute},
		{"-5", 0},
		{"abc", 0},
		{now.Add(time.Hour).Format(http.TimeFormat), time.Hour},
		{now.Add(-time.Hour).Format(http.TimeFormat), 0},
	}

	for _, tt := range cases {
		if got := parseRetryAfterHeader(tt.in, now); got != tt.want {
			t.Errorf("parseRetryAfterHeader(%q) = %s, want %s", tt.in, got, tt.want)
		}
	}
}

// TestHTTPExecutorPostsTaskEnvelope asserts the wire format: the four
// envelope fields the receiving side parses, plus the JSON content type.
func TestHTTPExecutorPostsTaskEnvelope(t *testing.T) {
	t.Parallel()

	var gotType, gotBody string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := NewHTTPExecutor(srv.URL).Execute(context.Background(), httpTask()); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if gotType != "application/json" {
		t.Errorf("content type = %q, want application/json", gotType)
	}

	var env struct {
		ID      string         `json:"id"`
		Project string         `json:"project"`
		Type    string         `json:"type"`
		Payload jsontext.Value `json:"payload"`
		Attempt int            `json:"attempt"`
	}
	if err := json.Unmarshal([]byte(gotBody), &env); err != nil {
		t.Fatalf("envelope not JSON: %v: %q", err, gotBody)
	}

	if env.ID != "t-http-1" || env.Project != "demo" || env.Type != "webhook" || env.Attempt != 1 {
		t.Errorf("envelope = %+v, want id/project/type/attempt", env)
	}

	if string(env.Payload) != `{"cmd":"ping"}` {
		t.Errorf("payload = %s, want passthrough", env.Payload)
	}
}

// TestHTTPExecutorEmptyPayloadBecomesObject pins payloadOrEmpty: an empty
// payload still renders a valid JSON object so the envelope always parses.
func TestHTTPExecutorEmptyPayloadBecomesObject(t *testing.T) {
	t.Parallel()

	var gotBody string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
	}))
	defer srv.Close()

	tk := task.Task{ID: "t-http-2", Type: "webhook"}
	if err := NewHTTPExecutor(srv.URL).Execute(context.Background(), tk); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !jsontext.Value([]byte(gotBody)).IsValid() {
		t.Fatalf("envelope with empty payload must stay valid JSON, got %q", gotBody)
	}
}

func TestHTTPExecutorMalformedURLIsPermanent(t *testing.T) {
	t.Parallel()

	err := NewHTTPExecutor("://no-scheme").Execute(context.Background(), httpTask())
	if err == nil {
		t.Fatal("malformed URL must fail")
	}

	if _, ok := errors.AsType[*PermanentError](err); !ok {
		t.Fatalf("malformed URL must be permanent, got %v", err)
	}
}

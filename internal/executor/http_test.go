package executor

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

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
		ID      string          `json:"id"`
		Project string          `json:"project"`
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
		Attempt int             `json:"attempt"`
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

	if !json.Valid([]byte(gotBody)) {
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

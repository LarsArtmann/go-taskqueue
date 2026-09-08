package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

func newTestAPI(t *testing.T) (*Server, *queue.SQLiteStore) {
	t.Helper()

	s, err := queue.OpenSQLite(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	t.Cleanup(func() { _ = s.Close() })

	srv, err := New(s, "secret-token", nil)
	if err != nil {
		t.Fatalf("new api: %v", err)
	}

	return srv, s
}

func TestTokenRequiredAtConstruction(t *testing.T) {
	if _, err := New(nil, "", nil); err == nil {
		t.Fatal("empty token must refuse to build the API (it writes)")
	}
}

func TestAuthMatrix(t *testing.T) {
	srv, _ := newTestAPI(t)
	h := srv.Handler()

	cases := []struct {
		name, header, query string
		want                int
	}{
		{"missing token", "", "", http.StatusUnauthorized},
		{"wrong token", "Bearer nope", "", http.StatusUnauthorized},
		{"bearer token", "Bearer secret-token", "", http.StatusOK},
		{"query token", "", "token=secret-token", http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			url := "/api/v1/healthz"
			if tc.query != "" {
				url += "?" + tc.query
			}

			req := httptest.NewRequest(http.MethodGet, url, nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}

			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

func TestEnqueueContract(t *testing.T) {
	srv, store := newTestAPI(t)
	h := srv.Handler()

	body := `{
		"project": "billing",
		"type": "sh",
		"payload": {"cmd": "echo hi"},
		"priority": 5,
		"maxAttempts": 2,
		"deps": [],
		"dedupKey": "api-1"
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret-token")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	var resp enqueueResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	got, err := store.Get(context.Background(), task.ID(resp.ID))
	if err != nil {
		t.Fatalf("stored task missing: %v", err)
	}

	if got.Project != "billing" || got.Type != "sh" || got.Priority != 5 || got.MaxAttempts != 2 {
		t.Fatalf("stored task = %+v", got)
	}

	if string(got.Payload) != `{"cmd": "echo hi"}` {
		t.Fatalf("payload passed through verbatim, got %s", got.Payload)
	}
}

func TestEnqueueValidation(t *testing.T) {
	srv, _ := newTestAPI(t)
	h := srv.Handler()

	cases := []struct {
		name, body string
	}{
		{"missing type", `{"payload":{}}`},
		{"missing payload", `{"type":"sh"}`},
		{"bad notBefore", `{"type":"sh","payload":{},"notBefore":"tomorrow"}`},
		{"not json", `nope`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", bytes.NewReader([]byte(tc.body)))
			req.Header.Set("Authorization", "Bearer secret-token")

			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d (%s), want 400 with an actionable error", rec.Code, rec.Body.String())
			}

			var errDoc struct {
				Error string `json:"error"`
				Fix   string `json:"fix"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &errDoc); err != nil || errDoc.Error == "" || errDoc.Fix == "" {
				t.Fatalf("error doc = %s, want {error, fix}", rec.Body.String())
			}
		})
	}
}

func TestDedupKeyIdempotency(t *testing.T) {
	srv, store := newTestAPI(t)
	h := srv.Handler()

	post := func() string {
		body := `{"project":"p","type":"sh","payload":{},"dedupKey":"same"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer secret-token")

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		var resp enqueueResponse

		_ = json.Unmarshal(rec.Body.Bytes(), &resp)

		return resp.ID
	}

	first, second := post(), post()
	if first == "" || first != second {
		t.Fatalf("dedup key not idempotent: %q vs %q", first, second)
	}

	// Exactly one task row exists.
	pending := task.Pending

	tasks, err := store.List(context.Background(), queue.Filter{Project: new("p"), Status: &pending})
	if err != nil || len(tasks) != 1 {
		t.Fatalf("tasks = %d (%v), want 1", len(tasks), err)
	}
}

//go:fix inline
func ptr(s string) *string { return new(s) }

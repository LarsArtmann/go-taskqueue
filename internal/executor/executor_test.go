package executor

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

func TestRegistryLookup(t *testing.T) {
	r := NewRegistry()
	r.RegisterFunc("a", func(context.Context, task.Task) error { return nil })

	if _, err := r.Lookup("a"); err != nil {
		t.Fatalf("Lookup(a): %v", err)
	}

	if _, err := r.Lookup("b"); !errors.Is(err, ErrUnknownType) {
		t.Fatalf("Lookup(b) err = %v, want ErrUnknownType", err)
	}
}

func TestCommandExecutorSuccess(t *testing.T) {
	e := NewCommandExecutor("echo hello {{PROJECT}}")
	if err := e.Execute(context.Background(), task.Task{ID: task.ID("x"), Project: "demo", Type: "echo"}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

func TestUnwrapCommandPayloadShapes(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		want    string
	}{
		{"raw shell line", "echo hi", "echo hi"},
		{"cmd object", `{"cmd":"go test ./..."}`, "go test ./..."},
		{"json string (CLI wraps non-JSON sh payloads)", `"echo hi"`, "echo hi"},
		{"empty", "", "true"},
	}
	for _, tc := range cases {
		if got := unwrapCommand([]byte(tc.payload)); got != tc.want {
			t.Errorf("%s: unwrapCommand(%q) = %q, want %q", tc.name, tc.payload, got, tc.want)
		}
	}
}

func TestCommandExecutorFailureCarriesOutput(t *testing.T) {
	e := NewCommandExecutor("echo disaster >&2; exit 3")

	err := e.Execute(context.Background(), task.Task{ID: task.ID("x"), Type: "boom"})
	if err == nil {
		t.Fatal("expected error")
	}

	if !contains(err.Error(), "disaster") {
		t.Fatalf("error missing output tail: %v", err)
	}
}

func TestCommandExecutorCancellation(t *testing.T) {
	e := NewCommandExecutor("sleep 5")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := e.Execute(ctx, task.Task{ID: task.ID("x"), Type: "sleep"}); err == nil {
		t.Fatal("expected cancellation error")
	}
}

func TestHTTPExecutorRoundTrip(t *testing.T) {
	var (
		gotBody string
		gotPath string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		gotBody = string(buf[:n])
		gotPath = r.URL.Path

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	e := NewHTTPExecutor(srv.URL + "/hook")

	err := e.Execute(context.Background(), task.Task{
		ID: task.ID("t1"), Project: "p", Type: "notify", Payload: []byte(`{"m":"hi"}`),
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !contains(gotBody, `"type":"notify"`) || !contains(gotBody, `"m":"hi"`) {
		t.Fatalf("body = %q", gotBody)
	}

	if gotPath != "/hook" {
		t.Fatalf("path = %q", gotPath)
	}
}

func TestHTTPExecutorNon2xxFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	e := NewHTTPExecutor(srv.URL)
	if err := e.Execute(context.Background(), task.Task{ID: task.ID("x"), Type: "t"}); err == nil {
		t.Fatal("expected error on 500")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}

	return -1
}

func TestPermanentContract(t *testing.T) {
	if Permanent(nil) != nil {
		t.Fatal("Permanent(nil) must stay nil so success paths are unaffected")
	}

	inner := errors.New("bad payload")
	err := Permanent(fmt.Errorf("wrapped: %w", inner))

	got, ok := errors.AsType[*PermanentError](err)
	if !ok {
		t.Fatalf("AsType missed PermanentError through fmt wrapping: %v", err)
	}

	if !errors.Is(err, inner) {
		t.Fatal("PermanentError must unwrap to its cause")
	}

	if again := Permanent(error(got)); !errors.Is(again, error(got)) {
		t.Fatal("double Permanent wrap must be a no-op")
	}

	if !strings.HasPrefix(got.Error(), "permanent: ") {
		t.Fatalf("Error() = %q, want class prefix", got.Error())
	}
}

func TestCommandFailureIsPermanentCancellationIsNot(t *testing.T) {
	err := NewCommandExecutor("exit 3").Execute(context.Background(), task.Task{ID: task.ID("x"), Type: "boom"})
	if _, ok := errors.AsType[*PermanentError](err); !ok {
		t.Fatalf("non-zero exit must be permanent, got %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = NewCommandExecutor("sleep 1").Execute(ctx, task.Task{ID: task.ID("x"), Type: "sleep"})
	if err == nil {
		t.Fatal("expected cancellation error")
	}

	if _, ok := errors.AsType[*PermanentError](err); ok {
		t.Fatalf("cancellation must stay transient, got %v", err)
	}
}

func TestHTTPStatusClassification(t *testing.T) {
	cases := []struct {
		code      int
		permanent bool
	}{
		{http.StatusBadRequest, true},
		{http.StatusUnauthorized, true},
		{http.StatusNotFound, true},
		{http.StatusInternalServerError, false},
		{http.StatusServiceUnavailable, false},
		{http.StatusTooManyRequests, false},
		{http.StatusRequestTimeout, false},
	}
	for _, tc := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(tc.code)
		}))
		err := NewHTTPExecutor(srv.URL).Execute(context.Background(), task.Task{ID: task.ID("x"), Type: "t"})
		srv.Close()

		_, got := errors.AsType[*PermanentError](err)
		if got != tc.permanent || err == nil {
			t.Errorf("status %d: permanent=%v (err=%v), want permanent=%v", tc.code, got, err, tc.permanent)
		}
	}
}

func TestExtractResultPayload(t *testing.T) {
	out := "did stuff\nTQ_RESULT: {\"files_changed\":[\"a.go\",\"b.go\"],\"commit_sha\":\"abc123\"}\ndone\n"

	files, sha, ok := ExtractResultPayload(out)
	if !ok || len(files) != 2 || files[0] != "a.go" || sha != "abc123" {
		t.Fatalf("got %q %q %v, want files+sha", files, sha, ok)
	}

	if _, _, ok := ExtractResultPayload("no marker here"); ok {
		t.Error("output without marker reported ok")
	}

	if _, _, ok := ExtractResultPayload("TQ_RESULT: {broken json}"); ok {
		t.Error("malformed JSON reported ok")
	}
}

func TestWriteOutputSidecar(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TQ_LOG_DIR", dir)

	id := task.ID("testtask0001")
	if path := writeOutputSidecar(id, "agent out", "verify out"); path == "" {
		t.Fatal("sidecar not written")
	} else {
		b, err := os.ReadFile(path)
		if err != nil || !strings.Contains(string(b), "agent out") || !strings.Contains(string(b), "verify out") {
			t.Fatalf("sidecar content wrong: %q %v", b, err)
		}
	}

	t.Setenv("TQ_LOG_DIR", "")

	if path := writeOutputSidecar(id, "x", "y"); path != "" {
		t.Fatalf("sidecar written without TQ_LOG_DIR: %q", path)
	}
}

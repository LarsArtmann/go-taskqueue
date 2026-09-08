package webui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

// TestRoutesAreReadOnly is the ADR-0003 guardrail: the dashboard registers
// no mutating handler. A new POST/PUT/DELETE route fails here, and with it
// the invariant that the worst webui failure is a stale dashboard, never
// journal corruption.
func TestRoutesAreReadOnly(t *testing.T) {
	t.Parallel()

	bindings := (&Server{}).routeBindings()

	if len(bindings) == 0 {
		t.Fatal("route table is empty")
	}

	for _, route := range bindings {
		if route.method != http.MethodGet {
			t.Errorf("route %s %s is not read-only (method %s)", route.method, route.pattern, route.method)
		}
	}
}

func TestSecurityHeadersOnEveryResponse(t *testing.T) {
	t.Parallel()

	s := New(newTestStore(t), Config{})
	server := httptest.NewServer(s.Handler())
	t.Cleanup(server.Close)

	paths := []string{"/", "/task/nope", "/api/stats", "/static/definitely-missing.css"}

	for _, path := range paths {
		resp, err := http.Get(server.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}

		_ = resp.Body.Close()

		csp := resp.Header.Get("Content-Security-Policy")
		if !strings.Contains(csp, "default-src 'none'") {
			t.Errorf("%s: CSP missing default-src 'none', got %q", path, csp)
		}

		if !strings.Contains(csp, "script-src 'self'") || strings.Contains(csp, "unsafe-inline") {
			t.Errorf("%s: CSP must be script-src 'self' without unsafe-inline, got %q", path, csp)
		}

		if got := resp.Header.Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("%s: X-Content-Type-Options = %q, want nosniff", path, got)
		}

		if got := resp.Header.Get("Referrer-Policy"); got != "no-referrer" {
			t.Errorf("%s: Referrer-Policy = %q, want no-referrer", path, got)
		}

		if got := resp.Header.Get("X-Frame-Options"); got != "DENY" {
			t.Errorf("%s: X-Frame-Options = %q, want DENY", path, got)
		}
	}
}

// TestSecurityHeadersBeforeAuth pins the middleware order: even a 401 from
// the token guard must carry the security headers (browsers apply CSP to
// error pages too).
func TestSecurityHeadersBeforeAuth(t *testing.T) {
	t.Parallel()

	s := New(newTestStore(t), Config{AuthToken: "secret"})
	server := httptest.NewServer(s.Handler())
	t.Cleanup(server.Close)

	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}

	if csp := resp.Header.Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'none'") {
		t.Fatalf("401 response missing CSP, got %q", csp)
	}
}

func TestParseFilterPageClamping(t *testing.T) {
	t.Parallel()

	cases := []struct {
		raw  string
		want int
	}{
		{"", 1},
		{"?page=0", 1},
		{"?page=-3", 1},
		{"?page=abc", 1},
		{"?page=2", 2},
		{"?page=999", 999},
	}

	for _, tc := range cases {
		got := parseFilter(httptest.NewRequest(http.MethodGet, "/"+tc.raw, nil))
		if got.Page != tc.want {
			t.Errorf("page for %q = %d, want %d", tc.raw, got.Page, tc.want)
		}
	}
}

// TestPaginationEdges drives loadSnapshot past both page boundaries:
// page 0 clamps to 1, a page past the last one renders an empty table
// with a pager that still says where the end is.
func TestPaginationEdges(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newTestStore(t)

	for range 5 {
		if _, err := store.Enqueue(
			ctx,
			task.New{Project: "p", Type: "sh", Payload: json.RawMessage(`"true"`)},
		); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	s := New(store, Config{})

	snap := func(page int) DashboardData {
		t.Helper()

		data, err := s.loadSnapshot(ctx, FilterState{Page: page})
		if err != nil {
			t.Fatalf("loadSnapshot(page=%d): %v", page, err)
		}

		return data
	}

	first := snap(0)
	if first.Page != 1 || len(first.Tasks) != 5 || first.TotalPages != 1 {
		t.Fatalf("page 0: page=%d rows=%d pages=%d, want 1/5/1", first.Page, len(first.Tasks), first.TotalPages)
	}

	overflow := snap(9)
	if overflow.Page != 9 || len(overflow.Tasks) != 0 {
		t.Fatalf("overflow: page=%d rows=%d, want page 9 with 0 rows", overflow.Page, len(overflow.Tasks))
	}

	if overflow.TotalPages != 1 {
		t.Fatalf("overflow pages = %d, want 1", overflow.TotalPages)
	}
}

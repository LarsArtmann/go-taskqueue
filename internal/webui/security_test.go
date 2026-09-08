package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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

	s := New(nil, Config{}) // handlers under test never touch the store
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

	s := New(nil, Config{AuthToken: "secret"})
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

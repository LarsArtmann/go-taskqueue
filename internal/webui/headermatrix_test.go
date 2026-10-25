package webui

import (
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/httpapi"
)

// This file pins the SECURITY.md "Response headers, per surface" matrix:
// every surface's response-header SET must match its matrix row — a header
// added to code without a row (or vice versa) fails here, so the doc and
// the shipping code cannot drift apart silently. Per SECURITY.md's
// same-change rule: a new header means a matrix row AND an entry in the
// corresponding set below, in the same change.

// matrixHeaders is one surface's allowed response headers: name → value
// check (nil = presence only). A response header outside the set, or a set
// member missing from the response, fails.
type matrixHeaders map[string]func(t *testing.T, got string)

// surface is a matrix row plus the names that may legitimately be absent
// from that surface's responses (conditionally present, e.g. Retry-After
// only on the lockout 429). Every other member's presence is strict: a
// missing member fails, so the per-surface list must name each exception
// deliberately instead of a global header-name skip.
type surface struct {
	row         matrixHeaders
	conditional map[string]bool
}

func (m matrixHeaders) surface(conditional ...string) surface {
	s := surface{row: m, conditional: map[string]bool{}}
	for _, name := range conditional {
		s.conditional[strings.ToLower(name)] = true
	}

	return s
}

// transportHeaders are framing headers no matrix row tracks (they vary
// with the body, not the security posture).
var transportHeaders = map[string]bool{
	"date":              true,
	"content-length":    true,
	"content-type":      true,
	"set-cookie":        true,
	"trailer":           true,
	"x-accel-buffering": true,
}

func (s surface) check(t *testing.T, header http.Header, where string) {
	t.Helper()

	for name, vals := range header {
		lower := strings.ToLower(name)

		if transportHeaders[lower] {
			continue
		}

		check, ok := s.row[lower]
		if !ok {
			t.Errorf("%s: response header %q is not in the SECURITY.md matrix for this surface", where, name)

			continue
		}

		if check != nil {
			check(t, strings.Join(vals, ", "))
		}
	}

	for name := range s.row {
		if header.Get(name) == "" && !s.conditional[name] {
			t.Errorf("%s: SECURITY.md matrix header %q missing from response", where, name)
		}
	}
}

func eq(want string) func(*testing.T, string) {
	return func(t *testing.T, got string) {
		t.Helper()

		if got != want {
			t.Errorf("header = %q, want %q", got, want)
		}
	}
}

func contains(sub string) func(*testing.T, string) {
	return func(t *testing.T, got string) {
		t.Helper()

		if !strings.Contains(got, sub) {
			t.Errorf("header = %q, want substring %q", got, sub)
		}
	}
}

func notContains(sub string) func(*testing.T, string) {
	return func(t *testing.T, got string) {
		t.Helper()

		if strings.Contains(got, sub) {
			t.Errorf("header = %q, must not contain %q", got, sub)
		}
	}
}

// serveDashboardRow is the matrix's `tq serve` task-page row: the full
// strict set, no robots header (task pages carry a robots meta instead),
// no Cache-Control (HTML and SSE unset).
var serveDashboardRow = matrixHeaders{
	"content-security-policy": func(t *testing.T, got string) {
		t.Helper()

		contains("default-src 'none'")(t, got)
		contains("frame-ancestors 'none'")(t, got)
		contains("form-action 'self'")(t, got)
		notContains("unsafe-eval")(t, got)
	},
	"x-content-type-options": eq("nosniff"),
	"x-frame-options":        eq("DENY"),
	"referrer-policy":        eq("no-referrer"),
	"permissions-policy":     eq("camera=(), microphone=(), geolocation=()"),
}

// serveHealthRow is the `/health*` row: same strict base set plus the
// health override (library CSP with 'unsafe-eval', re-hardened) and the
// X-Robots-Tag noindex smoke (18-12:82).
var serveHealthRow = matrixHeaders{
	"content-security-policy": func(t *testing.T, got string) {
		t.Helper()

		contains("unsafe-eval")(t, got)
		contains("base-uri 'none'")(t, got)
		contains("frame-ancestors 'none'")(t, got)
		contains("form-action 'none'")(t, got)
	},
	"x-content-type-options": eq("nosniff"),
	"x-frame-options":        eq("DENY"),
	"referrer-policy":        eq("no-referrer"),
	"permissions-policy":     eq("camera=(), microphone=(), geolocation=()"),
	"x-robots-tag":           eq("noindex"),
	"cache-control":          eq("no-cache"),
}

// TestServeHeaderMatrix pins the `tq serve` column: task pages, the 401
// realm, and the /health* override all match their matrix rows exactly.
func TestServeHeaderMatrix(t *testing.T) {
	t.Parallel()

	t.Run("task page", func(t *testing.T) {
		t.Parallel()

		s := New(newTestStore(t), Config{})
		server := httptest.NewServer(s.Handler())
		t.Cleanup(server.Close)

		resp, err := http.Get(server.URL + "/")
		if err != nil {
			t.Fatalf("GET /: %v", err)
		}
		defer resp.Body.Close()

		serveDashboardRow.surface().check(t, resp.Header, "GET /")

		if got := resp.Header.Get("X-Robots-Tag"); got != "" {
			t.Errorf("task page X-Robots-Tag = %q, want unset (robots meta instead)", got)
		}
	})

	t.Run("401 carries realm and full set", func(t *testing.T) {
		t.Parallel()

		s := New(newTestStore(t), Config{AuthToken: "secret"})
		server := httptest.NewServer(s.Handler())
		t.Cleanup(server.Close)

		resp, err := http.Get(server.URL + "/")
		if err != nil {
			t.Fatalf("GET /: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", resp.StatusCode)
		}

		with401 := matrixHeaders{
			"www-authenticate": eq(`Bearer realm="tq dashboard"`),
		}
		for k, v := range serveDashboardRow {
			with401[k] = v
		}
		with401.surface().check(t, resp.Header, "GET / (401)")
	})

	t.Run("write-route CSRF lockout 429 carries Retry-After and the full set", func(t *testing.T) {
		t.Parallel()

		s := New(newTestStore(t), Config{AllowWrites: true})
		server := httptest.NewServer(s.Handler())
		t.Cleanup(server.Close)

		// A page GET issues the CSRF cookie; the lockout limiter counts
		// 403 CSRF failures and locks the write routes on the third.
		first, err := http.Get(server.URL + "/")
		if err != nil {
			t.Fatalf("GET /: %v", err)
		}
		first.Body.Close()

		var csrf string
		for _, c := range first.Cookies() {
			if c.Name == tqCSRFCookie {
				csrf = c.Value
			}
		}
		if csrf == "" {
			t.Fatal("no CSRF cookie issued on page GET")
		}

		post := func() *http.Response {
			t.Helper()

			req, err := http.NewRequest(http.MethodPost, server.URL+"/task/x/cancel", strings.NewReader("csrf=bad"))
			if err != nil {
				t.Fatalf("new request: %v", err)
			}
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.AddCookie(&http.Cookie{Name: tqCSRFCookie, Value: csrf})

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("POST /task/x/cancel: %v", err)
			}

			return resp
		}

		for i := range 3 {
			resp := post()
			resp.Body.Close()

			if resp.StatusCode != http.StatusForbidden {
				t.Fatalf("bad CSRF %d = %d, want 403", i+1, resp.StatusCode)
			}
		}

		resp := post()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusTooManyRequests {
			t.Fatalf("status = %d, want 429 after CSRF lockout", resp.StatusCode)
		}

		with429 := matrixHeaders{
			"retry-after": func(t *testing.T, got string) {
				t.Helper()

				if n, err := strconv.Atoi(got); err != nil || n < 1 {
					t.Errorf("Retry-After = %q, want a positive second count", got)
				}
			},
		}
		maps.Copy(with429, serveDashboardRow)
		with429.surface().check(t, resp.Header, "POST /task/x/cancel (429)")
	})

	t.Run("health page overrides CSP and adds noindex", func(t *testing.T) {
		t.Parallel()

		s := New(newTestStore(t), Config{})
		server := httptest.NewServer(s.Handler())
		t.Cleanup(server.Close)

		resp, err := http.Get(server.URL + "/health")
		if err != nil {
			t.Fatalf("GET /health: %v", err)
		}
		defer resp.Body.Close()

		serveHealthRow.surface().check(t, resp.Header, "GET /health")
	})

	t.Run("remaining /health* routes match the health row", func(t *testing.T) {
		t.Parallel()

		s := New(newTestStore(t), Config{})
		server := httptest.NewServer(s.Handler())
		t.Cleanup(server.Close)

		for _, path := range []string{
			"/health/favicon.svg",
			"/healthz",
			"/readyz",
			"/startupz",
		} {
			resp, err := http.Get(server.URL + path)
			if err != nil {
				t.Fatalf("GET %s: %v", path, err)
			}
			resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("GET %s: status = %d, want 200", path, resp.StatusCode)
			}

			// The bare probes (healthz/readyz/startupz) carry no
			// Cache-Control — only /health HTML and the SDK assets do;
			// the favicon row below pins its own cacheable value.
			row := serveHealthRow.surface("cache-control")
			if path == "/health/favicon.svg" {
				// The favicon is a static asset like the SDK bundle: the
				// matrix pins it cacheable, not no-cache.
				favicon := matrixHeaders{}
				maps.Copy(favicon, serveHealthRow)
				favicon["cache-control"] = eq("public, max-age=86400")
				row = favicon.surface()
			}

			row.check(t, resp.Header, "GET "+path)
		}
	})

	t.Run("health SDK bundle is the only cacheable response", func(t *testing.T) {
		t.Parallel()

		s := New(newTestStore(t), Config{})
		server := httptest.NewServer(s.Handler())
		t.Cleanup(server.Close)

		resp, err := http.Get(server.URL + "/health/datastar.js")
		if err != nil {
			t.Fatalf("GET /health/datastar.js: %v", err)
		}
		defer resp.Body.Close()

		withCache := matrixHeaders{}
		for k, v := range serveHealthRow {
			withCache[k] = v
		}
		withCache["cache-control"] = eq("public, max-age=86400")
		withCache.surface().check(t, resp.Header, "GET /health/datastar.js")
	})
}

// TestAPIHeaderMatrix pins the `tq api` column: JSON responses carry
// nosniff (401/429 included) and no-store when authenticated, the API
// realm on 401, Retry-After on the bearer lockout — and none of the
// dashboard-only headers, because the API renders nothing.
func TestAPIHeaderMatrix(t *testing.T) {
	t.Parallel()

	srv := newAPIHeaderMatrixServer(t)
	h := srv.Handler()

	authed := matrixHeaders{
		"x-content-type-options": eq("nosniff"),
		"cache-control":          eq("no-store"),
	}
	unauthed := matrixHeaders{
		"x-content-type-options": eq("nosniff"),
		"www-authenticate":       eq(`Bearer realm="tq-api"`),
	}
	locked := matrixHeaders{
		"x-content-type-options": eq("nosniff"),
		"retry-after":            nil,
	}

	// Dashboard-only headers: everything in the matrix's `tq serve`
	// column minus the headers the API's own rows carry — a new dashboard
	// header joins this absence check automatically.
	serveColumn := matrixHeaders{}
	maps.Copy(serveColumn, serveDashboardRow)
	maps.Copy(serveColumn, serveHealthRow)

	apiRows := matrixHeaders{}
	maps.Copy(apiRows, authed)
	maps.Copy(apiRows, unauthed)
	maps.Copy(apiRows, locked)

	apiHeaders := []string{}
	for name := range serveColumn {
		if _, owned := apiRows[name]; !owned {
			apiHeaders = append(apiHeaders, name)
		}
	}
	slices.Sort(apiHeaders)

	t.Run("authenticated response", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
		req.Header.Set("Authorization", "Bearer secret-token")

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		authed.surface().check(t, rec.Header(), "GET /api/v1/stats")
	})

	t.Run("401 carries the API realm and no dashboard headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil)

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
		unauthed.surface().check(t, rec.Header(), "GET /api/v1/healthz (401)")

		for _, name := range apiHeaders {
			if got := rec.Header().Get(name); got != "" {
				t.Errorf("401 header %q = %q, want unset (the API renders nothing)", name, got)
			}
		}
	})

	t.Run("429 lockout carries Retry-After", func(t *testing.T) {
		for i := range 3 {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil)
			req.Header.Set("Authorization", "Bearer wrong")

			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if i < 2 && rec.Code != http.StatusUnauthorized {
				t.Fatalf("failed auth %d = %d, want 401", i+1, rec.Code)
			}
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil)
		req.Header.Set("Authorization", "Bearer wrong")

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("status = %d, want 429 after lockout", rec.Code)
		}
		locked.surface().check(t, rec.Header(), "GET /api/v1/healthz (429)")
	})
}

func newAPIHeaderMatrixServer(t *testing.T) *httpapi.Server {
	t.Helper()

	srv, err := httpapi.New(newTestStore(t), "secret-token", nil)
	if err != nil {
		t.Fatalf("new api: %v", err)
	}

	return srv
}

package webui

import (
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
type matrixHeaders map[string]func(t testing.TB, got string)

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

func (s surface) check(t testing.TB, header http.Header, where string) {
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

func eq(want string) func(testing.TB, string) {
	return func(t testing.TB, got string) {
		t.Helper()

		if got != want {
			t.Errorf("header = %q, want %q", got, want)
		}
	}
}

func contains(sub string) func(testing.TB, string) {
	return func(t testing.TB, got string) {
		t.Helper()

		if !strings.Contains(got, sub) {
			t.Errorf("header = %q, want substring %q", got, sub)
		}
	}
}

func notContains(sub string) func(testing.TB, string) {
	return func(t testing.TB, got string) {
		t.Helper()

		if strings.Contains(got, sub) {
			t.Errorf("header = %q, must not contain %q", got, sub)
		}
	}
}

// mergeRow overlays additions onto a base row (shared matrix idiom).
func mergeRow(base, additions matrixHeaders) matrixHeaders {
	row := matrixHeaders{}
	maps.Copy(row, base)
	maps.Copy(row, additions)

	return row
}

func positiveSeconds(t testing.TB, got string) {
	t.Helper()

	if n, err := strconv.Atoi(got); err != nil || n < 1 {
		t.Errorf("Retry-After = %q, want a positive second count", got)
	}
}

// serveDashboardRow is the matrix's `tq serve` task-page row: the full
// strict set, no robots header (task pages carry a robots meta instead),
// no Cache-Control (HTML and SSE unset).
var serveDashboardRow = matrixHeaders{
	"content-security-policy": func(t testing.TB, got string) {
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

// serve401Row is the dashboard 401 realm: the full task-page set plus the
// WWW-Authenticate challenge.
var serve401Row = mergeRow(serveDashboardRow, matrixHeaders{
	"www-authenticate": eq(`Bearer realm="tq dashboard"`),
})

// serve429Row is the write-route CSRF-lockout 429: the full task-page set
// plus the Retry-After second count.
var serve429Row = mergeRow(serveDashboardRow, matrixHeaders{
	"retry-after": positiveSeconds,
})

// apiAuthedRow / apiUnauthedRow / apiLockedRow are the `tq api` column:
// nosniff everywhere, no-store when authenticated, the API realm on 401,
// Retry-After on the bearer lockout.
var (
	apiAuthedRow = matrixHeaders{
		"x-content-type-options": eq("nosniff"),
		"cache-control":          eq("no-store"),
	}
	apiUnauthedRow = matrixHeaders{
		"x-content-type-options": eq("nosniff"),
		"www-authenticate":       eq(`Bearer realm="tq-api"`),
	}
	apiLockedRow = matrixHeaders{
		"x-content-type-options": eq("nosniff"),
		"retry-after":            positiveSeconds,
	}
)

// matrixRows is the canonical inventory of every surface row in the
// SECURITY.md "Response headers, per surface" matrix. The doc test derives
// the expected header-name set from this slice, so the doc table and these
// Go tables cannot diverge in either direction; a new header means an entry
// in the row(s) here AND a doc table row, in the same change.
var matrixRows = []matrixHeaders{
	serveDashboardRow,
	serve401Row,
	serve429Row,
	serveHealthRow,
	apiAuthedRow,
	apiUnauthedRow,
	apiLockedRow,
}

// serveHealthRow is the `/health*` row: same strict base set plus the
// health override (library CSP with 'unsafe-eval', re-hardened) and the
// X-Robots-Tag noindex smoke (18-12:82).
var serveHealthRow = matrixHeaders{
	"content-security-policy": func(t testing.TB, got string) {
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

		serve401Row.surface().check(t, resp.Header, "GET / (401)")
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

		with429 := mergeRow(serveDashboardRow, matrixHeaders{
			"retry-after": positiveSeconds,
		})
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

		for _, binding := range s.healthBindings() {
			path := binding.pattern
			if path == HealthDashboardPath || path == HealthSDKPath {
				continue // pinned by their own subtests above/below
			}
			if path == HealthSSEPath {
				// The SSE stream 503s without a live hub; its headers are
				// pinned by the SSE subtest, not the plain-GET matrix.
				continue
			}
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
		maps.Copy(withCache, serveHealthRow)
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

	authed := apiAuthedRow
	unauthed := apiUnauthedRow
	locked := apiLockedRow

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

// failingTB records Errorf calls instead of failing the real test, so the
// negative-path self-check can assert that surface.check actually reports
// broken headers.
type failingTB struct {
	testing.TB
	failures []string
}

func (f *failingTB) Errorf(format string, args ...any) {
	f.failures = append(f.failures, fmt.Sprintf(format, args...))
}

func (f *failingTB) Helper() {}

// TestMatrixCheckNegativePath proves the matrix pin cannot rot vacuous: a
// fixture handler emitting a deliberately broken header set (value drifted
// from the matrix, strict member missing, or an unlisted header present)
// must produce failures; a fully conforming set must produce none.
func TestMatrixCheckNegativePath(t *testing.T) {
	t.Parallel()

	row := serveDashboardRow.surface()

	fixtures := []struct {
		name   string
		header http.Header
	}{
		{
			name:   "clean fixture passes",
			header: http.Header{"Content-Security-Policy": []string{`default-src 'none'; frame-ancestors 'none'; form-action 'self'`}, "X-Content-Type-Options": []string{"nosniff"}, "X-Frame-Options": []string{"DENY"}, "Referrer-Policy": []string{"no-referrer"}, "Permissions-Policy": []string{"camera=(), microphone=(), geolocation=()"}},
		},
		{
			name:   "broken CSP value fails",
			header: http.Header{"Content-Security-Policy": []string{`default-src 'self'`}, "X-Content-Type-Options": []string{"nosniff"}, "X-Frame-Options": []string{"DENY"}, "Referrer-Policy": []string{"no-referrer"}, "Permissions-Policy": []string{"camera=(), microphone=(), geolocation=()"}},
		},
		{
			name:   "missing strict member fails",
			header: http.Header{"Content-Security-Policy": []string{`default-src 'none'; frame-ancestors 'none'; form-action 'self'`}, "X-Content-Type-Options": []string{"nosniff"}, "Referrer-Policy": []string{"no-referrer"}, "Permissions-Policy": []string{"camera=(), microphone=(), geolocation=()"}},
		},
		{
			name:   "unlisted header fails",
			header: http.Header{"Content-Security-Policy": []string{`default-src 'none'; frame-ancestors 'none'; form-action 'self'`}, "X-Content-Type-Options": []string{"nosniff"}, "X-Frame-Options": []string{"DENY"}, "Referrer-Policy": []string{"no-referrer"}, "Permissions-Policy": []string{"camera=(), microphone=(), geolocation=()"}, "X-Powered-By": []string{"broken"}},
		},
	}

	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()

			handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				for name, vals := range fixture.header {
					for _, v := range vals {
						w.Header().Add(name, v)
					}
				}
			})
			server := httptest.NewServer(handler)
			t.Cleanup(server.Close)

			resp, err := http.Get(server.URL)
			if err != nil {
				t.Fatalf("GET fixture: %v", err)
			}
			resp.Body.Close()

			tb := &failingTB{TB: t}
			row.check(tb, resp.Header, "fixture")

			if fixture.name == "clean fixture passes" {
				if len(tb.failures) != 0 {
					t.Errorf("clean fixture reported %d failures, want 0: %v", len(tb.failures), tb.failures)
				}

				return
			}

			if len(tb.failures) == 0 {
				t.Error("deliberately broken fixture header passed the matrix check — the pin is vacuous")
			}
		})
	}
}

func newAPIHeaderMatrixServer(t *testing.T) *httpapi.Server {
	t.Helper()

	srv, err := httpapi.New(newTestStore(t), "secret-token", nil)
	if err != nil {
		t.Fatalf("new api: %v", err)
	}

	return srv
}

// TestSecurityDocMatrixMatchesGoMatrix single-sources the SECURITY.md
// "Response headers, per surface" matrix against the Go rows: the doc
// table's header-name column must equal the union of matrixRows exactly,
// in both directions. A header added to the doc without a Go row — or to
// a Go row without a doc row — fails here, so the two hand-edited copies
// cannot drift apart independently.
func TestSecurityDocMatrixMatchesGoMatrix(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join("..", "..", "SECURITY.md"))
	if err != nil {
		t.Fatalf("read SECURITY.md: %v", err)
	}

	docNames := map[string]bool{}
	inMatrix := false

	for line := range strings.SplitSeq(string(raw), "\n") {
		switch {
		case strings.HasPrefix(line, "## Response headers, per surface"):
			inMatrix = true
		case inMatrix && strings.HasPrefix(line, "## "):
			inMatrix = false
		case inMatrix && strings.HasPrefix(line, "| `"):
			// Row format: | `Header-Name` | serve cell | api cell |
			_, rest, _ := strings.Cut(line, "`")
			name, _, _ := strings.Cut(rest, "`")
			docNames[strings.ToLower(name)] = true
		}
	}

	if len(docNames) == 0 {
		t.Fatal("no matrix rows parsed from SECURITY.md — section renamed or table reformatted?")
	}

	goNames := map[string]bool{}
	for _, row := range matrixRows {
		for name := range row {
			goNames[name] = true
		}
	}

	for name := range docNames {
		if !goNames[name] {
			t.Errorf("SECURITY.md matrix header %q has no Go matrix row — add it to matrixRows in headermatrix_test.go", name)
		}
	}

	for name := range goNames {
		if !docNames[name] {
			t.Errorf("Go matrix row header %q missing from the SECURITY.md matrix — add a table row", name)
		}
	}
}

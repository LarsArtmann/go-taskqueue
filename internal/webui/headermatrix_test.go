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
	contentType func(testing.TB, string)
}

// withContentType pins the Content-Type the surface's responses must carry
// (full media type incl. charset, as observed live 2026-10-30). Content-Type
// stays in transportHeaders — rows are shared across surfaces whose media
// types diverge (the task-page row rides JSON and SSE routes too) — so the
// pin lives on the surface, not the row.
func (s surface) withContentType(mediaType string) surface {
	s.contentType = eq(mediaType)

	return s
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
	"connection":        true,
}

func (s surface) check(t testing.TB, header http.Header, where string) {
	t.Helper()

	if s.contentType != nil {
		s.contentType(t, header.Get("Content-Type"))
	}

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

// getContextGET issues a GET via NewRequestWithContext + client.Do (the
// noctx-clean form) for the matrix family.
func getContextGET(t testing.TB, url string) *http.Response {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("new request %s: %v", url, err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}

	return resp
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

// serveFaviconRow is the `/health/favicon.svg` row: the health set with a
// cacheable Cache-Control (static asset like the SDK bundle, not no-cache).
var serveFaviconRow = mergeRow(serveHealthRow, matrixHeaders{
	"cache-control": eq("public, max-age=86400"),
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

// newServeServer boots a `tq serve` test instance for the matrix family:
// one server per surface test, handler under the given config.
func newServeServer(t *testing.T, cfg Config) (*Server, *httptest.Server) {
	t.Helper()

	s := New(newTestStore(t), cfg)
	server := httptest.NewServer(s.Handler())
	t.Cleanup(server.Close)

	return s, server
}

// The TestServeHeaderMatrix* functions pin the `tq serve` column of the
// SECURITY.md "Response headers, per surface" matrix — one surface per
// function (split from a single TestServeHeaderMatrix to keep gocognit
// down): task pages, the 401 realm, the CSRF-lockout 429, the /health
// override, the remaining /health* probes, and the SDK bundle all match
// their matrix rows exactly.

func TestServeHeaderMatrixTaskPage(t *testing.T) {
	t.Parallel()

	_, server := newServeServer(t, Config{})

	resp := getContextGET(t, server.URL+"/")
	defer resp.Body.Close()

	serveDashboardRow.surface().withContentType("text/html; charset=utf-8").check(t, resp.Header, "GET /")

	if got := resp.Header.Get("X-Robots-Tag"); got != "" {
		t.Errorf("task page X-Robots-Tag = %q, want unset (robots meta instead)", got)
	}
}

func TestServeHeaderMatrix401(t *testing.T) {
	t.Parallel()

	_, server := newServeServer(t, Config{AuthToken: "secret"})

	resp := getContextGET(t, server.URL+"/")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}

	serve401Row.surface().withContentType("text/plain; charset=utf-8").check(t, resp.Header, "GET / (401)")
}

func TestServeHeaderMatrix429Lockout(t *testing.T) {
	t.Parallel()

	_, server := newServeServer(t, Config{AllowWrites: true})

	// A page GET issues the CSRF cookie; the lockout limiter counts
	// 403 CSRF failures and locks the write routes on the third.
	first := getContextGET(t, server.URL+"/")
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

	serve429Row.surface().withContentType("text/plain; charset=utf-8").check(t, resp.Header, "POST /task/x/cancel (429)")
}

func TestServeHeaderMatrixHealthPage(t *testing.T) {
	t.Parallel()

	_, server := newServeServer(t, Config{})

	resp := getContextGET(t, server.URL+"/health")
	defer resp.Body.Close()

	serveHealthRow.surface().withContentType("text/html; charset=utf-8").check(t, resp.Header, "GET /health")
}

func TestServeHeaderMatrixHealthProbes(t *testing.T) {
	t.Parallel()

	s, server := newServeServer(t, Config{})

	for _, binding := range s.healthBindings() {
		path := binding.pattern
		if path == HealthDashboardPath || path == HealthSDKPath {
			continue // pinned by their own TestServeHeaderMatrix* functions
		}
		if path == HealthSSEPath {
			// The SSE stream 503s without a live hub; its headers are
			// pinned by the SSE subtest, not the plain-GET matrix.
			continue
		}
		resp := getContextGET(t, server.URL+path)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s: status = %d, want 200", path, resp.StatusCode)
		}

		// The bare probes (healthz/readyz/startupz) carry no
		// Cache-Control — only /health HTML and the SDK assets do;
		// the favicon pins its own cacheable value.
		row := serveHealthRow.surface("cache-control").withContentType("application/json")
		if path == "/health/favicon.svg" {
			row = serveFaviconRow.surface().withContentType("image/svg+xml")
		}

		row.check(t, resp.Header, "GET "+path)
	}
}

func TestServeHeaderMatrixSDKBundle(t *testing.T) {
	t.Parallel()

	_, server := newServeServer(t, Config{})

	resp := getContextGET(t, server.URL+"/health/datastar.js")
	defer resp.Body.Close()

	withCache := mergeRow(serveHealthRow, matrixHeaders{
		"cache-control": eq("public, max-age=86400"),
	})
	withCache.surface().withContentType("text/javascript; charset=utf-8").check(t, resp.Header, "GET /health/datastar.js")
}

// TestServeHeaderMatrixRouteCoverage is the meta-guard over the route
// tables: every `routeBindings()`/`healthBindings()` entry must be
// exercised by an asserting subtest here, or the guard fails — a new
// binding (or a renamed pattern) can never join the server silently
// unpinned, and no skip-list entry can quietly drop a route's assertion.
// The expected row/status per route is spelled out in
// serveRouteExpectations; a binding path without an expectation entry is
// a failure, and every expectation entry is asserted on every run.
func TestServeHeaderMatrixRouteCoverage(t *testing.T) {
	t.Parallel()

	s, server := newServeServer(t, Config{})
	expectations := serveRouteExpectations()

	var boundPaths []string
	for _, binding := range s.routeBindings() {
		boundPaths = append(boundPaths, binding.pattern)
	}
	for _, binding := range s.healthBindings() {
		boundPaths = append(boundPaths, binding.pattern)
	}
	slices.Sort(boundPaths)

	for _, pattern := range boundPaths {
		t.Run(pattern, func(t *testing.T) {
			t.Parallel()

			want, ok := expectations[pattern]
			if !ok {
				t.Fatalf("route %q has no TestServeHeaderMatrixRouteCoverage expectation — pin its matrix row in serveRouteExpectations before shipping the binding", pattern)
			}

			resp := getContextGET(t, server.URL+serveRouteURL(pattern))
			defer resp.Body.Close()

			if want.status != 0 && resp.StatusCode != want.status {
				t.Errorf("GET %s: status = %d, want %d", pattern, resp.StatusCode, want.status)
			}

			want.row.check(t, resp.Header, "GET "+pattern)
		})
	}
}

// serveRouteExpectation is one guard entry: the matrix row a route's
// plain GET must match and the status that GET must return (0 = don't
// pin; the empty-store 404/503 shape is owned by the handler tests).
type serveRouteExpectation struct {
	row    surface
	status int
}

// serveRouteExpectations maps every routeBindings()/healthBindings()
// pattern to its expectation. A binding path without an entry fails the
// guard; every entry is asserted on every run.
func serveRouteExpectations() map[string]serveRouteExpectation {
	probes := serveHealthRow.surface("cache-control").withContentType("application/json")

	return map[string]serveRouteExpectation{
		// Dashboard read routes carry the full task-page set.
		"/{$}":            {serveDashboardRow.surface().withContentType("text/html; charset=utf-8"), http.StatusOK},
		"/project/{name}": {serveDashboardRow.surface().withContentType("text/html; charset=utf-8"), 0},
		"/task/{id}":      {serveDashboardRow.surface().withContentType("text/html; charset=utf-8"), 0},
		"/api/stats":      {serveDashboardRow.surface().withContentType("application/json"), 0},
		"/api/facts":      {serveDashboardRow.surface().withContentType("application/json"), 0},
		"/static/":        {serveDashboardRow.surface().withContentType("text/html; charset=utf-8"), 0},
		// SSE endpoints stream (Cache-Control no-cache from the SSE
		// writer; the per-task stream 503s without a live hub and
		// omits it, so its absence is conditional there).
		"/api/events":       {mergeRow(serveDashboardRow, matrixHeaders{"cache-control": eq("no-cache")}).surface().withContentType("text/event-stream"), 0},
		"/task/{id}/events": {serveDashboardRow.surface("cache-control").withContentType("text/html; charset=utf-8"), 0},
		// Health surface routes carry the health override set.
		HealthDashboardPath: {serveHealthRow.surface().withContentType("text/html; charset=utf-8"), http.StatusOK},
		HealthSSEPath:       {probes.withContentType("text/plain; charset=utf-8"), 0},
		HealthSDKPath:       {serveFaviconRow.surface().withContentType("text/javascript; charset=utf-8"), http.StatusOK},
		HealthFaviconPath:   {serveFaviconRow.surface().withContentType("image/svg+xml"), http.StatusOK},
		HealthLivenessPath:  {probes, http.StatusOK},
		HealthReadinessPath: {probes, http.StatusOK},
		HealthStartupPath:   {probes, http.StatusOK},
	}
}

// serveRouteURL turns a route pattern into a concrete GET URL for the
// header guard: wildcard segments get filler values that exercise the
// handler without depending on store contents.
func serveRouteURL(pattern string) string {
	replacements := map[string]string{
		"/{$}":              "/",
		"/project/{name}":   "/project/x",
		"/task/{id}":        "/task/x",
		"/task/{id}/events": "/task/x/events",
	}
	if url, ok := replacements[pattern]; ok {
		return url
	}

	return pattern
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
		authed.surface().withContentType("application/json").check(t, rec.Header(), "GET /api/v1/stats")
	})

	t.Run("401 carries the API realm and no dashboard headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil)

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
		unauthed.surface().withContentType("application/json").check(t, rec.Header(), "GET /api/v1/healthz (401)")

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
		locked.surface().withContentType("application/json").check(t, rec.Header(), "GET /api/v1/healthz (429)")
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

			resp := getContextGET(t, server.URL)
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

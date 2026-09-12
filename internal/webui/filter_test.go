package webui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

// TestFilterRoundTripAllFields pins the emitter↔parser contract for every
// FilterState field: the param names are a single source of truth (parsed
// names == emitted names), allowlists fall back exactly as documented, and
// filterHref → parseFilter round-trips byte-identical — including values
// that need URL escaping (the literal-leak class bit twice).
func TestFilterRoundTripAllFields(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		raw   string
		want  FilterState
		eqAll bool // compare all fields (false: only the named field pins)
	}{
		{
			"project",
			"/?project=alpha",
			FilterState{Project: "alpha", Page: 1, View: viewTable},
			true,
		},
		{
			"status",
			"/?status=running",
			FilterState{Status: task.Running, Page: 1, View: viewTable},
			true,
		},
		{
			"query",
			"/?q=sh",
			FilterState{Query: "sh", Page: 1, View: viewTable},
			true,
		},
		{
			"sort known",
			"/?sort=age-desc",
			FilterState{Sort: "age-desc", Page: 1, View: viewTable},
			true,
		},
		{
			"sort unknown falls back to default order",
			"/?sort=EVIL",
			FilterState{Sort: "", Page: 1, View: viewTable},
			true,
		},
		{
			"view board",
			"/?view=board",
			FilterState{Page: 1, View: viewBoard},
			true,
		},
		{
			"view unknown falls back to table",
			"/?view=grid",
			FilterState{Page: 1, View: viewTable},
			true,
		},
		{
			"board drops the status filter (columns ARE the statuses)",
			"/?view=board&status=dead",
			FilterState{Page: 1, View: viewBoard},
			true,
		},
		{
			"page rides but is not part of QueryString",
			"/?project=alpha&page=3",
			FilterState{Project: "alpha", Page: 3, View: viewTable},
			true,
		},
		{
			"escaped values round-trip: space, ampersand, equals, percent",
			"/?project=" + url.QueryEscape("a & b") + "&q=" + url.QueryEscape("50%x=1"),
			FilterState{Project: "a & b", Query: "50%x=1", Page: 1, View: viewTable},
			true,
		},
		{
			"unicode project round-trips",
			"/?project=" + url.QueryEscape("über-projekt"),
			FilterState{Project: "über-projekt", Page: 1, View: viewTable},
			true,
		},
	}

	for _, tc := range cases {
		got := parseFilter(httptest.NewRequest(http.MethodGet, tc.raw, nil))
		if got != tc.want {
			t.Errorf("%s: parseFilter(%q) = %+v, want %+v", tc.name, tc.raw, got, tc.want)
		}

		// Round-trip: the state re-emitted as an href must parse back to
		// the same state (page deliberately resets — chips reset to page 1).
		if got.QueryString() != "" {
			back := parseFilter(httptest.NewRequest(http.MethodGet, filterHref(got), nil))

			wantRT := got
			wantRT.Page = 1

			if back != wantRT {
				t.Errorf("%s: href round-trip = %+v, want %+v (href %q)", tc.name, back, wantRT, filterHref(got))
			}
		}
	}
}

// TestQueryStringEscapesMetacharacters pins the escape itself: a raw
// &, =, space or % in any filter value must never leak into the emitted
// query string unescaped (the class that deadened the webui filter once).
func TestQueryStringEscapesMetacharacters(t *testing.T) {
	t.Parallel()

	f := FilterState{Project: "a&b=c d", Query: "50%x"}

	qs := f.QueryString()

	for _, raw := range []string{"a&b=c d", "50%x"} {
		if strings.Contains(qs, raw) {
			t.Errorf("QueryString = %q leaks raw value %q", qs, raw)
		}
	}

	if !strings.Contains(qs, url.QueryEscape("a&b=c d")) {
		t.Errorf("QueryString = %q missing escaped project", qs)
	}
}

// TestParseFilterQueryCap pins the ?q= length cap: the query feeds a LIKE
// scan, and an unbounded value lets any request pin the read path.
func TestParseFilterQueryCap(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("x", maxQueryLen+100)

	got := parseFilter(httptest.NewRequest(http.MethodGet, "/?q="+url.QueryEscape(long), nil))
	if len(got.Query) != maxQueryLen {
		t.Errorf("capped query length = %d, want %d", len(got.Query), maxQueryLen)
	}

	exact := strings.Repeat("y", maxQueryLen)

	if got := parseFilter(httptest.NewRequest(http.MethodGet, "/?q="+exact, nil)); got.Query != exact {
		t.Error("query at exactly the cap must not be altered")
	}
}

// TestDashboardFilterE2E drives the real handler: /?q= and /?project=
// narrow the rendered table and the active filter renders back as chips.
func TestDashboardFilterE2E(t *testing.T) {
	srv, s := newTestServer(t)
	enqueue(t, s, "sh", "alpha")
	enqueue(t, s, "sh", "beta")

	get := func(target string) string {
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))

		return rec.Body.String()
	}

	alpha := get("/?project=alpha")
	if !strings.Contains(alpha, "alpha") || strings.Contains(tableFragment(alpha), "beta") {
		t.Error("project filter did not narrow the rendered table")
	}

	if !strings.Contains(alpha, "project=alpha") {
		t.Error("active project filter did not render a removable chip")
	}

	query := get("/?q=alpha")
	if strings.Contains(tableFragment(query), "beta") {
		t.Error("query filter did not narrow the rendered table")
	}

	projectPage := get("/project/alpha")
	if !strings.Contains(projectPage, "alpha") || strings.Contains(tableFragment(projectPage), "beta") {
		t.Error("/project/{name} did not pin the project filter")
	}
}

// TestPerProjectUXBatch pins the round-13 T14 board decisions: chips carry
// a total + R/P/D breakdown, an "all projects" reset chip appears when any
// filter is active, the chips row collapses beyond the cap, and the project
// column disappears from the task table when pinned to one project.
func TestPerProjectUXBatch(t *testing.T) {
	srv, s := newTestServer(t)
	enqueue(t, s, "sh", "alpha")

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/?project=alpha", nil))
	page := rec.Body.String()

	if !strings.Contains(page, "all projects") {
		t.Error("filtered view must offer the all-projects reset chip")
	}

	if !strings.Contains(page, " · ") || !strings.Contains(page, "0R/1P/0D") {
		t.Errorf("project chip must carry total + R/P/D breakdown, got: %s", page)
	}

	// The task table drops the project column when pinned: no per-row
	// /project/ link and no "project" header remain in the fragment (the
	// filter chip's ?project= link is a different element and stays).
	if strings.Contains(tableFragment(page), `href="/project/`) {
		t.Error("project column must be dropped when pinned to one project")
	}

	if thIdx := strings.Index(tableFragment(page), ">project<"); thIdx >= 0 {
		t.Errorf("project header must be dropped when pinned, found at %d", thIdx)
	}

	// Unfiltered view keeps the project column.
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	unfiltered := tableFragment(rec.Body.String())

	if !strings.Contains(unfiltered, `href="/project/`) || !strings.Contains(unfiltered, ">project<") {
		t.Error("unfiltered table must render the project column")
	}

	if strings.Contains(rec.Body.String(), "all projects") {
		t.Error("reset chip must not render without an active filter")
	}
}

// TestVisibleProjectsCap pins the chips-row overflow helper.
func TestVisibleProjectsCap(t *testing.T) {
	t.Parallel()

	all := make([]ProjectSummary, 0, chipMaxVisible+3)
	for i := range chipMaxVisible + 3 {
		all = append(all, ProjectSummary{Name: fmt.Sprintf("p%02d", i), Total: i})
	}

	shown, hidden := visibleProjects(all)
	if len(shown) != chipMaxVisible || hidden != 3 {
		t.Errorf("visibleProjects = %d shown/%d hidden, want %d/3", len(shown), hidden, chipMaxVisible)
	}

	small := all[:4]

	if got, hidden := visibleProjects(small); len(got) != 4 || hidden != 0 {
		t.Errorf("visibleProjects(4) = %d shown/%d hidden, want 4/0", len(got), hidden)
	}
}

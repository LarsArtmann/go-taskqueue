package webui

import (
	"context"
	"strings"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/journal"
)

// TestFactTypes pins the feed filter chips' input: distinct types in
// first-seen order, one chip per type — never a per-line chip.
func TestFactTypes(t *testing.T) {
	t.Parallel()

	facts := []journal.Fact{
		{Type: journal.Claimed},
		{Type: journal.Completed},
		{Type: journal.Claimed},
	}
	got := factTypes(facts)
	want := []string{string(journal.Claimed), string(journal.Completed)}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("factTypes = %v, want %v", got, want)
	}
	if factTypes(nil) != nil {
		t.Fatal("empty window: want no chips")
	}
}

// TestFactFeedChipsMarkup pins the rendered feed contract the client-side
// filter (design-pass §f24) listens on: every line carries data-fact-type
// and the chips row carries data-feed-type buttons including the "all"
// reset — the journal stays server-side free of the filter.
func TestFactFeedChipsMarkup(t *testing.T) {
	t.Parallel()

	data := DashboardData{
		Facts: []journal.Fact{
			{Type: journal.Claimed, TaskID: "t1"},
			{Type: journal.Completed, TaskID: "t2"},
		},
	}
	html := renderComponent(context.Background(), FactFeed(data))
	for _, want := range []string{
		`data-fact-type="` + string(journal.Claimed) + `"`,
		`data-fact-type="` + string(journal.Completed) + `"`,
		`data-feed-type=""`,
		`data-feed-type="` + string(journal.Claimed) + `"`,
		`tq-feed-chip`,
		`aria-label="filter facts by type"`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("FactFeed markup missing %q", want)
		}
	}
}

// TestReviewFindingsAnchorsMarkup pins the review-findings permalink
// contract (design-pass §f27): each rendered finding row carries a
// stable #finding-N id (1-based, order = render order) so fix prompts
// and reviews can cite positions. The scroll margin clears the sticky
// topbar (pageHeader h-14 = 3.5rem) with breathing room:
// scroll-mt-20 = 5rem = 80px (measured 2026-11-22 against layout.templ).
func TestReviewFindingsAnchorsMarkup(t *testing.T) {
	t.Parallel()

	html := renderComponent(context.Background(), reviewFindings(executor.ReviewResult{
		Findings: []executor.ReviewFinding{
			{Title: "first", Severity: "high"},
			{Title: "second"},
		},
	}))
	for _, want := range []string{
		`id="finding-1"`,
		`id="finding-2"`,
		`scroll-mt-20`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("reviewFindings markup missing %q", want)
		}
	}
	if strings.Contains(html, `id="finding-3"`) {
		t.Fatal("reviewFindings minted an id past the findings list")
	}
}

package webui

import (
	"context"
	"strings"
	"testing"

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

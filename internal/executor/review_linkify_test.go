package executor

import "testing"

// TestLinkifyFindingCites pins the finding-cite linkification (design-pass
// §f27): bare `#finding-N` citations in a minted prompt become
// /task/<id>#finding-N anchor links so the fixer lands on the exact
// finding row; citations already inside a path are left alone, and text
// without citations passes through verbatim.
func TestLinkifyFindingCites(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		prompt string
		taskID string
		want   string
	}{
		{
			name:   "bare citation becomes a detail-page anchor link",
			prompt: "The offset lies in #finding-2 of the ledger.",
			taskID: "0001ab",
			want:   "The offset lies in [#finding-2](/task/0001ab#finding-2) of the ledger.",
		},
		{
			name:   "start-of-text citation is linked too",
			prompt: "#finding-3 hides the real bug",
			taskID: "0001ab",
			want:   "[#finding-3](/task/0001ab#finding-3) hides the real bug",
		},
		{
			name:   "citation inside an existing task link is untouched",
			prompt: "see [the row](/task/0001ff#finding-1) first, then #finding-1 context",
			taskID: "0001ab",
			want:   "see [the row](/task/0001ff#finding-1) first, then [#finding-1](/task/0001ab#finding-1) context",
		},
		{
			name:   "no citations passes through verbatim",
			prompt: "fix the frobnicator\n\nTask-Queue-ID: {{TASK_ID}}",
			taskID: "0001ab",
			want:   "fix the frobnicator\n\nTask-Queue-ID: {{TASK_ID}}",
		},
		{
			name:   "empty task id disables the rewrite",
			prompt: "cites #finding-1",
			taskID: "",
			want:   "cites #finding-1",
		},
		{
			name:   "every distinct citation is linked",
			prompt: "#finding-1 and #finding-12",
			taskID: "t9",
			want:   "[#finding-1](/task/t9#finding-1) and [#finding-12](/task/t9#finding-12)",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := LinkifyFindingCites(tc.prompt, tc.taskID); got != tc.want {
				t.Fatalf(
					"LinkifyFindingCites(%q, %q) = %q, want %q",
					tc.prompt,
					tc.taskID,
					got,
					tc.want,
				)
			}
		})
	}
}

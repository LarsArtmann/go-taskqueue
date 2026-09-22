package executor

import (
	"strings"
	"testing"
)

func TestExcerpt(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "empty",
			in:   "",
			want: "",
		},
		{
			name: "single line",
			in:   "fix the login bug",
			want: "fix the login bug",
		},
		{
			name: "surrounding whitespace trimmed",
			in:   "  fix the login bug\nsecond line",
			want: "fix the login bug",
		},
		{
			name: "first line only",
			in:   "first\nsecond\nthird",
			want: "first",
		},
		{
			name: "at the bound stays intact",
			in:   strings.Repeat("a", excerptMaxLen) + "\nmore",
			want: strings.Repeat("a", excerptMaxLen),
		},
		{
			name: "over the bound gains ellipsis",
			in:   strings.Repeat("a", excerptMaxLen+10) + "\nmore",
			want: strings.Repeat("a", excerptMaxLen) + "…",
		},
		{
			name: "trailing space inside the cut is trimmed",
			in:   strings.Repeat("a", excerptMaxLen-2) + "  b\nnext",
			want: strings.Repeat("a", excerptMaxLen-2),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := Excerpt(tt.in); got != tt.want {
				t.Errorf("Excerpt() = %q, want %q", got, tt.want)
			}
		})
	}
}

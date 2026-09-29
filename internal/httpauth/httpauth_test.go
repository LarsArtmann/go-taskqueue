package httpauth

import (
	"context"
	"net/http"
	"testing"
)

func TestAuthorizationToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		header string
		want   string
	}{
		{name: "absent", header: "", want: ""},
		{name: "bearer canonical", header: "Bearer sekrit", want: "sekrit"},
		{name: "bearer lowercase scheme", header: "bearer sekrit", want: "sekrit"},
		{name: "bearer mixed case scheme", header: "BeArEr sekrit", want: "sekrit"},
		{name: "other scheme", header: "Basic sekrit", want: ""},
		{name: "missing value", header: "Bearer", want: ""},
		{name: "missing value with space", header: "Bearer ", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := tokenRequest(t, tt.header)

			if got := AuthorizationToken(r); got != tt.want {
				t.Errorf("AuthorizationToken(header=%q) = %q, want %q", tt.header, got, tt.want)
			}
		})
	}
}

func tokenRequest(t *testing.T, header string) *http.Request {
	t.Helper()

	r, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	if header != "" {
		r.Header.Set("Authorization", header)
	}

	return r
}

func TestQueryToken(t *testing.T) {
	t.Parallel()

	r, err := http.NewRequestWithContext(
		context.Background(), http.MethodGet, "/api/stats?project=demo&token=sekrit", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	if got := QueryToken(r); got != "sekrit" {
		t.Errorf("QueryToken() = %q, want %q", got, "sekrit")
	}

	empty, err := http.NewRequestWithContext(
		context.Background(), http.MethodGet, "/api/stats", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	if got := QueryToken(empty); got != "" {
		t.Errorf("QueryToken(no query) = %q, want empty", got)
	}
}

func TestTokenMatches(t *testing.T) {
	t.Parallel()

	expected := HashToken("sekrit")

	if !TokenMatches(expected, "sekrit") {
		t.Error("TokenMatches(correct) = false, want true")
	}

	if TokenMatches(expected, "wrong") {
		t.Error("TokenMatches(wrong) = true, want false")
	}

	if TokenMatches(expected, "") {
		t.Error("TokenMatches(empty) = true, want false")
	}
}

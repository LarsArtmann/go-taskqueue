package webui

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// TestServerTimingHeaderBehindRequestLog pins the server_timing trial's
// mount contract (fleet review f21): the Server-Timing header (per-segment
// + total) ships ONLY where the operator opted into request logging — the
// header leaks internal performance detail, so the default loopback
// posture never carries it.
func TestServerTimingHeaderBehindRequestLog(t *testing.T) {
	t.Parallel()

	logged := New(newTestStore(t), Config{RequestLog: true})
	loggedServer := httptest.NewServer(logged.Handler())
	t.Cleanup(loggedServer.Close)

	code, header, _ := getBody(t, loggedServer.URL+"/api/stats")
	if code != http.StatusOK {
		t.Fatalf("GET /api/stats = %d, want 200", code)
	}

	if st := header.Get("Server-Timing"); !strings.Contains(st, "stats") || !strings.Contains(st, "total") {
		t.Errorf("Server-Timing = %q, want stats + total segments under RequestLog", st)
	}

	plain := New(newTestStore(t), Config{})
	plainServer := httptest.NewServer(plain.Handler())
	t.Cleanup(plainServer.Close)

	_, header, _ = getBody(t, plainServer.URL+"/api/stats")
	if st := header.Get("Server-Timing"); st != "" {
		t.Errorf("Server-Timing shipped without RequestLog: %q", st)
	}
}

// TestRedactedRequestURITable pins the redaction contract directly (the
// end-to-end TestRequestLogRedactsToken proves the logging wrapper; this
// pins the function itself): the `?token=` auth channel never survives into
// a logged URI — in any position, count, or alongside any other params —
// while every non-token query parameter does.
func TestRedactedRequestURITable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "no query", raw: "https://tq.local/api/stats", want: "/api/stats"},
		{
			name: "token only drops the query entirely",
			raw:  "https://tq.local/api/stats?token=sekrit",
			want: "/api/stats",
		},
		{
			name: "token among others",
			raw:  "https://tq.local/?project=demo&token=sekrit&status=running",
			want: "/?project=demo&status=running",
		},
		{
			name: "duplicate token params all stripped",
			raw:  "https://tq.local/?token=a&token=b&project=demo",
			want: "/?project=demo",
		},
		{name: "empty-valued token stripped", raw: "https://tq.local/?token=&project=demo", want: "/?project=demo"},
		{
			name: "similar param names survive",
			raw:  "https://tq.local/?token2=a&x-token=b",
			want: "/?token2=a&x-token=b",
		},
		{name: "token-less query untouched", raw: "https://tq.local/?q=flake", want: "/?q=flake"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			u, err := url.Parse(tt.raw)
			if err != nil {
				t.Fatalf("url.Parse(%q): %v", tt.raw, err)
			}

			if got := redactedRequestURI(u); got != tt.want {
				t.Errorf("redactedRequestURI(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

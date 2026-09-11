package executor

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// incidentFixture loads the EXACT lastError tail of dead task
// 000001a08edf (the 2026-09-11 Z.ai 429 wall) so provider phrasing drift
// fails the suite instead of silently degrading to the fallback.
func incidentFixture(tb testing.TB) string {
	tb.Helper()

	data, err := os.ReadFile(filepath.Join("testdata", "ratelimit_incident_000001a08edf.txt"))
	if err != nil {
		tb.Fatalf("read incident fixture: %v", err)
	}

	return string(data)
}

// TestDetectRateLimit pins recognition of the provider exhaustion shapes
// observed in the wild, and the delay-derivation order: provider reset
// timestamp > numeric retry-after > conservative default.
func TestDetectRateLimit(t *testing.T) {
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.Local)

	zai := fmt.Sprintf(
		`WARN Provider request failed, retrying retry_delay=5s status_code=429 title="too many requests" message="Usage limit reached for 5 hour. Your limit will reset at %s"`,
		now.Add(2*time.Hour).Format("2006-01-02 15:04:05"),
	)

	tests := []struct {
		name     string
		output   string
		want     bool
		wantFrom time.Duration
		wantTo   time.Duration
	}{
		{
			name:     "zai usage limit with reset timestamp",
			output:   zai,
			want:     true,
			wantFrom: 2*time.Hour + rateLimitGrace,
			wantTo:   2*time.Hour + rateLimitGrace + time.Minute,
		},
		{
			name:     "zai reset timestamp already past (timezone accident)",
			output:   `status_code=429 message="Usage limit reached for 5 hour. Your limit will reset at 2026-09-10 08:00:00"`,
			want:     true,
			wantFrom: defaultRateLimitBackoff,
			wantTo:   defaultRateLimitBackoff,
		},
		{
			name:     "synthetic new openai style quota refusal, no timestamp",
			output:   `WARN Provider request failed status_code=429 title="too many requests" message={"error":{"message":"You have reached your subscription limit.","code":"insufficient_quota"}}`,
			want:     true,
			wantFrom: defaultRateLimitBackoff,
			wantTo:   defaultRateLimitBackoff,
		},
		{
			name:     "numeric retry after hint",
			output:   "ERROR ModelProvider called provider=synthetic status_code=429 retry_after=120 too many requests",
			want:     true,
			wantFrom: 2*time.Minute + rateLimitGrace,
			wantTo:   2*time.Minute + rateLimitGrace,
		},
		{
			name:   "rfc3339 renews-at timestamp",
			output: `status_code=429 quota exceeded; your quota renews at 2026-09-11T18:00:00Z`,
			want:   true,
			// Expected is computed against the UTC instant so the test is
			// host-timezone independent (the reset parses to an absolute
			// time); the 6h cap applies on hosts far ahead of UTC.
			wantFrom: min(time.Date(2026, 9, 11, 18, 0, 0, 0, time.UTC).Sub(now)+rateLimitGrace, maxRateLimitWait),
			wantTo: min(
				time.Date(2026, 9, 11, 18, 0, 0, 0, time.UTC).Sub(now)+rateLimitGrace+time.Minute,
				maxRateLimitWait,
			),
		},
		{
			name: "reset far in the future is capped",
			output: "status_code=429 usage limit reached; resets at " + now.Add(24*time.Hour).
				Format("2006-01-02 15:04:05"),
			want:     true,
			wantFrom: maxRateLimitWait,
			wantTo:   maxRateLimitWait,
		},
		{
			name:   "plain failure is not a rate limit",
			output: "agent run failed: exit status 1: fatal: not a git repository",
			want:   false,
		},
		{
			name:   "retry_delay is crush's own ladder, not a quota signal",
			output: "ERROR agent processing failed: failed to start agent processing stream: connection refused",
			want:   false,
		},
		{
			// The EXACT tail of dead task 000001a08edf: reset at
			// 19:40:34 local wall-clock, probe time 12:00 local → 7h40m
			// until, which the 6h cap clamps. Deterministic across host
			// timezones because both the parse and `now` are local.
			name:     "incident fixture 000001a08edf (zai 5h usage limit)",
			output:   incidentFixture(t),
			want:     true,
			wantFrom: maxRateLimitWait,
			wantTo:   maxRateLimitWait,
		},
		{
			// The incident's OTHER failure shape (attempt-1 evidence):
			// request-rate 429 with NO reset timestamp — must fall back to
			// the conservative default, never to a bogus parse.
			name:     "incident request-rate shape, no reset timestamp",
			output:   `WARN Provider request failed, retrying retry_delay=5s status_code=429 title="too many requests" message="Rate limit reached for requests"`,
			want:     true,
			wantFrom: defaultRateLimitBackoff,
			wantTo:   defaultRateLimitBackoff,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := DetectRateLimit(tt.output, now)
			if ok != tt.want {
				t.Fatalf("DetectRateLimit ok = %v, want %v (delay=%s)", ok, tt.want, got)
			}

			if !tt.want {
				return
			}

			if got < tt.wantFrom || got > tt.wantTo {
				t.Fatalf("delay = %s, want between %s and %s", got, tt.wantFrom, tt.wantTo)
			}
		})
	}
}

// FuzzDetectRateLimit hammers the untrusted-output regexes (same treatment
// as FuzzParseRepo): no panics, no negative delays, no unbounded waits.
// Seed corpus: the real provider failure logs from the 2026-09-11 incident.
func FuzzDetectRateLimit(f *testing.F) {
	f.Add("", time.Now().UnixNano())
	f.Add(incidentFixture(f), time.Date(2026, 9, 11, 12, 0, 0, 0, time.Local).UnixNano())
	f.Add(
		`WARN Provider request failed, retrying retry_delay=5s status_code=429 title="too many requests" message="Rate limit reached for requests"`,
		time.Now().UnixNano(),
	)
	f.Add(`status_code=429 quota exceeded; your quota renews at 2026-09-11T18:00:00Z`, time.Now().UnixNano())
	f.Add("ERROR 429 too many requests retry_after=99999999999", time.Now().UnixNano())
	f.Add("resets at 9999-99-99 99:99:99 rate limit", time.Now().UnixNano())
	// Regression seed (fuzz finding): a far-future reset used to overflow
	// time.Duration in reset.Sub and produce a bogus delay.
	f.Add("quotA eXCeededrenew At 4000-01-01T00:00:00", int64(1789129566108809255))

	f.Fuzz(func(t *testing.T, output string, nowUnix int64) {
		now := time.Unix(0, nowUnix)

		delay, ok := DetectRateLimit(output, now)
		if !ok {
			return
		}

		if delay <= 0 || delay > maxRateLimitWait {
			t.Fatalf("DetectRateLimit(delay=%s) out of bounds for %q", delay, output)
		}
	})
}

// TestRateLimitedIdempotent: defensive double wrapping stays a single class.
func TestRateLimitedIdempotent(t *testing.T) {
	cause := errors.New("429 too many requests")
	first := RateLimited(cause, 5*time.Minute)

	second := RateLimited(first, 9*time.Hour)
	if !errors.Is(second, first) {
		t.Fatalf("RateLimited double-wrap returned a new error: %v", second)
	}

	if RateLimited(nil, time.Minute) != nil {
		t.Fatal("RateLimited(nil) must be nil")
	}
}

// TestRateLimitGate pins the in-process gate: arm keeps the MAX of
// concurrent observations, the wait shrinks as the window elapses, and a
// stale gate reports clear.
func TestRateLimitGate(t *testing.T) {
	e := &AgentExecutor{}

	if _, limited := e.rateLimitWait(); limited {
		t.Fatal("fresh executor must not be gated")
	}

	e.armRateLimit(time.Minute)

	if wait, limited := e.rateLimitWait(); !limited || wait <= 50*time.Second || wait > time.Minute {
		t.Fatalf("after arm(1m): wait=%s limited=%v, want ~1m", wait, limited)
	}

	e.armRateLimit(2 * time.Minute)

	if wait, _ := e.rateLimitWait(); wait <= time.Minute+30*time.Second {
		t.Fatalf("arm must keep the max observation, wait=%s", wait)
	}

	e.armRateLimit(30 * time.Second)

	if wait, _ := e.rateLimitWait(); wait <= time.Minute+30*time.Second {
		t.Fatalf("shorter observation must not shrink the gate, wait=%s", wait)
	}

	e.rateLimitUntil.Store(time.Now().Add(-time.Second).UnixNano())

	if _, limited := e.rateLimitWait(); limited {
		t.Fatal("stale gate must report clear")
	}
}

// TestWithoutCloseoutCarriesSettings pins the review/status clone contract:
// every runtime setting rides along, the close-out prompt is stripped, and
// the clone starts with a FRESH rate-limit gate (an armed atomic.Int64 must
// never be struct-copied — cmd/tq/agentpool.go registers this clone).
func TestWithoutCloseoutCarriesSettings(t *testing.T) {
	e := &AgentExecutor{
		Bin:           "/opt/crush",
		ProjectsDir:   "/srv/projects",
		Yolo:          true,
		MaxConcurrent: 7,
	}
	e.armRateLimit(time.Hour)

	clone := e.WithoutCloseout()

	if clone.Bin != e.Bin || clone.ProjectsDir != e.ProjectsDir || !clone.Yolo || clone.MaxConcurrent != 7 {
		t.Fatalf("clone lost settings: %+v", clone)
	}

	if clone.CloseoutPrompt != "" {
		t.Fatalf("clone kept the close-out prompt %q, want empty", clone.CloseoutPrompt)
	}

	if _, limited := clone.rateLimitWait(); limited {
		t.Fatal("clone inherited an armed rate-limit gate, want fresh")
	}
}

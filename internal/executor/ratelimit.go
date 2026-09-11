package executor

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"
)

// RateLimitError signals the model provider behind an agent run (Z.ai,
// synthetic.new, …) refused with an exhaustion response — HTTP 429 with a
// usage/rate-limit message. This is never the task's fault: the identical
// retry fails identically UNTIL the provider's limit window resets, and
// while it lasts every retry is a wasted (possibly billed) process spawn.
//
// Workers must requeue the task WITHOUT burning an attempt, delayed by
// RetryAfter (the parsed reset time when the provider reports one, a
// conservative default otherwise).
type RateLimitError struct {
	Cause error
	// RetryAfter is how long to wait before the next attempt. Always
	// positive; capped by maxRateLimitWait.
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limited (retry after %s): %s", e.RetryAfter.Truncate(time.Second), e.Cause)
}

func (e *RateLimitError) Unwrap() error { return e.Cause }

// RateLimited wraps cause in a *RateLimitError; nil passes through
// unchanged. A cause that is already a *RateLimitError is returned as-is,
// so defensive double wrapping stays a single class.
func RateLimited(cause error, retryAfter time.Duration) error {
	if cause == nil {
		return nil
	}

	if _, ok := errors.AsType[*RateLimitError](cause); ok {
		return cause
	}

	return &RateLimitError{Cause: cause, RetryAfter: retryAfter}
}

const (
	// defaultRateLimitBackoff covers providers that report exhaustion
	// without a reset time — synthetic.new's OpenAI-style 429
	// (insufficient_quota) carries no timestamp in its error body. Fifteen
	// minutes re-probes at a fraction of the cost of a burned agent
	// attempt; a still-limited probe re-arms itself.
	defaultRateLimitBackoff = 15 * time.Minute
	// maxRateLimitWait caps a parsed reset time so a timezone-parsing
	// accident cannot park a task for days. Z.ai's usage windows are
	// hours; six is generous headroom.
	maxRateLimitWait = 6 * time.Hour
	// rateLimitGrace pads a parsed reset instant: the worker claims at the
	// boundary, then crush still needs seconds to reach the provider, so
	// landing exactly ON the reset risks one wasted probe.
	rateLimitGrace = 30 * time.Second
)

// rateLimitRe decides WHETHER an agent output reports provider exhaustion.
// It matches the two families observed in the wild:
//   - crush's provider log line: `WARN Provider request failed, retrying
//     retry_delay=5s status_code=429 title="too many requests"`
//     message="Usage limit reached for 5 hour. Your limit will reset at
//     2026-09-11 19:40:34"` (Z.ai)
//   - OpenAI-convention bodies relayed by agents (synthetic.new and any
//     OpenAI-compatible provider): "insufficient_quota", "rate limit",
//     "subscription limit".
var rateLimitRe = regexp.MustCompile(
	`(?i)status[ _-]?code[=: ]+["']?429|https? 429|too many requests|rate limit|usage limit|insufficient_quota|quota exceeded|subscription limit`,
)

// resetAtRe extracts WHEN the limit resets. Known shapes: Z.ai's
// "Your limit will reset at 2026-09-11 19:40:34" (wall-clock, no zone —
// parsed as the agent host's local time, matching what the operator's
// provider dashboard shows) and RFC3339 timestamps.
var resetAtRe = regexp.MustCompile(
	`(?i)(?:reset|renew)[a-z]*\s+at\s+(\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})?)`,
)

var resetAtLayouts = []string{
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05",
	time.RFC3339,
}

// retryAfterRe extracts a numeric retry hint ("retry_after=120",
// "Retry-After: 30"). Deliberately does NOT match crush's retry_delay=
// (its 5s/10s/20s ladder is the agent's own short game, not the provider's
// quota window).
var retryAfterRe = regexp.MustCompile(`(?i)retry[ _-]after(?:[ _-]seconds)?[=: ]+["']?(\d{1,6})["']?`)

// DetectRateLimit scans agent output for provider exhaustion and returns
// how long to wait before the next attempt. The delay is, in order of
// trustworthiness: the provider's reset timestamp (+ grace), a numeric
// retry-after hint (+ grace), then defaultRateLimitBackoff. now is
// injectable for tests.
func DetectRateLimit(output string, now time.Time) (time.Duration, bool) {
	if !rateLimitRe.MatchString(output) {
		return 0, false
	}

	delay := defaultRateLimitBackoff

	if m := resetAtRe.FindStringSubmatch(output); m != nil {
		for _, layout := range resetAtLayouts {
			if reset, err := time.ParseInLocation(layout, m[1], time.Local); err == nil {
				if until := reset.Sub(now); until > 0 {
					delay = until + rateLimitGrace
				}

				break
			}
		}
	} else if m := retryAfterRe.FindStringSubmatch(output); m != nil {
		if secs, err := strconv.ParseInt(m[1], 10, 64); err == nil && secs > 0 {
			delay = time.Duration(secs)*time.Second + rateLimitGrace
		}
	}

	return min(delay, maxRateLimitWait), true
}

// detectRateLimit is the production entry point (wall clock).
func detectRateLimit(output string) (time.Duration, bool) {
	return DetectRateLimit(output, time.Now())
}

// armRateLimit records the soonest the provider is expected to accept
// requests again, keeping the max of concurrent observations. The gate is
// in-process: it fast-refuses sibling agent tasks in THIS pool without
// spawning the agent binary — a 429 probe costs a process spawn and, on
// metered providers, potentially billed tokens.
func (e *AgentExecutor) armRateLimit(retryAfter time.Duration) {
	if retryAfter <= 0 {
		return
	}

	until := time.Now().Add(retryAfter).UnixNano()

	for {
		cur := e.rateLimitUntil.Load()
		if cur >= until || e.rateLimitUntil.CompareAndSwap(cur, until) {
			return
		}
	}
}

// rateLimitWait reports how much longer the provider is believed
// exhausted (false when the gate is clear or stale).
func (e *AgentExecutor) rateLimitWait() (time.Duration, bool) {
	if d := time.Until(time.Unix(0, e.rateLimitUntil.Load())); d > 0 {
		return d, true
	}

	return 0, false
}

// rateLimitedTurn classifies one failed agent turn (work or closeout): on
// provider exhaustion it arms the executor's gate — so sibling tasks
// fast-refuse instead of probing a spent quota — and returns the
// requeue-able *RateLimitError carrying the parsed wait. nil means the
// failure is NOT a rate limit and the caller's ordinary wrapping applies.
func (e *AgentExecutor) rateLimitedTurn(stage string, runErr error, output string) error {
	wait, ok := detectRateLimit(output)
	if !ok {
		return nil
	}

	e.armRateLimit(wait)

	return RateLimited(fmt.Errorf("%s failed: %w: %s", stage, runErr, tailBytes([]byte(output), 8192)), wait)
}

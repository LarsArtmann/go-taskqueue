package executor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

// HTTPExecutor POSTs the task as JSON to a URL. Any 2xx completes the task;
// anything else (or transport error) fails the attempt.
type HTTPExecutor struct {
	URL    string
	Client *http.Client
}

// NewHTTPExecutor builds an HTTP executor with a sane default client.
func NewHTTPExecutor(url string) *HTTPExecutor {
	return &HTTPExecutor{
		URL: url,
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Execute posts the task envelope. Classification: a malformed URL is
// permanent; a 429 is a *RateLimitError (same no-attempt-burn requeue as
// agent tasks — an HTTP 429 burns the retry budget exactly like the
// 2026-09-11 incident otherwise); transport errors and 408/5xx responses
// are transient; any other non-2xx (404 route, 401 auth, 400 body) is
// permanent — the same request would fail the same way again.
func (e *HTTPExecutor) Execute(ctx context.Context, t task.Task) error {
	body := fmt.Sprintf(`{"id":%q,"project":%q,"type":%q,"payload":%s,"attempt":%d}`,
		t.ID.String(), t.Project, t.Type, payloadOrEmpty(t.Payload), t.Attempts+1)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.URL, bytes.NewReader([]byte(body)))
	if err != nil {
		return Permanent(fmt.Errorf("http executor: build request: %w", err))
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := e.Client.Do(req)
	if err != nil {
		return fmt.Errorf("http executor: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		if resp.StatusCode == http.StatusTooManyRequests {
			return e.rateLimited(resp)
		}

		if transientStatus(resp.StatusCode) {
			return fmt.Errorf("http executor: transient status %d", resp.StatusCode)
		}

		return Permanent(fmt.Errorf("http executor: status %d", resp.StatusCode))
	}

	return nil
}

// rateLimited builds the *RateLimitError for a 429 response. Trust order:
// the Retry-After header (authoritative, seconds or HTTP-date), then a
// reset-timestamp / retry-after hint in the response body (same
// DetectRateLimit contract as agent output), then defaultRateLimitBackoff.
func (e *HTTPExecutor) rateLimited(resp *http.Response) error {
	delay := defaultRateLimitBackoff

	if ra := parseRetryAfterHeader(resp.Header.Get("Retry-After"), time.Now()); ra > 0 {
		delay = ra
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if d, ok := DetectRateLimit(string(body), time.Now()); ok {
		delay = d
	}

	return RateLimited(fmt.Errorf("http executor: status 429"), delay)
}

// parseRetryAfterHeader parses an HTTP Retry-After value: delay-seconds or
// an HTTP-date. Zero on absent/garbage values (the caller keeps its default).
func parseRetryAfterHeader(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}

	if secs, err := strconv.ParseInt(value, 10, 64); err == nil {
		if secs <= 0 {
			return 0
		}

		return time.Duration(secs) * time.Second
	}

	if at, err := http.ParseTime(value); err == nil {
		if d := at.Sub(now); d > 0 {
			return d
		}
	}

	return 0
}

// transientStatus reports whether retrying this status can plausibly succeed.
func transientStatus(code int) bool {
	switch code {
	case http.StatusRequestTimeout, http.StatusTooManyRequests:
		return true
	}

	return code >= 500
}

func payloadOrEmpty(p []byte) string {
	if len(p) == 0 {
		return "{}"
	}

	return string(p)
}

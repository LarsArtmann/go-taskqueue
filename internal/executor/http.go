package executor

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
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

// Execute posts the task envelope.
func (e *HTTPExecutor) Execute(ctx context.Context, t task.Task) error {
	body := fmt.Sprintf(`{"id":%q,"project":%q,"type":%q,"payload":%s,"attempt":%d}`,
		t.ID.String(), t.Project, t.Type, payloadOrEmpty(t.Payload), t.Attempts+1)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.URL, bytes.NewReader([]byte(body)))
	if err != nil {
		return fmt.Errorf("http executor: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.Client.Do(req)
	if err != nil {
		return fmt.Errorf("http executor: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("http executor: status %d", resp.StatusCode)
	}
	return nil
}

func payloadOrEmpty(p []byte) string {
	if len(p) == 0 {
		return "{}"
	}
	return string(p)
}

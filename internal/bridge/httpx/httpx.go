// Package httpx carries the bridges' shared HTTP client policy.
package httpx

import (
	"net/http"
	"time"
)

const (
	// maxIdleConns and maxIdlePerHost raise the zero-configuration
	// transport's two-idle-connections-per-host ceiling so concurrent
	// requests to one endpoint reuse keep-alive connections instead of
	// churning TCP handshakes.
	maxIdleConns    = 64
	maxIdlePerHost  = 16
	idleConnTimeout = 90 * time.Second
)

// TunedClient returns a client whose transport pools connections for
// burst concurrency (poller + bridge sweeps firing against the same
// endpoint). A non-clonable DefaultTransport degrades to the plain
// client — correctness first, tuning second.
func TunedClient(timeout time.Duration) *http.Client {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return &http.Client{Timeout: timeout}
	}

	transport := base.Clone()
	transport.MaxIdleConns = maxIdleConns
	transport.MaxIdleConnsPerHost = maxIdlePerHost
	transport.IdleConnTimeout = idleConnTimeout

	return &http.Client{Timeout: timeout, Transport: transport}
}

package webui

import (
	"log/slog"
	"net/http"
	"time"
)

// withRequestLog wraps next so every completed request is logged to the
// default slog logger at Info level: method, path, status, duration. An
// SSE connection is logged once, when the stream closes.
func withRequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w}
		start := time.Now()

		defer func() {
			slog.Info("webui: request",
				"method", r.Method,
				"path", r.URL.RequestURI(),
				"status", rec.status,
				"duration", time.Since(start),
			)
		}()

		next.ServeHTTP(rec, r)
	})
}

// statusRecorder captures the status code a handler wrote so the
// request-logging wrapper can report it. Flush is forwarded so SSE
// streaming keeps working through the wrapper; a handler that only calls
// Write (never WriteHeader) is recorded as http.StatusOK.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(body []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}

	return r.ResponseWriter.Write(body)
}

func (r *statusRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

package webui

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-sse/ssetest"
)

func TestConfigValidateLoopbackMatrix(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		token   string
		wantErr bool
	}{
		{name: "default addr no token", addr: "", token: "", wantErr: false},
		{name: "ipv4 loopback", addr: "127.0.0.1:8090", token: "", wantErr: false},
		{name: "localhost name", addr: "localhost:8090", token: "", wantErr: false},
		{name: "ipv6 loopback", addr: "[::1]:8090", token: "", wantErr: false},
		{name: "wildcard all interfaces", addr: "0.0.0.0:8090", token: "", wantErr: true},
		{name: "empty host binds all", addr: ":8090", token: "", wantErr: true},
		{name: "lan ip", addr: "192.168.1.5:8090", token: "", wantErr: true},
		{name: "non-loopback hostname", addr: "tank.local:8090", token: "", wantErr: true},
		{name: "lan ip with token", addr: "192.168.1.5:8090", token: "sekrit", wantErr: false},
		{name: "wildcard with token", addr: "0.0.0.0:8090", token: "sekrit", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Config{Addr: tt.addr, AuthToken: tt.token}.Validate()

			if tt.wantErr {
				if !errors.Is(err, ErrTokenRequiredOnLAN) {
					t.Fatalf("Validate(%q, token=%q) error = %v, want ErrTokenRequiredOnLAN", tt.addr, tt.token, err)
				}

				return
			}

			if err != nil {
				t.Fatalf("Validate(%q, token=%q) unexpected error: %v", tt.addr, tt.token, err)
			}
		})
	}
}

func TestRunRefusesLANBindWithoutToken(t *testing.T) {
	srv := New(newTestStore(t), Config{
		Addr: "0.0.0.0:8090", Poll: time.Millisecond, Heartbeat: time.Millisecond,
	})

	if err := srv.Run(context.Background()); !errors.Is(err, ErrTokenRequiredOnLAN) {
		t.Fatalf("Run on 0.0.0.0 without token: error = %v, want ErrTokenRequiredOnLAN", err)
	}
}

func TestTokenAuthMiddlewareMatrix(t *testing.T) {
	const token = "sekrit"

	srv := New(newTestStore(t), Config{
		Poll: time.Millisecond, Heartbeat: time.Millisecond, AuthToken: token,
	})
	handler := srv.Handler()

	tests := []struct {
		name string
		path string
		auth string
		want int
	}{
		{name: "no credentials", path: "/api/stats", auth: "", want: http.StatusUnauthorized},
		{name: "wrong bearer", path: "/api/stats", auth: "Bearer nope", want: http.StatusUnauthorized},
		{name: "lowercase bearer scheme accepted", path: "/api/stats", auth: "bearer sekrit", want: http.StatusOK},
		{name: "right bearer", path: "/api/stats", auth: "Bearer sekrit", want: http.StatusOK},
		{name: "query token", path: "/api/stats?token=sekrit", auth: "", want: http.StatusOK},
		{name: "wrong query token", path: "/api/stats?token=nope", auth: "", want: http.StatusUnauthorized},
		{name: "page without token", path: "/", auth: "", want: http.StatusUnauthorized},
		{name: "static without token", path: "/static/app.js", auth: "", want: http.StatusUnauthorized},
		{name: "static with token", path: "/static/app.js?token=sekrit", auth: "", want: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			if tt.auth != "" {
				req.Header.Set("Authorization", tt.auth)
			}

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.want {
				t.Errorf("GET %s (auth %q) status = %d, want %d", tt.path, tt.auth, rec.Code, tt.want)
			}
		})
	}
}

func TestTokenAuthChallengeHeaders(t *testing.T) {
	srv := New(newTestStore(t), Config{
		Poll: time.Millisecond, Heartbeat: time.Millisecond, AuthToken: "sekrit",
	})

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/stats", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}

	if got := rec.Header().Get("WWW-Authenticate"); !strings.Contains(got, "Bearer") {
		t.Errorf("WWW-Authenticate = %q, want a Bearer challenge", got)
	}
}

// TestTokenAuthKeepsSSEStreaming proves the SSE endpoint authenticates too
// and still streams the connect snapshot when the token rides the query
// string (EventSource cannot set headers).
func TestTokenAuthKeepsSSEStreaming(t *testing.T) {
	s := newTestStore(t)
	srv := New(s, Config{
		Poll: time.Millisecond, Heartbeat: time.Millisecond, AuthToken: "sekrit",
	})
	tk := enqueue(t, s, "sh", "demo")

	events := ssetest.CollectN(t, srv.Handler(), 6,
		ssetest.WithPath("/api/events?token=sekrit"))

	sawTask := false

	for _, evt := range events {
		if evt.Type == testFragEvent && strings.Contains(evt.Data(), tk.ID.String()) {
			sawTask = true
		}
	}

	if !sawTask {
		t.Fatalf("SSE snapshot with token missing task %s", tk.ID)
	}

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/events", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("SSE without token status = %d, want 401", rec.Code)
	}
}

// TestRequestLogRedactsToken proves the access log never records the auth
// token that reached the server as ?token=….
func TestRequestLogRedactsToken(t *testing.T) {
	logs := captureDefaultLogger(t)

	srv := New(newTestStore(t), Config{
		Poll: time.Millisecond, Heartbeat: time.Millisecond,
		RequestLog: true, AuthToken: "sekrit",
	})

	handler := srv.Handler()
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/stats?token=sekrit", nil))
	handler.ServeHTTP(
		httptest.NewRecorder(),
		httptest.NewRequest(http.MethodGet, "/?project=demo&status=running&q=flake&token=sekrit", nil),
	)

	out := logs.String()

	if strings.Contains(out, "sekrit") {
		t.Errorf("request log leaked the token:\n%s", out)
	}

	if !strings.Contains(out, "path=/api/stats") {
		t.Errorf("token-less path not logged in full:\n%s", out)
	}

	if !strings.Contains(out, "project=demo") || !strings.Contains(out, "status=running") {
		t.Errorf("other query params dropped from log:\n%s", out)
	}
}

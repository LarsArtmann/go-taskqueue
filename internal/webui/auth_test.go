package webui

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-sse/ssetest"
	"github.com/larsartmann/go-taskqueue/internal/task"
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

// TestTokenAuthCookieSession pins the LAN fix: a browser authenticates the
// HTML document via ?token=, but subresource requests (CSS, JS, favicon,
// SSE) never carry the query — they 401'd before the session cookie existed.
func TestTokenAuthCookieSession(t *testing.T) {
	const token = "sekrit"

	srv := New(newTestStore(t), Config{
		Poll: time.Millisecond, Heartbeat: time.Millisecond, AuthToken: token,
	})
	handler := srv.Handler()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/?token="+token, nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("page with query token: status = %d, want 200", rec.Code)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("Set-Cookie count = %d, want exactly the session + CSRF cookies", len(cookies))
	}

	var c *http.Cookie

	for _, cookie := range cookies {
		switch cookie.Name {
		case tqTokenCookie:
			c = cookie
		case tqCSRFCookie:
			// The CSRF cookie rides along; its flow has its own test.
		default:
			t.Errorf("unexpected cookie %q", cookie.Name)
		}
	}

	if c == nil {
		t.Fatalf("no %s cookie among the %d issued", tqTokenCookie, len(cookies))
	}

	if c.Value != token {
		t.Fatalf("cookie value = %q, want the presented token", c.Value)
	}

	if !c.HttpOnly {
		t.Error("session cookie must be HttpOnly")
	}

	if c.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want Lax", c.SameSite)
	}

	if c.Path != "/" {
		t.Errorf("cookie Path = %q, want /", c.Path)
	}

	// Subresource with ONLY the cookie must pass — the pre-cookie failure.
	cssReq := httptest.NewRequest(http.MethodGet, "/static/app.css", nil)
	cssReq.AddCookie(c)

	cssRec := httptest.NewRecorder()
	handler.ServeHTTP(cssRec, cssReq)

	if cssRec.Code != http.StatusOK {
		t.Fatalf("static with session cookie: status = %d, want 200", cssRec.Code)
	}

	if ct := cssRec.Header().Get("Content-Type"); !strings.Contains(ct, "text/css") {
		t.Errorf("static Content-Type = %q, want text/css (a 401 body here is what broke stylesheets)", ct)
	}

	// Wrong cookie value must still be rejected.
	badReq := httptest.NewRequest(http.MethodGet, "/static/app.css", nil)
	badReq.AddCookie(&http.Cookie{Name: tqTokenCookie, Value: "nope"})

	badRec := httptest.NewRecorder()
	handler.ServeHTTP(badRec, badReq)

	if badRec.Code != http.StatusUnauthorized {
		t.Errorf("static with wrong cookie: status = %d, want 401", badRec.Code)
	}

	// A cookie-authenticated request must not re-issue the cookie.
	quietRec := httptest.NewRecorder()
	pageReq := httptest.NewRequest(http.MethodGet, "/", nil)
	pageReq.AddCookie(c)
	handler.ServeHTTP(quietRec, pageReq)

	if quietRec.Code != http.StatusOK {
		t.Fatalf("page with cookie: status = %d, want 200", quietRec.Code)
	}

	// The tq_token session cookie must NOT be re-issued on cookie auth
	// (the CSRF issuer may still hand out its own cookie to a fresh
	// browser — that one is page hygiene, not session state).
	for _, cookie := range quietRec.Result().Cookies() {
		if cookie.Name == tqTokenCookie {
			t.Error("cookie-authed request re-issued the session cookie")
		}
	}
}

// csrfFrom extracts the issued CSRF cookie value from a recorder.
func csrfFrom(rec *httptest.ResponseRecorder) string {
	for _, c := range rec.Result().Cookies() {
		if c.Name == tqCSRFCookie {
			return c.Value
		}
	}

	return ""
}

// TestWriteRoutesNeedAllowWrites pins the ADR-0003 default: without
// Config.AllowWrites the admin routes are not registered at all.
func TestWriteRoutesNeedAllowWrites(t *testing.T) {
	srv := New(newTestStore(t), Config{Poll: time.Millisecond, Heartbeat: time.Millisecond})

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/task/task_x/cancel", nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("POST cancel without AllowWrites: status = %d, want 404 (route not registered)", rec.Code)
	}
}

// TestWriteFlowCancelAndRescue exercises both admin endpoints end to end:
// CSRF challenge, cooperative stop for running, withdrawal for pending,
// rescue for dead — each landing as a fact in the journal.
func TestWriteFlowCancelAndRescue(t *testing.T) {
	s := newTestStore(t)
	srv := New(s, Config{
		Poll: time.Millisecond, Heartbeat: time.Millisecond,
		AuthToken: "sekrit", AllowWrites: true,
	})
	handler := srv.Handler()

	// Authenticate once: tokens + cookies issued together.
	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/?token=sekrit", nil))

	csrf := csrfFrom(first)
	if csrf == "" {
		t.Fatal("no CSRF cookie issued on page GET")
	}

	post := func(path, csrfValue string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, path,
			strings.NewReader("csrf="+csrfValue+"&reason=operator+was+here&attempts=5"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Authorization", "Bearer sekrit")

		for _, c := range first.Result().Cookies() {
			req.AddCookie(c)
		}

		handler.ServeHTTP(rec, req)

		return rec
	}

	// --- cancel a PENDING task ---
	tk := enqueue(t, s, "sh", "demo")

	rec := post("/task/"+tk.ID.String()+"/cancel", csrf)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("cancel pending: status = %d, want 303", rec.Code)
	}

	got, err := s.Get(context.Background(), tk.ID)
	if err != nil {
		t.Fatalf("get after cancel: %v", err)
	}

	if got.Status != task.Cancelled {
		t.Fatalf("status after cancel = %s, want cancelled", got.Status)
	}

	// --- CSRF rejection: right cookie, wrong field ---
	tk2 := enqueue(t, s, "sh", "demo2")

	rec = post("/task/"+tk2.ID.String()+"/cancel", "forged")
	if rec.Code != http.StatusForbidden {
		t.Errorf("cancel with forged CSRF: status = %d, want 403", rec.Code)
	}

	// --- cooperative stop for RUNNING ---
	owner := "worker-test"
	if _, err := s.ClaimDue(context.Background(), owner, time.Minute); err != nil {
		t.Fatalf("claim: %v", err)
	}

	running, err := s.Get(context.Background(), tk2.ID)
	if err != nil || running.Status != task.Running {
		t.Fatalf("task should be running, got %s (%v)", running.Status, err)
	}

	rec = post("/task/"+tk2.ID.String()+"/cancel", csrf)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("stop running: status = %d, want 303", rec.Code)
	}

	after, err := s.Get(context.Background(), tk2.ID)
	if err != nil {
		t.Fatalf("get after stop: %v", err)
	}

	if after.Status != task.Running && after.Status != task.Cancelled {
		t.Errorf("status after stop request = %s, want running (cancel requested) or cancelled", after.Status)
	}

	// --- rescue a DEAD task ---
	dead := enqueue(t, s, "sh", "demo3")

	// Dead-lettering requires a RUNNING task: claim it with the same
	// lease owner the failure reports.
	claimed, err := s.ClaimDue(context.Background(), owner, time.Minute)
	if err != nil {
		t.Fatalf("claim for dead-lettering: %v", err)
	}

	if claimed.ID != dead.ID {
		t.Fatalf("claimed %s, want the freshly enqueued %s", claimed.ID, dead.ID)
	}

	if err := s.FailPermanent(context.Background(), dead.ID, owner, "boom: rescue test", json.RawMessage(`{}`)); err != nil {
		t.Fatalf("fail permanent: %v", err)
	}

	rec = post("/task/"+dead.ID.String()+"/rescue", csrf)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("rescue dead: status = %d, want 303", rec.Code)
	}

	rescued, err := s.Get(context.Background(), dead.ID)
	if err != nil {
		t.Fatalf("get after rescue: %v", err)
	}

	if rescued.Status != task.Pending {
		t.Fatalf("status after rescue = %s, want pending", rescued.Status)
	}

	if rescued.Attempts != 0 {
		t.Errorf("attempts after rescue = %d, want a fresh budget (0 used)", rescued.Attempts)
	}

	// --- conflict: rescue a task that is not dead ---
	rec = post("/task/"+tk.ID.String()+"/rescue", csrf)
	if rec.Code != http.StatusConflict {
		t.Errorf("rescue non-dead: status = %d, want 409", rec.Code)
	}
}

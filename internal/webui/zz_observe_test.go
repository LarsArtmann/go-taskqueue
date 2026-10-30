package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestZZObserveCT2(t *testing.T) {
	_, server := newServeServer(t, Config{AuthToken: "secret"})
	resp := getContextGET(t, server.URL+"/")
	t.Logf("dashboard 401 status=%d content-type=%q", resp.StatusCode, resp.Header.Get("Content-Type"))
	resp.Body.Close()

	_, server2 := newServeServer(t, Config{AllowWrites: true})
	first := getContextGET(t, server2.URL+"/")
	first.Body.Close()
	var csrf string
	for _, c := range first.Cookies() {
		if c.Name == tqCSRFCookie {
			csrf = c.Value
		}
	}
	for i := range 4 {
		req, _ := http.NewRequest(http.MethodPost, server2.URL+"/task/x/cancel", strings.NewReader("csrf=bad"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: tqCSRFCookie, Value: csrf})
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("lockout POST %d status=%d content-type=%q", i, r.StatusCode, r.Header.Get("Content-Type"))
		r.Body.Close()
	}

	srv := newAPIHeaderMatrixServer(t)
	h := srv.Handler()
	for range 4 {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil)
		req.Header.Set("Authorization", "Bearer wrong")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		t.Logf("api lockout status=%d content-type=%q", rec.Code, rec.Header().Get("Content-Type"))
	}
}

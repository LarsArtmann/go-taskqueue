package webui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/larsartmann/go-sse/ssetest"
)

// Scratch verification for the f40 design spike: the SSE stream renders under
// ITS OWN request's filter (empty, as app.js connects without forwarding the
// page URL's filter params), so a filtered page gets clobbered by the first
// tick. NOT FOR COMMIT.
func TestScratchFilterClobber(t *testing.T) {
	srv, s := newTestServer(t)
	enqueue(t, s, "sh", "alpha")
	enqueue(t, s, "sh", "beta")

	// The page the browser is on: /?project=alpha (narrowed table).
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/?project=alpha", nil))
	page := tableFragment(rec.Body.String())
	t.Logf("page table has alpha=%v beta=%v", strings.Contains(page, "alpha"), strings.Contains(page, "beta"))

	// The stream the browser actually opens: /api/events with NO filter params.
	events := ssetest.CollectN(t, srv.Handler(), 6, ssetest.WithPath("/api/events"))

	for _, evt := range events {
		if evt.Type != testFragEvent {
			continue
		}

		var frag fragment
		if err := json.Unmarshal([]byte(evt.Data()), &frag); err != nil {
			t.Fatalf("decode: %v", err)
		}

		if frag.ID == fragTable {
			t.Logf("stream table (what a filtered page would swap in) has alpha=%v beta=%v",
				strings.Contains(frag.HTML, "alpha"), strings.Contains(frag.HTML, "beta"))
		}

		if evt.ID != "" {
			t.Logf("event %q carried id=%q", evt.Type, evt.ID)
		}
	}

	fmt.Println("scratch done")
}

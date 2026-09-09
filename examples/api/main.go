// Command api is a thin HTTP wrapper over the queue Store — the
// proof-of-concept for non-Go producers, plus a Prometheus /metrics endpoint
// over the journal and a minimal live stats page. PoC scope: no auth, no
// write-path beyond enqueue — bind it to localhost only.
//
// Usage:
//
//	go run ./examples/api --db tasks.db --addr 127.0.0.1:8095
//
// Endpoints:
//
//	POST /enqueue  {"type":"sh","project":"demo","payload":{"cmd":"echo hi"}}
//	GET  /stats    per-status task counts (JSON)
//	GET  /metrics  Prometheus text format (fact counters + status gauges)
//	GET  /         single-page live view of /stats
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

func main() {
	db := flag.String("db", "tasks.db", "queue database path")
	addr := flag.String("addr", "127.0.0.1:8095", "listen address (keep it localhost: no auth)")

	flag.Parse()

	store, err := sqlite.Open(*db)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	http.HandleFunc("POST /enqueue", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Type    string          `json:"type"`
			Project string          `json:"project"`
			Payload json.RawMessage `json:"payload"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Type == "" {
			http.Error(w, `want {"type","project","payload"}`, http.StatusBadRequest)

			return
		}

		t, err := queue.New(store).Enqueue(r.Context(), task.New{
			Project: req.Project,
			Type:    req.Type,
			Payload: req.Payload,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": t.ID.String()})
	})

	http.HandleFunc("GET /stats", func(w http.ResponseWriter, r *http.Request) {
		counts, err := statusCounts(store, r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		_ = json.NewEncoder(w).Encode(counts)
	})

	http.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		counts, err := statusCounts(store, r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		facts, err := store.Facts(r.Context(), 0, 0)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		byType := map[journal.FactType]int{}
		for _, f := range facts {
			byType[f.Type]++
		}

		var b strings.Builder
		fmt.Fprintf(&b, "# HELP tq_tasks_total Tasks by status.\n# TYPE tq_tasks_total gauge\n")

		for status, n := range counts {
			fmt.Fprintf(&b, "tq_tasks_total{status=%q} %d\n", status, n)
		}

		fmt.Fprintf(&b, "# HELP tq_facts_total Journal facts by type.\n# TYPE tq_facts_total counter\n")

		for typ, n := range byType {
			fmt.Fprintf(&b, "tq_facts_total{type=%q} %d\n", typ, n)
		}

		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = w.Write([]byte(b.String()))
	})

	http.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(statsPage))
	})

	log.Printf("api: serving %s on http://%s (no auth — localhost only)", *db, *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

func statusCounts(s *sqlite.Store, ctx context.Context) (map[string]int, error) {
	tasks, err := s.List(ctx, queue.Filter{})
	if err != nil {
		return nil, err
	}

	counts := map[string]int{}
	for _, t := range tasks {
		counts[string(t.Status)]++
	}

	return counts, nil
}

const statsPage = `<!doctype html>
<title>tq live</title>
<meta http-equiv="refresh" content="2">
<style>body{font-family:ui-monospace,monospace;background:#111;color:#eee;padding:2rem}
td,th{padding:.2rem 1.2rem;text-align:left}th{color:#888}</style>
<h1>tq — live queue</h1>
<table id="s"><tr><th>status</th><th>count</th></tr></table>
<script>
setInterval(async () => {
  const r = await fetch('/stats'); const c = await r.json();
  const t = document.getElementById('s');
  t.innerHTML = '<tr><th>status</th><th>count</th></tr>';
  for (const [k, v] of Object.entries(c).sort()) {
    t.insertAdjacentHTML('beforeend', '<tr><td>'+k+'</td><td>'+v+'</td></tr>');
  }
}, 2000);
</script>
`

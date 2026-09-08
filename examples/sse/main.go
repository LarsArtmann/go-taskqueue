// Command sse streams the tq journal as Server-Sent Events — the
// proof-of-concept for live dashboards without a polling client.
//
// Usage:
//
//	go run ./examples/sse --db tasks.db --addr :8090
//	curl -N localhost:8090/events            # replay, then follow
//	curl -N 'localhost:8090/events?after=42' # only newer facts
//
// Each event carries the fact JSON, with the journal sequence as the SSE id
// so a reconnecting client can resume where it left off (`Last-Event-ID`
// header maps to `after`). PoC scope: one poller per connection; production
// would fan out from a single tailer.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/queue"
)

func main() {
	db := flag.String("db", "tasks.db", "queue database path")
	addr := flag.String("addr", ":8090", "listen address")
	poll := flag.Duration("poll", 500*time.Millisecond, "journal tail interval")

	flag.Parse()

	store, err := queue.OpenSQLite(*db)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	http.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)

			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")

		after := int64(0)
		if v := r.URL.Query().Get("after"); v != "" {
			_, _ = fmt.Sscanf(v, "%d", &after)
		}

		if lid := r.Header.Get("Last-Event-ID"); lid != "" {
			_, _ = fmt.Sscanf(lid, "%d", &after)
		}

		ctx := r.Context()
		for {
			facts, err := store.Facts(ctx, after, 0)
			if err != nil {
				return
			}

			for _, f := range facts {
				body, err := json.Marshal(f)
				if err != nil {
					return
				}

				_, _ = fmt.Fprintf(w, "id: %d\nevent: fact\ndata: %s\n\n", f.Seq, body)
				after = f.Seq
			}

			if len(facts) > 0 {
				flusher.Flush()
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(*poll):
			}
		}
	})
	log.Printf("sse: streaming %s on http://%s/events", *db, *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

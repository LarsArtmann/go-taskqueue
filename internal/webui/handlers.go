package webui

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/larsartmann/go-sse"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// parseFilter reads the view filter from the request URL query.
func parseFilter(r *http.Request) FilterState {
	q := r.URL.Query()

	return FilterState{
		Project: q.Get("project"),
		Status:  task.Status(q.Get("status")),
		Query:   q.Get("q"),
	}
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	data, err := s.loadSnapshot(r.Context(), parseFilter(r))
	if err != nil {
		http.Error(w, "load projection: "+err.Error(), http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := Page(data).Render(r.Context(), w); err != nil {
		slog.Error("webui: render index", "err", err)
	}
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	data, err := s.loadSnapshot(r.Context(), FilterState{})
	if err != nil {
		http.Error(w, "load projection: "+err.Error(), http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(map[string]any{
		badgePending: data.Counts[task.Pending],
		"running":    data.Counts[task.Running],
		"completed":  data.Counts[task.Completed],
		"dead":       data.Counts[task.Dead],
		"cancelled":  data.Counts[task.Cancelled],
		"total":      data.Total,
	}); err != nil {
		slog.Error("webui: encode stats", "err", err)
	}
}

func (s *Server) handleTaskDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	t, err := s.store.Get(r.Context(), task.ID(id))
	if err != nil {
		http.Error(w, "task not found: "+id, http.StatusNotFound)

		return
	}

	facts, err := s.factsForTask(r.Context(), id)
	if err != nil {
		http.Error(w, "load facts: "+err.Error(), http.StatusInternalServerError)

		return
	}

	data := DashboardData{Now: time.Now()}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := TaskDetailPage(data, taskDetailData{Task: t, Facts: facts, ID: id}).Render(r.Context(), w); err != nil {
		slog.Error("webui: render detail", "err", err)
	}
}

// handleEvents is the SSE endpoint. Protocol per client connection:
//
//  1. Subscribe to the hub FIRST (no gap between snapshot and live events).
//  2. Send a full snapshot (all fragments, rendered under this request's
//     filter). On reconnect this IS the resume: state is a projection, so
//     any Last-Event-ID is satisfied by a fresh snapshot — unknown or stale
//     IDs fall back to the same full snapshot, never a partial patch.
//  3. On every hub tick, coalesce the burst, re-render, and send a fresh
//     snapshot. Event ids carry the tick's journal watermark.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if _, ok := w.(http.Flusher); !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)

		return
	}

	w.Header().Set("X-Accel-Buffering", "no")

	stream := sse.NewStream(w, r)
	defer func() { _ = stream.Close() }()

	ctx := r.Context()

	eventCh := s.hub.Subscribe()
	defer s.hub.Unsubscribe(eventCh)

	// Last-Event-ID is accepted for protocol compatibility; because every
	// event is a full re-render of the projection, the snapshot below is
	// always the correct resume regardless of the ID's freshness.
	lastID := stream.LastEventID()
	if !lastID.IsZero() {
		slog.Debug("webui: client resume", "last-event-id", lastID.String())
	}

	if err := s.sendSnapshot(ctx, stream, r, watermarkUnknown); err != nil {
		return
	}

	go stream.Heartbeat(ctx, s.cfg.Heartbeat)

	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-eventCh:
			if !ok {
				return
			}

			if evt.Event != "tick" {
				continue
			}

			seq := int64(watermarkUnknown)
			if n, err := strconv.ParseInt(evt.ID.Get(), 10, 64); err == nil {
				seq = n
			}

			if err := s.sendSnapshot(ctx, stream, r, seq); err != nil {
				return
			}
		}
	}
}

// watermarkUnknown marks snapshot events not tied to a specific tick.
const watermarkUnknown = -1

func (s *Server) sendSnapshot(ctx context.Context, stream *sse.Stream, r *http.Request, seq int64) error {
	data, err := s.loadSnapshot(ctx, parseFilter(r))
	if err != nil {
		if ctx.Err() != nil {
			return err
		}

		slog.Error("webui: snapshot query", "err", err)

		return err
	}

	for _, frag := range renderFragments(ctx, data) {
		if err := stream.SendJSON("frag", frag); err != nil {
			return err
		}
	}

	evt := sse.Event{Event: "title", Data: pageTitle(data)}
	if seq >= 0 {
		evt.ID = sse.NewEventID(formatSeq(seq))
	}

	if err := stream.Send(evt); err != nil {
		return err
	}

	return ctx.Err()
}

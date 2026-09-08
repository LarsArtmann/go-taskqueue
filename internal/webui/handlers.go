package webui

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/larsartmann/go-sse"
	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// parseFilter reads the view filter from the request URL query, including
// the 1-based ?page= (values < 1 clamp to page 1).
func parseFilter(r *http.Request) FilterState {
	q := r.URL.Query()

	page := 1
	if v := q.Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = n
		}
	}

	// Allowlist: unknown sort values fall back to the default order.
	sort := q.Get("sort")
	switch sort {
	case "", "age-asc", "age-desc", "priority-asc", "priority-desc", "attempts-asc", "attempts-desc":
	default:
		sort = ""
	}

	return FilterState{
		Project: q.Get("project"),
		Status:  task.Status(q.Get("status")),
		Query:   q.Get("q"),
		Page:    page,
		Sort:    sort,
	}
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	s.renderIndex(w, r, parseFilter(r))
}

// handleProject serves /project/{name}: the full dashboard with the filter
// pinned to one project — a shareable, bookmarkable project page that
// reuses the whole filter/sort/pagination pipeline.
func (s *Server) handleProject(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		http.NotFound(w, r)
		return
	}

	f := parseFilter(r)
	f.Project = name
	s.renderIndex(w, r, f)
}

func (s *Server) renderIndex(w http.ResponseWriter, r *http.Request, f FilterState) {
	data, err := s.loadSnapshot(r.Context(), f)
	if err != nil {
		http.Error(w, "load projection: "+err.Error(), http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := Page(data).Render(r.Context(), w); err != nil {
		slog.Error("webui: render index", "err", err)
	}
}

// handleFacts serves GET /api/facts?after=SEQ&limit=N: a forward cursor
// over the journal in ascending seq order (the infinite-scroll viewer's
// data source; the dashboard feed stays the SSE tail). limit is capped.
func (s *Server) handleFacts(w http.ResponseWriter, r *http.Request) {
	after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)

	limit := factViewerPageSize
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 && v <= factViewerPageSize {
		limit = v
	}

	facts, err := s.store.Facts(r.Context(), after, limit)
	if err != nil {
		http.Error(w, "facts: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var next int64

	if len(facts) > 0 {
		next = facts[len(facts)-1].Seq
	}

	if err := json.NewEncoder(w).Encode(map[string]any{
		"facts": facts,
		"next":  next,
	}); err != nil {
		slog.Error("webui: encode facts", "err", err)
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
		badgePending:   data.Counts[task.Pending],
		labelRunning:   data.Counts[task.Running],
		labelCompleted: data.Counts[task.Completed],
		labelDead:      data.Counts[task.Dead],
		labelCancelled: data.Counts[task.Cancelled],
		labelTotal:     data.Total,
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

	// The review loop's verdict, when this task is a finished review: the
	// badge + findings card render from the completion-fact detail.
	if t.Type == executor.TaskTypeReview && t.Status == task.Completed {
		if res, ok := reviewResultFor(r.Context(), s.store, id); ok {
			data.Reviews = map[string]executor.ReviewResult{t.ID.String(): res}
		}
	}

	// The status loop's outcome, when this task is a finished report: the
	// badge + report card render from the completion-fact detail.
	if t.Type == executor.TaskTypeStatus && t.Status == task.Completed {
		if res, ok := statusResultFor(r.Context(), s.store, id); ok {
			data.Statuses = map[string]executor.StatusResult{t.ID.String(): res}
		}
	}

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
	// always the correct resume regardless of the ID's freshness. A stale
	// id still carries reconnect-lag signal: head − id is how far the
	// browser's last view trailed the journal when it dropped.
	lastID := stream.LastEventID()
	if !lastID.IsZero() {
		if n, err := strconv.ParseInt(lastID.Get(), 10, 64); err == nil && n >= 0 {
			if head, err := s.store.HeadSeq(r.Context()); err == nil && head > n {
				slog.Info("webui: client reconnect", "last-event-id", n, "head", head, "reconnect lag", head-n)
			}
		}
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

// handleTaskEvents streams the per-task detail page's live fragments: the
// same subscribe → snapshot → tick protocol as /api/events, scoped to one
// task's record card and fact timeline. Every event is a full re-render of
// both fragments, so any Last-Event-ID is satisfied by a fresh snapshot.
func (s *Server) handleTaskEvents(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if _, err := s.store.Get(r.Context(), task.ID(id)); err != nil {
		http.Error(w, "task not found: "+id, http.StatusNotFound)

		return
	}

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

	if err := s.sendTaskSnapshot(ctx, stream, id, watermarkUnknown); err != nil {
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

			if err := s.sendTaskSnapshot(ctx, stream, id, seq); err != nil {
				return
			}
		}
	}
}

func (s *Server) sendTaskSnapshot(ctx context.Context, stream *sse.Stream, id string, seq int64) error {
	t, err := s.store.Get(ctx, task.ID(id))
	if err != nil {
		if ctx.Err() != nil {
			return err
		}

		slog.Error("webui: task snapshot query", "task", id, "err", err)

		return err
	}

	facts, err := s.factsForTask(ctx, id)
	if err != nil {
		if ctx.Err() != nil {
			return err
		}

		slog.Error("webui: task facts query", "task", id, "err", err)

		return err
	}

	data := DashboardData{Now: time.Now()}

	for _, frag := range renderTaskFragments(ctx, data, t, facts) {
		if err := stream.SendJSON("frag", frag); err != nil {
			return err
		}
	}

	// The trailing title event carries the journal watermark id, keeping
	// the detail stream's resume semantics identical to /api/events.
	evt := sse.Event{Event: "title", Data: detailPageTitle(id)}
	if seq >= 0 {
		evt.ID = sse.NewEventID(formatSeq(seq))
	}

	if err := stream.Send(evt); err != nil {
		return err
	}

	return ctx.Err()
}

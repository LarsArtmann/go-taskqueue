package webui

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/larsartmann/go-sse"
	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// parseFilter reads the view filter from the request URL query, including
// the 1-based ?page= (values < 1 clamp to page 1).
func parseFilter(r *http.Request) FilterState {
	query := r.URL.Query()

	page := 1

	if v := query.Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = n
		}
	}

	// Allowlist: unknown sort values fall back to the default order.
	sort := query.Get("sort")
	switch sort {
	case "", "age-asc", "age-desc", "priority-asc", "priority-desc", "attempts-asc", "attempts-desc":
	default:
		sort = ""
	}

	// Allowlist: only the two known projections; anything else is the table.
	view := query.Get("view")
	if view != viewBoard {
		view = viewTable
	}

	filter := FilterState{
		Project: query.Get("project"),
		Status:  task.Status(query.Get("status")),
		Query:   query.Get("query"),
		Page:    page,
		Sort:    sort,
		View:    view,
	}

	// The board's columns ARE the statuses: a status filter would empty four
	// of five columns, so it is dropped at the boundary (chips and toggles
	// never render it back).
	if filter.View == viewBoard {
		filter.Status = ""
	}

	return filter
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
// after=-N selects a tail window: the newest N facts — the journal browser
// opens at the live end and pages older from there.
func (s *Server) handleFacts(w http.ResponseWriter, r *http.Request) {
	after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)

	if after < 0 {
		head, err := s.store.HeadSeq(r.Context())
		if err != nil {
			http.Error(w, "facts: "+err.Error(), http.StatusInternalServerError)

			return
		}

		if after = head + after; after < 0 {
			after = 0
		}
	}

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

// handleStats emits the per-status counts + total. The wire keys are the
// task.Status values themselves, not the HTML badge labels: this endpoint
// and the write API's GET /api/v1/stats (internal/httpapi) share one
// stats contract, pinned equal by TestStatsSurfacesAgree. It reads the
// counts directly — a full snapshot projection would be wasted work for a
// stats poll — and always reports every known status, zeros included.
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	counts, err := s.store.StatusCounts(r.Context())
	if err != nil {
		http.Error(w, "load stats: "+err.Error(), http.StatusInternalServerError)

		return
	}

	total := 0

	for _, n := range counts {
		total += n
	}

	out := make(map[string]int, len(allStatuses)+1)

	for _, st := range allStatuses {
		out[string(st)] = counts[st]
	}

	out[labelTotal] = total

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(out); err != nil {
		slog.Error("webui: encode stats", "err", err)
	}
}

func (s *Server) handleTaskDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	t, ok := s.taskFromPath(w, r, id)
	if !ok {
		return
	}

	facts, err := s.factsForTask(r.Context(), id)
	if err != nil {
		http.Error(w, "load facts: "+err.Error(), http.StatusInternalServerError)

		return
	}

	data := DashboardData{Now: time.Now()}

	// The review loop's verdict and the status loop's outcome, when this
	// task is one of those finished kinds: the badge + result card render
	// from the completion-fact detail.
	data.Reviews = pageResults(r.Context(), []task.Task{t}, executor.TaskTypeReview, s.reviewResultFor)
	data.Statuses = pageResults(r.Context(), []task.Task{t}, executor.TaskTypeStatus, s.statusResultFor)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := TaskDetailPage(data, taskDetailData{Task: t, Facts: facts, ID: id}).Render(r.Context(), w); err != nil {
		slog.Error("webui: render detail", "err", err)
	}
}

// handleEvents is the SSE endpoint: the shared subscribe → snapshot →
// tick-pump protocol (runEventStream) over the dashboard fragments,
// rendered under this request's filter.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	// Last-Event-ID is accepted for protocol compatibility; because every
	// event is a full re-render of the projection, the snapshot below is
	// always the correct resume regardless of the ID's freshness. A stale
	// id still carries reconnect-lag signal: head − id is how far the
	// browser's last view trailed the journal when it dropped.
	if lastID := sse.LastEventIDFromRequest(r); !lastID.IsZero() {
		if n, err := strconv.ParseInt(lastID.Get(), 10, 64); err == nil && n >= 0 {
			if head, err := s.store.HeadSeq(r.Context()); err == nil && head > n {
				slog.Info("webui: client reconnect", "last-event-id", n, "head", head, "reconnect lag", head-n)
			}
		}
	}

	s.runEventStream(w, r, func(ctx context.Context, stream *sse.Stream, seq int64) error {
		return s.sendSnapshot(ctx, stream, r, seq)
	})
}

// watermarkUnknown marks snapshot events not tied to a specific tick.
const watermarkUnknown = -1

// runEventStream is the shared SSE session behind /api/events and the
// per-task detail stream. Protocol per client connection:
//
//  1. Subscribe to the hub FIRST (no gap between snapshot and live events).
//  2. Send a full snapshot via snapshot(ctx, stream, watermarkUnknown). On
//     reconnect this IS the resume: state is a projection, so any
//     Last-Event-ID is satisfied by a fresh snapshot — unknown or stale IDs
//     fall back to the same full snapshot, never a partial patch.
//  3. On every hub tick, coalesce the burst, re-render, and send a fresh
//     snapshot. Event ids carry the tick's journal watermark.
func (s *Server) runEventStream(
	w http.ResponseWriter,
	r *http.Request,
	snapshot func(ctx context.Context, stream *sse.Stream, seq int64) error,
) {
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

	if err := snapshot(ctx, stream, watermarkUnknown); err != nil {
		return
	}

	// The heartbeat goroutine must be STOPPED before this handler returns:
	// net/http tears down the response (and its bufio writer) at handler
	// exit, and an in-flight Heartbeat Flush after that is a nil-pointer
	// SIGSEGV — in a goroutine net/http cannot recover, killing the whole
	// serve process. stopHeartbeat is deferred LAST so it runs FIRST,
	// before stream.Close and hub.Unsubscribe.
	stopHeartbeat := s.startHeartbeat(ctx, stream)
	defer stopHeartbeat()

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

			if err := snapshot(ctx, stream, seq); err != nil {
				return
			}
		}
	}
}

// sendSnapshot renders and streams one full dashboard snapshot burst.
func (s *Server) sendSnapshot(ctx context.Context, stream *sse.Stream, r *http.Request, seq int64) error {
	data, err := s.loadSnapshot(ctx, parseFilter(r))
	if err != nil {
		return queryErr(ctx, "webui: snapshot query", err)
	}

	return sendSnapshotPayload(ctx, stream, renderFragments(ctx, data), pageTitle(data), seq)
}

// handleTaskEvents streams the per-task detail page's live fragments: the
// same subscribe → snapshot → tick protocol as /api/events (runEventStream),
// scoped to one task's record card and fact timeline. Every event is a full
// re-render of both fragments, so any Last-Event-ID is satisfied by a fresh
// snapshot.
func (s *Server) handleTaskEvents(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if _, ok := s.taskFromPath(w, r, id); !ok {
		return
	}

	s.runEventStream(w, r, func(ctx context.Context, stream *sse.Stream, seq int64) error {
		return s.sendTaskSnapshot(ctx, stream, id, seq)
	})
}

// startHeartbeat launches the SSE keepalive and returns a stop function
// that cancels it AND waits for the goroutine to exit — the wait is the
// point: an unwaited heartbeat can Flush a response that net/http has
// already torn down (SIGSEGV in an unrecoverable goroutine).
func (s *Server) startHeartbeat(ctx context.Context, stream *sse.Stream) func() {
	hbCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})

	go func() {
		defer close(done)

		stream.Heartbeat(hbCtx, s.cfg.Heartbeat)
	}()

	return func() {
		cancel()
		<-done
	}
}

// queryErr converts a snapshot query failure into the returned error,
// logging it unless the request context is already done — a client
// disconnect abandons the query; that is abandonment, not a failure worth
// a log line.
func queryErr(ctx context.Context, msg string, err error, attrs ...any) error {
	if ctx.Err() == nil {
		slog.Error(msg, append(attrs, "err", err)...)
	}

	return err
}

// sendSnapshotPayload streams one full snapshot burst: the fragments
// followed by the trailing title event carrying the journal watermark id,
// keeping every stream's resume semantics identical.
func sendSnapshotPayload(ctx context.Context, stream *sse.Stream, frags []fragment, title string, seq int64) error {
	for _, frag := range frags {
		if err := stream.SendJSON("frag", frag); err != nil {
			return fmt.Errorf("send snapshot fragment: %w", err)
		}
	}

	evt := sse.Event{Event: "title", Data: title}
	if seq >= 0 {
		evt.ID = sse.NewEventID(formatSeq(seq))
	}

	if err := stream.Send(evt); err != nil {
		return fmt.Errorf("send snapshot title: %w", err)
	}

	return ctx.Err()
}

func (s *Server) sendTaskSnapshot(ctx context.Context, stream *sse.Stream, id string, seq int64) error {
	t, err := s.store.Get(ctx, task.ID(id))
	if err != nil {
		return queryErr(ctx, "webui: task snapshot query", err, "task", id)
	}

	facts, err := s.factsForTask(ctx, id)
	if err != nil {
		return queryErr(ctx, "webui: task facts query", err, "task", id)
	}

	data := DashboardData{Now: time.Now()}

	return sendSnapshotPayload(ctx, stream, renderTaskFragments(ctx, data, t, facts), detailPageTitle(id), seq)
}

// taskFromPath fetches the {id} path task, rendering the styled 404 when
// it doesn't resolve. ok=false means the response is already written.
func (s *Server) taskFromPath(w http.ResponseWriter, r *http.Request, id string) (task.Task, bool) {
	t, err := s.store.Get(r.Context(), task.ID(id))
	if err != nil {
		s.renderTaskNotFound(w, r, id)

		return task.Task{}, false
	}

	return t, true
}

// handleTaskCancelPOST withdraws a pending task or requests a cooperative
// stop for a running one (the agent honors the request between steps; an
// expired lease finalizes it). The reason lands in the task.cancelled fact
// detail — forensics over silence. Registered only with AllowWrites.
func (s *Server) handleTaskCancelPOST(w http.ResponseWriter, r *http.Request) {
	id := task.ID(r.PathValue("id"))
	reason := strings.TrimSpace(r.PostFormValue("reason"))

	t, ok := s.taskFromPath(w, r, id.String())
	if !ok {
		return
	}

	var err error

	switch t.Status {
	case task.Pending:
		err = s.store.Cancel(r.Context(), id, reason)
	case task.Running:
		err = s.store.CancelRunning(r.Context(), id, reason)
	default:
		http.Error(
			w,
			"task is "+string(t.Status)+"; only pending or running tasks can be cancelled",
			http.StatusConflict,
		)

		return
	}

	if err != nil {
		http.Error(w, "cancel failed: "+err.Error(), http.StatusInternalServerError)

		return
	}

	http.Redirect(w, r, "/task/"+id.String(), http.StatusSeeOther)
}

// handleTaskRescuePOST re-queues a dead-lettered task with a fresh attempt
// budget (RescueDead). Registered only with AllowWrites.
func (s *Server) handleTaskRescuePOST(w http.ResponseWriter, r *http.Request) {
	id := task.ID(r.PathValue("id"))

	t, ok := s.taskFromPath(w, r, id.String())
	if !ok {
		return
	}

	if t.Status != task.Dead {
		http.Error(w, "task is "+string(t.Status)+"; only dead-lettered tasks can be rescued", http.StatusConflict)

		return
	}

	attempts := 3
	if v, err := strconv.Atoi(r.PostFormValue("attempts")); err == nil {
		attempts = min(max(v, 1), 10)
	}

	if err := s.store.RescueDead(r.Context(), id, attempts); err != nil {
		http.Error(w, "rescue failed: "+err.Error(), http.StatusInternalServerError)

		return
	}

	http.Redirect(w, r, "/task/"+id.String(), http.StatusSeeOther)
}

// renderTaskNotFound serves the styled 404 through the dashboard layout —
// a bare http.Error left the operator on a white void with no way back.
func (s *Server) renderTaskNotFound(w http.ResponseWriter, r *http.Request, id string) {
	w.WriteHeader(http.StatusNotFound)

	if err := TaskNotFoundPage(id).Render(r.Context(), w); err != nil {
		slog.Error("webui: render 404", "err", err)
	}
}

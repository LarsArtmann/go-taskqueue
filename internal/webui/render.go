package webui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
	"github.com/larsartmann/templ-components/display"
)

const (
	timeFormat          = "2006-01-02 15:04:05 MST"
	errorPreviewLen     = 60
	factFeedLen         = 50
	factViewerPageSize  = 100 // /api/facts page cap for the journal browser
	taskTableLimit      = 200
	detailFactsLimit    = 500
	tailBatchLimit      = 1000
	hoursPerDay         = 24 * time.Hour
	renderErrPreviewLen = 120
	boardColumnLimit    = 25 // board view: newest tasks shown per status column
)

// journalFactView aliases the fact type so templ templates can reference it
// without importing the journal package.
type journalFactView = journal.Fact

// taskDetailData carries everything the per-task page renders.
type taskDetailData struct {
	Task  task.Task
	Facts []journalFactView
	ID    string
}

// ProjectSummary aggregates one project's live counts for the overview chips.
type ProjectSummary struct {
	Name    string
	Pending int
	Running int
	Dead    int
}

// View names for the task projection region (#frag-table): the default
// ledger table, or the board — a Kanban-style column per lifecycle status.
// Read-only by construction either way (ADR-0003); moving cards between
// columns is queue mutation and stays behind the future --allow-writes gate.
const (
	viewTable = ""
	viewBoard = "board"
)

// FilterState is the URL-carried view filter (?project=&status=&q=).
// Page rides in the URL (?page=N) but is deliberately not part of
// QueryString: filter chips always reset to page 1.
type FilterState struct {
	Project string
	Status  task.Status
	Query   string
	Page    int
	// Sort selects the task-table ordering: "", "age-asc", "age-desc",
	// "priority-asc", "priority-desc", "attempts-asc", "attempts-desc".
	// Empty keeps the severity order (dead, running, pending, ...).
	Sort string
	// View picks the task projection: viewTable (default) or viewBoard.
	// Board drops Status (columns ARE the statuses) and ignores Sort/Page.
	View string
}

// Empty reports whether no filter is active.
func (f FilterState) Empty() bool {
	return f.Project == "" && f.Status == "" && f.Query == ""
}

// QueryString renders the filter as URL query parameters.
func (f FilterState) QueryString() string {
	var b strings.Builder

	if f.Project != "" {
		fmt.Fprintf(&b, "project=%s&", f.Project)
	}

	if f.Status != "" {
		fmt.Fprintf(&b, "status=%s&", f.Status)
	}

	if f.Query != "" {
		fmt.Fprintf(&b, "q=%s&", f.Query)
	}

	if f.Sort != "" {
		fmt.Fprintf(&b, "sort=%s&", f.Sort)
	}

	if f.View != "" && f.View != viewTable {
		fmt.Fprintf(&b, "view=%s&", f.View)
	}

	s := b.String()

	return strings.TrimSuffix(s, "&")
}

// BudgetView is the daily agent-spend projection: enqueued today vs the
// operator-set cap. Nil in the snapshot when no cap is configured.
type BudgetView struct {
	Cap   int
	Spent int
}

// Tone picks the card's semantic color: green under 75%, amber under the
// cap, red at/over it.
func (b BudgetView) Tone() display.StatTone {
	switch {
	case b.Spent >= b.Cap:
		return display.StatToneRed
	case b.Spent*4 >= b.Cap*3:
		return display.StatToneYellow

	default:
		return display.StatToneGreen
	}
}

// BoardColumn is one lifecycle column of the board view: the true count
// of tasks in this status (under the filter's project/query scope) plus
// the newest boardColumnLimit cards. Truncated is how many older tasks
// the column omits (the "+N older" link opens them in the table view).
type BoardColumn struct {
	Status    task.Status
	Count     int
	Tasks     []task.Task
	Truncated int
}

// DashboardData is the full projection snapshot one burst renders from.
type DashboardData struct {
	Counts     map[task.Status]int
	Total      int
	Tasks      []task.Task
	Dead       []task.Task
	Facts      []journal.Fact
	Filter     FilterState
	Now        time.Time
	Projects   []ProjectSummary
	Page       int
	TotalPages int
	MatchTotal int
	Budget     *BudgetView
	// Board holds the per-status columns when the filter selects the board
	// view; nil on the table view (which uses Tasks/Page/TotalPages).
	Board []BoardColumn
	// JournalSeq is the journal watermark (highest fact seq) — the resume
	// point every SSE/bridge consumer carries.
	JournalSeq int64
	// FactBuckets counts facts per equal slice of the last hour (the
	// activity sparkline's series).
	FactBuckets []float64
	// CompleteMinutes is the time-to-complete (queue wait + run) of recent
	// completed tasks in minutes, newest first (the completion histogram's
	// raw data).
	CompleteMinutes []float64
	// Reviews holds the parsed verdict of every COMPLETED review task on
	// the visible page (keyed by task id) — the agent-review loop made
	// visible: an approve/request-changes badge in the table and findings
	// on the detail page. Absent when the page shows no finished reviews.
	Reviews map[string]executor.ReviewResult
	// AllowWrites mirrors Config.AllowWrites: the templates render the
	// admin affordances (cancel/stop/rescue forms) only when writes are
	// enabled server-side.
	AllowWrites bool
	// Statuses holds the parsed outcome of every COMPLETED status task on
	// the visible page (keyed by task id) — the done-prompt loop made
	// visible: a report badge in the table and the report path + next-item
	// count on the detail page. Absent when the page shows no finished
	// status reports.
	Statuses map[string]executor.StatusResult
}

// statusResultFor reads a completed status task's outcome from its own
// completion-fact detail (executor.StatusResult JSON). ok=false when the
// task never completed or its detail is absent or foreign — never an error:
// a foreign shape renders as “no report”, not a broken page.
func statusResultFor(ctx context.Context, src queue.Store, id string) (executor.StatusResult, bool) {
	facts, err := src.FactsForTask(ctx, id, 0)
	if err != nil {
		return executor.StatusResult{}, false
	}

	for _, fact := range slices.Backward(facts) {
		if fact.Type != journal.Completed {
			continue
		}

		var res executor.StatusResult
		if json.Unmarshal(fact.Detail, &res) != nil || res.Report == "" {
			return executor.StatusResult{}, false
		}

		return res, true
	}

	return executor.StatusResult{}, false
}

// reviewResultFor reads a completed review task's verdict from its own
// completion-fact detail (executor.ReviewResult JSON). ok=false when the
// task never completed or its detail is absent or foreign — never an
// error: a foreign shape renders as “no verdict”, not a broken page.
func reviewResultFor(ctx context.Context, src queue.Store, id string) (executor.ReviewResult, bool) {
	facts, err := src.FactsForTask(ctx, id, 0)
	if err != nil {
		return executor.ReviewResult{}, false
	}

	for _, fact := range slices.Backward(facts) {
		if fact.Type != journal.Completed {
			continue
		}

		var res executor.ReviewResult
		if json.Unmarshal(fact.Detail, &res) != nil || res.Verdict == "" {
			return executor.ReviewResult{}, false
		}

		return res, true
	}

	return executor.ReviewResult{}, false
}

// clearProject / clearStatus / clearQuery are used by the filter chips.
func clearProject(f FilterState) string {
	f.Project = ""

	return filterHref(f)
}

func clearStatus(f FilterState) string {
	f.Status = ""

	return filterHref(f)
}

func clearQuery(f FilterState) string {
	f.Query = ""

	return filterHref(f)
}

func filterURL(f FilterState, reset func(FilterState) string) string {
	return reset(f)
}

func filterHref(f FilterState) string {
	q := f.QueryString()
	if q == "" {
		return "/"
	}

	return "/?" + q
}

// viewToggleHref switches the task projection (table <-> board) keeping the
// project/query scope. Switching resets page and sort; the board also drops
// the status filter — columns ARE the statuses.
func viewToggleHref(f FilterState, view string) string {
	f.View = view
	f.Page = 1
	f.Sort = ""

	if view == viewBoard {
		f.Status = ""
	}

	return filterHref(f)
}

// pageHref renders the current filter pinned to a specific page; filter
// chips keep using filterHref, which resets to page 1.
func pageHref(f FilterState, page int) string {
	q := f.QueryString()
	if page > 1 {
		if q != "" {
			q += "&"
		}

		q += "page=" + strconv.Itoa(page)
	}

	if q == "" {
		return "/"
	}

	return "/?" + q
}

func filterSuffix(f FilterState) string {
	if f.Empty() {
		return ""
	}

	return " (filtered)"
}

var allStatuses = []task.Status{
	task.Pending,
	task.Running,
	task.Completed,
	task.Dead,
	task.Cancelled,
}

// loadSnapshot queries the store for the complete current projection
// (status counts, visible tasks, DLQ, recent facts) under the filter.
func (s *Server) loadSnapshot(ctx context.Context, filter FilterState) (DashboardData, error) {
	now := time.Now()

	data := DashboardData{
		Counts:      make(map[task.Status]int),
		Tasks:       []task.Task{},
		Dead:        []task.Task{},
		Facts:       []journal.Fact{},
		Filter:      filter,
		Now:         now,
		AllowWrites: s.cfg.AllowWrites,
	}

	counts, err := s.store.StatusCounts(ctx)
	if err != nil {
		return data, err
	}

	for st, n := range counts {
		data.Counts[st] += n
		data.Total += n
	}

	projectCounts, err := s.store.ProjectCounts(ctx)
	if err != nil {
		return data, err
	}

	data.Projects = projectSummaries(projectCounts)

	page := max(filter.Page, 1)

	if filter.View == viewBoard {
		// The board replaces the table page: columns carry their own
		// bounded newest-first lists, so the paginated Tasks load (and its
		// verdict lookups) would be wasted work.
		board, err := s.loadBoard(ctx, filter)
		if err != nil {
			return data, err
		}

		data.Board = board

		for _, col := range board {
			data.MatchTotal += col.Count
		}
	} else {
		data.Page = page

		qf := filter.toQueueFilter(0)
		qf.SeverityOrder = true
		qf.Limit = taskTableLimit
		qf.Offset = (page - 1) * taskTableLimit

		tasks, err := s.store.List(ctx, qf)
		if err != nil {
			return data, err
		}

		data.Tasks = tasks

		// Verdicts for the page's finished review tasks (best effort: a failed
		// read renders no badge, never a broken snapshot).
		for _, t := range tasks {
			if t.Type != executor.TaskTypeReview || t.Status != task.Completed {
				continue
			}

			if res, ok := reviewResultFor(ctx, s.store, t.ID.String()); ok {
				if data.Reviews == nil {
					data.Reviews = map[string]executor.ReviewResult{}
				}

				data.Reviews[t.ID.String()] = res
			}
		}

		// Outcomes for the page's finished status tasks — same best-effort
		// contract as the review verdicts above.
		for _, t := range tasks {
			if t.Type != executor.TaskTypeStatus || t.Status != task.Completed {
				continue
			}

			if res, ok := statusResultFor(ctx, s.store, t.ID.String()); ok {
				if data.Statuses == nil {
					data.Statuses = map[string]executor.StatusResult{}
				}

				data.Statuses[t.ID.String()] = res
			}
		}

		matches, err := s.store.CountTasks(ctx, filter.toQueueFilter(0))
		if err != nil {
			return data, err
		}

		data.MatchTotal = matches
		data.TotalPages = max(1, (matches+taskTableLimit-1)/taskTableLimit)
	}

	if s.cfg.DailyBudget > 0 {
		spent, err := s.store.CountFacts(ctx, journal.Enqueued, startOfDay(now))
		if err != nil {
			return data, err
		}

		data.Budget = &BudgetView{Cap: s.cfg.DailyBudget, Spent: int(spent)}
	}

	dead := task.Dead

	deadTasks, err := s.store.List(ctx, queue.Filter{Status: &dead})
	if err != nil {
		return data, err
	}

	data.Dead = deadTasks

	facts, err := s.store.LastFacts(ctx, factFeedLen)
	if err != nil {
		return data, err
	}

	data.Facts = facts

	// Journal watermark (best effort: a failed read renders 0, the page
	// still works).
	if seq, err := s.store.HeadSeq(ctx); err == nil {
		data.JournalSeq = seq
	}

	data.FactBuckets = factBuckets(data.Now, facts, 12, time.Hour)

	if completed := recentCompletedDurations(ctx, s.store, 200); len(completed) > 0 {
		data.CompleteMinutes = completed
	}

	return data, nil
}

// loadBoard builds the board view's lifecycle columns under the filter's
// project/query scope (a status filter is meaningless when columns ARE the
// statuses — parseFilter drops it): the true count per status plus the
// newest boardColumnLimit cards, oldest truncated with an escape hatch
// into the status-filtered table view.
func (s *Server) loadBoard(ctx context.Context, filter FilterState) ([]BoardColumn, error) {
	columns := make([]BoardColumn, 0, len(allStatuses))

	for _, st := range allStatuses {
		qf := filter.toQueueFilter(0)
		qf.Status = &st

		count, err := s.store.CountTasks(ctx, qf)
		if err != nil {
			return nil, err
		}

		qf.Limit = boardColumnLimit
		qf.Sort = "age-desc" // newest first, like a kanban column

		tasks, err := s.store.List(ctx, qf)
		if err != nil {
			return nil, err
		}

		columns = append(columns, BoardColumn{
			Status:    st,
			Count:     count,
			Tasks:     tasks,
			Truncated: max(count-len(tasks), 0),
		})
	}

	return columns, nil
}

// factBuckets counts facts per equal slice of window ending at now (the
// activity sparkline). Facts older than the window are ignored.
func factBuckets(now time.Time, facts []journal.Fact, n int, window time.Duration) []float64 {
	buckets := make([]float64, n)
	if n <= 0 {
		return buckets
	}

	start := now.Add(-window)
	slice := window / time.Duration(n)

	for _, f := range facts {
		if f.Time.Before(start) {
			continue
		}

		idx := int(now.Sub(f.Time) / slice)
		if idx >= n {
			idx = n - 1
		}

		buckets[n-1-idx]++ // oldest bucket first, like the chart's X axis
	}

	return buckets
}

// recentCompletedDurations returns time-to-complete (CompletedAt minus
// CreatedAt: queue wait + execution, honestly labeled) in minutes for the
// newest n completed tasks.
func recentCompletedDurations(ctx context.Context, store queue.Store, n int) []float64 {
	completed := task.Completed

	tasks, err := store.List(ctx, queue.Filter{Status: &completed, Limit: n, Sort: "age-desc"})
	if err != nil {
		return nil
	}

	var out []float64

	for _, t := range tasks {
		if t.CompletedAt == nil {
			continue
		}

		mins := t.CompletedAt.Sub(t.CreatedAt).Minutes()
		if mins < 0 {
			continue
		}

		out = append(out, mins)
	}

	return out
}

// toQueueFilter maps the URL-carried filter onto the store's SQL filter,
// bounded to limit rows (0 = unbounded).
func (f FilterState) toQueueFilter(limit int) queue.Filter {
	qf := queue.Filter{Query: f.Query, Limit: limit, Sort: f.Sort}

	if f.Project != "" {
		qf.Project = &f.Project
	}

	if f.Status != "" {
		qf.Status = &f.Status
	}

	return qf
}

func projectSummaries(counts map[string]map[task.Status]int) []ProjectSummary {
	out := make([]ProjectSummary, 0, len(counts))

	for name, byStatus := range counts {
		display := name
		if display == "" {
			display = "(default)"
		}

		out = append(out, ProjectSummary{
			Name:    display,
			Pending: byStatus[task.Pending],
			Running: byStatus[task.Running],
			Dead:    byStatus[task.Dead],
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })

	return out
}

func pageTitle(data DashboardData) string {
	base := "tq"
	if r := data.Counts[task.Running]; r > 0 {
		return fmt.Sprintf("%s — %d running", base, r)
	}

	if d := data.Counts[task.Dead]; d > 0 {
		return fmt.Sprintf("%s — %d dead", base, d)
	}

	return base
}

// detailPageTitle is the detail page's static document title.
func detailPageTitle(id string) string {
	return id + " · tq"
}

func formatInt(n int) string {
	return strconv.Itoa(n)
}

func truncate(s string, limit int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= limit {
		return s
	}

	return s[:limit-1] + "…"
}

// durationUntil renders a coarse humanized forward duration (waits).
func durationUntil(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < hoursPerDay:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/hoursPerDay.Hours()))
	}
}

// timeAgo renders a coarse humanized duration between now and t.
func timeAgo(now, t time.Time) string {
	return durationUntil(max(now.Sub(t), 0))
}

// startOfDay truncates to local midnight (same semantics as the budget
// guard's calendar day).
func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()

	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// readiness describes when a pending task becomes claimable.
func readiness(now time.Time, t task.Task) string {
	if t.Status != task.Pending || t.NotBefore.IsZero() {
		return ""
	}

	d := t.NotBefore.Sub(now)
	if d <= 0 {
		return "ready"
	}

	return "in " + durationUntil(d)
}

// renderComponent renders a templ component to an HTML string.
func renderComponent(ctx context.Context, c templ.Component) string {
	var buf bytes.Buffer

	if err := c.Render(ctx, &buf); err != nil {
		return "<p class=\"err\">render error: " + truncate(err.Error(), renderErrPreviewLen) + "</p>"
	}

	return buf.String()
}

// fragment is one named HTML patch the SSE client swaps by container id.
type fragment struct {
	ID   string `json:"id"`
	HTML string `json:"html"`
}

// Badge class shared with the status CSS classes.
const badgePending = "pending"

// Fragment container ids shared by the page layout, the SSE payloads and
// the client JS.
const (
	fragStats   = "frag-stats"
	fragFilters = "frag-filters"
	fragTable   = "frag-table"
	fragDLQ     = "frag-dlq"
	fragFeed    = "frag-feed"

	fragDetail   = "frag-detail"
	fragTimeline = "frag-timeline"
)

// renderFragments renders every dashboard fragment from the snapshot.
// The #frag-table container carries whichever task projection the filter
// selects: the ledger table or the board — the SSE swap is view-blind.
func renderFragments(ctx context.Context, data DashboardData) []fragment {
	tasks := TaskTable(data)
	if data.Filter.View == viewBoard {
		tasks = Board(data)
	}

	return []fragment{
		{ID: fragStats, HTML: renderComponent(ctx, StatusCards(data))},
		{ID: fragFilters, HTML: renderComponent(ctx, FilterBar(data))},
		{ID: fragTable, HTML: renderComponent(ctx, tasks)},
		{ID: fragDLQ, HTML: renderComponent(ctx, DeadLetterTable(data))},
		{ID: fragFeed, HTML: renderComponent(ctx, FactFeed(data))},
	}
}

// renderTaskFragments renders the task detail page's live fragments: the
// record card and the fact timeline.
func renderTaskFragments(ctx context.Context, data DashboardData, t task.Task, facts []journal.Fact) []fragment {
	return []fragment{
		{ID: fragDetail, HTML: renderComponent(ctx, taskDetailCard(data, t))},
		{ID: fragTimeline, HTML: renderComponent(ctx, taskDetailTimeline(data, facts))},
	}
}

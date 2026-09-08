package webui

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
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
		Counts: make(map[task.Status]int),
		Tasks:  []task.Task{},
		Dead:   []task.Task{},
		Facts:  []journal.Fact{},
		Filter: filter,
		Now:    now,
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

	page := filter.Page
	if page < 1 {
		page = 1
	}

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

	matches, err := s.store.CountTasks(ctx, filter.toQueueFilter(0))
	if err != nil {
		return data, err
	}

	data.MatchTotal = matches
	data.TotalPages = max(1, (matches+taskTableLimit-1)/taskTableLimit)

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

	return data, nil
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
func renderFragments(ctx context.Context, data DashboardData) []fragment {
	return []fragment{
		{ID: fragStats, HTML: renderComponent(ctx, StatusCards(data))},
		{ID: fragFilters, HTML: renderComponent(ctx, FilterBar(data))},
		{ID: fragTable, HTML: renderComponent(ctx, TaskTable(data))},
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

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
)

const (
	timeFormat          = "2006-01-02 15:04:05 MST"
	errorPreviewLen     = 60
	factFeedLen         = 50
	taskTableLimit      = 200
	detailFactsLimit    = 500
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
type FilterState struct {
	Project string
	Status  task.Status
	Query   string
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

	s := b.String()

	return strings.TrimSuffix(s, "&")
}

// DashboardData is the full projection snapshot one burst renders from.
type DashboardData struct {
	Counts   map[task.Status]int
	Total    int
	Tasks    []task.Task
	Dead     []task.Task
	Facts    []journal.Fact
	Filter   FilterState
	Now      time.Time
	Projects []ProjectSummary
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

	all, err := s.store.List(ctx, queue.Filter{})
	if err != nil {
		return data, err
	}

	data.Total = len(all)

	for _, t := range all {
		data.Counts[t.Status]++

		if matchesFilter(t, filter) {
			data.Tasks = append(data.Tasks, t)
		}

		if t.Status == task.Dead {
			data.Dead = append(data.Dead, t)
		}
	}

	data.Projects = projectSummaries(all)
	data.Tasks = sortTasks(data.Tasks)

	if len(data.Tasks) > taskTableLimit {
		data.Tasks = data.Tasks[:taskTableLimit]
	}

	facts, err := s.store.Facts(ctx, 0)
	if err != nil {
		return data, err
	}

	if len(facts) > factFeedLen {
		facts = facts[len(facts)-factFeedLen:]
	}

	data.Facts = facts

	return data, nil
}

func matchesFilter(t task.Task, f FilterState) bool {
	if f.Project != "" && t.Project != f.Project {
		return false
	}

	if f.Status != "" && t.Status != f.Status {
		return false
	}

	if f.Query != "" && !matchesQuery(t, f.Query) {
		return false
	}

	return true
}

func matchesQuery(t task.Task, q string) bool {
	q = strings.ToLower(q)

	hay := []string{
		t.ID.String(),
		t.Type,
		t.Project,
		string(t.Payload),
		t.LeaseOwner,
		t.LastError,
	}

	for _, h := range hay {
		if strings.Contains(strings.ToLower(h), q) {
			return true
		}
	}

	return false
}

// sortTasks orders by status severity (dead, running, pending, cancelled,
// completed), then age descending (newest first within a status).
func sortTasks(tasks []task.Task) []task.Task {
	const (
		rankDead = iota
		rankRunning
		rankPending
		rankCancelled
		rankCompleted
	)

	rank := map[task.Status]int{
		task.Dead:      rankDead,
		task.Running:   rankRunning,
		task.Pending:   rankPending,
		task.Cancelled: rankCancelled,
		task.Completed: rankCompleted,
	}

	sorted := append([]task.Task(nil), tasks...)

	sort.SliceStable(sorted, func(i, j int) bool {
		ri, rj := rank[sorted[i].Status], rank[sorted[j].Status]
		if ri != rj {
			return ri < rj
		}

		return sorted[i].CreatedAt.After(sorted[j].CreatedAt)
	})

	return sorted
}

func projectSummaries(all []task.Task) []ProjectSummary {
	byName := map[string]*ProjectSummary{}

	for _, t := range all {
		name := t.Project
		if name == "" {
			name = "(default)"
		}

		p, ok := byName[name]
		if !ok {
			p = &ProjectSummary{Name: name}
			byName[name] = p
		}

		switch t.Status {
		case task.Pending:
			p.Pending++
		case task.Running:
			p.Running++
		case task.Dead:
			p.Dead++
		case task.Completed, task.Cancelled:
		}
	}

	out := make([]ProjectSummary, 0, len(byName))
	for _, p := range byName {
		out = append(out, *p)
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

// timeAgo renders a coarse humanized duration between now and t.
func timeAgo(now, t time.Time) string {
	d := max(now.Sub(t), 0)

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

// factBadgeClass maps a fact type to a status-like CSS badge class.
func factBadgeClass(t journal.FactType) string {
	switch t {
	case journal.Completed:
		return "completed"
	case journal.DeadLettered:
		return "dead"
	case journal.Failed:
		return "failed"
	case journal.Cancelled:
		return "cancelled"
	case journal.Claimed:
		return "running"
	case journal.Enqueued, journal.Heartbeat, journal.Released, journal.Requeued:
		return badgePending
	}

	return badgePending
}

// renderComponent renders a templ component to an HTML string.
func renderComponent(ctx context.Context, c templ.Component) string {
	var buf bytes.Buffer

	if err := c.Render(ctx, &buf); err != nil {
		return "<p class=\"err\">render error: " + truncate(err.Error(), 120) + "</p>"
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

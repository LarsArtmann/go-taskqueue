package webui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/task"
	"github.com/larsartmann/templ-components/display"
	"github.com/larsartmann/templ-components/layout"
)

// This file maps domain vocabulary (task.Status, journal.FactType) onto the
// templ-components visual language. It is the single place where the queue's
// status colors are decided — badges, stat cards, and the journal pane all
// read from these maps so the color language stays coherent.

// statusStyle is ONE status's visual vocabulary: the badge language AND
// the board accent read from this single table, so the two renderers can
// never drift apart (00:21 report e5: two hand-synced maps).
type statusStyle struct {
	badge  display.BadgeType
	accent string
}

// statusColorTable is the single source of truth for status colors.
// Hue vocabulary: amber=pending, cyan-family blue=running, green=completed,
// red=dead, gray=cancelled (theme.css remaps the blue ramp onto signal
// cyan).
var statusColorTable = map[task.Status]statusStyle{
	task.Pending:   {badge: display.BadgeWarning, accent: "border-amber-400 dark:border-amber-500"},
	task.Running:   {badge: display.BadgeInfo, accent: "border-blue-400 dark:border-blue-500"},
	task.Completed: {badge: display.BadgeSuccess, accent: "border-green-500 dark:border-green-400"},
	task.Dead:      {badge: display.BadgeError, accent: "border-red-500 dark:border-red-400"},
	task.Cancelled: {badge: display.BadgeNeutral, accent: "border-gray-300 dark:border-gray-700"},
}

// statusStyleFallback is the look for unknown statuses (defensive: the
// enum is closed, but a zero value must still render sanely).
var statusStyleFallback = statusStyle{
	badge:  display.BadgeNeutral,
	accent: "border-gray-300 dark:border-gray-700",
}

func statusStyleFor(st task.Status) statusStyle {
	if s, ok := statusColorTable[st]; ok {
		return s
	}

	return statusStyleFallback
}

// statusBadgeType maps a task status onto the library badge language.
func statusBadgeType(st task.Status) display.BadgeType {
	return statusStyleFor(st).badge
}

// statusAccentClass is the board column's status-colored top rule — the
// same hue vocabulary as the badges and the stat cards.
func statusAccentClass(st task.Status) string {
	return statusStyleFor(st).accent
}

// verdictBadgeType maps an agent-review verdict onto the badge language:
// approve is green, request_changes is red — the review loop's pass/fail.
func verdictBadgeType(v executor.ReviewVerdict) display.BadgeType {
	if v == executor.VerdictApprove {
		return display.BadgeSuccess
	}

	return display.BadgeError
}

// verdictLabel is the human text of a verdict badge.
func verdictLabel(v executor.ReviewVerdict) string {
	if v == executor.VerdictApprove {
		return "review: approve"
	}

	return "review: request changes"
}

// statusBadgeText is the human text of a status-report badge; the next-item
// count is the loop's heartbeat (the fresh work the report minted).
func statusBadgeText(res executor.StatusResult) string {
	if res.NextItems > 0 {
		return fmt.Sprintf("status: report +%d next", res.NextItems)
	}

	return "status: report"
}

// findingSeverityType maps a review finding's severity hint onto the badge
// language (normalized to low/medium/high by the executor's parser).
func findingSeverityType(severity string) display.BadgeType {
	switch severity {
	case "high":
		return display.BadgeError
	case "low":
		return display.BadgeNeutral
	default:
		return display.BadgeWarning
	}
}

// factTone maps a fact type onto the journal pane's tone colors.
func factTone(t journal.FactType) display.ScrollbackTone {
	switch t {
	case journal.Claimed:
		return display.ScrollbackToneInfo
	case journal.Completed:
		return display.ScrollbackToneSuccess
	case journal.Failed:
		return display.ScrollbackToneWarning
	case journal.DeadLettered:
		return display.ScrollbackToneDanger
	case journal.Released, journal.Requeued:
		return display.ScrollbackToneWarning
	case journal.CancelRequested:
		return display.ScrollbackToneWarning
	case journal.Enqueued, journal.Heartbeat, journal.Cancelled:
		return display.ScrollbackToneNeutral
	}

	return display.ScrollbackToneNeutral
}

// factLines renders the journal pane's terminal lines: one per fact, newest
// last (a tail, like `tq tail -f`).
func factLines(data DashboardData) []display.ScrollbackLine {
	lines := make([]display.ScrollbackLine, 0, len(data.Facts))

	for _, fact := range data.Facts {
		text := "#" + formatInt(int(fact.Seq)) + " " + shortIDTail(fact.TaskID)
		if fact.Error != "" {
			text += " " + truncate(fact.Error, errorPreviewLen)
		}

		lines = append(lines, display.ScrollbackLine{
			Timestamp: factTimestamp(data.Now, fact.Time),
			Tag:       string(fact.Type),
			Text:      text,
			Tone:      factTone(fact.Type),
		})
	}

	return lines
}

// factTimestamp renders a wall-clock time; facts older than a day gain a
// date prefix so the tail stays unambiguous.
func factTimestamp(now, t time.Time) string {
	if now.Sub(t) > hoursPerDay {
		return t.Format("01-02 15:04:05")
	}

	return t.Format("15:04:05")
}

// Shared column/label vocabulary (table headers, stat labels, detail terms).
const (
	labelProject   = "project"
	labelType      = "type"
	labelStatus    = "status"
	labelAttempts  = "attempts"
	labelReady     = "ready"
	labelAge       = "age"
	labelError     = "last error"
	labelID        = "id"
	labelBudget    = "budget today"
	labelCompleted = "completed"
	labelTotal     = "total"
)

// Payload intentionally stays OUT of the record's definition list: it is
// the task's content, not its metadata, and it renders as its own type-aware
// section (payloadSection in fragments.templ) below this card.
func detailItems(t task.Task, now time.Time) []display.DefinitionItem {
	items := []display.DefinitionItem{
		{Term: labelProject, Detail: t.Project},
		{Term: labelType, Detail: t.Type},
		{Term: labelStatus, DetailComponent: statusBadge(string(t.Status), statusBadgeType(t.Status))},
		{Term: labelAttempts, Detail: formatInt(t.Attempts) + "/" + formatInt(t.MaxAttempts)},
		{Term: "priority", Detail: formatInt(t.Priority)},
		{Term: "created", Detail: timeAgo(now, t.CreatedAt) + " ago"},
		{Term: "updated", Detail: timeAgo(now, t.UpdatedAt) + " ago"},
	}

	if t.LeaseOwner != "" {
		items = append(items, display.DefinitionItem{Term: "lease owner", Detail: t.LeaseOwner})
	}

	if t.CompletedAt != nil {
		items = append(items, display.DefinitionItem{
			Term:   labelCompleted,
			Detail: timeAgo(now, *t.CompletedAt) + " ago",
		})
	}

	return items
}

// detailFacts renders the detail page's fact timeline as journal lines.
// Cancel facts carry their reason in the detail JSON — surfaced inline so a
// withdrawn task's trail answers "why" without reading the raw journal.
func detailFacts(now time.Time, facts []journalFactView) []display.ScrollbackLine {
	lines := make([]display.ScrollbackLine, 0, len(facts))

	for _, fact := range facts {
		text := "#" + formatInt(int(fact.Seq))
		if fact.Owner != "" {
			text += " " + fact.Owner
		}

		// Error and Detail.reason often carry the SAME text (a requeue
		// refusal is stored in both fields); repeating it reads like the
		// journal stuttered. factLineText merges them into one line.
		if line := factLineText(fact); line != "" {
			text += " " + truncate(line, errorPreviewLen)
		}

		if fact.Attempt > 0 {
			text += fmt.Sprintf(" (attempt %d)", fact.Attempt)
		}

		lines = append(lines, display.ScrollbackLine{
			Timestamp: factTimestamp(now, fact.Time),
			Tag:       string(fact.Type),
			Text:      text,
			Tone:      factTone(fact.Type),
		})
	}

	return lines
}

// factLineText merges a fact's Error and its Detail reason into ONE honest
// line: identical texts collapse to the fuller one, genuinely different
// texts keep the classic "error — reason" pair.
func factLineText(fact journalFactView) string {
	reason := factReason(fact)

	switch {
	case reason == "":
		return fact.Error
	case fact.Error == "":
		return reason
	case strings.Contains(reason, fact.Error):
		return reason
	case strings.Contains(fact.Error, reason):
		return fact.Error
	default:
		return fact.Error + " — " + reason
	}
}

// factReason extracts the human cancellation reason from a fact's detail
// JSON (the "reason" key tq cancel --reason and the reclaim finalize write).
// Empty when the fact carries none.
func factReason(fact journalFactView) string {
	if len(fact.Detail) == 0 {
		return ""
	}

	var detail struct {
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(fact.Detail, &detail); err != nil {
		return ""
	}

	return detail.Reason
}

// dashboardProps builds the shared page shell for both pages. HTMX is
// suppressed entirely: the dashboard streams over vanilla EventSource.
// The nonce comes from the security-headers middleware (ctxNonce) and is
// stamped onto the theme bootstrap + toggle inline scripts so the strict
// script-src CSP admits them.
func dashboardProps(title, nonce string) layout.PageProps {
	props := layout.DefaultPageProps()
	props.Title = title
	props.Description = "Live, read-only projection of the tq task-queue journal."
	props.CSSPath = "/static/app.css"
	props.Favicon = "/static/favicon.svg"
	props.HTMXVersion = ""
	props.HTMXSrc = ""
	props.HTMXResponseTargets = false
	props.ThemeColor = "#f6f8fb"
	props.DarkThemeColor = "#0a0f1a"
	props.HeadContent = refreshMeta()
	props.Footer = pageFooter()
	props.Nonce = nonce

	return props
}

// shortID renders a display form of a task id: the random tail (ids are
// time-prefixed, so the leading chars are shared by same-session tasks).
// Full id stays on the title tooltip and the detail page.
func shortID(id task.ID) string {
	return shortIDTail(id.String())
}

func shortIDTail(s string) string {
	if len(s) <= shortIDLen {
		return s
	}

	return "…" + s[len(s)-shortIDLen:]
}

const shortIDLen = 8

// factToneClass mirrors the library Scrollback's tag colors (its
// scrollbackToneClass is unexported) for the linked fact feed variant.
func factToneClass(tone display.ScrollbackTone) string {
	switch tone {
	case display.ScrollbackToneInfo:
		return "text-blue-600 dark:text-blue-400"
	case display.ScrollbackToneSuccess:
		return "text-green-600 dark:text-green-400"
	case display.ScrollbackToneWarning:
		return "text-amber-600 dark:text-amber-400"
	case display.ScrollbackToneDanger:
		return "text-red-600 dark:text-red-400"
	default:
		return "text-gray-500 dark:text-gray-400"
	}
}

// statusHref links a stat card to its filtered view.
func statusHref(st task.Status) string {
	return filterHref(FilterState{Status: st})
}

// sortableColumns is the task-table's sort vocabulary: column key -> the
// URL sort values it cycles through.
var sortableColumns = map[string][]string{
	"age":      {"", "age-desc", "age-asc"},
	"priority": {"", "priority-desc", "priority-asc"},
	"attempts": {"", "attempts-desc", "attempts-asc"},
}

// sortHeaderDirection reports the current SortDirection shown on a column
// (asc/desc when the active sort belongs to it, none otherwise).
func sortHeaderDirection(f FilterState, column string) display.SortDirection {
	for _, v := range sortableColumns[column] {
		if v == f.Sort {
			if strings.HasSuffix(v, "-asc") {
				return display.SortAsc
			}

			return display.SortDesc
		}
	}

	return display.SortNone
}

// sortHeaderHref cycles a column's sort: none -> descending -> ascending ->
// none (back to the severity order). Clicking a column while ANOTHER
// column's sort is active starts this column's own cycle (descending
// first). The href keeps the current filter.
func sortHeaderHref(filter FilterState, column string) string {
	cycle := sortableColumns[column]

	// Default: activate the column's first meaningful sort (descending).
	next := cycle[1]

	for i, v := range cycle {
		if v == filter.Sort {
			if i+1 < len(cycle) {
				next = cycle[i+1]
			} else {
				next = cycle[0]
			}

			break
		}
	}

	toggled := filter
	toggled.Sort = next
	toggled.Page = 1

	return filterHref(toggled)
}

// taskRowClass carries the row's status marker plus hover affordance; dead
// rows get a faint alarm tint.
func taskRowClass(t task.Task) string {
	class := "row-" + string(t.Status) + " hover:bg-gray-100 dark:hover:bg-gray-800"
	if t.Status == task.Dead {
		class += " bg-red-50/50 dark:bg-red-950/20"
	}

	return class
}

// inputClass is the shared text-control style for the filter bar.
const inputClass = "rounded-lg border border-gray-300 bg-white px-3 py-1.5 text-sm text-gray-900 " +
	"placeholder:text-gray-400 focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500/30 " +
	"dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100 dark:placeholder:text-gray-500"

// tdCellClass is the table body cell style; nowrap keeps the ledger dense.
func tdCellClass(wrap bool) string {
	const base = "px-4 py-2 text-gray-700 dark:text-gray-300"
	if wrap {
		return base
	}

	return base + " whitespace-nowrap"
}

// completeHistogram buckets recent time-to-complete values (minutes) into
// power-of-two-ish buckets: 0-1m, 1-5m, 5-15m, 15-60m, 60m+. The counts
// form the completion histogram's series.
func completeHistogram(data DashboardData) []float64 {
	buckets := make([]float64, 5)

	for _, m := range data.CompleteMinutes {
		switch {
		case m < 1:
			buckets[0]++
		case m < 5:
			buckets[1]++
		case m < 15:
			buckets[2]++
		case m < 60:
			buckets[3]++
		default:
			buckets[4]++
		}
	}

	return buckets
}

func completeHistogramLabels(data DashboardData) []string {
	if len(data.CompleteMinutes) == 0 {
		return nil
	}

	return []string{"<1m", "1-5m", "5-15m", "15-60m", "60m+"}
}

// splitTasks partitions the visible page into active (queued or running)
// and settled (terminal) rows for the two-tier table. The page's own sort
// order is preserved inside each tier.
func splitTasks(rows []task.Task) (active, settled []task.Task) {
	for _, t := range rows {
		if t.Status == task.Completed || t.Status == task.Cancelled || t.Status == task.Dead {
			settled = append(settled, t)
		} else {
			active = append(active, t)
		}
	}

	return active, settled
}

func activeTasks(rows []task.Task) []task.Task {
	active, _ := splitTasks(rows)

	return active
}

func settledTasks(rows []task.Task) []task.Task {
	_, settled := splitTasks(rows)

	return settled
}

func countStatus(rows []task.Task, st task.Status) int {
	count := 0

	for _, t := range rows {
		if t.Status == st {
			count++
		}
	}

	return count
}

// settledDeadSuffix renders the summary's dead clause only when dead tasks
// are on the page (" · 2 dead") — the DLQ section stays the dead hub.
func settledDeadSuffix(rows []task.Task) string {
	if n := countStatus(rows, task.Dead); n > 0 {
		return fmt.Sprintf(" · %d dead", n)
	}

	return ""
}

// taskHeaders builds the task table's header row; the actions column exists
// only when writes are enabled, so header and body cells always agree.
func taskHeaders(data DashboardData) []display.TableHeader {
	headers := []display.TableHeader{
		{Label: labelID},
		{Label: labelProject},
		{Label: labelType},
		{Label: labelStatus},
	}
	if data.AllowWrites {
		// Actions sit right after status so they stay inside the visible
		// card width; the table tail (age, error) is the clip-prone end.
		headers = append(headers, display.TableHeader{Label: "actions"})
	}

	headers = append(
		headers,
		display.TableHeader{
			Label:         labelAttempts,
			Sortable:      true,
			SortDirection: sortHeaderDirection(data.Filter, "attempts"),
			Href:          sortHeaderHref(data.Filter, "attempts"),
		},
		display.TableHeader{Label: labelReady},
		display.TableHeader{
			Label:         labelAge,
			Sortable:      true,
			SortDirection: sortHeaderDirection(data.Filter, "age"),
			Href:          sortHeaderHref(data.Filter, "age"),
		},
		display.TableHeader{Label: labelError},
	)

	return headers
}

// reasonPlaceholder keeps the cancel form honest: a running agent deserves
// a stop request, a queued one a withdrawal — both want a why.
func reasonPlaceholder(running bool) string {
	if running {
		return "why stop it? (stored in the fact)"
	}

	return "why cancel it? (stored in the fact)"
}

package webui

import (
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/task"
	"github.com/larsartmann/templ-components/display"
	"github.com/larsartmann/templ-components/layout"
)

// This file maps domain vocabulary (task.Status, journal.FactType) onto the
// templ-components visual language. It is the single place where the queue's
// status colors are decided — badges, stat cards, and the journal pane all
// read from these maps so the color language stays coherent.

// statusBadgeType maps a task status onto the library badge language.
func statusBadgeType(st task.Status) display.BadgeType {
	switch st {
	case task.Pending:
		return display.BadgeWarning
	case task.Running:
		return display.BadgeInfo
	case task.Completed:
		return display.BadgeSuccess
	case task.Dead:
		return display.BadgeError
	case task.Cancelled:
		return display.BadgeNeutral
	}

	return display.BadgeNeutral
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
	case journal.Enqueued, journal.Heartbeat, journal.Cancelled:
		return display.ScrollbackToneNeutral
	}

	return display.ScrollbackToneNeutral
}

// factLines renders the journal pane's terminal lines: one per fact, newest
// last (a tail, like `tq tail -f`).
func factLines(data DashboardData) []display.ScrollbackLine {
	lines := make([]display.ScrollbackLine, 0, len(data.Facts))

	for _, f := range data.Facts {
		text := "#" + formatInt(int(f.Seq)) + " " + shortIDTail(f.TaskID)
		if f.Error != "" {
			text += " " + truncate(f.Error, errorPreviewLen)
		}

		lines = append(lines, display.ScrollbackLine{
			Timestamp: factTimestamp(data.Now, f.Time),
			Tag:       string(f.Type),
			Text:      text,
			Tone:      factTone(f.Type),
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
	labelProject  = "project"
	labelType     = "type"
	labelStatus   = "status"
	labelAttempts = "attempts"
	labelAge      = "age"
	labelError    = "last error"
	labelID       = "id"
	labelPending  = "pending"
	labelRunning  = "running"
	labelTotal    = "total"
)

const labelCompleted = "completed"

// detailItems builds the task detail page's definition list.
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
		items = append(items, display.DefinitionItem{Term: labelCompleted, Detail: timeAgo(now, *t.CompletedAt) + " ago"})
	}

	items = append(items, display.DefinitionItem{Term: "payload", DetailComponent: payloadCode(string(t.Payload))})

	return items
}

// detailFacts renders the detail page's fact timeline as journal lines.
func detailFacts(now time.Time, facts []journalFactView) []display.ScrollbackLine {
	lines := make([]display.ScrollbackLine, 0, len(facts))

	for _, fv := range facts {
		text := "#" + formatInt(int(fv.Seq))
		if fv.Owner != "" {
			text += " " + fv.Owner
		}

		if fv.Error != "" {
			text += " " + truncate(fv.Error, errorPreviewLen)
		}

		lines = append(lines, display.ScrollbackLine{
			Timestamp: factTimestamp(now, fv.Time),
			Tag:       string(fv.Type),
			Text:      text,
			Tone:      factTone(fv.Type),
		})
	}

	return lines
}

// dashboardProps builds the shared page shell for both pages. HTMX is
// suppressed entirely: the dashboard streams over vanilla EventSource.
func dashboardProps(title string) layout.PageProps {
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

// statusHref links a stat card to its filtered view.
func statusHref(st task.Status) string {
	return filterHref(FilterState{Status: st})
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

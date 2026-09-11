package webui

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// This file projects a task payload for the detail page. The payload is the
// task's CONTENT — for agent tasks the harvested work item plus the prompt
// contract — and it used to render as one break-all JSON line inside the
// record's definition list, which made the most important thing on the page
// the least readable. The projection is type-aware: it parses with the
// executor's own payload structs (the single source of the contracts), leads
// with the work item, and always keeps the raw text one click away.

// payloadKind is the closed set of payload renderings. Anything unrecognized
// — unknown type, or a known type whose JSON no longer parses — degrades to
// payloadRaw instead of guessing: a structured view that fabricates fields
// is worse than an honest blob.
type payloadKind string

const (
	payloadAgent  payloadKind = "agent"
	payloadReview payloadKind = "review"
	payloadStatus payloadKind = "status"
	payloadSh     payloadKind = "sh"
	payloadRaw    payloadKind = "raw"
)

// retryReasonLen caps one retry-trail reason; the full text stays on the
// timeline lines below it.
const retryReasonLen = 160

// payloadField is one label/value row of the payload's spec grid.
type payloadField struct {
	Label string
	Value string
	// Href turns the value into a link (a reviewed task's detail page).
	Href string
}

// payloadView is the typed projection of one task payload: the task's actual
// content first (work item, or shell command), the executor contract as a
// spec grid, and the raw text always available. Built once per render; the
// template only branches on Kind.
type payloadView struct {
	Kind payloadKind

	// Lede is the payload's content line: the harvested work item
	// (agent/review) or the shell command (sh). Empty for payloads that
	// live entirely in their fields (status windows).
	Lede string
	// LedeMono renders the lede as a command block (sh) instead of prose.
	LedeMono bool

	// Note is secondary prose (a review's operator focus instructions).
	Note string

	// Fields is the executor contract as label/value rows.
	Fields []payloadField

	// Prompt is the full agent instruction (agent payloads): rendered
	// collapsed — it is boilerplate contract wrapped around the item above.
	Prompt string

	// Window lists the completed task ids a status report covers.
	Window []string

	// Raw is the full payload text, pretty-printed when the payload parses
	// as JSON. Always populated: a structured view that silently drops
	// unknown fields is a lie; this pane cannot lie.
	Raw string
}

// hasRaw reports whether the raw pane adds anything the structured view does
// not already show (a non-JSON sh payload's raw text IS its command).
func (v payloadView) hasRaw() bool {
	return v.Raw != "" && v.Raw != v.Lede
}

// payloadViewFor projects a task's payload for the detail page.
func payloadViewFor(t task.Task) payloadView {
	view := payloadView{Kind: payloadRaw, Raw: prettyJSON(string(t.Payload))}

	switch t.Type {
	case executor.TaskTypeAgent:
		if view.fromAgent(string(t.Payload)) {
			return view
		}
	case executor.TaskTypeReview:
		if view.fromReview(string(t.Payload)) {
			return view
		}
	case executor.TaskTypeStatus:
		if view.fromStatus(string(t.Payload)) {
			return view
		}
	case "sh":
		view.Kind = payloadSh
		view.Lede = executor.CommandFromPayload(t.Payload)
		view.LedeMono = true

		return view
	}

	return view
}

// fromAgent builds the agent projection; reports whether the payload parsed
// as a usable AgentPayload (repo is the contract's required anchor).
func (v *payloadView) fromAgent(raw string) bool {
	var ap executor.AgentPayload
	if err := json.Unmarshal([]byte(raw), &ap); err != nil || ap.Repo == "" {
		return false
	}

	v.Kind = payloadAgent
	v.Fields = append(v.Fields,
		payloadField{Label: "repo", Value: ap.Repo},
		payloadField{Label: "verify gate", Value: cmp.Or(ap.Verify, "auto-detect")},
	)

	// The work item is the task; the prompt is the boilerplate contract
	// around it. Without a harvested item the prompt IS the content, so it
	// takes the lede instead of hiding behind a fold.
	if ap.Item != "" {
		v.Lede = ap.Item
		v.Prompt = ap.Prompt
	} else {
		v.Lede = ap.Prompt
	}

	if ap.Model != "" {
		v.Fields = append(v.Fields, payloadField{Label: "model", Value: ap.Model})
	}

	if ap.Session != "" {
		v.Fields = append(v.Fields, payloadField{Label: "session", Value: ap.Session})
	}

	if ap.Dedup != "" {
		v.Fields = append(v.Fields, payloadField{Label: "dedup key", Value: ap.Dedup})
	}

	if ap.TimeoutMinutes != 0 {
		v.Fields = append(v.Fields, payloadField{Label: "timeout", Value: fmt.Sprintf("%d min", ap.TimeoutMinutes)})
	}

	if ap.RequireClean != nil && !*ap.RequireClean {
		v.Fields = append(v.Fields, payloadField{Label: "clean tree", Value: "not required"})
	}

	if ap.Yolo {
		v.Fields = append(v.Fields, payloadField{Label: "autonomy", Value: "yolo"})
	}

	return true
}

// fromReview builds the review projection (repo is the required anchor).
func (v *payloadView) fromReview(raw string) bool {
	var rp executor.ReviewPayload
	if err := json.Unmarshal([]byte(raw), &rp); err != nil || rp.Repo == "" {
		return false
	}

	v.Kind = payloadReview
	v.Lede = rp.Item
	v.Note = rp.Extra
	v.Fields = append(v.Fields, payloadField{Label: "repo", Value: rp.Repo})

	if rp.ReviewedTask != "" {
		v.Fields = append(v.Fields, payloadField{
			Label: "reviewed task",
			Value: shortIDTail(rp.ReviewedTask),
			Href:  "/task/" + rp.ReviewedTask,
		})
	}

	if rp.CommitSHA != "" {
		v.Fields = append(v.Fields, payloadField{Label: "commit", Value: rp.CommitSHA})
	}

	if len(rp.FilesChanged) > 0 {
		v.Fields = append(v.Fields, payloadField{Label: "files changed", Value: strings.Join(rp.FilesChanged, ", ")})
	}

	if rp.Model != "" {
		v.Fields = append(v.Fields, payloadField{Label: "model", Value: rp.Model})
	}

	if rp.TimeoutMinutes != 0 {
		v.Fields = append(v.Fields, payloadField{Label: "timeout", Value: fmt.Sprintf("%d min", rp.TimeoutMinutes)})
	}

	return true
}

// fromStatus builds the status projection (repo is the required anchor).
func (v *payloadView) fromStatus(raw string) bool {
	var sp executor.StatusPayload
	if err := json.Unmarshal([]byte(raw), &sp); err != nil || sp.Repo == "" {
		return false
	}

	v.Kind = payloadStatus
	v.Fields = append(v.Fields,
		payloadField{Label: "repo", Value: sp.Repo},
		payloadField{Label: "project", Value: sp.Project},
	)

	if len(sp.Completed) > 0 {
		v.Lede = fmt.Sprintf("reporting window: %s", formatInt(len(sp.Completed))+" completed "+plural(len(sp.Completed), "task", "tasks"))

		for _, c := range sp.Completed {
			v.Window = append(v.Window, c.TaskID)
		}
	}

	if sp.Verify != "" {
		v.Fields = append(v.Fields, payloadField{Label: "verify gate", Value: sp.Verify})
	}

	if sp.Model != "" {
		v.Fields = append(v.Fields, payloadField{Label: "model", Value: sp.Model})
	}

	if sp.TimeoutMinutes != 0 {
		v.Fields = append(v.Fields, payloadField{Label: "timeout", Value: fmt.Sprintf("%d min", sp.TimeoutMinutes)})
	}

	return true
}

// prettyJSON pretty-prints s when it parses as a JSON object or array;
// anything else (a raw shell line, a bare JSON string) passes through
// trimmed and untouched.
func prettyJSON(s string) string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" || !(strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[")) {
		return trimmed
	}

	if !json.Valid([]byte(trimmed)) {
		return trimmed
	}

	var out bytes.Buffer
	if err := json.Indent(&out, []byte(trimmed), "", "  "); err != nil {
		return trimmed
	}

	return out.String()
}

// plural picks singular/plural (tiny helper; the count is already rendered).
func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}

	return many
}

// retryReason aggregates one distinct requeue/failure reason: how often the
// task came back and what it said last time.
type retryReason struct {
	Count  int
	Reason string
	Last   time.Time
}

// retryTrail aggregates a task's refusals and failures (task.requeued,
// task.failed, task.released) into distinct reasons — the one-glance answer
// to "why does this keep coming back?" above the full, faithful trail. Nil
// until there is something to summarize: a single occurrence is already
// readable in the timeline.
func retryTrail(facts []journalFactView) []retryReason {
	var (
		order []string
		byKey = map[string]*retryReason{}
		total int
	)

	for _, f := range facts {
		switch f.Type {
		case journal.Requeued, journal.Failed, journal.Released:
		default:
			continue
		}

		reason := f.Error
		if r := factReason(f); r != "" && (reason == "" || strings.Contains(r, reason)) {
			reason = r
		}

		if reason == "" {
			reason = "(no reason recorded)"
		}

		reason = truncate(reason, retryReasonLen)
		total++

		if existing, ok := byKey[reason]; ok {
			existing.Count++
			existing.Last = f.Time

			continue
		}

		byKey[reason] = &retryReason{Count: 1, Reason: reason, Last: f.Time}
		order = append(order, reason)
	}

	if total < 2 {
		return nil
	}

	trail := make([]retryReason, 0, len(order))
	for _, key := range order {
		trail = append(trail, *byKey[key])
	}

	slices.SortStableFunc(trail, func(a, b retryReason) int {
		if a.Count != b.Count {
			return b.Count - a.Count
		}

		return b.Last.Compare(a.Last)
	})

	return trail
}

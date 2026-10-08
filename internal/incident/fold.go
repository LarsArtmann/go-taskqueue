package incident

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
)

// Status is an incident's derived lifecycle state. It is NEVER stored —
// the fold recomputes it from facts, so "resolved" means "the latest
// minted fix task completed", by construction.
type Status string

const (
	// StatusOpen: observed, no fix task minted yet (or regressed after a
	// terminal fix task — the regression path reopens the incident).
	StatusOpen Status = "open"
	// StatusFixDispatched: a fix task is minted and not terminal.
	StatusFixDispatched Status = "fix-dispatched"
	// StatusResolved: the latest minted fix task completed.
	StatusResolved Status = "resolved"
	// StatusFixFailed: the latest minted fix task dead-lettered.
	StatusFixFailed Status = "fix-failed"
)

// Mint records one fix task minted for an incident, as journaled by the
// incident.task-minted fact.
type Mint struct {
	TaskID     string `json:"taskID"`
	SourceSeq  int64  `json:"sourceSeq"`
	Regression bool   `json:"regression"`
	Priority   int    `json:"priority"`
	// Outcome is the minted task's terminal state folded from its own
	// lifecycle facts: "" while running, "completed", or "dead".
	Outcome string `json:"outcome"`
	// DoneSeq is the Seq of the terminal fact (ordering evidence for the
	// regression rule).
	DoneSeq int64 `json:"doneSeq,omitempty"`
}

// Terminal reports whether the minted fix task reached a terminal state.
func (m Mint) Terminal() bool { return m.Outcome != "" }

// Incident is one fingerprint's folded state — the read model of the
// error.observed / incident.task-minted fact family.
type Incident struct {
	Fingerprint string
	Project     string
	Kind        Kind
	Message     string
	FirstSeen   time.Time
	LastSeen    time.Time
	Occurrences int
	Regressions int
	Status      Status
	Mints       []Mint
}

// State is the fold over the journal: incidents keyed by fingerprint,
// first-appearance order preserved, plus the task→incident mint index.
// A replayed fact (Seq at or below the folded high-water mark) is a no-op,
// so at-least-once delivery never double-counts.
type State struct {
	lastSeq int64
	order   []string
	byFP    map[string]*Incident
	byTask  map[string]string
}

// NewState returns an empty fold.
func NewState() *State {
	return &State{
		byFP:   map[string]*Incident{},
		byTask: map[string]string{},
	}
}

// Apply folds one fact. Returns false when the fact was already folded
// (Seq guard) or does not belong to a known family — both are no-ops.
func (s *State) Apply(f journal.Fact) bool {
	if f.Seq <= s.lastSeq {
		return false
	}

	s.lastSeq = f.Seq

	switch f.Type {
	case journal.ErrorObserved:
		s.applyObserved(f)
	case journal.IncidentTaskMinted:
		s.applyMinted(f)
	case journal.Completed, journal.DeadLettered:
		s.applyTaskTerminal(f)
	}

	return true
}

// LastSeq returns the highest folded fact Seq.
func (s *State) LastSeq() int64 { return s.lastSeq }

// Get returns a copy of one incident's folded state.
func (s *State) Get(fingerprint string) (Incident, bool) {
	inc, ok := s.byFP[fingerprint]
	if !ok {
		return Incident{}, false
	}

	return *inc, true
}

// Incidents returns every folded incident in first-appearance order.
func (s *State) Incidents() []Incident {
	out := make([]Incident, 0, len(s.order))

	for _, fp := range s.order {
		out = append(out, *s.byFP[fp])
	}

	return out
}

// FingerprintOfTask resolves which incident a task was minted for (""
// when the task is not a minted fix task).
func (s *State) FingerprintOfTask(taskID string) string {
	return s.byTask[taskID]
}

func (s *State) incident(fp string) *Incident {
	inc, ok := s.byFP[fp]
	if ok {
		return inc
	}

	inc = &Incident{Fingerprint: fp, Status: StatusOpen}
	s.byFP[fp] = inc
	s.order = append(s.order, fp)

	return inc
}

func (s *State) applyObserved(f journal.Fact) {
	if !strings.HasPrefix(f.TaskID, IDPrefix) {
		return
	}

	fp := FingerprintOfID(f.TaskID)
	inc, existed := s.byFP[fp]
	if !existed {
		inc = s.incident(fp)
	}

	var rep Report
	if len(f.Detail) > 0 {
		_ = json.Unmarshal(f.Detail, &rep)
	}

	inc.Occurrences++

	if rep.Project != "" {
		inc.Project = rep.Project
	}

	if rep.Kind != "" {
		inc.Kind = rep.Kind
	}

	if rep.Message != "" {
		inc.Message = rep.Message
	}

	if inc.FirstSeen.IsZero() {
		inc.FirstSeen = f.Time
	}

	inc.LastSeen = f.Time

	// An observation after the latest fix task reached a terminal state
	// reopens the incident: the fix did not hold (or did not cover this
	// path). The next policy pass mints a regression task at machine
	// band.
	if existed && (inc.Status == StatusResolved || inc.Status == StatusFixFailed) {
		inc.Regressions++
		inc.Status = StatusOpen
	}
}

func (s *State) applyMinted(f journal.Fact) {
	if !strings.HasPrefix(f.TaskID, IDPrefix) {
		return
	}

	var d Mint
	if err := json.Unmarshal(f.Detail, &d); err != nil || d.TaskID == "" {
		return
	}

	fp := FingerprintOfID(f.TaskID)
	inc := s.incident(fp)

	for _, m := range inc.Mints {
		if m.TaskID == d.TaskID {
			return // replayed mint fact: converge
		}
	}

	d.Outcome = ""
	inc.Mints = append(inc.Mints, d)
	s.byTask[d.TaskID] = fp

	if inc.Status == StatusOpen {
		inc.Status = StatusFixDispatched
	}
}

func (s *State) applyTaskTerminal(f journal.Fact) {
	fp, ok := s.byTask[f.TaskID]
	if !ok {
		return
	}

	inc := s.byFP[fp]
	if inc == nil {
		return
	}

	outcome := "completed"
	if f.Type == journal.DeadLettered {
		outcome = "dead"
	}

	for i := range inc.Mints {
		if inc.Mints[i].TaskID == f.TaskID && !inc.Mints[i].Terminal() {
			inc.Mints[i].Outcome = outcome
			inc.Mints[i].DoneSeq = f.Seq
		}
	}

	if last := latestMint(inc); last != nil && last.TaskID == f.TaskID {
		if outcome == "completed" {
			inc.Status = StatusResolved
		} else {
			inc.Status = StatusFixFailed
		}
	}
}

func latestMint(inc *Incident) *Mint {
	if inc == nil || len(inc.Mints) == 0 {
		return nil
	}

	return &inc.Mints[len(inc.Mints)-1]
}

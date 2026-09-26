package task

// Status is the task lifecycle state. Transitions are validated by CanTransitionTo.
type Status string

const (
	// Pending means the task is waiting to be claimed (possibly delayed via NotBefore).
	Pending Status = "pending"
	// Running means a worker holds the lease.
	Running Status = "running"
	// Completed means the task succeeded (terminal).
	Completed Status = "completed"
	// Dead means attempts were exhausted and the task is parked in the dead-letter queue (terminal).
	Dead Status = "dead"
	// Cancelled means the task was withdrawn before completion (terminal).
	Cancelled Status = "cancelled"
)

// AllStatuses returns every Status in lifecycle order (the single list
// other packages range over for enum-complete iteration). A function, not
// an exported slice var, so no caller can mutate the enum list for the
// whole process.
func AllStatuses() []Status {
	return []Status{Pending, Running, Completed, Dead, Cancelled}
}

// StatusCountsView flattens per-status counts into the stats wire shape:
// a string-keyed map holding every known status (zeros included) plus the
// total. The HTTP stats surfaces (httpapi, webui) share one contract
// pinned by TestStatsSurfacesAgree; this is that contract's single source.
func StatusCountsView(counts map[Status]int) (view map[string]int, total int) {
	for _, n := range counts {
		total += n
	}

	view = make(map[string]int, len(AllStatuses())+1)

	for _, st := range AllStatuses() {
		view[string(st)] = counts[st]
	}

	return view, total
}

// transitions lists every legal from→to pair. Anything else is invalid.
var transitions = map[Status]map[Status]bool{
	Pending:   {Running: true, Cancelled: true},
	Running:   {Pending: true, Completed: true, Dead: true, Cancelled: true},
	Completed: {},
	Dead:      {Pending: true, Cancelled: true}, // rescue re-queues; dismiss (autopsy verdict / operator) withdraws
	Cancelled: {},
}

// CanTransitionTo reports whether from → to is a legal lifecycle transition.
func CanTransitionTo(from, to Status) bool {
	return transitions[from][to]
}

// Terminal reports whether s is a terminal status.
func Terminal(s Status) bool {
	switch s {
	case Completed, Dead, Cancelled:
		return true
	}

	return false
}

// Valid reports whether s is a known status value.
func (s Status) Valid() bool {
	switch s {
	case Pending, Running, Completed, Dead, Cancelled:
		return true
	}

	return false
}

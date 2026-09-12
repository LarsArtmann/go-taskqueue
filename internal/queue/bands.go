package queue

// Band names one range of the single priority scale (ADR-0015 §1). One
// scale, three reserved bands: backlog computations clamp below hot;
// crossing upward requires an explicit human/flag act.
type Band string

const (
	// BandBacklog: 0–99 — humans (markers), importance, AI scores,
	// keyword fallback.
	BandBacklog Band = "backlog"
	// BandHot: 100–149 — same-session urgency (--same-session-priority)
	// only.
	BandHot Band = "hot"
	// BandMachine: 150+ — machine-generated operational tasks (cqa fixes).
	BandMachine Band = "machine"
)

// Band boundaries of the priority scale (ADR-0015 §1).
const (
	// BacklogMax is the ceiling backlog computations clamp to: no scorer,
	// keyword rule, or importance value reaches the hot band.
	BacklogMax = 99
	// HotMin is the floor of the hot band.
	HotMin = 100
	// HotMax is the ceiling of the hot band.
	HotMax = 149
	// MachineMin is the floor of the machine band.
	MachineMin = 150
)

// BandOf reports which band a priority value lands in. Values below zero
// are backlog (the scale's floor is open); the band ladder is total.
func BandOf(priority int) Band {
	switch {
	case priority >= MachineMin:
		return BandMachine
	case priority >= HotMin:
		return BandHot
	default:
		return BandBacklog
	}
}

// ClampBacklog clamps a computed backlog priority into [0, BacklogMax] so
// no derived score can smuggle a task into the hot or machine band
// (ADR-0015 §1: crossing a band boundary requires an explicit act).
func ClampBacklog(priority int) int {
	if priority < 0 {
		return 0
	}

	if priority > BacklogMax {
		return BacklogMax
	}

	return priority
}

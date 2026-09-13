// Package queue is the public facade over the store-contract module
// (ADR-0016): the Store interface, Filter, the Queue convenience facade,
// priority bands (ADR-0015), and the sentinel errors. The implementation
// stays internal; this package pins the names. Store drivers live in the
// sibling queue/sqlite and queue/postgres modules and satisfy Store here.
package queue

import (
	internalqueue "github.com/larsartmann/go-taskqueue/internal/queue"
)

// Contract types.
type (
	Store                = internalqueue.Store
	Filter               = internalqueue.Filter
	Queue                = internalqueue.Queue
	Band                 = internalqueue.Band
	PriorityScore        = internalqueue.PriorityScore
	RequeueEvidence      = internalqueue.RequeueEvidence
	ReprioritizeEvidence = internalqueue.ReprioritizeEvidence
	UnblockChange        = internalqueue.UnblockChange
	WatermarkEntry       = internalqueue.WatermarkEntry
)

// Priority bands (ADR-0015).
const (
	BandBacklog = internalqueue.BandBacklog
	BandHot     = internalqueue.BandHot
	BandMachine = internalqueue.BandMachine

	BacklogMax = internalqueue.BacklogMax
	HotMin     = internalqueue.HotMin
	HotMax     = internalqueue.HotMax
	MachineMin = internalqueue.MachineMin
)

// Priority aging (ADR-0015) and unblock bumps.
const (
	PriorityAgingDaysPerPoint = internalqueue.PriorityAgingDaysPerPoint
	PriorityAgingMaxBonus     = internalqueue.PriorityAgingMaxBonus
	PrioritySourceUnblock     = internalqueue.PrioritySourceUnblock
	UnblockBumpPriority       = internalqueue.UnblockBumpPriority
)

// Sentinel errors.
var (
	ErrNoTaskDue = internalqueue.ErrNoTaskDue
	ErrEmptyType = internalqueue.ErrEmptyType
)

// Functions.
var (
	New           = internalqueue.New
	BandOf        = internalqueue.BandOf
	ClampBacklog  = internalqueue.ClampBacklog
)

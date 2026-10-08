package incident

import (
	"context"
	"fmt"

	"github.com/larsartmann/go-taskqueue/internal/journal"
)

// FactSink appends NON-task journal facts (Store.AppendFact satisfies it;
// the session-bridge seam precedent — facts ride the same append-only
// log as task facts, in their own transaction).
type FactSink interface {
	AppendFact(ctx context.Context, f journal.Fact) error
}

// Recorder is the ReportError command handler: validate, cap, fingerprint,
// and append ONE error.observed fact to the journal. It reacts to
// nothing — the policy sweeper owns the reactions, so recording stays
// cheap enough for the error path of a production service.
type Recorder struct {
	sink FactSink
}

// NewRecorder returns a Recorder appending through sink.
func NewRecorder(sink FactSink) *Recorder {
	return &Recorder{sink: sink}
}

// RecordResult reports where the report landed.
type RecordResult struct {
	// Incident is the incident stream identity ("incident:<fingerprint>").
	Incident string `json:"incident"`
}

// Record executes the ReportError command. The report is clipped and
// validated BEFORE anything is journaled; an invalid report appends
// nothing.
func (rec *Recorder) Record(ctx context.Context, rep Report) (RecordResult, error) {
	rep = rep.Clip()

	if err := rep.Validate(); err != nil {
		return RecordResult{}, fmt.Errorf("incident: invalid report: %w", err)
	}

	detail, err := marshalDetail(rep)
	if err != nil {
		return RecordResult{}, fmt.Errorf("incident: encode report: %w", err)
	}

	fp := Fingerprint(rep)
	id := IncidentID(fp)

	if err := rec.sink.AppendFact(ctx, journal.Fact{
		TaskID: id,
		Type:   journal.ErrorObserved,
		Detail: detail,
	}); err != nil {
		return RecordResult{}, fmt.Errorf("incident: append %s: %w", journal.ErrorObserved, err)
	}

	return RecordResult{Incident: id}, nil
}

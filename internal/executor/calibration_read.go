package executor

import (
	"bufio"
	"encoding/json/v2"
	"fmt"
	"os"
	"sort"
)

// CalibrationKindStats is the per-executor slice of a calibration dataset
// summary: the counts the auto-dismiss/review tuning rules read (rows per
// verdict value, approve→commits-shipped cross-tab, attestation refusals).
type CalibrationKindStats struct {
	// Kind is the executor family the rows tuned ("agent", "review", …).
	Kind string
	// Rows is the row count for this kind.
	Rows int
	// WithVerdict counts rows whose verdict channel carried a body (a
	// plain work turn rides the footer derivation and carries none).
	WithVerdict int
	// VerdictValues is the distribution of the parsed verdict JSON's
	// `verdict` field (review: approve/request_changes; dlqfix:
	// fixed/wontfix). Rows without that field land under "(none)".
	VerdictValues map[string]int
	// ApproveRows and ApproveWithCommits cross-tab the review approve
	// verdict against the derived outcome: how often an approve came with
	// zero footer-attributed commits (the auto-dismiss tuning signal).
	ApproveRows        int
	ApproveWithCommits int
	// AttestedOK counts rows whose write-time attestation passed.
	AttestedOK int
	// Failures groups the row's FAILED attempts by stamped class
	// (parse-miss, verify deaths, rate-limit requeues); completion rows
	// carry no failure half and do not appear here.
	Failures map[string]int
	// AttestationRefusals groups rows by refusal class (receipt absent
	// from the row is "(no-receipt)"); the receipt-refusal rate the
	// tuning rules watch.
	AttestationRefusals map[string]int
	// TotalCostUSD sums the session cost of the rows (0 tolerated).
	TotalCostUSD float64
}

// CalibrationSummary is the whole-dataset rollup behind `tq calibration`.
type CalibrationSummary struct {
	// Path is the dataset file the summary read.
	Path string
	// Rows is the total parsed row count; Malformed counts lines that did
	// not parse as a CalibrationRow (a torn tail write, never silently
	// swallowed).
	Rows      int
	Malformed int
	// ByKind is one stats entry per executor kind, sorted by kind name.
	ByKind []CalibrationKindStats
}

// SummarizeCalibration reads the calibration.jsonl dataset at path and
// rolls it up per executor kind. A missing file is an error — the caller
// (tq calibration) points at the pool's log dir explicitly.
func SummarizeCalibration(path string) (CalibrationSummary, error) {
	f, err := os.Open(path)
	if err != nil {
		return CalibrationSummary{}, fmt.Errorf("open calibration dataset: %w", err)
	}
	defer func() { _ = f.Close() }()

	summary := CalibrationSummary{Path: path}
	kinds := map[string]*CalibrationKindStats{}

	scan := bufio.NewScanner(f)
	scan.Buffer(make([]byte, 0, 1<<20), 16<<20)

	for scan.Scan() {
		line := scan.Bytes()
		if len(line) == 0 {
			continue
		}

		var row CalibrationRow
		if err := json.Unmarshal(line, &row); err != nil {
			summary.Malformed++

			continue
		}

		summary.Rows++

		st := kinds[row.Kind]
		if st == nil {
			st = &CalibrationKindStats{Kind: row.Kind}
			kinds[row.Kind] = st
		}

		st.Rows++
		st.TotalCostUSD += row.CostUSD

		if row.Failure != nil {
			class := row.Failure.Class
			if class == "" {
				class = "(unclassified)"
			}

			if st.Failures == nil {
				st.Failures = map[string]int{}
			}

			st.Failures[class]++
		}

		if len(row.Verdict) > 0 {
			st.WithVerdict++

			var parsed struct {
				Verdict string `json:"verdict"`
			}

			_ = json.Unmarshal(row.Verdict, &parsed)
			if parsed.Verdict == "" {
				// The row body is JSON but carries no verdict field
				// (status reports, prioritize tables, agent free-form).
				parsed.Verdict = "(none)"
			}

			if st.VerdictValues == nil {
				st.VerdictValues = map[string]int{}
			}

			st.VerdictValues[parsed.Verdict]++

			if parsed.Verdict == string(VerdictApprove) {
				st.ApproveRows++
				if row.CommitCount > 0 {
					st.ApproveWithCommits++
				}
			}
		}

		switch {
		case row.Attestation != nil:
			if row.Attestation.OK {
				st.AttestedOK++
			} else {
				refusal := row.Attestation.Refusal
				if refusal == "" {
					refusal = "(unattested)"
				}

				if st.AttestationRefusals == nil {
					st.AttestationRefusals = map[string]int{}
				}

				st.AttestationRefusals[refusal]++
			}
		case row.Verify != nil:
			// A receipt without a write-time verdict is UNVERIFIED, not
			// refused — counted separately from the refusal classes.
			if st.AttestationRefusals == nil {
				st.AttestationRefusals = map[string]int{}
			}

			st.AttestationRefusals[AttestationNoReceipt]++
		}
	}

	if err := scan.Err(); err != nil {
		return CalibrationSummary{}, fmt.Errorf("read calibration dataset: %w", err)
	}

	for _, st := range kinds {
		summary.ByKind = append(summary.ByKind, *st)
	}

	sort.Slice(summary.ByKind, func(i, j int) bool {
		return summary.ByKind[i].Kind < summary.ByKind[j].Kind
	})

	return summary, nil
}

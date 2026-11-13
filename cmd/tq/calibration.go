package main

import (
	"encoding/json/v2"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/larsartmann/go-taskqueue/internal/executor"
)

// cmdCalibration summarizes the verdict→outcome calibration dataset
// ($TQ_LOG_DIR/calibration.jsonl) so auto-dismiss/review rules tune from
// data instead of by feel: rows per kind, the verdict-value distribution,
// the review approve→commits-shipped cross-tab, and the attestation
// refusal rates. Read-only; the dataset is the same one every paid run
// appends to (best-effort by contract — no log dir, no rows).
func cmdCalibration(args []string) error {
	fs := flag.NewFlagSet("calibration", flag.ExitOnError)
	fs.SetOutput(io.Discard)

	logDir := fs.String(
		"log-dir",
		os.Getenv("TQ_LOG_DIR"),
		"pool log dir holding calibration.jsonl (default $TQ_LOG_DIR)",
	)
	asJSON := fs.Bool("json", false, "emit the summary as JSON instead of the human table")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() != 0 {
		return errors.New("usage: tq calibration [--log-dir DIR] [--json]")
	}

	if *logDir == "" {
		return errors.New(
			"no dataset: pass --log-dir DIR or set $TQ_LOG_DIR — calibration.jsonl is written only when a pool runs with a log dir",
		)
	}

	path := filepath.Join(*logDir, executor.CalibrationDataset)

	summary, err := executor.SummarizeCalibration(path)
	if err != nil {
		return err
	}

	if *asJSON {
		b, err := json.Marshal(summary)
		if err != nil {
			return err
		}

		fmt.Println(string(b))

		return nil
	}

	fmt.Printf("dataset: %s\nrows: %d", summary.Path, summary.Rows)
	if summary.Malformed > 0 {
		fmt.Printf(" (malformed lines skipped: %d)", summary.Malformed)
	}
	fmt.Println()

	if summary.Rows == 0 {
		fmt.Println("(empty dataset — paid runs append rows only while the pool runs with TQ_LOG_DIR set)")

		return nil
	}

	for _, st := range summary.ByKind {
		fmt.Printf("\n%s: %d rows (%d with verdict, cost $%.2f)\n", st.Kind, st.Rows, st.WithVerdict, st.TotalCostUSD)

		values := make([]string, 0, len(st.VerdictValues))
		for v := range st.VerdictValues {
			values = append(values, v)
		}
		sort.Strings(values)
		for _, v := range values {
			fmt.Printf("  verdict %-18s %d\n", v+":", st.VerdictValues[v])
		}

		if st.ApproveRows > 0 {
			fmt.Printf("  approve → commits shipped: %d/%d (%.0f%%)\n",
				st.ApproveWithCommits, st.ApproveRows,
				100*float64(st.ApproveWithCommits)/float64(st.ApproveRows))
		}

		fmt.Printf("  attested ok: %d", st.AttestedOK)
		refusals := make([]string, 0, len(st.AttestationRefusals))
		for r := range st.AttestationRefusals {
			refusals = append(refusals, r)
		}
		sort.Strings(refusals)
		for _, r := range refusals {
			fmt.Printf("; %s: %d", r, st.AttestationRefusals[r])
		}
		fmt.Println()
	}

	return nil
}

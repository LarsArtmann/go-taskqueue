package main

import (
	"context"
	"flag"
	"fmt"
	"os"
)

// Process exit codes: a green gate, a projection mismatch (the cutover
// blocker), and a setup failure (the gate could not run at all).
const (
	exitOK       = 0
	exitMismatch = 1
	exitSetup    = 2
)

func main() {
	var (
		fromPath   = flag.String("from", "", "source tq journal (hand-rolled store db) — required, opened read-only")
		toPath     = flag.String("to", "", "target engine-store db — required, must not exist")
		verifyOnly = flag.Bool(
			"verify-only",
			false,
			"skip migration; only run the projection-equality gate against an existing replay",
		)
	)
	flag.Parse()

	if *fromPath == "" || *toPath == "" {
		flag.Usage()
		os.Exit(exitSetup)
	}

	ctx := context.Background()

	if !*verifyOnly {
		stats, err := Migrate(ctx, *fromPath, *toPath)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "replay: migration failed: %v\n", err)
			os.Exit(exitSetup)
		}

		_, _ = fmt.Fprintf(
			os.Stdout,
			"replayed %d tasks, %d deps, %d facts, %d watermarks, %d priority scores, %d archived facts, %d journal-meta rows\n",
			stats.Tasks,
			stats.Deps,
			stats.Facts,
			stats.Watermarks,
			stats.PriorityScores,
			stats.FactsArchive,
			stats.JournalMeta,
		)
	}

	report, err := Verify(ctx, *fromPath, *toPath)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "replay: verification failed: %v\n", err)
		os.Exit(exitSetup)
	}

	_, _ = fmt.Fprint(os.Stdout, report.Summary())

	if !report.OK() {
		_, _ = fmt.Fprintln(os.Stdout, "REPLAY: PROJECTION MISMATCH — cutover is NOT safe")

		os.Exit(exitMismatch)
	}

	_, _ = fmt.Fprintln(os.Stdout, "REPLAY: all projections equal — cutover gate green")

	os.Exit(exitOK)
}

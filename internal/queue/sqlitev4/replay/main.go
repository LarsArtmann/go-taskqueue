// Command replay migrates a tq fact journal into a fresh go-cqrs-lite
// engine store and verifies projection equality — the ADR-0019 S1
// data-migration gate. See the replay package documentation for the
// design and the C12 decision record.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
)

func main() {
	var (
		from       = flag.String("from", "", "source tq journal (hand-rolled store db) — required, opened read-only")
		to         = flag.String("to", "", "target engine-store db — required, must not exist")
		verifyOnly = flag.Bool("verify-only", false, "skip migration; only run the projection-equality gate against an existing replay")
	)
	flag.Parse()

	if *from == "" || *to == "" {
		flag.Usage()
		os.Exit(2)
	}

	ctx := context.Background()

	if !*verifyOnly {
		stats, err := replay.Migrate(ctx, *from, *to)
		if err != nil {
			fmt.Fprintf(os.Stderr, "replay: migration failed: %v\n", err)
			os.Exit(2)
		}

		fmt.Printf("replayed %d tasks, %d deps, %d facts, %d watermarks, %d priority scores, %d archived facts, %d journal-meta rows\n",
			stats.Tasks, stats.Deps, stats.Facts, stats.Watermarks, stats.PriorityScores, stats.FactsArchive, stats.JournalMeta)
	}

	report, err := replay.Verify(ctx, *from, *to)
	if err != nil {
		fmt.Fprintf(os.Stderr, "replay: verification failed: %v\n", err)
		os.Exit(2)
	}

	fmt.Print(report.Summary())

	if !report.OK() {
		fmt.Println("REPLAY: PROJECTION MISMATCH — cutover is NOT safe")
		os.Exit(1)
	}

	fmt.Println("REPLAY: all projections equal — cutover gate green")
}

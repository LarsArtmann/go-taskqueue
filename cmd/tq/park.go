package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

// maxParkTTL caps --until: a park is a disposition, not a retirement —
// re-park to extend (mirrors ask's maxQuestionTTL ceiling).
const maxParkTTL = 7 * 24 * time.Hour

// cmdPark is the operator's reversible disposition for a PENDING task:
//
//	tq park TASK_ID --until 24h [--reason WHY]
//	tq park TASK_ID --release [--reason WHY]
//
// Claims are suppressed until the park expires without burning an
// attempt (NotBefore moves + the task.parked fact lands in the same
// transaction); --release re-arms claims immediately. Cancel stays the
// destructive lever — park exists so "not now" stops costing attempts.
func cmdPark(args []string) error {
	fs := flag.NewFlagSet("park", flag.ExitOnError)

	until := fs.String("until", "", "how long claims stay suppressed (e.g. 24h)")
	release := fs.Bool("release", false, "release a parked task now: claims re-arm immediately")
	reason := fs.String("reason", "", "why the task is parked; stored on the task.parked fact for forensics")
	db := dbFlag(fs)

	if err := fs.Parse(partitionFlags(args, map[string]bool{"until": true, "reason": true, "db": true})); err != nil {
		return err
	}

	if fs.NArg() != 1 {
		return errors.New("usage: tq park TASK_ID --until DUR [--reason WHY]  |  tq park TASK_ID --release")
	}

	if *until != "" && *release {
		return errors.New("tq park: --until and --release are mutually exclusive — park for a window, or release now")
	}

	store := mustOpenDB(resolveDB(*db))
	defer store.Close()

	ctx := context.Background()

	t, err := resolveTask(ctx, store, fs.Arg(0))
	if err != nil {
		return err
	}

	if t.Status != task.Pending {
		return fmt.Errorf(
			"tq park: task %s is %s, not pending — park holds work back, it does not stop it (tq cancel --force is the running-task lever)",
			t.ID,
			t.Status,
		)
	}

	resume := time.Now()
	if !*release {
		ttl, err := time.ParseDuration(*until)
		if err != nil || ttl <= 0 {
			return fmt.Errorf("tq park: --until %q is not a positive duration (e.g. 24h — units are h/m/s, days are not one)", *until)
		}

		if ttl > maxParkTTL {
			return fmt.Errorf(
				"tq park: --until %s exceeds the %s cap — re-park to extend instead of parking past the horizon",
				ttl,
				maxParkTTL,
			)
		}

		resume = time.Now().Add(ttl)
	}

	if err := store.Park(ctx, t.ID, *reason, resume, parkBy()); err != nil {
		return fmt.Errorf("tq park: task %s: %w", t.ID, err)
	}

	if *release {
		fmt.Printf("task %s released — claims re-arm now\n", t.ID)

		return nil
	}

	fmt.Printf("task %s parked until %s (by %s)\n", t.ID, resume.Format(time.RFC3339), parkBy())

	return nil
}

// parkBy names the operator on the task.parked fact: USER when a human
// ran the verb from a shell, "operator" as the auditable fallback.
func parkBy() string {
	if user := os.Getenv("USER"); user != "" {
		return user
	}

	return "operator"
}

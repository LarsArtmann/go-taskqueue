// Command tq is the CLI for go-taskqueue: enqueue work, run workers, inspect
// the queue, and replay the journal.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
	"github.com/larsartmann/go-taskqueue/internal/worker"
)

const usage = `tq — projects-aware task work queue

Usage:
  tq enqueue --type TYPE [--project P] [--payload JSON] [--deps id,...] [--priority N]
            [--max-attempts N] [--delay DUR] [--db PATH]
  tq worker [--concurrency N] [--db PATH] [--poll DUR] [--lease DUR]
  tq stats [--project P] [--status S] [--db PATH] [--json]
  tq show TASK_ID [--db PATH]
  tq dlq [--db PATH] [--rescue TASK_ID [--max-attempts N]]
  tq cancel TASK_ID [--db PATH]
  tq facts [--db PATH] [--after SEQ]
  tq tail [-f] [--db PATH] [--after SEQ]

Default database: $TQ_DB or ./tasks.db
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "enqueue":
		err = cmdEnqueue(os.Args[2:])
	case "worker":
		err = cmdWorker(os.Args[2:])
	case "stats":
		err = cmdStats(os.Args[2:])
	case "show":
		err = cmdShow(os.Args[2:])
	case "dlq":
		err = cmdDLQ(os.Args[2:])
	case "cancel":
		err = cmdCancel(os.Args[2:])
	case "facts":
		err = cmdFacts(os.Args[2:])
	case "tail":
		err = cmdTail(os.Args[2:])
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "tq: unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "tq: %v\n", err)
		os.Exit(1)
	}
}

func defaultDB() string {
	if p := os.Getenv("TQ_DB"); p != "" {
		return p
	}
	return "tasks.db"
}

func mustOpenDB(path string) *queue.SQLiteStore {
	s, err := queue.OpenSQLite(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tq: open db: %v\n", err)
		os.Exit(1)
	}
	return s
}

func dbFlag(fs *flag.FlagSet) *string {
	return fs.String("db", "", "database path (default $TQ_DB or ./tasks.db)")
}

func resolveDB(v string) string {
	if v != "" {
		return v
	}
	return defaultDB()
}

func cmdEnqueue(args []string) error {
	fs := flag.NewFlagSet("enqueue", flag.ExitOnError)
	project := fs.String("project", "", "project the task belongs to")
	taskType := fs.String("type", "", "task type (required)")
	payload := fs.String("payload", "", "JSON payload (or @file)")
	deps := fs.String("deps", "", "comma-separated dependency task IDs")
	priority := fs.Int("priority", 0, "higher claims first")
	maxAttempts := fs.Int("max-attempts", 0, "default 3")
	delay := fs.Duration("delay", 0, "delay before claimable (e.g. 30s, 5m)")
	db := dbFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *taskType == "" {
		return fmt.Errorf("--type is required")
	}

	var payloadJSON json.RawMessage
	if *payload != "" {
		raw := []byte(*payload)
		if after, ok := strings.CutPrefix(*payload, "@"); ok {
			b, err := os.ReadFile(after)
			if err != nil {
				return fmt.Errorf("read payload file: %w", err)
			}
			raw = b
		}
		if !json.Valid(raw) {
			// The shell path takes the payload as the command line itself
			// (tq enqueue --type sh --payload 'echo hi'), so wrap a non-JSON
			// payload as a JSON string instead of rejecting it. The stored
			// payload is always valid JSON.
			if *taskType != "sh" {
				return fmt.Errorf("payload is not valid JSON: %s", raw)
			}
			wrapped, err := json.Marshal(string(raw))
			if err != nil {
				return fmt.Errorf("wrap payload: %w", err)
			}
			raw = wrapped
		}
		payloadJSON = raw
	}

	n := task.New{
		Project:     *project,
		Type:        *taskType,
		Payload:     payloadJSON,
		Priority:    *priority,
		MaxAttempts: *maxAttempts,
		NotBefore:   time.Now().Add(*delay),
	}
	for d := range strings.SplitSeq(*deps, ",") {
		if d = strings.TrimSpace(d); d != "" {
			n.Deps = append(n.Deps, task.ID(d))
		}
	}

	s := mustOpenDB(resolveDB(*db))
	defer s.Close()
	t, err := queue.New(s).Enqueue(context.Background(), n)
	if err != nil {
		return err
	}
	fmt.Println(t.ID)
	return nil
}

func cmdWorker(args []string) error {
	fs := flag.NewFlagSet("worker", flag.ExitOnError)
	conc := fs.Int("concurrency", 2, "parallel executions")
	poll := fs.Duration("poll", 250*time.Millisecond, "idle poll interval")
	lease := fs.Duration("lease", 2*time.Minute, "claim lease length")
	timeout := fs.Duration("task-timeout", 10*time.Minute, "per-task timeout")
	owner := fs.String("owner", "", "lease owner identity")
	db := dbFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	s := mustOpenDB(resolveDB(*db))
	defer s.Close()

	// The "sh" executor with empty template runs the payload itself as the
	// shell line ({"cmd":...} JSON is unwrapped). This keeps the CLI path
	// trivially usable while Go users register their own executors.
	reg := executor.NewRegistry()
	reg.Register("sh", executor.NewCommandExecutor(""))

	pool := worker.New(s, worker.Config{
		Owner:        *owner,
		Concurrency:  *conc,
		PollInterval: *poll,
		Lease:        *lease,
		TaskTimeout:  *timeout,
		Executors:    reg,
	}, nil)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return pool.Start(ctx)
}

func cmdStats(args []string) error {
	fs := flag.NewFlagSet("stats", flag.ExitOnError)
	project := fs.String("project", "", "filter by project")
	status := fs.String("status", "", "filter by status")
	asJSON := fs.Bool("json", false, "JSON output")
	db := dbFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	s := mustOpenDB(resolveDB(*db))
	defer s.Close()

	f := queue.Filter{}
	if *project != "" {
		f.Project = project
	}
	if *status != "" {
		st := task.Status(*status)
		f.Status = &st
	}
	tasks, err := s.List(context.Background(), f)
	if err != nil {
		return err
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(tasks)
	}

	byStatus := map[string]int{}
	byProject := map[string]map[string]int{}
	for _, t := range tasks {
		byStatus[string(t.Status)]++
		if byProject[t.Project] == nil {
			byProject[t.Project] = map[string]int{}
		}
		byProject[t.Project][string(t.Status)]++
	}

	fmt.Printf("%-12s %6s\n", "STATUS", "COUNT")
	for _, st := range []string{"pending", "running", "completed", "dead", "cancelled"} {
		if c, ok := byStatus[st]; ok {
			fmt.Printf("%-12s %6d\n", st, c)
		}
	}
	if *project == "" && len(byProject) > 0 {
		fmt.Println()
		fmt.Printf("%-28s %8s %8s %8s %8s %8s\n", "PROJECT", "pending", "running", "done", "dead", "cancld")
		projects := make([]string, 0, len(byProject))
		for p := range byProject {
			projects = append(projects, p)
		}
		sort.Strings(projects)
		for _, p := range projects {
			m := byProject[p]
			fmt.Printf("%-28s %8d %8d %8d %8d %8d\n", p,
				m["pending"], m["running"], m["completed"], m["dead"], m["cancelled"])
		}
	}
	return nil
}

func cmdShow(args []string) error {
	fs := flag.NewFlagSet("show", flag.ExitOnError)
	db := dbFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: tq show TASK_ID")
	}
	s := mustOpenDB(resolveDB(*db))
	defer s.Close()
	t, err := s.Get(context.Background(), task.ID(fs.Arg(0)))
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(t)
}

func cmdDLQ(args []string) error {
	fs := flag.NewFlagSet("dlq", flag.ExitOnError)
	rescue := fs.String("rescue", "", "re-queue this dead task ID")
	maxAttempts := fs.Int("max-attempts", 3, "attempt budget for rescued task")
	db := dbFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	s := mustOpenDB(resolveDB(*db))
	defer s.Close()
	if *rescue != "" {
		if err := s.RescueDead(context.Background(), task.ID(*rescue), *maxAttempts); err != nil {
			return err
		}
		fmt.Printf("rescued %s\n", *rescue)
		return nil
	}
	st := task.Dead
	tasks, err := s.List(context.Background(), queue.Filter{Status: &st})
	if err != nil {
		return err
	}
	if len(tasks) == 0 {
		fmt.Println("(empty)")
		return nil
	}
	for _, t := range tasks {
		fmt.Printf("%s  %-24s attempts=%d/%d  %s\n",
			t.ID, t.Project+"/"+t.Type, t.Attempts, t.MaxAttempts, truncate(t.LastError, 80))
	}
	return nil
}

func cmdCancel(args []string) error {
	fs := flag.NewFlagSet("cancel", flag.ExitOnError)
	db := dbFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: tq cancel TASK_ID")
	}
	s := mustOpenDB(resolveDB(*db))
	defer s.Close()
	return s.Cancel(context.Background(), task.ID(fs.Arg(0)))
}

func cmdFacts(args []string) error {
	fs := flag.NewFlagSet("facts", flag.ExitOnError)
	after := fs.Int64("after", 0, "only facts with seq > this")
	db := dbFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	s := mustOpenDB(resolveDB(*db))
	defer s.Close()
	facts, err := s.Facts(context.Background(), *after)
	if err != nil {
		return err
	}
	for _, f := range facts {
		fmt.Printf("%5d %s %s %-20s %s %s\n",
			f.Seq, f.Time.Format(time.RFC3339), f.TaskID, f.Type, f.Owner, f.Error)
	}
	fmt.Printf("(%d facts)\n", len(facts))
	return nil
}

func cmdTail(args []string) error {
	fs := flag.NewFlagSet("tail", flag.ExitOnError)
	after := fs.Int64("after", 0, "only facts with seq > this")
	follow := fs.Bool("f", false, "follow (live)")
	db := dbFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	s := mustOpenDB(resolveDB(*db))
	defer s.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	for {
		facts, err := s.Facts(ctx, *after)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		for _, f := range facts {
			fmt.Printf("%5d %s %s %-20s %s %s\n",
				f.Seq, f.Time.Format(time.RFC3339), f.TaskID, f.Type, f.Owner, f.Error)
			*after = f.Seq
		}
		if !*follow {
			return nil
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

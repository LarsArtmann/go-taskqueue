package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// isTerminal reports whether w is an interactive terminal, so the live view
// may repaint with ANSI cursor control; pipes and files get plain frames.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// projectView is one row of `tq top`: per-project counts plus the durations
// operators actually ask about — how long the last finished run took, and how
// long the currently running one has been going.
type projectView struct {
	Project   string        `json:"project"`
	Pending   int           `json:"pending"`
	Running   int           `json:"running"`
	Completed int           `json:"completed"`
	Dead      int           `json:"dead"`
	Cancelled int           `json:"cancelled"`
	LastDur   time.Duration `json:"last_dur"`   // run duration of the most recent completion
	ActiveDur time.Duration `json:"active_dur"` // elapsed of the current running task
	HasLast   bool          `json:"has_last"`
	HasActive bool          `json:"has_active"`
}

// aggregateTop builds the per-project view from the task table (counts,
// project names) and the journal (run durations). now stamps the active
// duration.
func aggregateTop(tasks []task.Task, facts []journal.Fact, now time.Time) []projectView {
	byProject := map[string]*projectView{}
	view := func(name string) *projectView {
		if byProject[name] == nil {
			byProject[name] = &projectView{Project: name}
		}
		return byProject[name]
	}
	taskProject := make(map[string]string, len(tasks))
	for i := range tasks {
		t := tasks[i]
		taskProject[t.ID.String()] = t.Project
		v := view(t.Project)
		switch t.Status {
		case task.Pending:
			v.Pending++
		case task.Running:
			v.Running++
		case task.Completed:
			v.Completed++
		case task.Dead:
			v.Dead++
		case task.Cancelled:
			v.Cancelled++
		}
	}

	// Latest claim per task: run duration = completion time minus the claim
	// time of the winning attempt (a reclaimed task's earlier claims are
	// overwritten). Facts arrive in seq order, so the last Completed fact per
	// project is the most recent completion.
	claimedAt := map[string]time.Time{}
	for _, f := range facts {
		switch f.Type {
		case journal.Claimed:
			claimedAt[f.TaskID] = f.Time
		case journal.Completed:
			if start, ok := claimedAt[f.TaskID]; ok {
				v := view(taskProject[f.TaskID])
				v.LastDur, v.HasLast = f.Time.Sub(start), true
			}
		}
	}
	for i := range tasks {
		t := tasks[i]
		if t.Status != task.Running {
			continue
		}
		if start, ok := claimedAt[t.ID.String()]; ok {
			v := view(t.Project)
			v.ActiveDur, v.HasActive = now.Sub(start), true
		}
	}

	out := make([]projectView, 0, len(byProject))
	for _, v := range byProject {
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Project < out[j].Project })
	return out
}

func cmdTop(args []string) error {
	fs := flag.NewFlagSet("top", flag.ExitOnError)
	interval := fs.Duration("interval", 2*time.Second, "refresh interval")
	once := fs.Bool("once", false, "render one frame and exit")
	asJSON := fs.Bool("json", false, "JSON output (one frame, then exit)")
	db := dbFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	s := mustOpenDB(resolveDB(*db))
	defer s.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	for {
		tasks, err := s.List(ctx, queue.Filter{})
		if err != nil {
			return err
		}
		facts, err := s.Facts(ctx, 0)
		if err != nil {
			return err
		}
		views := aggregateTop(tasks, facts, time.Now())
		if *asJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(views)
		}
		if !*once && isTerminal(os.Stdout) {
			fmt.Print("\x1b[2J\x1b[H")
		}
		renderTop(views, time.Now())
		if *once {
			return nil
		}
		select {
		case <-ctx.Done():
			fmt.Println()
			return nil
		case <-time.After(*interval):
		}
	}
}

func renderTop(views []projectView, now time.Time) {
	fmt.Printf("tq top — %s\n", now.Format("15:04:05"))
	fmt.Printf("%-28s %6s %6s %6s %6s %8s %9s\n", "PROJECT", "pend", "run", "done", "dead", "last", "active")
	var tp, tr, tc, td int
	for _, v := range views {
		last, active := "-", "-"
		if v.HasLast {
			last = shortDur(v.LastDur)
		}
		if v.HasActive {
			active = shortDur(v.ActiveDur)
		}
		fmt.Printf("%-28s %6d %6d %6d %6d %8s %9s\n",
			truncate(v.Project, 28), v.Pending, v.Running, v.Completed, v.Dead, last, active)
		tp += v.Pending
		tr += v.Running
		tc += v.Completed
		td += v.Dead
	}
	fmt.Printf("%-28s %6d %6d %6d %6d %8s %9s\n", "TOTAL", tp, tr, tc, td, "", "")
}

func shortDur(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	return d.Truncate(100 * time.Millisecond).String()
}

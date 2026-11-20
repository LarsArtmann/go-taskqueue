package main

import (
	"flag"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/webui"
)

// TestServeFlagParseWiresConfig pins the cmdServe flag → webui.Config
// wiring through the one seam (newServeFlagSet + serveOpts.config): a
// flag rename or a Config field drift fails here instead of silently
// serving a zeroed field (DailyBudget was once dead on serve).
func TestServeFlagParseWiresConfig(t *testing.T) {
	t.Setenv("TQ_SERVE_TOKEN", "")
	t.Setenv("TQ_SERVE_WRITES", "")
	t.Setenv("TQ_PAP_URL", "")
	t.Setenv("TQ_PAP_API_KEY", "")
	t.Setenv("TQ_POOL_CONFIG", "")

	fs, opts := newServeFlagSet()
	err := fs.Parse([]string{
		"--addr", "127.0.0.1:9999",
		"--poll", "2s",
		"--verbose",
		"--auth-token", "sekrit",
		"--allow-writes",
		"--read-model=false",
		"--dlq-backoff", "90m",
		"--daily-budget", "37",
		"--crush-server", "/tmp/crush.sock",
		"--alert-url", "http://pap.example",
		"--alert-api-key", "papkey",
		"--store", "/tmp/q.db",
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	cfg := opts.config()

	want := webui.Config{
		Addr:        "127.0.0.1:9999",
		Poll:        2 * time.Second,
		Version:     version,
		RequestLog:  true,
		AuthToken:   "sekrit",
		AllowWrites: true,
		DailyBudget: 37,
		DLQBackoff:  90 * time.Minute,
		CrushServer: "/tmp/crush.sock",
		AlertURL:    "http://pap.example",
		AlertAPIKey: "papkey",
	}
	if cfg != want {
		t.Errorf("cfg = %+v, want %+v", cfg, want)
	}

	if *opts.readModel {
		t.Error("read-model = true, want the parsed --read-model=false")
	}
	if *opts.store != "/tmp/q.db" {
		t.Errorf("store = %q, want /tmp/q.db", *opts.store)
	}

	// Zero-value parse: defaults leave the budget meter hidden.
	fs0, opts0 := newServeFlagSet()
	if err := fs0.Parse(nil); err != nil {
		t.Fatalf("parse defaults: %v", err)
	}
	if got := opts0.config().DailyBudget; got != 0 {
		t.Errorf("default DailyBudget = %d, want 0 (meter hidden)", got)
	}
}

// TestServeFlagsRegistered pins the flag NAMES so a rename breaks the
// operator surface loudly here rather than at the dashboard.
func TestServeFlagsRegistered(t *testing.T) {
	t.Parallel()

	fs, _ := newServeFlagSet()
	seen := map[string]bool{}
	fs.VisitAll(func(f *flag.Flag) { seen[f.Name] = true })

	for _, name := range []string{
		"addr", "poll", "verbose", "auth-token", "allow-writes",
		"read-model", "dlq-backoff", "config", "daily-budget",
		"crush-server", "alert-url", "alert-api-key", "db", "store",
	} {
		if !seen[name] {
			t.Errorf("serve flag --%s not registered", name)
		}
	}
}

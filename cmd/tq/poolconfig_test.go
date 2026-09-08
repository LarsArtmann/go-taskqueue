package main

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "pool.conf")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	return path
}

func newPoolFlags() (*flag.FlagSet, *string, *time.Duration, *int) {
	fs := flag.NewFlagSet("agent-pool", flag.ContinueOnError)
	projectsDir := fs.String("projects-dir", "", "dir")
	interval := fs.Duration("interval", 5*time.Minute, "cadence")
	conc := fs.Int("concurrency", 1, "parallel agents")
	fs.String("cqa-url", os.Getenv("CQA_URL"), "cqa base url")

	return fs, projectsDir, interval, conc
}

func TestLoadPoolConfigFileParsesCommentsQuotesAndBlanks(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, `
# comment line
; also a comment

projects-dir = ~/projects
interval = "10m"
concurrency = '3'
`)

	got, err := loadPoolConfigFile(path)
	if err != nil {
		t.Fatalf("loadPoolConfigFile: %v", err)
	}

	if got["projects-dir"] != "~/projects" || got["interval"] != "10m" || got["concurrency"] != "3" {
		t.Fatalf("parsed = %v", got)
	}

	if len(got) != 3 {
		t.Fatalf("expected 3 keys, got %d: %v", len(got), got)
	}
}

func TestLoadPoolConfigFileRejectsKeylessLine(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, "not-a-key-value-line\n")

	if _, err := loadPoolConfigFile(path); err == nil {
		t.Fatal("keyless line must error")
	}
}

func TestApplyPoolConfigFilePrecedenceFlagBeatsFile(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, "projects-dir = from-file\ninterval = 10m\nconcurrency = 4\n")
	fs, projectsDir, interval, conc := newPoolFlags()

	if err := fs.Parse([]string{"--projects-dir", "from-flag"}); err != nil {
		t.Fatalf("parse: %v", err)
	}

	if err := applyPoolConfigFile(fs, path); err != nil {
		t.Fatalf("apply: %v", err)
	}

	if *projectsDir != "from-flag" {
		t.Fatalf("explicit flag must beat file, got %q", *projectsDir)
	}

	if *interval != 10*time.Minute {
		t.Fatalf("file value must fill unset flag, got %v", *interval)
	}

	if *conc != 4 {
		t.Fatalf("file value must fill unset flag, got %d", *conc)
	}
}

func TestApplyPoolConfigFileEnvBeatsFile(t *testing.T) {
	t.Setenv("CQA_URL", "https://cqa-from-env")

	path := writeConfig(t, "cqa-url = https://cqa-from-file\n")
	fs, _, _, _ := newPoolFlags()

	if err := fs.Parse(nil); err != nil {
		t.Fatalf("parse: %v", err)
	}

	if err := applyPoolConfigFile(fs, path); err != nil {
		t.Fatalf("apply: %v", err)
	}

	got := fs.Lookup("cqa-url").Value.String()
	if got != "https://cqa-from-env" {
		t.Fatalf("env must beat file for env-backed flags, got %q", got)
	}
}

func TestApplyPoolConfigFileLogDirAppliesAndEnvBeatsFile(t *testing.T) {
	t.Setenv("TQ_LOG_DIR", "")

	path := writeConfig(t, "log-dir = /state/logs-from-file\n")
	fs := flag.NewFlagSet("agent-pool", flag.ContinueOnError)
	logDir := fs.String("log-dir", "", "sidecar dir")

	if err := fs.Parse(nil); err != nil {
		t.Fatalf("parse: %v", err)
	}

	if err := applyPoolConfigFile(fs, path); err != nil {
		t.Fatalf("apply: %v", err)
	}

	if got := *logDir; got != "/state/logs-from-file" {
		t.Fatalf("config-file log-dir must apply, got %q", got)
	}

	t.Setenv("TQ_LOG_DIR", "/state/logs-from-env")
	fs2 := flag.NewFlagSet("agent-pool", flag.ContinueOnError)
	fs2.String("log-dir", "", "sidecar dir")

	if err := fs2.Parse(nil); err != nil {
		t.Fatalf("parse: %v", err)
	}

	if err := applyPoolConfigFile(fs2, path); err != nil {
		t.Fatalf("apply: %v", err)
	}

	if got := fs2.Lookup("log-dir").Value.String(); got != "" {
		t.Fatalf("env must beat file for log-dir, got %q", got)
	}
}

func TestApplyPoolConfigFileUnknownKeyErrors(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, "concurrencyy = 4\n")
	fs, _, _, _ := newPoolFlags()

	if err := fs.Parse(nil); err != nil {
		t.Fatalf("parse: %v", err)
	}

	err := applyPoolConfigFile(fs, path)
	if err == nil {
		t.Fatal("unknown key must error")
	}

	if !strings.Contains(err.Error(), "unknown key") {
		t.Fatalf("error should name the problem, got %v", err)
	}
}

func TestApplyPoolConfigFileRejectsConfigKey(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, "config = /etc/evil\n")
	fs, _, _, _ := newPoolFlags()

	if err := fs.Parse(nil); err != nil {
		t.Fatalf("parse: %v", err)
	}

	if err := applyPoolConfigFile(fs, path); err == nil {
		t.Fatal("config key inside the file must be rejected")
	}
}

func TestApplyPoolConfigFileBadValueErrors(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, "interval = not-a-duration\n")
	fs, _, _, _ := newPoolFlags()

	if err := fs.Parse(nil); err != nil {
		t.Fatalf("parse: %v", err)
	}

	if err := applyPoolConfigFile(fs, path); err == nil {
		t.Fatal("unparsable value must error, not silently keep the default")
	}
}

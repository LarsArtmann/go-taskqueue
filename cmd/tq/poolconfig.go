package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
)

// poolConfigEnvBacked maps flags whose DEFAULTS come from the environment
// (they read os.Getenv at definition time). Precedence is flag > env > file:
// a config-file value must not clobber a live env setting for these.
var poolConfigEnvBacked = map[string]string{
	"cqa-url":   "CQA_URL",
	"cqa-owner": "CQA_OWNER_ID",
	"cqa-token": "CQA_TOKEN",
	"log-dir":   "TQ_LOG_DIR",
}

// loadPoolConfigFile reads a flat key=value file (one setting per line,
// `#` comments and blank lines ignored, values may be quoted with single
// or double quotes) into a map. Keys use the flag spelling (--projects-dir
// style, without the dashes prefix).
func loadPoolConfigFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("pool config: %w", err)
	}
	defer f.Close()

	out := map[string]string{}

	scanner := bufio.NewScanner(f)
	for lineNo := 1; scanner.Scan(); lineNo++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("pool config %s:%d: expected key=value, got %q", path, lineNo, line)
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = unquoteConfigValue(value)

		if key == "" {
			return nil, fmt.Errorf("pool config %s:%d: empty key", path, lineNo)
		}

		out[key] = value
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("pool config: read %s: %w", path, err)
	}

	return out, nil
}

func unquoteConfigValue(value string) string {
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
			return value[1 : len(value)-1]
		}
	}

	return value
}

// applyPoolConfigFile applies config-file values to flags the operator did
// NOT set on the command line, so precedence is flag > env > file > built-in
// default. Unknown keys are an error (typos must fail loudly, not silently
// run with defaults); the --config key itself is rejected (chicken-and-egg).
func applyPoolConfigFile(fs *flag.FlagSet, path string) error {
	file, err := loadPoolConfigFile(path)
	if err != nil {
		return err
	}

	if len(file) == 0 {
		return nil
	}

	explicit := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { explicit[f.Name] = true })

	for key, value := range file {
		if key == "config" {
			return fmt.Errorf("pool config %s: key \"config\" cannot live in the config file itself", path)
		}

		fl := fs.Lookup(key)
		if fl == nil {
			return fmt.Errorf("pool config %s: unknown key %q (use flag names like projects-dir, interval, concurrency; see tq agent-pool --help)", path, key)
		}

		if explicit[key] {
			continue
		}

		if envVar := poolConfigEnvBacked[key]; envVar != "" && os.Getenv(envVar) != "" {
			continue
		}

		if err := fs.Set(key, value); err != nil {
			return fmt.Errorf("pool config %s: key %q: %w", path, key, err)
		}
	}

	return nil
}

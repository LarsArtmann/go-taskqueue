package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
)

// cmdVerdict is the agent→queue result channel: an agent inside a queue
// task records its structured verdict (review/dlqfix/prioritize/status
// JSON) by running `tq verdict '<json>'`. The command validates the JSON,
// writes it to $TQ_RESULT_FILE (the per-run file the executor hands the
// agent process), and nothing else — no database access, no TQ_DB
// coupling. The executor reads the file after the agent exits; the prompt
// contracts teach this command instead of a TQ_RESULT stdout line.
func cmdVerdict(args []string) error {
	fs := flag.NewFlagSet("verdict", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("tq verdict: %w (usage: tq verdict '<json>' | tq verdict - < file)", err)
	}

	if fs.NArg() != 1 {
		return errors.New("usage: tq verdict '<json>'   (or: tq verdict - < file.json)")
	}

	var body []byte

	if fs.Arg(0) == "-" {
		raw, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("tq verdict: read stdin: %w", err)
		}

		body = raw
	} else {
		body = []byte(fs.Arg(0))
	}

	body = bytes.TrimSpace(body)

	if len(body) == 0 {
		return errors.New("tq verdict: empty payload — pass the result JSON as the argument (or via stdin with -)")
	}

	if !json.Valid(body) {
		return fmt.Errorf("tq verdict: payload is not valid JSON: %.200s — fix the JSON and re-run; the executor would reject this as a failed attempt", body)
	}

	if bytes.Contains(body, []byte("\n")) {
		return errors.New("tq verdict: payload must be ONE line — compact the JSON (the result line is single-line by contract)")
	}

	path := os.Getenv("TQ_RESULT_FILE")
	if path == "" {
		return errors.New(
			"TQ_RESULT_FILE is not set — this command records a verdict for a queue task and only works inside one. " +
				"Running outside a task there is nothing to record",
		)
	}

	if err := os.WriteFile(path, body, 0o600); err != nil {
		return fmt.Errorf("tq verdict: write %s: %w", path, err)
	}

	fmt.Fprintf(os.Stderr, "tq verdict: recorded %d bytes\n", len(body))

	return nil
}

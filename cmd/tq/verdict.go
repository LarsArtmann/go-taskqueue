package main

import (
	"bytes"
	"encoding/json/jsontext"
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
//
// `--file <path>` overrides the env channel: under `--crush-server` the
// bash tool sees only the SERVER env (M8), so $TQ_RESULT_FILE is invisible
// to routed agents — the work prompt bakes the path instead and the agent
// records with `tq verdict --file <path> '<json>'`. Local runs keep the
// env channel byte-identical (the flag is additive; unset = today).
func cmdVerdict(args []string) error {
	fs := flag.NewFlagSet("verdict", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fileFlag := fs.String("file", "",
		"write the verdict to this path (the routed-run channel; overrides $TQ_RESULT_FILE — the task prompt names the path)")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("tq verdict: %w (usage: tq verdict '<json>' | tq verdict --file <path> '<json>' | tq verdict - < file)", err)
	}

	if fs.NArg() != 1 {
		return errors.New("usage: tq verdict '<json>'   (or: tq verdict --file <path> '<json>'  |  tq verdict - < file.json)")
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

	if !jsontext.Value(body).IsValid() {
		return fmt.Errorf(
			"tq verdict: payload is not valid JSON: %.200s — fix the JSON and re-run; "+
				"the executor would reject this as a failed attempt",
			body,
		)
	}

	if bytes.Contains(body, []byte("\n")) {
		return errors.New(
			"tq verdict: payload must be ONE line — compact the JSON (the result line is single-line by contract)",
		)
	}

	path := *fileFlag
	if path == "" {
		path = os.Getenv("TQ_RESULT_FILE")
	}

	if path == "" {
		return errors.New(
			"no verdict channel: pass --file <path> (the task prompt names it) or set $TQ_RESULT_FILE — " +
				"this command records a verdict for a queue task and only works inside one. " +
				"Running outside a task there is nothing to record",
		)
	}

	if err := os.WriteFile(path, body, 0o600); err != nil {
		return fmt.Errorf("tq verdict: write %s: %w", path, err)
	}

	fmt.Fprintf(os.Stderr, "tq verdict: recorded %d bytes\n", len(body))

	return nil
}

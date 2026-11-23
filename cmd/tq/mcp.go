package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/larsartmann/go-taskqueue/internal/mcpserver"
)

// cmdMCP is `tq mcp`: the MCP stdio server (plan 2026-10-16_01-11
// Phase 1) — a JSON-RPC 2.0 tools surface over the queue's reads so
// agent sessions query the journal through tools instead of shelling
// out (`tq tasks | grep` quoting hazards) or trusting ambient env. The
// database is an EXPLICIT --db (resolved through the standard
// --db/$TQ_DB/./tasks.db chain, like every read command) — the server
// is config, not inheritance.
func cmdMCP(args []string) error {
	fs := flag.NewFlagSet("mcp", flag.ExitOnError)
	db := dbFlag(fs)
	_ = fs.Parse(args)

	store := mustOpenDB(resolveDB(*db))
	defer store.Close()

	server := mcpserver.New(store, version)

	if err := server.Serve(context.Background(), os.Stdin, os.Stdout); err != nil {
		return fmt.Errorf("tq mcp: %w", err)
	}

	return nil
}

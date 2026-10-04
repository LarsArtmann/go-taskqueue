package main

import (
	"flag"
	"testing"
)

// TestReadModelFlagDefaultsOn pins the S3 flip (ADR-0019 endgame P2):
// every serving command reads from the metaengine projection by default;
// --read-model=false is the explicit fallback to the store reads.
func TestReadModelFlagDefaultsOn(t *testing.T) {
	t.Parallel()

	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	got := readModelFlag(fs, "usage")

	if err := fs.Parse(nil); err != nil {
		t.Fatalf("parse defaults: %v", err)
	}

	if !*got {
		t.Fatal("read-model flag must default to true after the S3 flip")
	}

	if err := fs.Parse([]string{"--read-model=false"}); err != nil {
		t.Fatalf("parse explicit false: %v", err)
	}

	if *got {
		t.Fatal("--read-model=false must parse to false")
	}
}

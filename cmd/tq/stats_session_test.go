package main

import (
	"testing"
	"time"
)

func TestPrintSessionVolumeWindowBand(t *testing.T) {
	out := captureStdout(t, func() {
		printSessionVolume(10, 8, 3, 2, 168*time.Hour)
	})

	want := "sessions         10 opened / 8 closed\nsessions(168h0m0s) 3 opened / 2 closed\n"
	if out != want {
		t.Fatalf("printSessionVolume output = %q, want %q", out, want)
	}

	out = captureStdout(t, func() {
		printSessionVolume(10, 8, 0, 0, 168*time.Hour)
	})
	if out != "sessions         10 opened / 8 closed\n" {
		t.Fatalf("empty window printed a band line: %q", out)
	}

	out = captureStdout(t, func() {
		printSessionVolume(0, 0, 5, 4, 168*time.Hour)
	})
	if out != "sessions(168h0m0s) 5 opened / 4 closed\n" {
		t.Fatalf("window-only volume lost the band line: %q", out)
	}
}

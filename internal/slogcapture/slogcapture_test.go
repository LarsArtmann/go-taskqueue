package slogcapture

import (
	"context"
	"log/slog"
	"testing"
)

func TestHandlerRecordsWarnAndAbove(t *testing.T) {
	t.Parallel()

	h := &Handler{}

	if !h.Enabled(context.Background(), slog.LevelWarn) {
		t.Fatal("Enabled = false, want true for any level")
	}

	logger := slog.New(h)
	logger.Info("info line")
	logger.Warn("warn line")

	got := h.Records()

	if len(got) != 2 {
		t.Fatalf("records = %d, want 2 (info + warn)", len(got))
	}

	if got[1].Message != "warn line" {
		t.Fatalf("second record = %q, want the warn line", got[1].Message)
	}

	got[0].Message = "mutated"

	if h.Records()[0].Message == "mutated" {
		t.Fatal("Records returned the live slice, want a copy")
	}

	if h.WithAttrs(nil) != h || h.WithGroup("") != h {
		t.Fatal("WithAttrs/WithGroup must return the same handler")
	}

	if err := h.Handle(context.Background(), slog.Record{}); err != nil {
		t.Fatalf("Handle: %v", err)
	}
}

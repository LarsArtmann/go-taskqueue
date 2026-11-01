package webui

import (
	"testing"
	"time"
)

// TestStallTrackerQuietKeys pins the stuck-turn detector's clock (A10/
// V2.13) with a fake clock: a workspace goes quiet-candidate exactly at
// stuckTurnThreshold, a fresh frame clears it, and attach seeds the clock
// so never-emitting workspaces count from attach (not the epoch).
func TestStallTrackerQuietKeys(t *testing.T) {
	now := time.Unix(1700000000, 0)
	tracker := newStallTracker(func() time.Time { return now })

	if quiet := tracker.quietKeys(); len(quiet) != 0 {
		t.Fatalf("empty tracker reports quiet %v", quiet)
	}

	tracker.attach("ws-attached")
	tracker.touch("ws-live")

	now = now.Add(stuckTurnThreshold - time.Second)
	if quiet := tracker.quietKeys(); len(quiet) != 0 {
		t.Fatalf("one second before the bar, quiet = %v, want none", quiet)
	}

	// ws-live stays fresh (touched now); ws-attached has been silent since
	// attach and crosses the bar.
	tracker.touch("ws-live")
	now = now.Add(2 * time.Second)
	quiet := tracker.quietKeys()
	if len(quiet) != 1 || quiet[0] != "ws-attached" {
		t.Fatalf("attached-silent workspace must be the quiet candidate, got %v", quiet)
	}

	// A frame from the stuck workspace clears it for another window.
	tracker.touch("ws-attached")
	if quiet := tracker.quietKeys(); len(quiet) != 0 {
		t.Fatalf("fresh frame must clear the quiet candidate, got %v", quiet)
	}

	// The epoch seed: attach AFTER other entries exist still counts from
	// its own attach time, and an unattached key never appears.
	now = now.Add(stuckTurnThreshold + time.Minute)
	quiet = tracker.quietKeys()
	if len(quiet) != 2 { // ws-attached + ws-live, both past the bar now
		t.Fatalf("both silent workspaces must be candidates, got %v", quiet)
	}
}

package queue

import "testing"

func TestBandOf(t *testing.T) {
	t.Parallel()

	cases := []struct {
		priority int
		want     Band
	}{
		{-5, BandBacklog},
		{0, BandBacklog},
		{50, BandBacklog},
		{BacklogMax, BandBacklog},
		{HotMin, BandHot},
		{120, BandHot},
		{HotMax, BandHot},
		{MachineMin, BandMachine},
		{1000, BandMachine},
	}

	for _, tc := range cases {
		if got := BandOf(tc.priority); got != tc.want {
			t.Fatalf("BandOf(%d) = %s, want %s", tc.priority, got, tc.want)
		}
	}
}

// TestClampBacklog pins both directions of the band protection
// (ADR-0015 §1): nothing computed can cross into hot or machine, and
// nothing goes below the floor.
func TestClampBacklog(t *testing.T) {
	t.Parallel()

	cases := []struct {
		priority int
		want     int
	}{
		{-100, 0},
		{0, 0},
		{50, 50},
		{BacklogMax, BacklogMax},
		{HotMin, BacklogMax},     // hot floor clamped down
		{MachineMin, BacklogMax}, // machine floor clamped down
		{10000, BacklogMax},
	}

	for _, tc := range cases {
		if got := ClampBacklog(tc.priority); got != tc.want {
			t.Fatalf("ClampBacklog(%d) = %d, want %d", tc.priority, got, tc.want)
		}
	}
}

// TestAgingConstantsPinTheADR guards the invariant the ladder depends on:
// the aging cap must stay below the smallest marker-level gap (20) so
// aging reorders within a band but never across marker levels or bands.
func TestAgingConstantsPinTheADR(t *testing.T) {
	t.Parallel()

	if PriorityAgingDaysPerPoint <= 0 {
		t.Fatalf("PriorityAgingDaysPerPoint = %d, want > 0", PriorityAgingDaysPerPoint)
	}

	if PriorityAgingMaxBonus <= 0 || PriorityAgingMaxBonus >= 20 {
		t.Fatalf(
			"PriorityAgingMaxBonus = %d, want in (0, 20) — the marker-gap invariant (ADR-0015 §2/§4)",
			PriorityAgingMaxBonus,
		)
	}
}

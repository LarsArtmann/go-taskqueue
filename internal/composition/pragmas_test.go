package composition

import (
	"slices"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/config"
)

// TestProjectionPragmaUnionSingleSource pins TODO row 86: the composed
// serve path's engine pragmas (ProjectionEnginePragmas over the resolved
// deployment) and readmodel's self-open caller path both derive from the
// ONE config.Deployment builder — for every sync tier the composed helper
// returns exactly the builder's set (the readmodel-side twin,
// projectionHomeCallerPragmas, pins the caller half in
// internal/readmodel). A pragma edit landing on one side only fails here
// instead of rotting the "same union" claim as prose.
func TestProjectionPragmaUnionSingleSource(t *testing.T) {
	t.Parallel()

	for _, tier := range []config.SyncPolicy{config.SyncFull, config.SyncNormal, config.SyncOff} {
		d := config.SQLite("")
		d.SyncPolicy = tier

		got := ProjectionEnginePragmas(d)
		want := d.ProjectionPragmas()

		if len(got) == 0 {
			t.Fatalf("tier %s: composed pragma set is empty — the helper must derive, not stub", tier)
		}

		if !slices.Equal(got, want) {
			t.Fatalf("tier %s: composed pragmas %v != builder pragmas %v — the union forked", tier, got, want)
		}
	}
}

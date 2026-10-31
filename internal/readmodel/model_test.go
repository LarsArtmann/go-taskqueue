package readmodel

import (
	"slices"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/config"
)

func TestProjectionHomeCallerPragmasHonorSyncEnv(t *testing.T) {
	tests := []struct {
		env  string
		want []string
	}{
		{"", []string{"synchronous=NORMAL", "cache_size=-32768"}},
		{"full", []string{"synchronous=FULL", "cache_size=-32768"}},
		{"OFF", []string{"synchronous=OFF", "cache_size=-32768"}},
		{" garbage ", config.SQLite("").ProjectionPragmas()},
	}

	for _, tt := range tests {
		t.Run("TQ_SQLITE_SYNC="+tt.env, func(t *testing.T) {
			t.Setenv(config.EnvSyncPolicy, tt.env)

			got := projectionHomeCallerPragmas()
			if !slices.Equal(got, tt.want) {
				t.Fatalf("pragmas = %v, want %v", got, tt.want)
			}
		})
	}
}


// TestProjectionHomeCallerPragmasMatchBuilder pins the CALLER half of
// TODO row 86's same-union invariant (the composed half lives in
// internal/composition): for every sync tier the self-open fallback's
// pragma union is exactly what config.Deployment's builder yields at
// that tier — honored through TQ_SQLITE_SYNC, never a local literal.
func TestProjectionHomeCallerPragmasMatchBuilder(t *testing.T) {
	for _, tier := range []config.SyncPolicy{config.SyncFull, config.SyncNormal, config.SyncOff} {
		t.Run(string(tier), func(t *testing.T) {
			t.Setenv(config.EnvSyncPolicy, string(tier))

			want := config.SQLite("")
			want.SyncPolicy = tier

			got := projectionHomeCallerPragmas()
			builder := want.ProjectionPragmas()

			if len(got) == 0 || len(builder) == 0 {
				t.Fatalf("tier %s: empty pragma set (caller=%v builder=%v)", tier, got, builder)
			}

			if !slices.Equal(got, builder) {
				t.Fatalf("tier %s: caller pragmas %v != builder pragmas %v", tier, got, builder)
			}
		})
	}
}

package readmodel

import (
	"database/sql"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	_ "modernc.org/sqlite" // the driver the projection-home engine rides (probe opens it directly)

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

// TestSelfOpenPragmasApplyEnvSyncTier pins TODO row 844: the
// computation-to-application gap — the self-open path's pragma strings
// (tied to the builder by TestProjectionHomeCallerPragmasMatchBuilder)
// applied through the SAME sqlite driver the engine rides actually set
// the connection's synchronous level to the env tier. Only the list was
// pinned before; the application was untested.
func TestSelfOpenPragmasApplyEnvSyncTier(t *testing.T) {
	expected := map[config.SyncPolicy]int{
		config.SyncOff:    0,
		config.SyncNormal: 1,
		config.SyncFull:   2,
	}

	for tier, want := range expected {
		t.Run(string(tier), func(t *testing.T) {
			t.Setenv(config.EnvSyncPolicy, string(tier))

			pragmas := projectionHomeCallerPragmas()
			if len(pragmas) == 0 {
				t.Fatalf("tier %s: no pragmas to apply", tier)
			}

			parts := make([]string, 0, len(pragmas))
			for _, p := range pragmas {
				parts = append(parts, "_pragma="+p)
			}

			db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "probe.db")+"?"+strings.Join(parts, "&"))
			if err != nil {
				t.Fatalf("open probe: %v", err)
			}

			t.Cleanup(func() { _ = db.Close() })

			var got int
			if err := db.QueryRowContext(t.Context(), "PRAGMA synchronous").Scan(&got); err != nil {
				t.Fatalf("read applied synchronous: %v", err)
			}

			if got != want {
				t.Fatalf("tier %s: applied PRAGMA synchronous = %d, want %d (pragmas %v)", tier, got, want, pragmas)
			}
		})
	}
}

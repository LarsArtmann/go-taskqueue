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

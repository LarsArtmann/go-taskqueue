package config

import (
	"reflect"
	"testing"
)

func TestFromFlagsSelectsDriver(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		store      string
		wantDriver Driver
		wantPath   string
		wantDSN    string
	}{
		{
			name:       "bare path is sqlite",
			store:      "/tmp/tq/queue.db",
			wantDriver: DriverSQLite,
			wantPath:   "/tmp/tq/queue.db",
		},
		{
			name:       "postgres url selects postgres",
			store:      "postgres://tq@localhost/tq",
			wantDriver: DriverPostgres,
			wantDSN:    "postgres://tq@localhost/tq",
		},
		{
			name:       "postgresql url selects postgres",
			store:      "postgresql://tq@localhost/tq",
			wantDriver: DriverPostgres,
			wantDSN:    "postgresql://tq@localhost/tq",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			d, err := FromFlags(tt.store, true)
			if err != nil {
				t.Fatalf("FromFlags(%q): %v", tt.store, err)
			}

			if d.Driver != tt.wantDriver {
				t.Fatalf("driver = %q, want %q", d.Driver, tt.wantDriver)
			}

			if d.DBPath != tt.wantPath {
				t.Fatalf("dbPath = %q, want %q", d.DBPath, tt.wantPath)
			}

			if d.DSN != tt.wantDSN {
				t.Fatalf("dsn = %q, want %q", d.DSN, tt.wantDSN)
			}

			if !d.ReadModelEnabled {
				t.Fatal("read model flag did not carry")
			}

			if d.SyncPolicy != SyncNormal {
				t.Fatalf("default sync policy = %q, want normal", d.SyncPolicy)
			}
		})
	}
}

func TestSyncPolicyEnvMerge(t *testing.T) {
	tests := []struct {
		name    string
		env     string
		want    SyncPolicy
		wantErr bool
	}{
		{name: "empty means normal", env: "", want: SyncNormal},
		{name: "full is the strict tier", env: "full", want: SyncFull},
		{name: "uppercase accepted", env: " FULL ", want: SyncFull},
		{name: "off accepted", env: "off", want: SyncOff},
		{name: "garbage rejected", env: "sometimes", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(EnvSyncPolicy, tt.env)

			got, err := SyncPolicyFromEnv()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("SyncPolicyFromEnv(%q) = %q, want error", tt.env, got)
				}

				return
			}

			if err != nil {
				t.Fatalf("SyncPolicyFromEnv(%q): %v", tt.env, err)
			}

			if got != tt.want {
				t.Fatalf("SyncPolicyFromEnv(%q) = %q, want %q", tt.env, got, tt.want)
			}

			d, err := FromFlags("/tmp/x.db", false)
			if err != nil {
				t.Fatalf("FromFlags: %v", err)
			}

			if d.SyncPolicy != tt.want {
				t.Fatalf("env merge: policy = %q, want %q", d.SyncPolicy, tt.want)
			}
		})
	}
}

func TestValidateShapeInvariants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		d       Deployment
		wantErr bool
	}{
		{name: "sqlite ok", d: SQLite("q.db"), wantErr: false},
		{name: "sqlite without path", d: Deployment{Driver: DriverSQLite, SyncPolicy: SyncNormal}, wantErr: true},
		{name: "sqlite with a DSN", d: Deployment{Driver: DriverSQLite, DBPath: "q.db", DSN: "postgres://x", SyncPolicy: SyncNormal}, wantErr: true},
		{name: "postgres ok", d: Postgres("postgres://x"), wantErr: false},
		{name: "postgres without DSN", d: Deployment{Driver: DriverPostgres, SyncPolicy: SyncNormal}, wantErr: true},
		{name: "postgres with a path", d: Deployment{Driver: DriverPostgres, DSN: "postgres://x", DBPath: "q.db"}, wantErr: true},
		{name: "unknown driver", d: Deployment{Driver: "oracle", DBPath: "x"}, wantErr: true},
		{name: "unknown sync policy", d: Deployment{Driver: DriverSQLite, DBPath: "q.db", SyncPolicy: "sometimes"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.d.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestPragmasAreTheOneLiteral pins the pragma builders' exact output:
// the queue home renders the full DSN union (order load-bearing — WAL
// before synchronous) and the projection home renders the caller-pragma
// pair, both derived from the same sync tier.
func TestPragmasAreTheOneLiteral(t *testing.T) {
	t.Parallel()

	d := SQLite("q.db")
	d.SyncPolicy = SyncFull

	wantQueue := []string{
		"journal_mode(WAL)",
		"busy_timeout(5000)",
		"foreign_keys(1)",
		"synchronous(FULL)",
		"temp_store(MEMORY)",
		"cache_size(-32768)",
		"journal_size_limit(8388608)",
	}
	if got := d.QueuePragmas(); !reflect.DeepEqual(got, wantQueue) {
		t.Fatalf("queue pragmas drifted:\n got %v\nwant %v", got, wantQueue)
	}

	wantProj := []string{"synchronous=FULL", "cache_size=-32768"}
	if got := d.ProjectionPragmas(); !reflect.DeepEqual(got, wantProj) {
		t.Fatalf("projection pragmas drifted:\n got %v\nwant %v", got, wantProj)
	}

	def := SQLite("q.db")
	if got := def.QueuePragmas(); got[3] != "synchronous(NORMAL)" {
		t.Fatalf("default tier = %q, want synchronous(NORMAL)", got[3])
	}
}

package main

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCheckProjectsDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // os.UserHomeDir reads USERPROFILE on Windows

	tests := []struct {
		name string
		dir  string
		// wantErrs are substrings the refusal must carry; empty means the
		// guard must accept the directory.
		wantErrs []string
	}{
		{
			name:     "filesystem root refused",
			dir:      "/",
			wantErrs: []string{"whole filesystem", "name a directory"},
		},
		{
			name: "empty accepted (callers reject empty only when repos is empty too)",
			dir:  "",
		},
		{
			name:     "home directory refused",
			dir:      home,
			wantErrs: []string{"is your home directory", home + "/projects"},
		},
		{
			name: "projects subdir of home accepted",
			dir:  filepath.Join(home, "projects"),
		},
		{
			name: "unrelated directory accepted",
			dir:  t.TempDir(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkProjectsDir(tt.dir)

			if len(tt.wantErrs) == 0 {
				if err != nil {
					t.Fatalf("checkProjectsDir(%q) = %v, want nil", tt.dir, err)
				}

				return
			}

			if err == nil {
				t.Fatalf("checkProjectsDir(%q) = nil, want refusal", tt.dir)
			}

			for _, want := range tt.wantErrs {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("refusal %q missing %q", err, want)
				}
			}
		})
	}
}

func TestAllReposAbsolute(t *testing.T) {
	// filepath.IsAbs is platform-defined: "/srv/a" is absolute on POSIX but
	// relative on Windows (no drive letter). Build the absolute specs from
	// the platform's own notion so both runners exercise the same contract.
	absA, absB, absC := "/srv/a", "/srv/b", "/srv/c"
	if runtime.GOOS == "windows" {
		absA, absB, absC = `C:\srv\a`, `C:\srv\b`, `C:\srv\c`
	}

	tests := []struct {
		name string
		spec string
		want bool
	}{
		{name: "empty spec", spec: "", want: false},
		{name: "single absolute", spec: filepath.Join(absA, "foo"), want: true},
		{name: "all absolute", spec: absA + ", " + absB + " ," + absC, want: true},
		{name: "relative entry", spec: absA + ",foo", want: false},
		{name: "only relative", spec: "foo,bar", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := allReposAbsolute(tt.spec); got != tt.want {
				t.Fatalf("allReposAbsolute(%q) = %v, want %v", tt.spec, got, tt.want)
			}
		})
	}
}

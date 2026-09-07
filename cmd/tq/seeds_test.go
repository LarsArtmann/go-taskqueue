package main

import (
	"path/filepath"
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

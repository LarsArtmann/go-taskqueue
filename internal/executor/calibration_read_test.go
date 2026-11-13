package executor

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSummarizeCalibration(t *testing.T) {
	tests := []struct {
		name    string
		dataset string
		check   func(t *testing.T, s CalibrationSummary)
	}{
		{
			name:    "empty file is zero rows, not an error",
			dataset: "",
			check: func(t *testing.T, s CalibrationSummary) {
				t.Helper()

				if s.Rows != 0 || len(s.ByKind) != 0 {
					t.Fatalf("Rows=%d ByKind=%d, want 0/0", s.Rows, len(s.ByKind))
				}
			},
		},
		{
			name: "malformed line counts, valid rows still sum",
			dataset: `{"task_id":"t1","kind":"review","at":1,"verdict":{"verdict":"approve"},"commit_count":2,"cost_usd":1.5}` + "\n" +
				"not json at all\n" +
				`{"task_id":"t2","kind":"review","at":2,"verdict":{"verdict":"request_changes"}}` + "\n",
			check: func(t *testing.T, s CalibrationSummary) {
				t.Helper()

				if s.Rows != 2 || s.Malformed != 1 {
					t.Fatalf("Rows=%d Malformed=%d, want 2/1", s.Rows, s.Malformed)
				}

				if len(s.ByKind) != 1 || s.ByKind[0].Kind != "review" {
					t.Fatalf("ByKind=%+v, want one review entry", s.ByKind)
				}

				st := s.ByKind[0]
				if st.VerdictValues["approve"] != 1 || st.VerdictValues["request_changes"] != 1 {
					t.Fatalf("VerdictValues=%+v", st.VerdictValues)
				}

				if st.ApproveRows != 1 || st.ApproveWithCommits != 1 {
					t.Fatalf("ApproveRows=%d ApproveWithCommits=%d, want 1/1", st.ApproveRows, st.ApproveWithCommits)
				}

				if st.TotalCostUSD != 1.5 {
					t.Fatalf("TotalCostUSD=%v, want 1.5", st.TotalCostUSD)
				}
			},
		},
		{
			name: "attestation refusals and no-receipt rows classify",
			dataset: `{"task_id":"t1","kind":"agent","at":1,"verify":{"command":"make","rc":1,"ok":0,"fail":2},"attestation":{"ok":false,"refusal":"count-mismatch"}}` + "\n" +
				`{"task_id":"t2","kind":"agent","at":2,"verify":{"command":"make","rc":0,"ok":1,"fail":0}}` + "\n" +
				`{"task_id":"t3","kind":"agent","at":3,"attestation":{"ok":true}}` + "\n" +
				`{"task_id":"t4","kind":"status","at":4,"verdict":{"report":"docs/status/x.md","next_items":3}}` + "\n",
			check: func(t *testing.T, s CalibrationSummary) {
				t.Helper()

				if s.Rows != 4 {
					t.Fatalf("Rows=%d, want 4", s.Rows)
				}

				agent := s.ByKind[0]
				if agent.Kind != "agent" || agent.AttestedOK != 1 {
					t.Fatalf("agent stats=%+v, want AttestedOK=1", agent)
				}

				if agent.AttestationRefusals["count-mismatch"] != 1 ||
					agent.AttestationRefusals[AttestationNoReceipt] != 1 {
					t.Fatalf("refusals=%+v, want one count-mismatch and one no-receipt", agent.AttestationRefusals)
				}

				status := s.ByKind[1]
				if status.Kind != "status" || status.VerdictValues["(none)"] != 1 {
					t.Fatalf("status stats=%+v, want one (none) verdict", status)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), CalibrationDataset)
			if err := os.WriteFile(path, []byte(tt.dataset), 0o600); err != nil {
				t.Fatalf("write dataset: %v", err)
			}

			got, err := SummarizeCalibration(path)
			if err != nil {
				t.Fatalf("SummarizeCalibration: %v", err)
			}

			if !strings.HasSuffix(got.Path, CalibrationDataset) {
				t.Fatalf("Path=%q, want dataset suffix", got.Path)
			}

			tt.check(t, got)
		})
	}
}

func TestSummarizeCalibrationMissingFile(t *testing.T) {
	if _, err := SummarizeCalibration(filepath.Join(t.TempDir(), "absent.jsonl")); err == nil {
		t.Fatal("missing dataset must error, not summarize zero")
	}
}

func TestTrimCalibrationDataset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, CalibrationDataset)

	row := `{"task_id":"t1","kind":"agent","at":1}` + "\n"

	var seed strings.Builder
	for range 100 {
		seed.WriteString(row)
	}

	if err := os.WriteFile(path, []byte(seed.String()), 0o600); err != nil {
		t.Fatalf("seed dataset: %v", err)
	}

	trimmed, err := TrimCalibrationDataset(dir, 2048)
	if err != nil {
		t.Fatalf("TrimCalibrationDataset: %v", err)
	}

	if !trimmed {
		t.Fatal("oversized dataset must report a trim")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reread: %v", err)
	}

	if int64(len(data)) > 2048+int64(len(row)) {
		t.Fatalf("trimmed size %d exceeds cap+row", len(data))
	}

	if !bytes.HasPrefix(data, []byte(`{"task_id"`)) {
		t.Fatal("trim must land on a line boundary, never a torn row")
	}

	if !bytes.HasSuffix(data, []byte(row)) {
		t.Fatal("trim must keep the newest rows")
	}

	if trimmed, err := TrimCalibrationDataset(dir, 1<<30); err != nil || trimmed {
		t.Fatalf("under-cap trim=%v err=%v, want false/nil", trimmed, err)
	}

	if trimmed, err := TrimCalibrationDataset(t.TempDir(), 1024); err != nil || trimmed {
		t.Fatalf("absent-file trim=%v err=%v, want false/nil", trimmed, err)
	}
}

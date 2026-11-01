//go:build unix

package executor

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"reflect"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

// TestEnrichFailureEvidenceFiles pins the failure-forensics merge contract
// (row 806 slice 2): derived footer-commit files UNION into the recorded
// evidence — never replacing recorded entries — and every no-op path (no
// sink, no recorded evidence, empty derivation) leaves the evidence as-is.
func TestEnrichFailureEvidenceFiles(t *testing.T) {
	id := task.ID("000001a1enrich00000000000000000000001")

	newRepo := func() (string, func(name, body string), func(msg, trailer string)) {
		repo, write, commit := gitFixtureRepo(t)
		write("a.go", "one")
		commit("work one", TaskTrailer+": "+id.String())

		return repo, write, commit
	}

	decode := func(t *testing.T, raw jsontext.Value) FailureEvidence {
		t.Helper()

		var evidence FailureEvidence
		if err := json.Unmarshal(raw, &evidence); err != nil {
			t.Fatalf("decode recorded evidence: %v", err)
		}

		return evidence
	}

	t.Run("unions derived files into recorded evidence", func(t *testing.T) {
		repo, _, _ := newRepo()

		ctx, sink := NewSink(context.Background())
		setFailureEvidence(ctx, FailureEvidence{
			Stage:        "verify",
			VerifyStage:  VerifyStageTest,
			ExitCode:     1,
			Tail:         "boom",
			FilesChanged: []string{"z.go"},
		})

		EnrichFailureEvidenceFiles(ctx, repo, id)

		got := decode(t, sink.Failure())
		want := FailureEvidence{
			Stage:        "verify",
			VerifyStage:  VerifyStageTest,
			ExitCode:     1,
			Tail:         "boom",
			FilesChanged: []string{"a.go", "z.go"},
		}

		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %+v, want %+v (union merge, recorded fields kept)", got, want)
		}
	})

	t.Run("no sink is a no-op", func(t *testing.T) {
		repo, _, _ := newRepo()

		EnrichFailureEvidenceFiles(context.Background(), repo, id)
	})

	t.Run("no recorded evidence stays unset", func(t *testing.T) {
		repo, _, _ := newRepo()

		ctx, sink := NewSink(context.Background())

		EnrichFailureEvidenceFiles(ctx, repo, id)

		if sink.Failure() != nil {
			t.Fatalf("evidence recorded from nothing: %s", sink.Failure())
		}
	})

	t.Run("empty derivation leaves evidence untouched", func(t *testing.T) {
		ctx, sink := NewSink(context.Background())
		SetFailureEvidence(ctx, "agent", errors.New("spawn died"), "tail")

		EnrichFailureEvidenceFiles(ctx, t.TempDir(), id)

		got := decode(t, sink.Failure())
		want := FailureEvidence{Stage: "agent", ExitCode: 0, Tail: "tail"}

		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %+v, want %+v (non-git dir derives nothing)", got, want)
		}
	})
}

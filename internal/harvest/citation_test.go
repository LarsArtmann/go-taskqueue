package harvest

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initGitRepo makes a real git repo with one reachable commit and one
// dangling commit (amended away, still resolvable through the reflog),
// returning (repo, reachable, dangling).
func initGitRepo(t *testing.T) (string, string, string) {
	t.Helper()

	repo := t.TempDir()

	git := func(args ...string) string {
		t.Helper()

		cmd := exec.Command("git", args...)
		cmd.Dir = repo

		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}

		return strings.TrimSpace(string(out))
	}

	for _, args := range [][]string{
		{"init", "-b", "main"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "test"},
	} {
		git(args...)
	}

	if err := os.WriteFile(filepath.Join(repo, "f.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	git("add", ".")
	git("commit", "-m", "dangle me")
	dangling := git("rev-parse", "HEAD")

	if err := os.WriteFile(filepath.Join(repo, "f.txt"), []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	git("add", ".")
	git("commit", "--amend", "--no-edit")
	reachable := git("rev-parse", "HEAD")

	return repo, reachable, dangling
}

func TestDanglingSHAs(t *testing.T) {
	repo, reachable, dangling := initGitRepo(t)
	ctx := context.Background()

	if got := danglingSHAs(ctx, repo, "close out commit "+reachable+" now"); len(got) != 0 {
		t.Fatalf("reachable SHA must not be flagged: %v", got)
	}

	if got := danglingSHAs(ctx, repo, "close out commit "+dangling); len(got) != 1 || got[0] != dangling {
		t.Fatalf("danglingSHAs = %v, want [%s]", got, dangling)
	}

	// Repeated citations are reported once; unresolvable hex words are prose.
	text := "commit " + dangling + " (also " + dangling + ") plus cafe babe deadbeef12"
	if got := danglingSHAs(ctx, repo, text); len(got) != 1 {
		t.Fatalf("danglingSHAs = %v, want exactly [%s]", got, dangling)
	}

	if got := danglingSHAs(ctx, t.TempDir(), dangling); len(got) != 0 {
		t.Fatalf("non-repo dir must flag nothing, got %v", got)
	}
}

func TestBuildPayloadAppendsCitationWarning(t *testing.T) {
	repo, _, dangling := initGitRepo(t)
	h := New(nil, Config{PromptTemplate: "repo {{REPO}} item: {{ITEM}}"})

	payload, err := h.buildPayload(context.Background(), Item{
		Repo:     repo,
		RepoName: filepath.Base(repo),
		Text:     "close out window commit " + dangling,
	}, "repo {{REPO}} item: {{ITEM}}", "key-1")
	if err != nil {
		t.Fatalf("buildPayload: %v", err)
	}

	var decoded harvestPayload
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode payload: %v", err)
	}

	if !strings.Contains(decoded.Prompt, "CITATION CHECK") || !strings.Contains(decoded.Prompt, dangling) {
		t.Fatalf("prompt must carry the citation warning naming %s, got: %s", dangling, decoded.Prompt)
	}

	if !strings.Contains(decoded.Prompt, "item: close out window commit") {
		t.Fatalf("template placeholders must still render, got: %s", decoded.Prompt)
	}
}

func TestBuildPayloadSkipsWarningWithoutDangles(t *testing.T) {
	repo, reachable, _ := initGitRepo(t)
	h := New(nil, Config{PromptTemplate: "item: {{ITEM}}"})

	payload, err := h.buildPayload(context.Background(), Item{
		Repo:     repo,
		RepoName: filepath.Base(repo),
		Text:     "clean item citing " + reachable,
	}, "item: {{ITEM}}", "key-1")
	if err != nil {
		t.Fatalf("buildPayload: %v", err)
	}

	var decoded harvestPayload
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode payload: %v", err)
	}

	if strings.Contains(decoded.Prompt, "CITATION CHECK") {
		t.Fatalf("clean text must not carry the warning, got: %s", decoded.Prompt)
	}
}

func TestBuildBatchPayloadAppendsCitationWarning(t *testing.T) {
	repo, _, dangling := initGitRepo(t)
	h := New(nil, Config{BatchPromptTemplate: "work {{ITEMS}}"})

	run := []Item{
		{Repo: repo, RepoName: filepath.Base(repo), Text: "item one citing " + dangling},
		{Repo: repo, RepoName: filepath.Base(repo), Text: "item two"},
	}

	payload, err := h.buildBatchPayload(context.Background(), run)
	if err != nil {
		t.Fatalf("buildBatchPayload: %v", err)
	}

	var decoded harvestPayload
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode payload: %v", err)
	}

	if !strings.Contains(decoded.Prompt, "CITATION CHECK") || !strings.Contains(decoded.Prompt, dangling) {
		t.Fatalf("batch prompt must carry the citation warning naming %s, got: %s", dangling, decoded.Prompt)
	}
}

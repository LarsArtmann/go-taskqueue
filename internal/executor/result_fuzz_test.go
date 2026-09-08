package executor

import (
	"slices"
	"strings"
	"testing"
)

// FuzzExtractResultPayload: the TQ_RESULT regex and the JSON decode behind it
// run over fully untrusted agent output — the "agent" is an LLM driving a
// shell, so its stdout is hostile territory, not a protocol. The parser must
// never panic, must be deterministic, and may only report ok when the output
// really carried a TQ_RESULT marker line.
func FuzzExtractResultPayload(f *testing.F) {
	f.Add("TQ_RESULT: {\"files_changed\":[\"a.go\",\"b.go\"],\"commit_sha\":\"abc123\"}\n")
	f.Add("noise before\nTQ_RESULT: {\"commit_sha\":\"deadbeef\"}\nafter\n")
	f.Add("TQ_RESULT: {broken json}\n")
	f.Add("tq_result: {\"files_changed\":[]}\n")
	f.Add("   TQ_RESULT:   {\"files_changed\":[\" spaced \"]}  \r\n")
	f.Add("mid-line TQ_RESULT: {\"a\":1}\n")
	f.Add("TQ_RESULT:{\"files_changed\":[\"nospace\"],\"commit_sha\":\"s\"}\n")
	f.Add("TQ_RESULT: {\"files_changed\":[\"未命名.go\",\"дир/файл\"],\"commit_sha\":\"\"}\n")
	f.Add("TQ_RESULT: {\"files_changed\":[\"a\"],\"commit_sha\":\"s\",\"extra\":true}\n")
	f.Add("TQ_RESULT: {\"files_changed\":[\"quote\\\"name\"],\"commit_sha\":\"sha\"}\n")
	f.Add("TQ_RESULT: {}\n")
	f.Add("TQ_RESULT: [\"not\",\"an\",\"object\"]\n")
	f.Add("TQ_RESULT: {\"files_changed\":\"not-an-array\"}\n")
	f.Add("TQ_RESULT: {unclosed\n")
	f.Add("TQ_RESULT: {\"files_changed\":[\"a\"]} trailing text\n")
	f.Add("TQ_RESULT: {\"files_changed\":[\"a\"]} TQ_RESULT: {\"commit_sha\":\"x\"}\n")
	f.Add("TQ_RESULT: {\n\"files_changed\":[\"multi-line\"]\n}\n")
	f.Add("\xEF\xBB\xBFTQ_RESULT: {\"files_changed\":[\"bom\"]}\n")
	f.Add(strings.Repeat("TQ_RESULT: {\"files_changed\":[\"f\"],\"commit_sha\":\"s\"}\n", 100))
	f.Add("TQ_RESULT: " + strings.Repeat("{\"a\":", 100) + "1" + strings.Repeat("}", 100) + "\n")
	f.Add(
		"TQ_RESULT: {\"files_changed\":[\"" + strings.Repeat(
			"x",
			10000,
		) + "\"],\"commit_sha\":\"" + strings.Repeat(
			"y",
			40,
		) + "\"}\n",
	)

	f.Fuzz(func(t *testing.T, output string) {
		files1, sha1, ok1 := ExtractResultPayload(output)
		files2, sha2, ok2 := ExtractResultPayload(output)

		if ok1 != ok2 || sha1 != sha2 || !slices.Equal(files1, files2) {
			t.Fatalf("non-deterministic parse: (%v, %q, %v) vs (%v, %q, %v)",
				files1, sha1, ok1, files2, sha2, ok2)
		}

		if !ok1 {
			return
		}

		// ok means resultLineRe matched, so the marker must literally be in
		// the output (the pattern is case-insensitive — compare lowered).
		if !strings.Contains(strings.ToLower(output), "tq_result:") {
			t.Fatalf("ok reported without any TQ_RESULT marker: %q", output)
		}
	})
}

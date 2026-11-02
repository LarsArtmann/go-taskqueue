package executor

import (
	"encoding/json/jsontext"
	"regexp"
	"testing"
)

// testVerdictLine bridges the parser-table fixtures to the file channel:
// the tables were written against the retired stdout TQ_RESULT line, so
// the helper strips the marker and hands the bare JSON to the parser —
// exactly the content the verdict file holds now.
var testVerdictLineRe = regexp.MustCompile(`(?im)^\s*TQ_RESULT:\s*(\{.+\})\s*$`)

func testVerdictLine(t *testing.T, output string) jsontext.Value {
	t.Helper()

	if m := testVerdictLineRe.FindAllStringSubmatch(output, -1); len(m) > 0 {
		return jsontext.Value(m[len(m)-1][1])
	}

	return jsontext.Value(output)
}

// stubVerdict renders a stub-agent shell line recording the verdict in the
// per-run file channel — the sole channel since the stdout line retired.
func stubVerdict(v string) string {
	return "printf '%s' '" + v + "' > \"$TQ_RESULT_FILE\""
}

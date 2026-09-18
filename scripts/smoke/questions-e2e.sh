#!/usr/bin/env bash
# End-to-end verification of the PapDashboard questions loop (no docker
# needed): a stub agent parks its task with `tq ask`, the bridge forwards
# the question to a stub dashboard, the stub answers it, the answer poller
# routes the ruling back (payload injection + unblock), and the resumed run
# — which SEES the rendered answer in its prompt — completes the task with
# no attempt burned by the park.
#
# Against a REAL dashboard: PAP_URL=https://pap.example ./scripts/smoke/questions-e2e.sh
set -euo pipefail
cd "$(dirname "$0")/../.."
REPO_ROOT="$(pwd)"

PAP_URL="${PAP_URL:-}"
REAL_MODE=""
[ -n "$PAP_URL" ] && REAL_MODE=1
TMP="$(mktemp -d)"
cleanup() {
	for pid in "${WORKER_PID:-}" "${STUB_PID:-}"; do
		[ -n "$pid" ] && kill "$pid" 2>/dev/null
	done
	rm -rf "$TMP"
}
trap cleanup EXIT

free_port() {
	python3 - <<'PY'
import socket
with socket.socket() as s:
    s.bind(("127.0.0.1", 0))
    print(s.getsockname()[1])
PY
}

STUB_PORT="${PAP_SMOKE_PORT:-$(free_port)}"

if [ -n "${TQ_BIN:-}" ]; then
	echo "== using prebuilt tq: $TQ_BIN"
	cp "$TQ_BIN" "$TMP/tq"
	chmod +x "$TMP/tq"
else
	echo "== build tq"
	"$REPO_ROOT/scripts/build-tq.sh" "$TMP/tq"
fi
"$TMP/tq" version

INGEST_LOG="$TMP/ingest.log"
if [ -z "$PAP_URL" ]; then
	echo "== start stub dashboard (answers every question.asked with 'yes')"
	cat >"$TMP/stub.py" <<'PYEOF'
import json
import sys
import threading
from datetime import datetime, timezone
from http.server import BaseHTTPRequestHandler, HTTPServer

LOG, PORT = sys.argv[1], int(sys.argv[2])
QUESTIONS = []
LOCK = threading.Lock()

class H(BaseHTTPRequestHandler):
    def do_POST(self):
        n = int(self.headers.get("Content-Length", 0))
        raw = self.rfile.read(n).decode()
        with open(LOG, "a") as f:
            f.write(self.path + " " + self.headers.get("Idempotency-Key", "") + " " + raw + "\n")
        try:
            doc = json.loads(raw)
        except ValueError:
            doc = {}
        with LOCK:
            if doc.get("type") == "question.asked":
                p = doc.get("payload", {})
                QUESTIONS.append({
                    "id": "pap-%d" % len(QUESTIONS),
                    "body": p.get("body", ""),
                    "title": p.get("title", ""),
                    "sourceApp": p.get("sourceApp", ""),
                    "isAnswered": True,
                    "answer": "yes",
                    "answeredAt": datetime.now(timezone.utc).isoformat(),
                })
        self.send_response(200); self.end_headers(); self.wfile.write(b"ok")

    def do_GET(self):
        if self.path.startswith("/api/questions"):
            with LOCK:
                data = list(QUESTIONS)
            payload = json.dumps({"body": {"data": data}}).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(payload)))
            self.end_headers()
            self.wfile.write(payload)
            return
        self.send_response(404); self.end_headers()

    def log_message(self, *a): pass

HTTPServer(("127.0.0.1", PORT), H).serve_forever()
PYEOF
	python3 "$TMP/stub.py" "$INGEST_LOG" "$STUB_PORT" &
	STUB_PID=$!
	for _ in $(seq 1 20); do
		kill -0 "$STUB_PID" 2>/dev/null || {
			echo "FAIL: stub dashboard exited (port $STUB_PORT taken? override with PAP_SMOKE_PORT)"
			exit 1
		}
		python3 -c "import socket; socket.create_connection((\"127.0.0.1\", $STUB_PORT), 0.2)" 2>/dev/null && break
		sleep 0.25
	done
	PAP_URL="http://127.0.0.1:$STUB_PORT"
fi

export TQ_DB="$TMP/tasks.db"

echo "== prepare repo + stub agent (asks once; completes once the answer is rendered)"
REPO="$TMP/projects/askrepo"
mkdir -p "$REPO"

ASKED_FLAG="$TMP/asked.flag"
DONE_FLAG="$TMP/done.flag"
# The prompt the executor hands the agent carries its {{TASK_ID}} already
# substituted, so the stub can ask for the RIGHT task without any
# enqueue-time knowledge of the id.
cat >"$TMP/stub-agent" <<STUB
#!/bin/sh
prompt="\$*"
case "\$prompt" in
  *"Answers from the owner"*)
	echo "completed-with-answer"
	touch "$DONE_FLAG"
	exit 0
	;;
esac
id=\$(printf '%s\n' "\$prompt" | tr ' ' '\n' | sed -n 's/^TASK:\([0-9a-zA-Z]*\)\$/\1/p' | head -1)
if [ -z "\$id" ]; then
	echo "FAIL: stub agent could not extract the task id from: \$prompt" >&2
	exit 1
fi
[ -f "$ASKED_FLAG" ] && { echo "FAIL: asked twice without seeing the answer" >&2; exit 1; }
touch "$ASKED_FLAG"
exec "$TMP/tq" ask --task "\$id" --db "$TQ_DB" "Proceed with option X?"
STUB
chmod +x "$TMP/stub-agent"

echo "== start worker with bridge + answer poller -> $PAP_URL"
TQ_AGENT_BIN="$TMP/stub-agent" timeout 90 "$TMP/tq" worker --agents \
	--alert-url "$PAP_URL" --alert-api-key smoke-key --alert-poll 1s \
	--poll 500ms --projects-dir "$TMP/projects" \
	>"$TMP/worker.log" 2>&1 &
WORKER_PID=$!

echo "== enqueue the asking task (prompt carries its own id via the executor placeholder)"
TASK_ID="$("$TMP/tq" enqueue --type agent --project ask-e2e \
	--payload "{\"repo\":\"$REPO\",\"prompt\":\"decide TASK:{{TASK_ID}}\",\"require_clean\":false,\"timeout_minutes\":2}")"
echo "   task $TASK_ID"

echo "== wait for the park (worker requeued the ask without burning an attempt)"
parked=1
for _ in $(seq 1 40); do
	if grep -q "parked on owner question" "$TMP/worker.log" 2>/dev/null; then
		parked=0
		break
	fi
	sleep 0.5
done
[ "$parked" -eq 0 ] || {
	echo "FAIL: task never parked on the question"
	cat "$TMP/worker.log"
	exit 1
}
echo "   parked OK"

ATTEMPTS="$("$TMP/tq" show --db "$TQ_DB" "$TASK_ID" | python3 -c 'import json,sys; print(json.load(sys.stdin)["task"]["attempts"])')"
[ "$ATTEMPTS" = "0" ] || {
	echo "FAIL: park burned an attempt (attempts=$ATTEMPTS)"
	exit 1
}
echo "   no attempt burn OK"

echo "== wait for the question to reach the stub dashboard"
[ -n "$REAL_MODE" ] && INGEST_LOG="$TMP/worker.log"
forwarded=1
for _ in $(seq 1 40); do
	if grep -q "question.asked" "$INGEST_LOG" 2>/dev/null &&
		grep -q "task:$TASK_ID" "$INGEST_LOG" 2>/dev/null &&
		grep -q "qref:" "$INGEST_LOG" 2>/dev/null; then
		forwarded=0
		break
	fi
	sleep 0.5
done
[ "$forwarded" -eq 0 ] || {
	echo "FAIL: question never forwarded"
	cat "$INGEST_LOG"
	exit 1
}
echo "   forwarded OK (task + qref tokens present)"

echo "== wait for the answer to come back and the task to complete"
completed=1
for _ in $(seq 1 60); do
	STATUS="$("$TMP/tq" show --db "$TQ_DB" "$TASK_ID" | python3 -c 'import json,sys; print(json.load(sys.stdin)["task"]["status"])' 2>/dev/null || echo unknown)"
	[ "$STATUS" = "completed" ] && {
		completed=0
		break
	}
	sleep 0.5
done
[ "$completed" -eq 0 ] || {
	echo "FAIL: task never completed (status=$STATUS)"
	cat "$TMP/worker.log"
	exit 1
}
echo "   completed OK"

echo "== verify the resumed run saw the rendered answer"
[ -f "$DONE_FLAG" ] || {
	echo "FAIL: stub agent completed without the answered prompt"
	exit 1
}

echo "== verify the questions section + attempt accounting"
"$TMP/tq" show --db "$TQ_DB" "$TASK_ID" | python3 -c '
import json, sys
d = json.load(sys.stdin)
t = d["task"]
qs = d.get("questions", [])
assert qs, "no questions section"
q = qs[0]
assert q["answered"] is True, "question not marked answered: %r" % q
assert q["answer"] == "yes", "answer lost: %r" % q
assert t["attempts"] == 0, "attempts = %s, want 0 (attempts count failures; the park must have burned none and neither did the successful run)" % t["attempts"]
print("   attempts=%s question answered=%r" % (t["attempts"], q["answer"]))
'

if [ -n "$REAL_MODE" ]; then
	echo "(real mode: check the dashboard UI for the answered question too)"
fi

echo "PASS: ask -> forward -> answer -> unblock -> resume -> complete, park burned no attempt"

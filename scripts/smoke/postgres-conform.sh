#!/usr/bin/env bash
# Postgres conformance smoke (2026-10-31; TODO row: parked-surface §postgres):
# spins a scratch trust-auth postgres cluster (initdb on a free port +
# socket dir, createdb), runs the postgresv4 conformance suite against it
# with TQ_TEST_POSTGRES exported, FAILS on skip (the postgres-proven bar
# is one command now — the recipe lived only in a store_test.go comment),
# then stops the cluster and cleans the temp dirs.
set -euo pipefail
cd "$(dirname "$0")/../.."
REPO_ROOT="$(pwd)"

if ! command -v initdb >/dev/null 2>&1; then
	echo "FAIL: initdb not on PATH — this smoke needs a local postgres toolchain"
	exit 1
fi

TMP="$(mktemp -d)"
PGDATA="$TMP/pgdata"
PGSOCK="$TMP/sock"
# A free port (the default collides with any concurrently running scratch
# cluster); $PGPORT overrides for pinned environments.
PGPORT="${PGPORT:-$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')}"

cleanup() {
	pg_ctl -D "$PGDATA" stop -m fast >/dev/null 2>&1 || true
	rm -rf "$TMP"
}
trap cleanup EXIT

echo "== initdb (trust auth, scratch)"
initdb -U tq --auth=trust -D "$PGDATA" >/dev/null

echo "== start cluster on 127.0.0.1:$PGPORT"
mkdir -p "$PGSOCK"
pg_ctl -D "$PGDATA" -l "$TMP/pg.log" -o "-p $PGPORT -k $PGSOCK -c listen_addresses=127.0.0.1" start >/dev/null

createdb -h 127.0.0.1 -p "$PGPORT" -U tq tqconform 2>/dev/null || true

for _ in 1 2 3 4 5 6 7 8 9 10; do
	if psql -h 127.0.0.1 -p "$PGPORT" -U tq -d tqconform -c 'SELECT 1' >/dev/null 2>&1; then
		break
	fi
	sleep 1
done

if ! psql -h 127.0.0.1 -p "$PGPORT" -U tq -d tqconform -c 'SELECT 1' >/dev/null 2>&1; then
	echo "FAIL: postgres never became ready — cluster log tail:"
	tail -20 "$TMP/pg.log" 2>/dev/null || echo "(no pg.log — pg_ctl start itself failed)"
	exit 1
fi

echo "== run postgresv4 conformance suite (zero-pass = FAIL)"
cd "$REPO_ROOT/internal/queue/postgresv4"
if ! TQ_TEST_POSTGRES="postgres://tq@127.0.0.1:${PGPORT}/tqconform?sslmode=disable" \
	GOEXPERIMENT=jsonv2 GOWORK=off CGO_ENABLED=0 \
	go test ./ -count=1 -run 'TestStoreConformance' -v >"$TMP/conform.log" 2>&1; then
	tail -40 "$TMP/conform.log"
	echo "FAIL: postgres conformance red — full log was $TMP/conform.log (cluster stopped)"
	exit 1
fi

# Some conform cases skip BY DESIGN on postgres (sqlite-only migration
# shims, perf baselines); the failure mode to catch is the WHOLE suite
# skipping because TQ_TEST_POSTGRES leaked unset — zero PASSes.
PASSCOUNT="$(grep -c '^    --- PASS' "$TMP/conform.log" || true)"
if [ "$PASSCOUNT" -eq 0 ]; then
	echo "FAIL: conformance passed ZERO cases (TQ_TEST_POSTGRES leaked unset?)"
	exit 1
fi

echo "PASS: postgres conformance smoke ($PASSCOUNT conform case(s) green on a scratch cluster)"

#!/usr/bin/env bash
# Proxy-wait loop extracted from release.sh (proxy verification after tag
# push) so an offline test can drive both die branches — network-dead vs
# proxy lag — without touching proxy.golang.org.
#
# Seams: tq_proxy_poke / tq_proxy_lists are overridable functions (the
# smoke test fakes them); die() is caller-supplied (release.sh's FAIL+exit).
tq_proxy_poke() {
	curl -fsS -o /dev/null "https://proxy.golang.org/$1/@v/$2.info"
}

tq_proxy_lists() {
	GOFLAGS='' go list -m -versions "$1" 2>/dev/null | tr ' ' '\n' | grep -qx "$2"
}

wait_for_proxy_version() {
	local module="$1" version="$2"
	local attempts="${3:-5}" sleep_secs="${4:-30}"
	local poke_ok=false attempt
	for attempt in $(seq 1 "$attempts"); do
		# Every attempt pokes @v/<ver>.info: a passive @v/list poll never
		# triggers the proxy's on-demand fill (v0.3.3 published only after a
		# manual .info fetch, 2026-10-07) — the poke itself requests + caches
		# it. Whether ANY poke succeeds separates network-dead from lag.
		if tq_proxy_poke "$module" "$version"; then
			poke_ok=true
			echo "poked demand-fill: https://proxy.golang.org/$module/@v/$version.info"
		else
			echo "WARN: .info poke failed (network down, or the proxy has not seen the tag yet)"
		fi
		if tq_proxy_lists "$module" "$version"; then
			echo "proxy serves $version"
			return 0
		fi
		echo "proxy does not list $version yet (attempt $attempt/$attempts) — propagation takes minutes"
		if [ "$attempt" = "$attempts" ]; then
			if [ "$poke_ok" = true ]; then
				die "every .info poke succeeded but @v/list never listed $version — proxy lag, not an outage; wait and re-check https://proxy.golang.org/$module/@v/$version.info before retrying anything (never re-tag)"
			fi
			die "all .info pokes failed — network is dead (or the proxy is unreachable); fix connectivity and re-run (never re-tag); the tag is pushed and safe"
		fi
		sleep "$sleep_secs"
	done
}

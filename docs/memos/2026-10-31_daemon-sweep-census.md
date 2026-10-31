# Daemon-sweep census since 2026-10-01 (TODO row 119)

Source: docs/status/tasks/2026-10-13_00-12_task-000001a12dc5140ba7c425fbd27600000000.md §b2.
Method: `git log --format='%h|%ad|%s' --date=short --since=2026-10-01`, filtered to
`auto-commit` subjects; per-sweep delta from `git show --numstat`. Generated noise is
excluded from the feature-bearing count: `vendor/`, `*_templ.go`, `*.sum`, `testdata/`,
`docs/status/**`, and the minified `internal/webui/static/app.css`.

## Aggregates (as of 2026-10-31)

| Metric                              | Count |
| ----------------------------------- | ----- |
| Commits since 2026-10-01            | 1718  |
| `chore: auto-commit` daemon sweeps  | 678   |
| Sweeps > 10 changed lines           | 556   |
| Feature-bearing sweeps ≥ 80 lines   | ~200  |
| …of which ≥ 400 lines               | 21    |

The apparent 6-9k-line monsters (f13cda9b, 1c414a99, a9bdcea1, 33a76ac9, 7052cb73, …)
are NOT feature deltas: every one is dominated by a regenerated minified `app.css`
(6.7-7.5k lines on one line) plus go.sum churn from the module split. After the
generated-noise filter the largest true feature sweep is 041e3d4e (1445 lines,
2026-10-05, the S1-flip era).

## Lineage (nearest footer-bearing parent)

Contiguous spans resolve by reading the parent commit's Task-Queue-ID trailer;
37 of the ~200 feature-bearing sweeps resolve this way. Largest resolved:

| Sweep      | Lines | Date       | Nearest-parent lineage                |
| ---------- | ----- | ---------- | ------------------------------------- |
| b36409fe   | 1274  | 2026-10-05 | 000001a1093141c714732bc7478d…         |
| c77621b7   | 624   | 2026-10-01 | 000001a0f4b46711933c958a92ef…         |
| cd1c7ad3   | 439   | 2026-10-20 | 000001a15679fc8447506e0272aa…         |
| f13cda9b   | 400   | 2026-10-27 | 000001a17c67e28cabc59d1359a1…         |
| dd510cae   | 311   | 2026-10-12 | 20261012-c16-park-forensics (session) |
| 248334ee   | 302   | 2026-10-29 | 000001a1815c3cc3048aaa3b6329…         |
| afd5d9f7   | 300   | 2026-10-13 | 000001a1323f9a3cbf0d5781f042…         |
| b67c0c10   | 279   | 2026-10-12 | 20261012-c16-park-forensics (session) |
| 4f5a7621   | 274   | 2026-10-05 | 000001a109757cf3d260a0de4e27…         |
| a2e30bef   | 233   | 2026-10-16 | 000001a141d63a4bb487c2dcdf4d…         |

Recurring resolved lineages: 000001a13251c305eb46f0c37e53… rides four sweeps
(2026-10-13), 20261012-c16-park-forensics three, 000001a141d63a4b… and
000001a17fd6b6f5… two each — the multi-day batch tasks where mid-edit sweeps
struck repeatedly.

## Unresolved sweeps and the authoritative reconciliation

The majority (~160) are mid-chain: no footer-bearing parent adjacent, because
multiple concurrent agents interleaved. Per-sweep git archaeology is NOT the
tracking mechanism — by construction, every footer-less sweep that the
daemon-attribution gate passes at HEAD is sha-pinned in
`scripts/daemon-sweep-baseline.txt` (regen rewrites it wholesale). The census
therefore needs no follow-up list: the baseline IS the reconciliation; this
memo sizes the blast radius and names the lineages resolvable from history.

Temporal shape: the feature-bearing sweeps cluster 2026-10-04..08 (the S1 flip +
multi-agent storm era, ~45% of the total) and thin out markedly after
2026-10-20 — consistent with commit-promptly discipline taking hold after the
2026-10-13 heal-maneuver reports.

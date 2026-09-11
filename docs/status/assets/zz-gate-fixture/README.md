# 2026-09-10 dogfood proof — archived evidence

Raw agent-task logs from the 2026-09-10 bounded `--once` pool run that
proved the full dogfood loop end to end (harvest → GLM-5.3-Flash agent →
`.tq-verify` race suite → commit with `Task-Queue-ID` footer → review
verdict **approve** → clean drain). Filenames are the queue-assigned task
IDs; contents are verbatim copies of the per-task stdout logs the pool
wrote under `/tmp/tq-dogfood-logs/`.

Provenance and narrative:

- `docs/status/2026-09-10_01-55_pool-deployment-fix-and-flash-dogfood-proof.md` (§c — the proof run)
- `docs/status/2026-09-10_02-00_self-review-dead-pool-fix-flash-dogfood-session.md` (§f9 — why this archive exists)
- Proof commit: `1586ed5` (footer `Task-Queue-ID: 000001a0888b9f367676ffb26ebe48f49571`)

Key files:

- `000001a0888b9f367676ffb26ebe48f49571.log` — the WORK task (dead-export
  audit script); its commit carries the footer above
- `000001a088933b491aa75e2ecf696163319c.log` — the REVIEW task (verdict approve)
- empty `<id>.log` files — heartbeat/requeue side turns that produced no stdout

Integrity: `SHA256SUMS` hashes every file in this archive; verify from
inside the directory with `sha256sum -c SHA256SUMS` (the
anti-ghost-archive gate re-checks it in CI).

Archived 2026-09-11 (TODO item 02:00 f9). The SQLite proof journal
(`/tmp/tq-dogfood.db`) could NOT be archived: it was already gone from
/tmp when this task ran (last seen 2026-09-10; the fact trail it held is
summarized in the 01-55 report). The logs above are everything that
survived.

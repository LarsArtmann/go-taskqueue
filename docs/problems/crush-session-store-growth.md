# Crush session-store growth: measurements + prune/archive proposal

Written 2026-11-17 (IO-review follow-up §f8). `/mnt/hot/crush` holds every
crush project's session store with NO retention: the store grows until the
disk fills. This note measures the growth so the next incident re-derives
nothing, and proposes the policy (owner decision pending on delete-vs-archive).

## Measured facts (2026-11-17, read-only sqlite probes)

Store total: **84 GiB** on `/mnt/hot` (nvme0n1p2, 749 GiB used / 125 GiB free
= 86% full). The stores are per-project `crush.db` SQLite files (sessions,
messages with `parts` TEXT blobs, files, read_files — ON DELETE CASCADE from
sessions). Schema comments claim Unix-milli timestamps; the values are Unix
SECONDS. Top stores:

| Project | crush.db | sessions | span | msgs last 14d | >90d old |
| --- | --- | --- | --- | --- | --- |
| SystemNix | 10.7 GiB | 7283 | 331 d | 69k (4955/d) | 5004 |
| CV | 7.4 GiB | 3158 | 186 d | 62k (4436/d) | 1475 |
| go-taskqueue | 6.3 GiB | 1656 | 73 d | 15k (1076/d) | 0 |
| go-cqrs-lite | 5.6 GiB | 4829 | 185 d | 19k | 4438 |
| projects-root | 5.1 GiB | 10535 | 231 d | 5k | 10386 |
| pbx-artmann | 3.2 GiB | 413 | 64 d | 31k (2201/d) | 0 |
| BuildFlow | 3.0 GiB | 3664 | 186 d | 11k | 3415 |
| DiscordSync | 2.6 GiB | 2602 | 179 d | 0.3k | 2425 |
| monitor365 | 2.3 GiB | 3115 | 207 d | 3k | 3027 |

(Top-9 db total 46.2 GiB; the remaining ~38 GiB is the long tail of ~40
smaller projects + logs/shell-output.) `parts` blobs dominate: go-taskqueue
messages.parts alone = 3.1 GiB of its 6.3 GiB db. The message rate is the
live growth driver — pool automation (4955 msgs/d on SystemNix, 4436/d on CV)
out-runs any human-driven project by 4-20x.

### Growth model

db-size ÷ active-span is a stable ~0.02-0.09 GiB/day per project; summing the
top stores gives **~0.28 GiB/day ≈ 8-9 GiB/month ≈ 100 GiB/year**, and the
trend is UP as pool automation spreads (go-taskqueue hit 0.086 GiB/d within
its first 73 days). At this rate /mnt/hot's 125 GiB free goes to zero in
**~15 months** — sooner with a busy quarter. zram page-cache famine (2026-11-17
IO review) also pays per cold read of these dbs.

## Recipe (read-only; never write a live crush.db)

```
python3 - <<'EOF'
import sqlite3, time, os
p = "/mnt/hot/crush/<project>/crush.db"
c = sqlite3.connect(f"file:{p}?mode=ro", uri=True)   # URI ro: no WAL write
print(c.execute("select min(created_at),max(created_at),count(*) from sessions").fetchone())
EOF
```

## Proposed policy (owner decision: delete vs archive)

1. **Retention = 90 days hot.** Sessions older than 90 d: 24k+ of the top-9
   stores' 39k sessions. Active-session needs are bounded: `tq
   --task-closeout` resumes only a task's own recent session, and verify
   receipts/attribution live in the tq journal, not in crush.db — a session's
   transcript is diagnosis material, valuable for weeks, not years.
2. **Archive (owner call):** zstd-compress per-month dumps of pruned sessions
   (`sqlite3 .dump` filtered by created_at, or copy the db then delete) to
   cold storage before delete; text `parts` compress well. Without an archive
   consumer identified, straight DELETE is the honest default.
3. **Mechanics:** per-project `DELETE FROM sessions WHERE created_at <
   ?` (cascades messages/files/read_files) with crush NOT running for that
   project, then `VACUUM` (reclaims the parts blobs — a 50-70% db shrink is
   realistic given >90d mass) and `PRAGMA wal_checkpoint(TRUNCATE)`.
4. **Automation:** a monthly cron/systemd timer sweeping every
   `/mnt/hot/crush/*/crush.db`, logging per-store before/after sizes to a
   journal — same discipline as the keeper's RSS log so growth stays visible.
5. **Alert:** monitor365 free-space threshold on /mnt/hot at 90% (sibling of
   the zram swap-in alert row).

Follow-up rows harvested to TODO_LIST: implement the prune script + timer
(after the owner rules delete-vs-archive); crush schema-comment bug (millis
comment, seconds values) belongs upstream in crush.

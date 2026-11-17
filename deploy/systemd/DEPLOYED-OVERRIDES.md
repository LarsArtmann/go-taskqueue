# Deployed-unit override contract (tq-agent-pool)

Recorded 2026-11-17 (IO-review report §a) so the next IO incident does not
re-derive the repo-template vs deployed-unit split. The 2026-11-17 review
burned a whole window discovering that its cgroup evidence did not match the
repo template.

## The contract

The REPO template (`deploy/systemd/tq-agent-pool.service`, also rendered by
`tq bootstrap --install`) carries **NO** `Memory*`, `IOScheduling*`, `CPUQuota`,
or `IOAccounting*` directives — resource control is deliberately out of the
template. The DEPLOYED unit on the host adds them via the root-owned file
`/etc/systemd/system/tq-agent-pool.service` (NOT a drop-in; `systemctl cat`
shows no `*.conf`):

- `MemoryMax=8G` — owner ruling 2026-11-17: KEEP as the fence. The only
  measured pool-harm was the DAEMON swapping inside the cap (majflt
  2.2k→25k in 30 min while the ceiling evicted page cache); the companion
  host row proposes `MemoryMin=256M` (cache eviction for a background job is
  by design, daemon-swap is not). The earlier drop-MemoryMax=24G
  recommendation is WITHDRAWN.
- `IOSchedulingPriority=7` (idle class) — owner ruling 2026-11-17: KEEP. A
  background job yields; NVMe blk-mq barely honors ioprio and swap-in reads
  ignore it. The earlier drop-ioprio recommendation is WITHDRAWN.
- `CPUQuota=400%`, `IOAccounting=true` — observed, no ruling against.

Consequences to remember:

- `memory.events.local` on the pool cgroup accumulates allocation throttles
  against the 8G ceiling (229,179 hits measured 2026-11-17) — the same
  counter `tq pool-health` now surfaces as `cgroupMemoryEvents` (cmd/tq
  poolhealth.go), so thrash is visible from inside tq.
- Attribution traps for this deployment: cgroup `io.stat` counts AGENT
  BUILD CHILDREN (cold caches, mold, crush runs), never the queue's own DB
  (5 MB cumulative on sda over 3.2 h); `/proc/<pid>/io` of the pool leader
  is a monotonic superset of every reaped child's IO — see
  docs/architecture/io-surface/proc-io-counter-fold.md.
- If you change the deployed unit, update this file + the AGENTS.md
  Known-Issues one-liner in the same change; pool-health's 2 GiB
  crush-server WARN band (`crushServerRSSWarnKB`) assumes the 8G ceiling
  (stays valid, more conservative under a bigger one).

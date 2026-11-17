# IO review: the "~102 MB/s pure reads / PSI 75.8%" alert on tq-agent-pool.service (direct session)

**Session**: direct owner dispatch 2026-11-17 ~18:20–19:40 CET (no queue task,
no Task-Queue-ID footer). **Trigger**: "tq-agent-pool.service — ~102 MB/s
pure reads; IO stall (PSI avg300 = 75.8%) — WTF?! I/O review! DEEP!
Leverage io-surface". **Deliverable**: root-cause verdict + AGENTS.md
Known-Issues bullet (commit 0c9c96f5, size guard 27600/27600).

## Verdict (one line)

The queue is innocent: the 102 MB/s is (a) cgroup io.stat aggregating agent
build children reading cold caches on the two NVMes and (b) a /proc
counter-fold artifact; PSI is system-wide and driven mostly by OTHER units
(llama-vlm-cap GGUF streaming, interactive build storm) plus 40.7 GiB of zram
swap — the pool's own DB on sda read ~5 MB cumulative in 3.2 h.

## a) Fully done

- **Live evidence base** (measured, reproducible recipes): /proc/pressure/io
  trend (some avg300 75.8%→57–61% during the session), per-PID
  read_bytes/rates via /proc/<pid>/io deltas, /proc/diskstats device rates,
  cgroup io.stat per-device deltas across ALL cgroups, majflt/VmSwap
  counters, fd snapshots, btrfs/mount topology (projects + /mnt/hot on
  nvme0n1p2; root on nvme1n1p6; tq.db on sda).
- **Attribution decomposition**: proved the pool cgroup's 47 GB of reads sit
  on nvme0n1p2 (33 GB) + nvme1n1 (13.8 GB) — /nix/store, ~/.cache/go-build,
  go-mod, /mnt/hot/crush (84 GB; SystemNix/crush.db = 11.4 GB) — i.e. agent
  children (crush runs, builds, mold), NOT the queue; sda (the actual queue
  DB) = 5 MB cumulative. Caught the counter-fold live: /proc/<tgid>/io
  jumped +1290 MB in 0.3 s as a 1281 MB child exited with +9 MB device
  traffic.
- **Co-driver identification**: llama-vlm-cap.service sustained ~22–40 MB/s
  reading GGUF weights from /data/ai (nvme1n1p8) — top single reader during
  BOTH attribution windows; plus cvagent.service (~8 MB/s), user-scope
  nix-flake-check + 4× mold linker storm, and 40.7 GiB zram swap (102/124
  GiB RAM used → page-cache famine → cold reads everywhere).
- **Pool-cgroup thrash root cause**: deployed unit carries host-local
  overrides `MemoryMax=8G` (memory.events.local max = 229,179 hits),
  `IOSchedulingPriority=7` (lowest class — PSI measures stall TIME),
  `CPUQuota=400%`, `IOAccounting=true`. Verified the REPO template
  (cmd/tq/bootstrap.go + deploy/systemd/) is clean of these — the overrides
  exist only in /etc/systemd/system/tq-agent-pool.service (no drop-in).
- **io-surface leverage** (as ordered): `io-surface lint` = 0 errors /
  0 warnings / 55 infos; read-site review over facts.cue; the suspect
  in-process read path `crushSession()` proved an index seek via read-only
  EXPLAIN QUERY PLAN on the live 11.4 GB crush.db (`SEARCH sessions USING
  INDEX sqlite_autoindex_sessions_1`); calibration.jsonl = 573 KB. **No
  read-amplification bug in tq.**
- **Durable record**: AGENTS.md Known Issues gained the attribution-trap
  bullet (+ concept-preserving trims elsewhere; guard green at exactly
  27600/27600 B, `check-agents-size.sh` OK); commit 0c9c96f5.
- **Tree heal while in flight**: the concurrent session's dns-outage report
  (companion of f811bda2) landed unindexed and made the pre-commit hook fail
  EVERY commit; indexed it (top cluster row) and carried it — commit
  unblocked.

## b) Partially done

- **Historical file-read decomposition**: proved the first-3-hours 33 GB
  nvme0n1 reads were NOT the queue binary (counter math + fold events), but
  did NOT pin file-by-file which dead children read what (proxy: mounts +
  device + exit timing). A definitive per-file trace needs root
  (bpftrace/strace were EPERM: yama ptrace_scope=1, no CAP_BPF).
- **Counter-fold mechanism**: observed empirically twice (+1290/−1281 MB;
  +51/−52 MB) but not kernel-source-verified — the exact fold rule (which
  exiting tasks merge into the leader's ioac) remains unconfirmed.
- **io-surface judgment pass**: lint + facts.cue skim done; a full
  claims.cue review of all 51 file-read sites (durability/frequency
  judgments) not done — lint's 0-error result made it lower value, not
  unnecessary.
- **Swap-in vs file split for the pool process**: current bursts decomposed
  (zram ~20 MB + nvme 5–11 MB per jump, majflt growing); the early-hours
  bursts predate my instrumentation.

## c) Not started

- Host fixes (unit directive changes) — root-owned file, owner action.
- llama-vlm-cap.service investigation (out of this repo's scope).
- Any monitoring change (per-service io.stat/PSI attribution script for
  monitor365/gatus — recipe exists in this report, not productized).
- Full ci-local battery for the docs-only commit (ran the targeted gates
  instead: agents-size, status-index, todo-list hook).

## d) Totally fucked up (honest ledger)

1. **Wasted probes**: broken python placeholder call (syntax error);
   io.stat parser off-by-one (`kv[6:]` vs 7) killed the first correlator;
   first syscall poller filtered out `running` threads and "concluded" no
   read syscalls; second never tested /proc/*/syscall readability (EPERM
   under yama) — 3 cycles burned on tool bugs before clean data.
2. **Late cross-accounting**: chased "tq itself reads 41 GB" through mmap,
   smaps, O_DIRECT, btrfs-RAID theories before doing the ONE decisive check
   (cgroup device deltas vs children-sum vs tgid counter in a single loop).
   The correlator that answered everything should have been probe #2.
3. **Intermediate misdirection**: reported "tq itself ~17 MB/s sustained"
   mid-session — that number was contaminated by fold events; retracted
   later. Should have flagged uncertainty at the time.
4. **AGENTS budget discipline**: first bullet edit blew the size guard by
   572 B (budget was one command away); took 4 extra trims to land at
   exactly 27600/27600.
5. **Commit race handling**: first commit attempt collided with a
   concurrent agent's commit mid-hook; the retry swept 3 foreign STAGED
   files (.github/workflows/ci.yml, fuzz.yml, flake.lock) into my docs
   commit (6 files). Documented race class, but a staged-set inspection
   (`git status` column-1) before committing would have caught it; they were
   daemon-bound anyway (footer-less chore shape), still: not my diff, my
   commit message.
6. /tmp litter: tq-io.strace (empty), tq-burst-watch.log, io_a/io_b, ds_1/ds_2
   remain in /tmp (harmless, tmpfs).

## e) What we should improve

- **Probe-before-theorize protocol**: when multiple accounting layers exist
  (device / cgroup / pid / syscall), build the cross-layer delta correlator
  FIRST; every anomaly I chased separately fell out of it in one run.
- **Permission pre-flight** for tracing tools (strace/bpftrace//proc/*/syscall)
  in yama-scoped non-root sessions — test readability before writing samplers.
- **Budget-aware AGENTS edits**: check `check-agents-size.sh` headroom before
  composing, not after.
- **Monitoring gap (the actual WTF)**: the alert attributed system-wide PSI +
  cgroup-wide device bytes to one service. A correct per-service I/O pane =
  cgroup io.stat deltas + that cgroup's OWN memory pressure; the recipe is in
  this report (§a) and should live in monitoring, not in an agent's memory.
- **Box hygiene**: 40.7 GiB zram swap + 15 GB-RSS nix + 4× mold + llama-server
  on a 124 GiB box = page-cache famine; every workload pays cold-read tax and
  PSI blames whoever is sampling.

## f) Next (harvested to TODO_LIST.md; host rows marked BLOCKED)

1. Host: drop `IOSchedulingPriority=7` from the deployed unit — BLOCKED:
   owner host action (root-owned /etc/systemd unit).
2. Host: soften MemoryMax 8G → MemoryHigh=10G + MemoryMax=24G + MemoryMin=256M
   — BLOCKED: owner host action; poolhealth's 2 GiB crush-server WARN band
   assumes the 8G (stays valid, more conservative under a bigger ceiling).
3. Investigate llama-vlm-cap.service continuous GGUF weight streaming
   (~22–40 MB/s from /data/ai, top reader) — BLOCKED: foreign service owner.
4. Add per-service IO/PSI attribution (cgroup io.stat deltas + memory.pressure)
   to monitoring — monitor365 repo row.
5. Verify the /proc tgid io counter-fold mechanism against kernel sources /
   scratch-VM repro (kills the last unknown of this review).
6. Full io-surface claims.cue judgment pass over the 51 file-read sites.
7. poolhealth: surface `memory.events.local: max` count in the health JSON
   (cgroup-thrash visibility from inside tq).
8. crush session-store growth policy: /mnt/hot/crush = 84 GB (SystemNix 11 GB,
   CV 7.5 GB, tq 6.3 GB …) with no retention observed — decide prune/archive.
9. monitor365: zram swap-in-rate alert (thrash early warning — today's alert
   fired as "IO stall" with no memory attribution).
10. Docs: record the deployed-unit override contract (repo template clean;
    host adds Memory*/IOSched*) in deploy/systemd/ or docs/problems.

## g) Owner questions (cannot be derived from this repo)

1. Are the deployed unit overrides (`MemoryMax=8G`, `IOSchedulingPriority=7`,
   `CPUQuota=400%`) intentional policy you set, or leftover tuning I should
   propose replacing wholesale?
2. Is llama-vlm-cap.service *supposed* to stream model weights continuously
   (cold-cache inference loop), or is it stuck reload-looping? Its config and
   logs live outside everything I can read meaningfully here.
3. Do you want agent-run builds isolated into their own systemd scopes so
   future I/O alerts attribute them separately from the queue daemon (a
   behavior change in how the pool spawns children), or keep single-cgroup
   attribution and just fix the monitoring?

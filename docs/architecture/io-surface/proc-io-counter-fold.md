# /proc/<pid>/io counter-fold mechanism — kernel-source verified

Verified 2026-11-17 against torvalds/linux master (`kernel/exit.c`,
`fs/proc/base.c`, `include/linux/task_io_accounting_ops.h`), closing the last
unknown of the 2026-11-17 IO review
(docs/status/2026-11-17_19-40_io-review-pool-attribution-verdict.md §b).

## The rule

`/proc/<tgid>/io` (kernel `proc_tgid_io_accounting` → `do_io_accounting(task, m, whole=1)`,
fs/proc/base.c) is NOT the leader's own counter. With `whole=1` it reads:

```
acct = sig->ioac;                      // the thread group's signal_struct accumulator
__for_each_thread(sig, t)
    task_io_accounting_add(&acct, &t->ioac);   // + every LIVE thread's per-task counter
```

Folds happen at exactly two sites in `kernel/exit.c`, both additive and
irreversible:

1. **Thread exit** (`__exit_signal`, ~line 203): a dying thread adds its
   `tsk->ioac` into its OWN thread group's `sig->ioac` before the group can
   observe it again.
2. **Child reaping** (`wait_task_zombie`, ~lines 1261-1262): when a parent
   wait()s an exited child, the child's `p->ioac` AND the child's whole
   `sig->ioac` (i.e. everything the child's own thread group ever folded)
   are added into the parent's `psig->ioac`.

`task_io_accounting_add` folds both the blk counters
(`read_bytes`/`write_bytes`/`cancelled_write_bytes`, CONFIG_TASK_IO_ACCOUNTING)
and the char counters (`rchar`/`wchar`/`syscr`/`syscw`, CONFIG_TASK_XACCT);
distro kernels enable both.

## Why the review saw a +1290 MB leader jump with +9 MB device traffic

The pool leader's `/proc/<tgid>/io` grew ~1290 MB in 0.3 s as a 1281 MB child
exited, while device-level traffic (cgroup io.stat / diskstats deltas) moved
only ~9 MB. The bytes were never "transferred": the child had ALREADY done
its ~1281 MB of reads (mostly warm page-cache and zram, hence the small
device delta), but they lived in the CHILD's `p->ioac`/`sig->ioac` — invisible
to any per-process sample of the leader — until the pool reaped the zombie,
at which point `wait_task_zombie` folded the child's totals into the leader's
`psig->ioac` and every subsequent read of `/proc/<leader>/io` includes them.

Consequences for IO attribution (see AGENTS.md Known Issues):

- `/proc/<pid>/io` of a parent with reaped children is a MONOTONIC
  SUPERSET counter: it counts dead children's historical IO forever. Sampling
  deltas over it attribute ancestor-reaped work to the ancestor.
- No counter decreases on fold — the observed +1290/−1281 pairing in the
  review was leader-gain vs child's pre-exit visibility (a dead child's
  `/proc` entry vanishes), not a transfer out of anything.
- Byte-exact totals are still correct system-wide; only the per-process
  ATTRIBUTION lies. cgroup `io.stat` per-device deltas remain the honest
  per-service device view (but note cgroup io.stat counts children too —
  agent build traffic, not the queue's own DB IO).

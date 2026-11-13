# First data-driven calibration tuning pass (2026-11-13 10:39)

**Dataset:** `$TQ_LOG_DIR/calibration.jsonl` (production pool log dir
`~/.local/state/tq/logs`), 1,320 rows, window 2026-11-02 18:40 → 2026-11-13
10:33 (11 days of live pool traffic). Threshold bar met: every closeout-free
kind cleared ≥50 rows (agent 1015, review 113, status 92, dlqfix 69). This is
the FIRST pass that diffs the auto-dismiss/review rules against observed
verdict→outcome rates instead of by feel (TODO row: "First data-driven
calibration tuning pass"). Method: direct python analysis over the JSONL
(analysis scripts under /tmp, not committed), cross-checked against the
reader semantics in `internal/executor/calibration_read.go`.

## a) Observed verdict→outcome rates per kind (raw counts, not claims)

| kind | rows | verdict distribution | approve→0-commits | attestations | failure classes |
| --- | --- | --- | --- | --- | --- |
| agent | 1015 | 991 no-verdict (plain work turns), 24 verdict bodies without a `verdict` key (summary-only) | n/a (agent has no approve verdict) | 158 carried, 17 refused (16 `command-mismatch`, 1 `head-mismatch`) | 9 transient |
| review | 113 | 74 approve / 39 request_changes | **74/74 = 100%** | **0** | 0 |
| status | 92 | 85 verdict bodies without a `verdict` key, 7 none | n/a | 34 (all ok) | 7 transient |
| dlqfix | 69 | 35 fixed / 34 wontfix (51% fixed) | n/a | **0** | 0 |
| class-as-kind strays | 21 | — | — | 0 | permanent 16, transient 5 (`broken` 3, `mystery` 3, `flaky` 6, `panic` 3, `gate` 3, `slow` 3 — see §c1; `sh` 10 and the remaining singletons are legit driver kinds) |

Zero-cost completions: 984/1015 agent rows, 100% of review/status/dlqfix
rows — the known sessionUsage join gap (TODO row "Investigate zero-cost
calibration rows", 2026-11-13 closeout §e10); not re-litigated here.

## b) Rule-vs-data diffs (what the current thresholds assume, what the data says)

1. **`mergeOnApprove` (internal/review/sweep.go:200) trusts a bare approve
   verdict with NO receipt join.** The data: 74 approve rows, zero of them
   carry a `verify` receipt or `attestation` — the closeout-free review
   clone records no gate run, so the merge gate's only input is the agent's
   own word. AGENTS.md doctrine says trust derives only from verification
   facts. **Tuning candidate (highest leverage):** carry `VerifyReceipt` +
   `Attestation` on review turns (the receipt lane already exists for agent
   turns) and gate `mergeOnApprove` on an attested-ok receipt once rows
   accumulate — until then the O11 ordering (approve is the LAST unmet
   condition) stays the only guard.
2. **The approve→commits cross-tab is structurally dead for review:**
   74/74 approve rows read 0 commits because closeout-free review turns
   structurally ship none. The reader's cross-tab (calibration_read.go:28,
   "the auto-dismiss tuning signal") mislabels this as a broken rule —
   already rowed as "Fix the `tq calibration` approve→commits-shipped
   cross-tab semantics for the review kind" (§d6 of the 2026-11-13
   closeout). Confirmed at 100%, not 0%-ish: per-kind gating is the fix,
   no threshold change.
3. **dlqfix fixed-rate 51% (35 fixed / 34 wontfix) sits exactly at the
   decision boundary** the autopsy-vs-dismiss rule uses: every wontfix is
   a dismissal, every fixed an autopsy claim. 69 rows is enough to say the
   split is a coin flip, NOT enough to move any threshold — the rule reads
   the verdict, not a rate. No tuning candidate; keep watching: if fixed
   drops below ~40% over the next window, the DLQ intake (what becomes a
   dlqfix task at all) is over-eager and the mint gate deserves a look.
4. **Agent `command-mismatch` attestation refusals are ONE cluster, not
   noise:** all 16 carry the same detail shape — executed command matches
   the receipt's `command` field modulo quote-escaping, pinned value reads
   `exit 61`. That is receipt-vs-pin drift on a single verify recipe (the
   GOEXPERIMENT=jsonv2 + gofmt-gate + nested-module loop recipe), not 16
   independent misbehaviors. **Tuning candidate:** treat `command-mismatch`
   + receipt-rc 61 as its own evidence-stage alias in the DLQ surface so
   these don't masquerade as per-task verify failures; also check whether
   the attestation comparator unescapes shell quoting before string-
   matching (16/16 look like pure escaping diffs).
5. **review request_changes severity mix is anchor-complete and
   severity-usable:** 39 request_changes verdicts carried 50 findings —
   19 high / 26 medium / 5 low, 50/50 with a verbatim anchor (the REJECT
   path never fired in this window). **Tuning candidate:** the minted fix
   tasks could inherit the finding severity as a scheduling signal (the
   ADR-0015 "signals, never scores" lane) instead of all landing at the
   same priority.
6. **status rows speak a different verdict dialect:** 85/92 verdict bodies
   have no `verdict` key (they are the status summary shape). The reader's
   `VerdictValues` lumps them under "(none)". No rule reads status
   verdicts today, so this is cosmetic — fold into the same per-kind
   cross-tab fix as (2) rather than a separate change.

## c) Dataset-integrity findings (rule inputs are partially poisoned)

1. **Kind pollution — 21 rows carry a FAILURE CLASS in the `kind` field**
   (`flaky` 6, `sh` 10, `panic` 3, `broken` 3, `mystery` 3, `gate` 3,
   `slow` 3 — counts sum to 31 across overlaps because `sh` also appears
   as a legit driver kind; the strays total 21 non-executor rows). A
   `RecordCalibrationFailure` call site passes the failure class (or the
   stage string) where the executor kind belongs, so per-kind tuning
   denominators under-count the agent family and invent six phantom
   "kinds". **Fix before the next tuning pass** — rowed in TODO_LIST.
2. The 17 agent attestation refusals mean 17 runs shipped a receipt the
   write-time judge refused, yet all appear as completion rows — correct
   per the row contract (attestation refusal ≠ task failure), but the
   tuning rules that watch `AttestedOK` must read refusals as a quality
   signal, not a success.

## d) Not done / left open

- No code changed in this pass — it is an ANALYSIS pass by contract
  ("report tuning candidates"). The candidates above route through the
  TODO rows minted alongside this report.
- The zero-cost join, the review cross-tab semantics, and the trim wiring
  were already rowed by the 2026-11-13 closeout (§b3/§b5/§d6/§e10) —
  not duplicated here.

## e) Verdict channel

`tq verdict` recorded at session end per the done-prompt contract.

## f) Harvest

New TODO rows (deduped against the existing calibration block):
kind-pollution fix; review-turn receipt join; command-mismatch evidence
alias + comparator escaping check; severity-inherited fix-task priority.

## g) Notes for the next pass

- Re-run at ~200 review / ~150 dlqfix rows (roughly 2-3 more weeks of
  pool traffic at this window's rate) before touching any rule: the 51%
  dlqfix split and 65% approve rate both sit too close to their decision
  boundaries for a threshold move on 3-digit samples.
- The analysis scripts were /tmp-only; if a second pass wants them,
  re-derive from this report's §a table (the queries are one-liners over
  `kind`/`verdict.verdict`/`commit_count`/`attestation.ok`/`failure.class`).

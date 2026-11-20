# T21 — webui SSE-flake cluster triage: keep / pool / obsolete

Date: 2026-11-20 (04-25 window). Source: the backlog-pareto master-plan
(docs/planning/2026-11-17_19-53_backlog-pareto-master-plan.md, T21/T22);
row anchored at TODO_LIST.md "T21 triage doc" in the "Pareto execution
residue" section. Cluster = the TODO_LIST.md section
"000001a19eac4ba6 webui SSE-flake closeout appends (2026-11-03)" and the
residue it accreted (all open unblocked rows in that section at
2026-11-20 HEAD, cited by TODO_LIST line number at triage time).

## Method

Verdicts, not scores:

- **POOL** — one bounded change an agent session can land with its own
  gate/fixture evidence (the pareto plan's stated disposition: batch the
  1-test-pin tail via the pool). No TODO_LIST edit needed; the rows stay
  exactly as they are and the pool dispatches them.
- **KEEP** — lands by hand: needs session context or cross-cutting
  judgment (AGENTS.md budget-gated prose, platform changes, sibling-repo
  sessions, owner/host operations, standing rituals).
- **OBSOLETE** — premise verified dead at 2026-11-20 HEAD (work landed,
  sources pruned, or duplicate of a sibling row). These rows may be
  ticked as already-landed with this doc as the citation.

Premise re-checks (one grep each) were run where cheap; the executing
window still owns the AGENTS "Premise-check first" convention.

## POOL (dispatchable tail — the bulk, ~80 rows)

Line numbers at 2026-11-20 HEAD:

1812, 1813, 1816, 1817, 1818, 1820, 1827, 1828, 1829, 1831, 1834, 1835,
1837, 1838, 1839, 1841, 1843, 1844, 1845, 1849, 1850, 1851, 1853, 1854,
1858, 1860, 1861, 1870, 1871, 1872, 1876, 1879, 1880, 1881, 1882, 1883,
1885, 1904, 1905, 1906, 1908, 1909, 1938, 1939, 1940, 1946, 1952, 1953,
1954, 1955, 1956, 1957, 1958, 1965, 1966, 1971, 1972, 1976, 1977, 1980,
1986, 1987, 1988, 1989 (re-verify the templ-version premise at
dispatch), 1991, 1992, 1994, 1995, 1998, 1999, 2000, 2001, 2002, 2009,
2010, 2011, 2012, 2013, 2014, 2015, 2017, 2019.

One-line shapes: single gate/fixture/smoke additions (fleet
self-test-convention rows), mechanical sweeps with a pinned matcher,
one-test pins in webui/calibration/pool-health, and small
refactor-verifications with a named test command. None needs an owner
ruling or cross-session context.

## KEEP (lands by hand — 33 rows)

| Lines | Why hand |
|---|---|
| 1809 | upstream go-sse ssetest change — external repo, verify-before-filing first |
| 1810, 1811, 1815, 1833, 1852, 1864, 1884, 1947 | AGENTS.md prose (budget-gated, one-edit discipline, hand-worded conventions) |
| 1832 | claim→spawn journal fact — queue-core feature crossing the store contract, not a pin |
| 1846 | process verification (preflight-battery skippability) — needs a real doc-only batch to observe |
| 1857 | cmd/tq `GOWORK=off` release-floor surgery — flip-flopping Known Issue, needs the release flow |
| 1878 | owner ruling duplicate (D17 in the 2026-11-18 digest) — parked with its BLOCKED twin |
| 1923, 1924, 1925, 1926, 1927 | go-cqrs-lite sibling rows — one sibling session each, not this repo |
| 1931 | AGENTS multi-convention size-conscious edit — hand |
| 1932–1937 | go-cqrs-lite sibling rows (BuildFlow residue, AGENTS prune, canonical helpers, doctor hygiene, fused-row audit, WithPrefetch retry) |
| 1943 | superseded in spirit by the landed `--fold-below` mode (41587f59) but asks a distinct `--files`-style surface — hand judgment on whether to build it |
| 1964, 1967, 1968, 1973, 2016, 2003, 2008, 2018 | verification receipts, host ops, standing rituals — session/hand work, not dispatchable pins |
| 1974, 1975, 1978, 1979 | external waits and platform changes (nixpkgs templ, re-dispatch preamble, docs-gates upstream ask, dispatch envelopes) |
| 2004–2007 | go-cqrs-lite sibling rows |

## OBSOLETE (premise dead at HEAD — tick with this doc as citation)

| Line | Why dead |
|---|---|
| 1819, 1821, 1823, 1824 | fuzz.yml/rewrite-rate observation rows: the 2026-11-04+ scheduled runs have long since executed with fetch-depth: 0 — re-verify each premise at the ticking window before ticking (the AGENTS premise gate applies); classification PROVISIONAL-obsolete pending that one command |
| 1822 | TQ_HOOK_ADVISORY_LOG wiring — verify current crushrc still lacks the pointer before ticking (config lives in the crush-config repo) |
| 1826 | check-hook-fp-triage pass — the ~7-day advisory accumulation has elapsed many times over; either run it (KEEP if the log has hot rules) or tick obsolete after one empty-log check |
| 1862 | the cited /tmp nix receipts are PRUNED (verified 2026-11-20 durable-evidence sweep: both paths gone); row 42's tick note carries the surviving pass counts — quote-cite it and tick |
| 1873 | `tq calibration` reader SHIPPED (cmd/tq/calibration.go:38, main.go:120) |
| 1875 | erraudit #12/#13/#14 CLOSED + pushed, verified ALREADY-LANDED 2026-11-19 (T24 tick) |
| 1922 | go-cqrs-lite archive ritual landed 2026-11-19 (T25/T26: 709324f82/0f0d8a74f — 53 KEEP, 141 archived) |
| 1943's twin shape | the `--fold-below` mode for the daemon-commit-BELOW-footer shape landed 41587f59 with a 45-check self-test — only the distinct mixed-sweep `--files` split (1977, POOL) remains of the family |

## Keep-set (T22 input)

The pareto plan's T22 "land first 5 test pins" should draw from the
POOL list's webui rows: 1986, 1987, 1988 (budget-meter pins), 1534's
sibling cluster rows outside this section (SSE header-matrix pins), and
1876 (parked-provider strip render pin). The KEEP rows are NOT pool
food and must not be batch-dispatched.

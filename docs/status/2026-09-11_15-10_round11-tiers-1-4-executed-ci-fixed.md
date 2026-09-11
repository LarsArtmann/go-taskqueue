# Status Report — 2026-09-11 15:10 CEST — Round-11 execution: tiers 1%+4% shipped, CI red root-caused & fixed

Session scope: execute `docs/planning/2026-09-11_14-01` (Pareto Round 11)
top-down. Delivered the complete 1% tier (pool revival prep), the complete
4% tier (reliability blind spots), and the first slice of the 20% tier
(queue↔git trust), plus small-fix sweeps.

---

## a) FULLY DONE

| Work | Evidence |
| --- | --- |
| **T1 RC gate at HEAD**: root race, all 7 sub-modules, ci-local, nix build + flake check, 6 smokes (incl. NEW ratelimit-e2e) — ALL GREEN | `docs/status/2026-09-11_14-50_rc-gate-evidence.md` |
| **T2 flip checklist + DLQ runbook**: post-flip smoke checklist (6 checks), 21-task DLQ triage table with per-task cancel/rescue/verify verdicts + exact commands, `--allow-writes` note, post-flip retro metric scaffold | `docs/planning/2026-09-11_14-45_FLIP-CHECKLIST-AND-DLQ-TRIAGE-RUNBOOK.md` |
| **T3 rate-limit armor**: incident fixture (exact lastError tail of `000001a08edf`) pinned; TZ sweep across 6 zones; parked-requeue-vs-stale-lease pin sqlite+postgres; `FuzzDetectRateLimit` (15s campaign green, seeds from real provider logs) | `internal/executor/ratelimit_test.go`, `testdata/ratelimit_incident_000001a08edf.txt` |
| **T5 CI red root-caused and FIXED — diagnosis REPLACED the plan's premise**: the red was NOT the nix FOD (nix job GREEN in the 12:27 run; the 06:25 runner-only variant never recurred). Actual cause: no CI job sets `GOEXPERIMENT=jsonv2` while packages use encoding/json/v2 stdlib symbols gated to go1.27 — local builds only passed via `~/.config/go/env`. Fixed with a workflow-level env in ci.yml (covers test/test-windows/test-postgres/govulncheck/gosec) | `.github/workflows/ci.yml`; reproduced locally (`env -u GOEXPERIMENT go build` dies, with it builds) |
| **T6 master-CI state gate**: `scripts/check-ci.sh` (gh run list, non-zero on red, CI_CHECK=off escape) wired as ci-local's FIRST step | live-verified against the actually-red master (correct FAIL) |
| **T8 rate-limit completeness**: http 429 → `*RateLimitError` (Retry-After header > body reset > default; parseRetryAfterHeader pinned); repo-keyed gates (`.crushrc` fixes the provider per repo → Z.ai 429 in repo A never parks repo B; repo evidence overrides stale shared gate); `tq tasks --parked` (SQL pushdown both stores + pin test); `tq stats` parked count (text+JSON); `tq doctor` parked check (WAITING not broken); bridge audit found a REAL alert-loss bug — PapDashboard 429 hit the permanent branch which ADVANCES the checkpoint (alert silently dropped); reclassified transient + pin test | `internal/executor/http.go`, `agent.go`, `ratelimit.go`, `cmd/tq/{tasks,main,doctor}.go`, `internal/bridge/papdashboard/papdashboard.go` |
| **T9 closeout-429 resume**: rate-limited close-out registers `closeoutPending` (task.ID → repo+session); re-claim resumes at closeout — the paid work turn NEVER re-runs because of a provider 429; end-to-end stub test proves work.log stays at 1 line across a 429'd closeout | `internal/executor/agent.go` `runCloseoutTurn`/`runAgent`; `TestCloseoutRateLimitResumesNotReRuns` |
| **T10 commit-msg trailer hook**: installer writes `.git/hooks/commit-msg` — exactly one, well-formed (hex 16+) `Task-Queue-ID:` footer; positive/malformed/duplicate smoke-tested. ID-reuse rejection deliberately NOT built (multiple commits per task are the norm) | `scripts/install-pre-commit.sh` |
| **T11 commits-per-ID view**: `tq show --commits` scans the payload repo's git log, verdicts MISSING FOOTER / AMBIGUOUS / ok (live-verified on a footer-less scratch task) | `cmd/tq/main.go` `commitsForTask` |
| **T13 lineage hygiene**: `41b817b` sweep clean (only TODO item text + plan snapshot); CHANGELOG cites no SHAs; no workflow FF assumptions; **lineage fork HEALED** — v0.2.0 + sub-tags are ancestors of master AND origin/master again → TODO items 164/165 premises resolved de facto, no force-push needed | `git merge-base --is-ancestor` checks |
| **Small fixes**: doctor err113 static sentinel; dead `waitFor` test helper deleted after verifying the gopls hint genuine; M79 literal-leak sweep clean; M80 help-text smoke clean (only false-positive `task(s)` prose); M82 dead-export triage — BudgetView/BoardColumn/DashboardData all LIVE, zero dead exports | `cmd/tq/doctor.go`, `internal/webui/webui_test.go` |
| **Docs**: SECURITY.md CI defense-layers section; FEATURES.md rate-limit row + http-429 note; DOMAIN_LANGUAGE.md rate-limit park/gate terms; AGENTS.md per-repo gate + closeout-resume contract, commit-msg hook + check-ci conventions; CHANGELOG Added section | — |

## b) PARTIALLY DONE

1. **Round-11 coverage**: tiers 1% (T1–T4) and 4% (T5–T9) COMPLETE; 20% tier
   has T10/T11/T13 done, T12 (index cross-checks) and T14–T19 open; the
   →100% tier (T20–T27) only touched (T21/T25/T27 micro-slices). TODO_LIST
   updated as items completed; ~15 unchecked new-plan items remain.
2. **T7 backend-tag proxy verification**: not started (needs network module
   resolution checks + a throwaway clean-room module).

## c) NOT STARTED

T12, T14–T20, T22–T24, and the remaining T25/T27 micro-tasks — all still
mapped in the Round-11 plan with micro-task granularity.

## d) TOTALLY FUCKED UP

1. **A concurrent agent clobbered my RFC3339 TZ-fix edit mid-session** (the
   14:25/14:28 daemon commits captured the file without my fix); I re-applied
   it after the TZ sweep caught the regression. Lesson reinforced: in hot
   files, re-read immediately before every write AND re-verify the edit
   landed in the file's current state, not the pre-commit-daemon's snapshot.
2. **First sqlite-pin test failed twice on API signatures** (Fail/Heartbeat/
   Complete arities) — I wrote the test from memory instead of grepping the
   store's signatures first. Cheap mistake, avoidable round trips.
3. **The smoke script failed 4 times on JSON-shape guesses** (`task_id` vs
   `taskId`, pretty vs compact). Should have inspected one real `facts
   --json` output BEFORE writing the assertions.

## e) WHAT WE SHOULD IMPROVE

1. **e2e smoke JSON assertions should use `python3`/`jq` parsing, not grep
   patterns** — the smoke now greps; a key rename breaks it silently-then-
   loudly again.
2. The DLQ triage runbook's VERIFY rows deserve a scripted pre-fill: a
   `tq dlq --triage` command emitting the table with lastError class +
   item text (the runbook was assembled by hand from 21 `tq show` calls).
3. `closeoutPending` is in-process only — a pool restart during a rate-limit
   window degrades to a full work re-run; a payload-side session field
   (payload v2) would persist it. Rides the next re-tag.
4. ci-local's check-ci step will block the NEXT push while this fix's CI run
   is still red — push consciously (the fix commit makes CI green, then the
   gate unblocks) or CI_CHECK=off once.
5. Provider-tag extraction from output (`provider=<x>`) remains unimplemented;
   repo-keying covers the isolation need, but a per-provider observability
   line in `tq doctor` would answer "WHICH provider is limited" directly.
6. gosec job is `continue-on-error` but still shows as a failed JOB in `gh
   run view` — confusing for triage; consider `--exclude` config to shrink
   the FP noise per the 2026-09-10 triage.

## f) Up to 50 things we should get done next

1. Push the ci.yml GOEXPERIMENT fix + verify the next CI run goes green (T5 closure).
2. Re-run `nix build` before any owner flip (the go.mod bumps could theoretically affect FOD; they didn't locally).
3. OWNER: SystemNix input flip + `nix run .#deploy` per the runbook §2.
4. OWNER: post-flip smoke checklist §3 (6 checks).
5. OWNER: DLQ triage per the runbook §4 table.
6. OWNER: `--allow-writes` on tq-serve in the same window.
7. T12: check-status-index filename-ID ↔ trailer cross-check + date-drift check (M42/M43).
8. T14: execute release.sh on a fixture repo (M47–M49).
9. T15: release.sh ancestry audit + doctor warning for unreachable release tags (M50–M53).
10. T16: extract `scripts/for-each-module.sh`; unify golangci pin; actionlint (M54–M56).
11. T16: advisory lint job summary line; testdata go.mod check; lll gate on changed lines (M57–M59).
12. T17: prove root .golangci.yml resolution inside a sub-module; document (M60/M61).
13. T17: per-module lint baseline quantification (M63).
14. T17: slice-triage round 2 — goconst, mnd, paralleltest, testpackage policy-or-fix (M64–M67).
15. T18: mark the three verified stale-done items [x]; checked-in baseline file (M68/M69).
16. T19: docs-health annotate the six 2026-09-07 reports (M70–M72).
17. T20: webui FilterState round-trip pins + `?q=` handler e2e + LIKE cap (M73–M78).
18. T21: KanbanBoard + SEO/icons adoption evals (M83/M84).
19. T22: fullcore hermetic smoke + drain-deadline pin + postgres example run (M85–M89).
20. T23: `tq pool-health` command + harvest /tmp hot-item flag (M90–M93).
21. T24: worktree-per-agent design doc; daemon-attribution proposal; history-rewrite policy line; consumer ghost ADR; interface-dedup comparison (M94–M101).
22. T25: varnamelen raw-site renames; runactor exitCause rename; scoped lint proof (M103–M105).
23. T25: examples hardening (IdleTimeout/ReadTimeout); SHA256SUMS manifest (M106/M109).
24. T25: AGENTS gotcha for `git check-ignore` before evidence copies (M108).
25. T26: session-close bridge design doc + PreToolUse prototype (M110–M113).
26. T27: required-checks proposal; gosec/govulncheck CI summaries; govulncheck hard gate (M116–M119).
27. T27: release-gates smoke under GIT_CONFIG_GLOBAL=/dev/null; task.AllStatuses prep; dependabot draft (M120–M123).
28. M124: post-deploy retro (2026-09-18, per runbook §5).
29. M7/M16 partial: hand the owner the runbook URL + flip checklist directly in the next owner interaction.
30. Verify the commit-msg hook survives a `rm -rf .git/hooks` + reinstall (idempotence proof is in the installer, but not smoke-tested end-to-end via git commit).
31. Add `--commits` view to `tq facts` output too (the show-only surface misses the facts-first workflow).
32. Extend the parked-pin to cover `MarkOrphaned` interplay (an orphaned RUNNING task vs a parked PENDING twin).
33. Consider persisting `closeoutPending` evidence into the requeue fact detail (observability: "will resume at closeout").
34. http executor: honor `Retry-After` HTTP-date format in tests via a fixture server (currently only unit-pinned).
35. Budget view: the new stats parked count is untested for the JSON shape — add a contract test.
36. doctor: parked check could name the earliest `not_before` ("until 19:40").
37. Rate-limit gate: log the armed provider tag when the output carries `provider=<x>` (cheap observability, no isolation change).
38. Bridge: same 429-transient treatment for `NotifyDeadPool` direct posts (audit only covered ingest).
39. Smoke: ratelimit-e2e should also assert the SECOND claim happens AFTER not_before (currently only checks park state).
40. SQLite `Fail` fix: add a stale-fail pin to the POSTGRES conformance battery too (subtest exists; assert the fact COUNT didn't grow).
41. ci.yml: consider `concurrency` group to cancel superseded runs (cheaper red-signal latency).
42. docs: the runbook's DLQ table should be regenerated at flip time (dead set grows).
43. Triage remaining `go.mod` hygiene flake (`go mod verify` buildcache miss) — maybe `GOMODCACHE` pin in ci-local.
44. TODO item 152's ID-reuse note: document the multi-commits-per-task norm in RELEASE.md so future hooks don't regress.
45. `tq show --commits`: bail out with a clear note when `git` is missing from PATH.
46. WebUI: surface the parked count on the dashboard nowband (stats payload already carries it).
47. Consider `retry_in_until` (absolute time) in RequeueEvidence — `retry_in_ms` alone forces readers to re-derive the wall clock.
48. Executor: `WithoutCloseout` should copy nothing gate-related — verify a repo-armed gate doesn't leak to the review clone (test only covers the shared gate).
49. Pre-commit hook installer: check it still works when `.git` is a worktree file (hooks path differs).
50. Add the rate-limit e2e smoke to ci-local.sh's smoke section (it currently runs only ad-hoc).

## g) Questions I can NOT figure out myself

1. The GOEXPERIMENT root cause suggests `go env -w GOEXPERIMENT=jsonv2` on the POOL user (owner-run against the writable store path) — is fixing the service env via SystemNix `Environment=` (runbook §3 check 4) the chosen fix, or do you want the repo to drop json/v2 usage instead?
2. Should the DLQ VERIFY rows (7 tasks whose work may have landed via other windows) be resolved by ME running the footer checks (`git log --grep Task-Queue-ID`) in each sibling repo before the flip, so the owner's table is decision-complete?
3. The bridge 429 fix changes alert delivery semantics (retry instead of drop) — acceptable to ride the next patch release, or does PapDashboard-side rate limiting need an explicit backoff instead?

---

Gates at window end: root build/vet/test GREEN; all 7 sub-modules GREEN;
gofmt clean; doc gates (todo/status-index/features/ghost-archives/doc-refs/
go-mods) GREEN; smokes ratelimit-e2e + status-loop + webui GREEN.

**Environmental note (end of window)**: /tmp (48G tmpfs) hit 100% from OTHER
projects' artifacts (monitor365 ~24G, playwright/pnpm caches) — late
verification runs needed `TMPDIR`/`GOTMPDIR` redirected to
`~/.cache/go-tmp` (git init + cgo write to /tmp regardless of GOTMPDIR).
One webui SSE race test flaked under the parallel-load of my own background
jobs and passed standalone + in a full re-run (no webui runtime code was
touched this window). The host's /tmp needs an owner-side cleanup or a
tmpfs size bump; other sessions' files were not touched.

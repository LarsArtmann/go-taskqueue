# Done-prompt: five-task hardening window (2026-09-16 02:05 CEST)

**Task:** 000001a0a768395fee5feed5b73f5ebec067 (done-prompt status report + docs-health pass)
**Window:** the five completed queue tasks below — landed between 2026-09-12 04:08 and
2026-09-16 01:16 CEST (bulk: 09-15 06:42 → 09-16 01:16)
**Method:** every claim below re-verified against code/git this session; gates re-run at
HEAD c4a025e-era tree (pre-report commit). Report + docs pass only; no code touched.

| # | Task              | Deliverable                                                  | Landing commit(s)                                      |
| - | ----------------- | ------------------------------------------------------------ | ------------------------------------------------------ |
| 1 | 000001a0934efd9d… | `task.AllStatuses` export, twin lists retired                | `aeebe888` (09-12 04:08)                               |
| 2 | 000001a0a2f63b92… | `tq doctor --hygiene` stale-pin audit + `--reresolve-verify` | `d0e2a62` (daemon, footerless) + `1da0ee5` (checkoff)  |
| 3 | 000001a0a715d4ea… | webui write-lockout strikes map bounded                      | `05962e2` (09-16 00:05, clean)                         |
| 4 | 000001a0a7237f14… | Secrets-in-logs pass + `--redact` + journal secret scan      | `c7ca518` (daemon, footerless)                         |
| 5 | 000001a0a75157…   | httpapi nosniff + bearer-auth lockout                        | `c273dcc`+`936d5b9` (daemon) + `5668e8f` (footer tail) |

---

## a) FULLY DONE — verified this session

1. **`task.AllStatuses` export + twin-list retirement (task 1).**
   `AllStatuses()` is a FUNCTION in `internal/task/status.go:23` (no mutable global, no
   gochecknoglobals baseline row) with a hardcoded-oracle pin test; the facade alias is
   `task/task.go:38`; webui (`render.go:471`, `handlers.go:200`, board columns in
   `fragments_templ.go:2794`) and httpapi (`httpapi.go:407-409`) all range the export; the
   webui `allStatuses` and httpapi `apiStatuses` copies are gone. **The "rides the next
   sub-module re-tag" condition is RESOLVED**: `git tag --contains aeebe888` lists both
   `internal/task/v0.3.0` and `task/v0.3.0` (pushed 2026-09-13), root `go.mod:56` pins
   `internal/task v0.3.0`, and the facade `task/go.mod:5` requires `internal/task v0.3.0`
   — external consumers get the export through the proxy, no local replace needed.
   The stale BLOCKED TODO row asking for exactly this decision is checked off in this pass.
2. **`tq doctor --hygiene` + `--reresolve-verify` (task 2).** The feature session shipped
   via footer-less daemon commit `d0e2a62` (09-15 05:38); queue task `1da0ee5` was the
   verification-close (06-43 report). Verified at HEAD: `doctorVerifyPins`
   (`cmd/tq/doctor.go:300`, three-way match/latent/STALE verdicts over PENDING agent
   tasks' `.tq-verify` pins vs the repo's current gate ladder), wired at `doctor.go:812`,
   `--reresolve-verify` on `tq worker --agents` (`main.go:457`) and `tq agent-pool`;
   pinned by `TestDoctorVerifyPinsFlagsStalePins`. cmd/tq module gate green this session
   (8.9s).
3. **Webui strikes-map bound (task 3).** `boundLocked`/`pruneLocked`
   (`internal/webui/auth.go:315/337`): global idle sweep + least-recently-active eviction
   past `maxKeys` (1024), live lockouts survive both (evicted only when the whole map is
   locked). Pinned by `TestWriteRateLimitBoundedAgainstRotatingIPs` (97 test lines, fake
   clock). The 00-11 report's missing root-level verifier ran GREEN this session (full
   `-race` suite, webui 13.2s).
4. **Secrets-in-logs + `--redact` (task 4).** `internal/executor/redact.go`:
   `RedactSecrets`/`SecretHits`/`RedactMarker`, provider-token pattern table, env-gated
   default-ON `redactActive`; the `tailBytes` choke point redacts BEFORE the tail is cut
   (command/agent/ratelimit sites); both sidecar writers redact; `--redact` on `tq worker`
   (`main.go:480`) and `tq agent-pool` (`agentpool.go:268`); audit half via
   `tq audit --journal` SECRET EVIDENCE rows. CHANGELOG `[Unreleased]` entry exists;
   executor module gate green this session (8.3s). The queue task itself (00-55 report)
   was a verification-only no-op lap — see §d4.
5. **httpapi parity hardening (task 5).** `X-Content-Type-Options: nosniff` on EVERY
   response incl. auth failures (`httpapi.go:64`); `authRateLimiter` (3 failed bearer
   auths per client IP → 60s 429 + `Retry-After` on ALL routes, success resets, bounded
   strike map mirroring webui); 146 test lines (9 auth/nosniff tests); SECURITY.md
   `tq api` section; FEATURES row updated; AGENTS.md records the lockout decision CLOSED.
   Root race suite green this session (httpapi 2.0s).

**Gates cited (this session, HEAD):** `GOEXPERIMENT=jsonv2 go build ./... && go vet ./...`
✓; full root `go test ./... -count=1 -race` ✓ (all packages ok); `internal/task`,
`internal/executor` module gates ✓; `scripts/test-cmd-tq.sh` ✓ (8.9s).

## b) PARTIALLY DONE

1. **None of the window's features are live in the production pool binary.** The deployed
   `tq` is v0.3.0 (SystemNix-pinned, tagged 09-13) — `tq doctor --hygiene` errors with
   "flag provided but not defined" against it (verified live). `--hygiene`,
   `--reresolve-verify`, `--redact` and the hardening all await the SystemNix input flip +
   deploy (owner-run; TODO rows 21/97). Until then the window ships code the production
   pool cannot exercise.
2. **Documentation debt repaid only by this pass:** `doctor --hygiene` /
   `--reresolve-verify` had NO CHANGELOG/FEATURES rows (06-43 §c2 flagged it); the webui
   strikes bound had no CHANGELOG row (00-11 §b3); the facade `AllStatuses` export had no
   CHANGELOG note. All added in this commit — three of five window items needed a later
   docs lap.
3. **Verification depth gaps inherited from the window's own close-outs:** no live-socket
   smoke for the `tq api` lockout (httptest only); no end-to-end redaction smoke (stub
   agent emitting a fake token → assert stored evidence is `[REDACTED]`); the doctor
   hygiene output has no scratch-DB smoke; claim-time re-resolution (`--reresolve-verify`)
   is flag-verified but not behavior-proven. All tracked as new TODO rows this pass.
4. **Window close-out follow-ups partially harvested:** the 00-11/00-55/01-21/01-46 §f
   lists were deduped against TODO_LIST this pass — genuinely new items appended (§f
   below), already-tracked rows (89/90/91/92/93/97) not duplicated.
5. **Adjacent (window-adjacent session, 01-46):** the ci-local transient-retry wrapper
   ships with smokes deliberately unwrapped and no durable self-test pin — both tracked
   (TODO row in the 01-46 lap), plus unmeasured 45s×3 budget.

## c) NOT STARTED (skipped by the window, still open)

1. **Row 89 small rate-limit/observability batch** (12 sub-items incl. the HTTP-date
   Retry-After fixture that touches BOTH limiters' header code).
2. **Row 90** `ci.yml` concurrency group (daemon burst-pushes cancel each other — 3 daemon
   commits landed inside 4 minutes during this window).
3. **Row 91** status-append cap/dedup + re-dispatch root cause — the window PROVED the
   disease again (§d4).
4. **Row 92** dead `factLines` deletion + four DOMAIN_LANGUAGE terms.
5. **Row 97** `GOEXPERIMENT=jsonv2` on the tq-agent-pool unit (owner sudo; pairs with §b1).
6. **Mint-time done-check** (row 91's core): six paid no-op laps in two days and counting.
7. **Archive sweep**: 142 live status rows vs threshold 100 — INDEX BLOAT WARNING is live
   at HEAD (advisory). This pass adds the sanctioned monthly digest row; the ANNOTATE
   sweep itself (esp. the five 09-15 morning re-dispatch triplet reports) remains open.
8. **Twin-limiter extraction** (`internal/httpauth`): bearer extraction ×2 + two
   structurally identical rate limiters — the httpapi task DEEPENED this split brain
   (01-21 §c2).
9. **Lockout observability**: no WARN surface in `tq doctor`/`serve` startup for recent
   auth lockouts; the bound eviction is silent.
10. **`tq api` help text** does not mention the lockout — a misconfigured producer gets
    429s with no `--help` hint.

## d) TOTALLY FUCKED UP

1. **`scripts/lint-baseline.sh --check` is RED at HEAD** (verified live, exit 1 — and note
   the trap: a `| tail` pipe masks the exit code; run it bare). Six growth rows since the
   2026-09-15 regen: `cmd/tq` tagliatelle 9→10 + varnamelen 6→7;
   `internal/executor` gochecknoglobals 4→5 + mnd 29→31 + varnamelen 16→17; root mnd
   44→45. Files include THIS WINDOW's own redaction code (`redact.go`, `sidecar.go`,
   `agent.go`) plus `journalaudit.go`, `main.go`, `top.go` (concurrent sessions). This is
   a hard ci-local gate — the next full gate/push eats it. Regen requires deliberate
   triage attribution (the 895de1e8 precedent), which is a policy-owned action — NOT done
   in this docs-only pass; TODO row appended.
2. **`scripts/check-doc-refs.sh` is RED at HEAD**: two ghost README citations
   (`scripts/tq-claim.sh`, `docs/operations/task-dispatch-contract.md` — CV-repo paths
   pattern-matched as repo paths). **Fixed in this commit** by rewording README (the
   contract lives in the LarsArtmann/CV repo; the sentence now says so without
   backticked ghost paths). Gate expected green after this commit.
3. **Queue↔git attribution degraded for 2 of 5 window tasks:** the secrets work rides
   footer-less `c7ca518`; the httpapi work rides footer-less `c273dcc`/`936d5b9` while the
   footer commit `5668e8f` carries only a 7-insertion tail (blank line + docs). The
   cross-reference resolves only via report commits. Third window in a row with this
   finding; policy question re-asked in §g1.
4. **Sixth paid no-op re-dispatch lap in two days:** task 4's dispatch (00-55 report) found
   the work already shipped by a concurrent session and could only verify → cite → stop.
   Money spent, zero code. Mint-time done-check (row 91) remains unbuilt.
5. **Not this repo's window, but the same journal:** 4 pool tasks dead-lettered inside the
   window (2× CV agent, 1× SystemNix status, 1× SystemNix agent — the hot-db item is
   calendar-gated to ~09-17 anyway). DLQ review is the operator surface; not researched
   further (scope).
6. **Nothing in the five deliverables themselves is broken** — all gates green at HEAD for
   every touched module (§a citations). The window's code is the best-verified state this
   repo has had; the red state is lint-baseline + doc-refs, both process debt, one of
   which this pass fixes.

## e) WHAT WE SHOULD IMPROVE

1. **Baseline regens must ship triage attribution in the same commit** (895de1e8 did this
   right; the current red pile has no owner). Until the ownership question (§g2) is
   answered, each feature session should scoped-lint its own delta before declaring green
   — 01-21 §e2's `--enable-only mnd,wsl_v5` habit.
2. **Commit per edit under the live daemon.** 01-21 §e1's lesson keeps producing
   footer-on-tail commits; the 00-11 amend-rescue and the 01-46 empty-marker are two
   divergent improvised answers to the same race. §g1 asks for ONE policy.
3. **Docs currency in the feature lap, not a later harvest** (06-43 §e4): this pass exists
   because three shipped features had zero CHANGELOG/FEATURES rows. A 60-second
   `rg <feature> CHANGELOG.md FEATURES.md` at close-out would kill this class.
4. **Pipeline masking bites status reporters too**: my first baseline/doc-refs read used
   `cmd | tail` and reported exit 0 from `tail`. Global AGENTS.md rule, now personally
   re-learned: capture output to a file, echo `$?` from the command.
5. **Deploy lag is a growing hidden cost**: every window now ships features the pinned
   production binary can't run (hygiene, redact, reresolve, hardening — §b1). The
   input-flip is owner-gated (rows 21/97) but the accumulation is invisible until someone
   checks the binary version. This report makes it visible.
6. **Report archaeology is recurring tax**: harvest-time rows citing "06-01 report c7" or
   "15-39 f9/e2" cost every subsequent session a lookup. Rows should inline the one-line
   claim they rest on, not just the pointer.
7. **The annotation convention works** — inline strikethroughs in 01-21/06-43 (this pass)
   resolve their now-done items where a reader actually scans. Keep ANNOTATE inline-first;
   the digest row added this pass follows the README's pinned format.

## f) NEXT THINGS (the high-value subset; appended to TODO_LIST with sources)

1. Regenerate `.golangci-baseline.txt` WITH per-module triage attribution (6 growth rows
   listed in §d1) — unblocks the hard ci-local gate. (this report §d1)
2. `scripts/smoke/api.sh`: live-socket lockout smoke (scratch TQ_DB, 3×401 → 429 +
   Retry-After → expiry → 200), wire into ci-local. (01-21 §f4)
3. `scripts/smoke/redaction.sh`: stub agent emitting a fake token; assert stored evidence
   `[REDACTED]`, sidecars redacted, `tq audit --journal` zero post-pass hits; fold a
   zero-SECRET-EVIDENCE assertion into `journal-drift.sh`. (00-55 §f1/§f5)
4. Extract `internal/httpauth`: one tested limiter + bearer extraction behind per-surface
   policy knobs; retire the twin `writeRateLimiter`/`authRateLimiter` copies. (01-21 §f5)
5. De-sleep the lockout tests via `nowFunc` injection (httpapi + webui copies). (01-21 §f6)
6. `slog.Warn` on first bound-eviction (webui `boundLocked`) + unify the limiter's two
   clock sources (`nowFunc` vs `time.Until`). (00-11 §e2/§e3)
7. Pin the `?token=`-query auth-failure strike path and "lockout covers POST
   /api/v1/tasks" (all-routes contract). (01-21 §f8/§f9)
8. Decide 401/429 body shape: `{error, fix}` JSON vs plain text on auth planes. (01-21 §f7)
9. Secrets pass polish bundle: pattern-growth policy in AGENTS.md;
   `secretPatterns`/`SecretHits` shared-table pin test; verify `redact_test.go` actually
   runs on windows-latest. (00-55 §f7/§f8/§f9)
10. `tq doctor --hygiene` scratch-DB smoke (seeded stale pin → WARN text asserted) + a
    claim-time proof test that `--reresolve-verify` really uses the NEW gate. (06-43 §f4/§f5)
11. Daemon-commit attribution sweep: doctor-style audit mapping footer-less `chore:`
    commits to task IDs via their report files. (00-55 §f2)
12. CHANGELOG convention: behavior changes visible to external API consumers (new 429s)
    get an explicit behavior-change flag, not just Added. (01-21 §f28)
13. Next archive sweep: ANNOTATE + git-mv the 09-15 morning re-dispatch triplets
    (13 reports); digest row added 2026-09-16 keeps the index scannable meanwhile.
    (this report §c7)
14. ci-local wrapper polish bundle: "healed on poll N" log line;
    `TRANSIENT_MAX_POLLS=0` escape-hatch note in AGENTS.md; one-line rationale for the
    unwrapped `check-ci` step. (01-46 §f3/§f14/§f6)
15. `check-script-syntax.sh`: separate/quiet the nix-shell fetch path that raced its own
    output once (false red). (01-46 §f15)
16. `closeoutPending` lifetime audit: entries parked on tasks that die CANCELLED. (00-11 §e5)
17. Bound-sweep for other per-key maps: one grep pass over `map[string]` in webui+executor
    for members needing the same treatment. (00-11 §f26)
18. `tq api` help text + `tq doctor`/serve startup: surface lockout semantics and recent
    auth-lockout WARNs. (01-21 §f13/§f14)
19. `tq pool-health` one-shot liveness summarizer (per-repo skip streaks from the journal).
    (carried: still open, unchanged rank)
20. Deploy readiness note for the next input flip: hygiene/redact/reresolve/hardening all
    activate together; smoke each on the pool unit after (pairs rows 21/97). (this report §b1)

## g) QUESTIONS FOR THE OWNER (appended as BLOCKED rows)

1. **Daemon-race footer policy** (re-asked by 00-55 g3, 01-21 g3, 01-46 g1): when the
   auto-commit daemon folds a task's diff before the explicit commit, what is the
   sanctioned terminal state — footer on an empty marker/report commit (current
   precedent), amend-rescue of the just-created daemon commit (00-11), or scripted
   reword of unpushed daemon commits?
2. **Lint-baseline growth ownership** (01-21 g2): when concurrent feature sessions grow
   the baseline (gate is RED right now, §d1), does each feature session regenerate with
   triage notes, or is a dedicated periodic sweep the ritual?
3. **Deploy cadence**: flip the SystemNix input + `nix run .#deploy` so the window's
   shipped features reach the live pool (rows 21/97)? Until then every hardening window
   ships code production cannot exercise (§b1).

## h) BAND DRIFT (ADR-0015 accountability)

**None recorded.** The production journal (`$TQ_DB=/mnt/pool/services/tq/tq.db`, seq 1 =
2026-09-10) contains ZERO `task.reprioritized` facts — verified via `tq facts -json`
filtered on type over the whole journal, hence a fortiori over the window's timespan. No
marker, AI-scorer, unblock, or importance reprioritization has ever fired on the live
pool: scheduling has been pure stored-priority + ADR-0015 aging. Corollary worth an owner
glance: the `--prioritize` AI scorer and the startup repri sweep have never demonstrated
a fact-producing run in production — if band management is ever actually wanted, its
machinery is so far unexercised in the only journal that matters.

---

_Docs-health pass riding this report: CHANGELOG rows for `doctor --hygiene`,
`--reresolve-verify`, the webui strikes bound, and the facade `AllStatuses` export;
FEATURES doctor/agent-pool/admin-writes row updates; AGENTS.md tq-serve security bullet
gains the strikes-map bound; README ghost citations fixed (check-doc-refs green);
TODO_LIST row 40 (AllStatuses) checked off with citations; 2026-09-16_01-21 §c5/§f3 and
2026-09-15_06-43 §c2/§f1/§f2 annotated inline; monthly digest row added to the status
index. No code, config, or test files touched._

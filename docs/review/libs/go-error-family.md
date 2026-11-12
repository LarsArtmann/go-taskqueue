# go-error-family — Library Utilization Review

**Verdict:** ADOPTED — low (2.5/12 capability rows leveraged; was 1.5/12 before the 2026-11-09 landing) — two load-bearing seams now: the readmodel fold → cqrs-lite dead-letter gate AND the executor's typed refusals classified into the shared taxonomy; the rest of the repo still runs parallel hand-rolled taxonomies.

**2026-11-11 refresh:** F2 PARTIALLY landed — `PermanentError` and `PreflightError` now implement `ErrorFamily()`/`ErrorCode()` into the SHARED taxonomy (internal/executor/executor.go:53-54, 94-95 — exactly the finding's first half, "fleet review f8"), with `errorfamily.Classify`/`IsRetryable` pins at internal/executor/executor_test.go:178-200; the `WrapOnce` collapse (finding's second half) was NOT taken — `Permanent` still hand-rolls the AsType double-wrap guard (executor.go:66-73). Score re-derived: 13 → 21. Call-site inventory grew 8 → 14 occurrences; §1/§2 cites re-verified against HEAD 2026-11-11 by grep; checkout now `v0.12.0-8-g5beae0e`, still pinned v0.12.0 = latest tag.

**Pinned:** v0.12.0 — direct requires: `go.mod:109` (root/app layer), `internal/readmodel/go.mod:13`; `// indirect` in 16 more modules (via go-retry + go-cqrs-lite). Verified 2026-11-08 by `grep -rn 'go-error-family' --include=go.mod . | grep -v vendor`.

**Latest:** v0.12.0 (local checkout /home/lars/projects/go-error-family, re-checked 2026-11-11 by `git -C … describe --tags` → `v0.12.0-8-g5beae0e`; Unreleased section empty).

**Prior coverage:** none (first libs-review report).

**Reviewed:** 2026-11-08 by reading /home/lars/projects/go-error-family/{README.md,FEATURES.md,CHANGELOG.md,classify.go,registry.go,handle.go,http.go} + repo greps (`grep -rn 'errorfamily\.' --include='*.go' . | grep -v '^./vendor/'` → exactly 8 call sites) + erraudit probe (see §3 F8).

## 1. Usage inventory

All 14 call sites (verified 2026-11-11 by the grep above; no other use):

| File | Line | Call | Role |
| --- | --- | --- | --- |
| internal/readmodel/events.go | 255 | `WrapCorruption` (`readmodel.enqueue_detail`) | malformed enqueue detail → non-retryable |
| internal/readmodel/events.go | 295 | `WrapCorruption` (`readmodel.repri_detail`) | malformed repri detail → non-retryable |
| internal/readmodel/host.go | 58 | `NewRejection` (`readmodel.sequence_not_positive`) | SeqToEventID input guard |
| internal/readmodel/host.go | 132 | `NewRejection` (`readmodel.checkpoint_not_sequence_derived`) | refuse foreign checkpoint IDs |
| internal/readmodel/host.go | 208 | `NewCorruption` (`readmodel.event_not_sequence_derived`) | fold refuses non-seq events |
| internal/executor/executor.go | 53 | `ErrorFamily() → Rejection` | `PermanentError` classified into the SHARED taxonomy (2026-11-09 landing) |
| internal/executor/executor.go | 94 | `ErrorFamily() → Transient` | `PreflightError` classified transient (requeue-without-burn) |
| internal/readmodel/host_test.go | 352 | `NewCorruption` (`test.poison`) | deterministic poison |
| internal/readmodel/host_test.go | 403 | `IsRetryable` | pins poison contract (non-retryable) |
| internal/httpapi/httpapi_test.go | 854 | `NewCorruption` (`test.api-poison`) | deterministic poison |
| internal/executor/executor_test.go | 178 | `Classify(perm) == Rejection` | pins the permanent refusal's family (2026-11-09 landing) |
| internal/executor/executor_test.go | 182 | `IsRetryable(perm)` | pins permanent → non-retryable through the lib's own predicate |
| internal/executor/executor_test.go | 196 | `Classify(pre) == Transient` | pins the preflight refusal's family |
| internal/executor/executor_test.go | 200 | `!IsRetryable(pre)` negation guard | pins preflight IS retryable |

The runtime classifier is NOT in this repo: the readmodel's fold errors flow into vendored go-cqrs-lite, whose projection host gates retries on `errorfamily.IsRetryable` (vendor/github.com/larsartmann/go-cqrs-lite/projectionhost/v4/worker.go:281 — "Unclassified errors stay retryable (IsRetryable fails open)"). tq mints the family; the platform consumes it. No registry, no classifiers, no stdlib defaults, no HTTP/CLI/logging boundary use anywhere.

## 2. Capability inventory

| Capability | Status | Evidence |
| --- | --- | --- |
| Family constructors (`New*`/`Wrap*`) with stable codes | 🟢 FULL | 5 production sites, 5 stable `readmodel.*` codes (§1) |
| Boundary classification (`Classify`/`IsRetryable`/`Code`) | 🟡 PARTIAL | runtime consumer is vendored cqrs-lite (worker.go:281); in-repo now 6 pin sites (host_test.go:403 + executor_test.go:178-200) but production code still never calls Classify directly |
| Multi-error worst-severity (`errors.Join` + `Classify`) | 🔴 MISSED | 7 join sites (runactor.go:157-174, cmd/tq/main.go:4363) render raw; never classified |
| Domain error types implementing the six interfaces | 🟢 FULL | `PermanentError`→Rejection (executor.go:53-54) + `PreflightError`→Transient (executor.go:94-95) — F2's first half landed 2026-11-09; cqrs-lite's gate and go-retry's default predicate classify executor refusals without bespoke checks |
| Sentinel registration (`RegisterClassification`) | 🔴 MISSED | `internal/task/errors.go:7-14` sentinels unregistered |
| Dynamic classifiers (`RegisterClassifier`) | 🔴 MISSED | store-busy detected by string sniff (internal/worker/worker.go:442-450, now also extended 517 BUSY_SNAPSHOT) |
| `WrapOnce` idempotent wrap | 🔵 REJECTED BY DESIGN (2026-11-12) | the hand-rolled `Permanent` guard is the DOMAIN analog: `WrapOnce` collapses onto the library's `*Error`, but `PermanentError` is tq's domain type — a swap would make `ClassifyFailure`'s `AsType[*PermanentError]` miss every fresh wrap and degrade the whole class to TRANSIENT (classify.go:46), break the `"permanent: "` prefix + identity no-op pins (executor_test.go:144-165) |
| `errorfamilytest` assertions | 🔴 MISSED | host_test.go:403-405 + executor_test.go:178-200 hand-roll `AssertRetryable`/`AssertFamily`'s job |
| Exit codes (`ExitCode`/`ExitCoder`) | 🔴 MISSED | cmd/tq/main.go:128,168,173,194 hardcode `os.Exit(1/2)` |
| HTTP boundary (`HTTPStatus`/`HTTPHandler`/`WithHTTPStatus`) | 🔴 MISSED | internal/httpapi hand-maps statuses (httpapi.go:333,351,385,409,421); family never read |
| `LogError` structured logging | 🔴 MISSED | slog calls hand-pack err fields (e.g. worker.go:406) |
| Registry scoping (`NewRegistry`/`Clone`) | ⚪ N/A | single-process CLI/service; the only classifier consumer (cqrs-lite host) uses the default registry |
| `RegisterStdlibDefaults` | ⚪ N/A | fold only needs the terminal classes it mints explicitly (Corruption/Rejection); fail-open Transient for everything else is the wanted restart semantics |
| `HandleError` CLI pipeline + templates | ⚪ N/A | tq's CLI error UX is its own evidence-rich rendering (`tq show` failure sections); What/Why/Fix stderr output is a different product contract |
| Submodules `diagnose`/`bridge`/`agent` | ⚪ N/A | separate Go modules; no oops enrichment or diagnostic-rule runner in this repo |

## 3. Findings

**F1 — The adopted seam is correct and load-bearing.** lib: `WrapCorruption`/`NewCorruption` constructors (checkout constructors.go; README "Classification precedence"). repo: events.go:255/295 classify malformed fact details as Corruption so the projection host dead-letters instead of burning its restart budget — the exact mechanism AGENTS.md's readmodel drift recovery relies on, pinned by host_test.go:352-405. This is the right use of the lib; nothing here should change.

**F2 — `Permanent` re-implements `WrapOnce` — FIRST HALF RESOLVED (2026-11-09, "fleet review f8").** lib: `WrapOnce(err, family, code, msg)` is the idempotent boundary wrap (checkout constructors.go; FEATURES "Error Construction"). repo: the finding's recommended methods LANDED — `PermanentError.ErrorFamily() → Rejection` + `ErrorCode() "executor.permanent"` (internal/executor/executor.go:53-54) and `PreflightError.ErrorFamily() → Transient` + `"executor.preflight"` (executor.go:94-95) — so cqrs-lite's dead-letter gate and go-retry's default predicate now classify executor refusals without bespoke checks, pinned by executor_test.go:178-200 (`Classify` + `IsRetryable` both directions). NOT taken — and RULED OUT 2026-11-12 (repair-verify-deepen T18 verification): `WrapOnce` collapses onto the library's `*errorfamily.Error`, while `PermanentError` is the domain type every consumer keys on — `ClassifyFailure` (classify.go:46) detects permanence via `AsType[*PermanentError]`, the double-wrap pin (executor_test.go:144-165) pins the identity no-op + `"permanent: "` prefix, and the domain guard already implements the same three clauses (nil passthrough, already-wrapped no-op, wrap) for the domain type. Swapping the guard onto `WrapOnce` would silently degrade every permanent refusal to the transient retry ladder. The wrap-construction path stays bespoke BY DESIGN; DirtyTree keeps its bolt-on. Impact: none — the family methods answer "is this retryable?" everywhere that matters.

**F3 — Store-busy string sniffing is a classifier's job.** lib: `RegisterClassifier(func(error) (Family, bool))` exists precisely for dynamic third-party errors (checkout registry.go; README "Classifiers"). repo: worker.go:442-450 `isTransientStoreBusy` does `strings.Contains(msg, "SQLITE_BUSY" \|\| "database is locked")` (now also extended 517 BUSY_SNAPSHOT) — and go-retry then consumes that predicate (worker.go:404). Registering a classifier for the sqlite/postgres busy error shapes would make `errorfamily.IsRetryable` correct at every future call site, not just this one.

**F4 — Task sentinels are invisible to the family.** lib: `RegisterClassification(sentinel, family)` maps stable sentinels (checkout classify.go). repo: internal/task/errors.go:7-14 (`ErrNotFound`, `ErrLeaseNotHeld`, `ErrDuplicateID`, `ErrInvalidTransition`) are plain `errors.New`; watermark.go:116 `PermanentOutcome` hand-lists two of them with `errors.Is`. Registration would express the same table once and classify any sentinel that ever crosses a family-aware boundary.

**F5 — HTTP boundary unused where an HTTP API exists.** lib: `HTTPStatus(err)`/`HTTPHandler(fn)` map family→status and write leak-safe JSON (checkout http.go). repo: internal/httpapi hand-maps every status (httpapi.go:333,351,385,409,421); its poison test (httpapi_test.go:854) mints family errors the API itself could never act on. Impact: modest — the API's handlers are success-shaped; the win is uniformity if error-returning handlers grow.

**F6 — CLI exit codes hardcoded.** lib: `ExitCode(err)` derives BSD sysexits codes from family (checkout classify.go). repo: cmd/tq/main.go:128,168,173,194 hardcode 1/2. Adopting `ExitCode` would distinguish EX_TEMPFAIL (75) from usage (2) — but it is a CLI behavior change (scripts may pin rc=1) and out of scope for this review's recommendation; recorded as an option, not a defect.

**F7 — Test helpers skipped.** lib: `errorfamilytest.AssertRetryable/AssertFamily/...` (checkout errorfamilytest/). repo: host_test.go:403-405 hand-rolls the assertion. One-line swap, zero risk.

**F8 — The erraudit gate does NOT force any of this.** The gate runs `--enforce-go-error-family` (scripts/check-erraudit.sh:44), which per `erraudit lint --help` "flags stdlib error constructors as violations". Verified 2026-11-08 by probing the installed build (`erraudit --version` → 5c841cf) on a scratch module containing bare `errors.New`, `fmt.Errorf` (both plain and `%w`), and `errors.Join` under `--max`: zero stdlib_constructor findings — the rule is inert for these shapes in this build. Consequence: family adoption is entirely convention; the 88 `errors.AsType` sites (re-counted 2026-11-11; AGENTS.md "Go 1.26+ idioms") and the single `//nolint:legacyerrors` site (executor_test.go:160) are the only error-discipline surfaces the gate actually pins.

## 4. Score + top opportunities

Inputs (re-derived 2026-11-11): N=15 rows, N/A=3, A=12. FULL=2 (family constructors; domain error types), PARTIAL=1 (0.5), MISSED=9, MISUSED=0 → Σ=2.5.
Score = round(100 × 2.5/12) = **21 — low** (was 13 before the landing).

Top opportunities, in value order: (1) F3 — classifier for store-busy, deleting the string sniff (landed 2026-11-12, T07); (2) F7 — `errorfamilytest` swap (now 10 hand-rolled pin sites). F2-second-half is RULED OUT (see F2 — WrapOnce would break the domain-type classification); F5/F6 are optional and behavior-visible.

## 5. Version currency + risk

Pinned v0.12.0 == latest tag (checkout ahead of tag by 8 commits, `v0.12.0-8-g5beae0e`, Unreleased empty) — no action. v0.12.0 (2026-10-14, CHANGELOG) fixes nil-receiver `(*Error).Unwrap` (defensive only for typed-nil forwards; tq has no such site). Risk notes: the module declares `go 1.27` (checkout go.mod:3) while tq modules are `go 1.27.3` — compatible; the go-retry CHANGELOG records an incident where an error-family minor bump dragged a consumer's go pin (v0.9.0 "Fixed"), so treat error-family bumps as pin-sensitive in every module graph loader (AGENTS.md "New module + unpushed tag").

## 6. Conclusion

Adopted, now at TWO seams: the readmodel fold's corruption/rejection minting that drives cqrs-lite's dead-letter gate, and (since 2026-11-09) the executor's typed refusals classified into the shared taxonomy so the platform's gate and go-retry's default predicate both understand them. Everywhere else the repo still answers the same questions (retryable? whose fault? what status?) with private machinery (`Permanent`'s hand-rolled wrap, string sniffs, hardcoded exits). The score moved 13 → 21 with the executor landing; the fix list stays short and mechanical — F3/F7 are the highest-value remaining swaps.

## How to verify

```bash
cd /home/lars/projects/go-taskqueue
grep -rn 'errorfamily\.' --include='*.go' . | grep -v '^./vendor/'        # → exactly the 14 sites in §1
grep -rn 'go-error-family' --include=go.mod . | grep -v vendor            # → pinned versions + direct/indirect
git -C /home/lars/projects/go-error-family describe --tags                # → v0.12.0-8-g5beae0e
sed -n '281p' vendor/github.com/larsartmann/go-cqrs-lite/projectionhost/v4/worker.go  # IsRetryable DLQ gate
sed -n '253,258p' internal/readmodel/events.go                            # WrapCorruption #1
sed -n '51,56p;92,96p' internal/executor/executor.go                       # family methods (2026-11-09 landing)
sed -n '176,202p' internal/executor/executor_test.go                       # Classify/IsRetryable pins
sed -n '442,450p' internal/worker/worker.go                               # string-sniff predicate
sed -n '64,74p' internal/executor/executor.go                             # Permanent + hand-rolled WrapOnce (uncollapsed)
sed -n '7,14p' internal/task/errors.go                                    # unregistered sentinels
sed -n '44p' scripts/check-erraudit.sh                                    # --enforce-go-error-family wiring
grep -rn 'errors.AsType' --include='*.go' . | grep -v '^./vendor/' | wc -l # → 88
grep -rn 'nolint:legacyerrors' --include='*.go' . | grep -v '^./vendor/'  # → executor_test.go:160 only
# F8 probe (scratch dir; remove after): module with bare errors.New/fmt.Errorf/errors.Join,
# then: erraudit lint ./... --max   → zero stdlib_constructor findings on build 5c841cf
```

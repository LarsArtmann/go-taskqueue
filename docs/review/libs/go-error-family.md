# go-error-family — Library Utilization Review

**Verdict:** ADOPTED — low (1.5/12 capability rows leveraged) — one load-bearing seam (readmodel fold → cqrs-lite dead-letter gate), zero adoption anywhere else; the rest of the repo runs parallel hand-rolled error taxonomies.

**Pinned:** v0.12.0 — direct requires: `go.mod:109` (root/app layer), `internal/readmodel/go.mod:13`; `// indirect` in 16 more modules (via go-retry + go-cqrs-lite). Verified 2026-11-08 by `grep -rn 'go-error-family' --include=go.mod . | grep -v vendor`.

**Latest:** v0.12.0 (local checkout /home/lars/projects/go-error-family, checked 2026-11-08 by `git -C … describe --tags` → `v0.12.0-6-g7ccca5c`; Unreleased section empty).

**Prior coverage:** none (first libs-review report).

**Reviewed:** 2026-11-08 by reading /home/lars/projects/go-error-family/{README.md,FEATURES.md,CHANGELOG.md,classify.go,registry.go,handle.go,http.go} + repo greps (`grep -rn 'errorfamily\.' --include='*.go' . | grep -v '^./vendor/'` → exactly 8 call sites) + erraudit probe (see §3 F8).

## 1. Usage inventory

All 8 call sites (verified 2026-11-08 by the grep above; no other production use):

| File | Line | Call | Role |
| --- | --- | --- | --- |
| internal/readmodel/events.go | 255 | `WrapCorruption` (`readmodel.enqueue_detail`) | malformed enqueue detail → non-retryable |
| internal/readmodel/events.go | 295 | `WrapCorruption` (`readmodel.repri_detail`) | malformed repri detail → non-retryable |
| internal/readmodel/host.go | 58 | `NewRejection` (`readmodel.sequence_not_positive`) | SeqToEventID input guard |
| internal/readmodel/host.go | 132 | `NewRejection` (`readmodel.checkpoint_not_sequence_derived`) | refuse foreign checkpoint IDs |
| internal/readmodel/host.go | 208 | `NewCorruption` (`readmodel.event_not_sequence_derived`) | fold refuses non-seq events |
| internal/readmodel/host_test.go | 352 | `NewCorruption` (`test.poison`) | deterministic poison |
| internal/readmodel/host_test.go | 403 | `IsRetryable` | pins poison contract (non-retryable) |
| internal/httpapi/httpapi_test.go | 854 | `NewCorruption` (`test.api-poison`) | deterministic poison |

The runtime classifier is NOT in this repo: the readmodel's fold errors flow into vendored go-cqrs-lite, whose projection host gates retries on `errorfamily.IsRetryable` (vendor/github.com/larsartmann/go-cqrs-lite/projectionhost/v4/worker.go:281 — "Unclassified errors stay retryable (IsRetryable fails open)"). tq mints the family; the platform consumes it. No registry, no classifiers, no stdlib defaults, no HTTP/CLI/logging boundary use anywhere.

## 2. Capability inventory

| Capability | Status | Evidence |
| --- | --- | --- |
| Family constructors (`New*`/`Wrap*`) with stable codes | 🟢 FULL | 5 production sites, 5 stable `readmodel.*` codes (§1) |
| Boundary classification (`Classify`/`IsRetryable`/`Code`) | 🟡 PARTIAL | runtime consumer is vendored cqrs-lite (worker.go:281); in-repo only host_test.go:403 |
| Multi-error worst-severity (`errors.Join` + `Classify`) | 🔴 MISSED | 7 join sites (runactor.go:157-174, cmd/tq/main.go:4363) render raw; never classified |
| Domain error types implementing the six interfaces | 🔴 MISSED | `PermanentError`/`PreflightError` hand-roll a transient/permanent taxonomy (internal/executor/executor.go:45-63) |
| Sentinel registration (`RegisterClassification`) | 🔴 MISSED | `internal/task/errors.go:7-14` sentinels unregistered |
| Dynamic classifiers (`RegisterClassifier`) | 🔴 MISSED | store-busy detected by string sniff (internal/worker/worker.go:442-450) |
| `WrapOnce` idempotent wrap | 🔴 MISSED | re-implemented by hand in `Permanent` (executor.go:53-62, `errors.AsType` double-wrap guard) |
| `errorfamilytest` assertions | 🔴 MISSED | host_test.go:403-405 hand-rolls `AssertRetryable`'s job |
| Exit codes (`ExitCode`/`ExitCoder`) | 🔴 MISSED | cmd/tq/main.go:128,168,173,194 hardcode `os.Exit(1/2)` |
| HTTP boundary (`HTTPStatus`/`HTTPHandler`/`WithHTTPStatus`) | 🔴 MISSED | internal/httpapi hand-maps statuses (httpapi.go:333,351,385,409,421); family never read |
| `LogError` structured logging | 🔴 MISSED | slog calls hand-pack err fields (e.g. worker.go:406) |
| Registry scoping (`NewRegistry`/`Clone`) | ⚪ N/A | single-process CLI/service; the only classifier consumer (cqrs-lite host) uses the default registry |
| `RegisterStdlibDefaults` | ⚪ N/A | fold only needs the terminal classes it mints explicitly (Corruption/Rejection); fail-open Transient for everything else is the wanted restart semantics |
| `HandleError` CLI pipeline + templates | ⚪ N/A | tq's CLI error UX is its own evidence-rich rendering (`tq show` failure sections); What/Why/Fix stderr output is a different product contract |
| Submodules `diagnose`/`bridge`/`agent` | ⚪ N/A | separate Go modules; no oops enrichment or diagnostic-rule runner in this repo |

## 3. Findings

**F1 — The adopted seam is correct and load-bearing.** lib: `WrapCorruption`/`NewCorruption` constructors (checkout constructors.go; README "Classification precedence"). repo: events.go:255/295 classify malformed fact details as Corruption so the projection host dead-letters instead of burning its restart budget — the exact mechanism AGENTS.md's readmodel drift recovery relies on, pinned by host_test.go:352-405. This is the right use of the lib; nothing here should change.

**F2 — `Permanent` re-implements `WrapOnce` + a second taxonomy.** lib: `WrapOnce(err, family, code, msg)` is the idempotent boundary wrap (checkout constructors.go; FEATURES "Error Construction"). repo: internal/executor/executor.go:53-62 hand-rolls the identical double-wrap guard via `errors.AsType[*PermanentError]`, and the `PermanentError`/`PreflightError` pair is a private transient/permanent taxonomy parallel to Rejection/Transient. Impact: two vocabularies for one question ("is this retryable?"); a `Classified` implementation on `PermanentError` (`ErrorFamily() → Rejection`) would let cqrs-lite and go-retry's default predicate classify executor refusals without bespoke checks.

```go
// internal/executor — today: private taxonomy + hand-rolled idempotent wrap.
func (e *PermanentError) ErrorFamily() errorfamily.Family { return errorfamily.Rejection }
func (e *PermanentError) ErrorCode() string               { return "executor.permanent" }
// Permanent's AsType guard then collapses to: errorfamily.WrapOnce(cause, errorfamily.Rejection, "executor.permanent", "permanent")
```

**F3 — Store-busy string sniffing is a classifier's job.** lib: `RegisterClassifier(func(error) (Family, bool))` exists precisely for dynamic third-party errors (checkout registry.go; README "Classifiers"). repo: worker.go:442-450 `isTransientStoreBusy` does `strings.Contains(msg, "SQLITE_BUSY" \|\| "database is locked")` — and go-retry then consumes that predicate (worker.go:404). Registering a classifier for the sqlite/postgres busy error shapes would make `errorfamily.IsRetryable` correct at every future call site, not just this one.

**F4 — Task sentinels are invisible to the family.** lib: `RegisterClassification(sentinel, family)` maps stable sentinels (checkout classify.go). repo: internal/task/errors.go:7-14 (`ErrNotFound`, `ErrLeaseNotHeld`, `ErrDuplicateID`, `ErrInvalidTransition`) are plain `errors.New`; watermark.go:116 `PermanentOutcome` hand-lists two of them with `errors.Is`. Registration would express the same table once and classify any sentinel that ever crosses a family-aware boundary.

**F5 — HTTP boundary unused where an HTTP API exists.** lib: `HTTPStatus(err)`/`HTTPHandler(fn)` map family→status and write leak-safe JSON (checkout http.go). repo: internal/httpapi hand-maps every status (httpapi.go:333,351,385,409,421); its poison test (httpapi_test.go:854) mints family errors the API itself could never act on. Impact: modest — the API's handlers are success-shaped; the win is uniformity if error-returning handlers grow.

**F6 — CLI exit codes hardcoded.** lib: `ExitCode(err)` derives BSD sysexits codes from family (checkout classify.go). repo: cmd/tq/main.go:128,168,173,194 hardcode 1/2. Adopting `ExitCode` would distinguish EX_TEMPFAIL (75) from usage (2) — but it is a CLI behavior change (scripts may pin rc=1) and out of scope for this review's recommendation; recorded as an option, not a defect.

**F7 — Test helpers skipped.** lib: `errorfamilytest.AssertRetryable/AssertFamily/...` (checkout errorfamilytest/). repo: host_test.go:403-405 hand-rolls the assertion. One-line swap, zero risk.

**F8 — The erraudit gate does NOT force any of this.** The gate runs `--enforce-go-error-family` (scripts/check-erraudit.sh:44 (AGENTS.md:355)), which per `erraudit lint --help` "flags stdlib error constructors as violations". Verified 2026-11-08 by probing the installed build (`erraudit --version` → 5c841cf) on a scratch module containing bare `errors.New`, `fmt.Errorf` (both plain and `%w`), and `errors.Join` under `--max`: zero stdlib_constructor findings — the rule is inert for these shapes in this build. Consequence: family adoption is entirely convention; the 87 `errors.AsType` sites (AGENTS.md "Go 1.26+ idioms") and the single `//nolint:legacyerrors` site (executor_test.go:159) are the only error-discipline surfaces the gate actually pins.

## 4. Score + top opportunities

Inputs: N=15 rows, N/A=3, A=12. FULL=1, PARTIAL=1 (0.5), MISSED=9, MISUSED=0 → Σ=1.5.
Score = round(100 × 1.5/12) = **13 — low**.

Top opportunities, in value order: (1) F2 — give `PermanentError`/`PreflightError` a family and collapse `Permanent` onto `WrapOnce` (kills a parallel taxonomy at the repo's hottest error seam); (2) F3 — classifier for store-busy, deleting the string sniff; (3) F7 — `errorfamilytest` swap. F5/F6 are optional and behavior-visible.

## 5. Version currency + risk

Pinned v0.12.0 == latest tag (checkout ahead of tag by 6 commits, Unreleased empty) — no action. v0.12.0 (2026-10-14, CHANGELOG) fixes nil-receiver `(*Error).Unwrap` (defensive only for typed-nil forwards; tq has no such site). Risk notes: the module declares `go 1.27` (checkout go.mod:3) while tq modules are `go 1.27.3` — compatible; the go-retry CHANGELOG records an incident where an error-family minor bump dragged a consumer's go pin (v0.9.0 "Fixed"), so treat error-family bumps as pin-sensitive in every module graph loader (AGENTS.md "New module + unpushed tag").

## 6. Conclusion

Adopted, correctly, at exactly one seam: the readmodel fold's corruption/rejection minting that drives cqrs-lite's dead-letter gate. Everywhere else the repo answers the same questions (retryable? whose fault? what status?) with private taxonomies (`PermanentError`, `PermanentOutcome`, string sniffs, hardcoded exits). The low score is real but the fix list is short and mechanical; F2/F3 would move the needle most because they sit on the executor/worker hot paths.

## How to verify

```bash
cd /home/lars/projects/go-taskqueue
grep -rn 'errorfamily\.' --include='*.go' . | grep -v '^./vendor/'        # → exactly the 8 sites in §1
grep -rn 'go-error-family' --include=go.mod . | grep -v vendor            # → pinned versions + direct/indirect
git -C /home/lars/projects/go-error-family describe --tags                # → v0.12.0-6-g7ccca5c
sed -n '281p' vendor/github.com/larsartmann/go-cqrs-lite/projectionhost/v4/worker.go  # IsRetryable DLQ gate
sed -n '253,258p' internal/readmodel/events.go                            # WrapCorruption #1
sed -n '442,450p' internal/worker/worker.go                               # string-sniff predicate
sed -n '45,63p' internal/executor/executor.go                             # PermanentError + hand-rolled WrapOnce
sed -n '7,14p' internal/task/errors.go                                    # unregistered sentinels
sed -n '355p' AGENTS.md; sed -n '44p' scripts/check-erraudit.sh        # --enforce-go-error-family wiring
grep -rn 'errors.AsType' --include='*.go' . | grep -v '^./vendor/' | wc -l # → 87
grep -rn 'nolint:legacyerrors' --include='*.go' . | grep -v '^./vendor/'  # → executor_test.go:159 only
# F8 probe (scratch dir; remove after): module with bare errors.New/fmt.Errorf/errors.Join,
# then: erraudit lint ./... --max   → zero stdlib_constructor findings on build 5c841cf
```

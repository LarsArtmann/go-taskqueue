# ADR-0004: Lifecycle and streaming library stance — patterns now, cordis only at the plugin horizon

**Status:** Accepted (2026-09-08; same-day amendment: trigger T3's
vendor claim replaced by the locally verified suite results —
`docs/planning/archived/2026-09-08_cordis-test-suite-verification.md`)
**Context:** An advisory session (05:24 report, `docs/status/2026-09-08_05-24_cordis-samber-evaluation-journal-bus-recommendation.md`)
compared three libraries against this repo — **cordis** (the local
`/home/lars/forks/cordis` fiber/lifecycle framework), **samber/do** (DI
container), **samber/ro** (ReactiveX-style observables) — and reached a
verdict that lived only in chat until that report. This ADR persists the
decision and its trigger conditions so it outlives the conversation.

What the libraries were measured against (re-verified at HEAD on
2026-09-08, not carried from the report): `cmd/tq/main.go` is 1,269 lines
with 4 `signal.NotifyContext` sites (a 5th lives in `top.go`) and 11
`defer …Close()` sites (3 more in `audit.go`/`top.go`/`poolconfig.go`) —
each subcommand hand-rolls its own interrupt and teardown story.
`internal/webui/hub.go` is 57 lines; per the journal-subscription
inventory, every fact consumer polls independently (tailer 500ms, bridge
5s, `tq tail` 500ms, examples) and no push path exists. The repo
constraints are fixed: zero external services, pure-Go deps
(`CGO_ENABLED=0`), facts-first, `internal/` until the API stabilizes.

**Decisions:**

1. **Patterns now, dependencies later.** Steal cordis's ideas as in-repo
   conventions — fiber-style per-component contexts and LIFO disposal —
   and build the `run.Group`-style actor composition root for `cmd/tq`
   (one interrupt story, deterministic teardown, an `executionScope`
   actor that makes the drain invariant structural instead of
   remembered). Implemented with stdlib (`context`/`sync`/channels).
   Fact checked: `errgroup` is NOT currently a dependency
   (`golang.org/x/sync` v0.22.0 is indirect-only, no usage) — the actor
   helper is stdlib-only or adopts `x/sync` deliberately, never
   incidentally.
2. **The journal is the bus.** The streaming need is a typed
   `Subscribe(ctx, since Seq)` fact-stream over `queue.Store` plus
   persisted per-bridge watermarks — not a streaming library. Per the
   inventory, the pull half already exists (`Facts(after, limit)` +
   `HeadSeq`), and `journal.Journal` is production-dead — Subscribe
   targets the Store seam. Fact delivery is ordered, cursor-based,
   at-least-once with seq-derived idempotency keys; channels plus a
   dispatcher suffice. An `Observable[T]`-over-facts spike (ro) is an
   optional evaluation, never a design input.
3. **If small-library value is ever needed, do+ro beat cordis.** The
   05:24 matrix verdict: do+ro deliver ≈80% of cordis's value as two
   small single-purpose libraries; cordis's unique 20% is app-wide
   lifecycle reactivity — which only pays at the composition root, and
   decision 1 gives us that with stdlib.
4. **cordis as a dependency is acceptable ONLY at the plugin horizon,
   and only when ALL five triggers hold:**

   | #  | Trigger                                                                                                                                                                                                                  | Observed 2026-09-08                                                                                                                                                                                                                                                 |
   | -- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
   | T1 | The plugin era is real: a third-party executor/bridge API is an approved, started milestone (owner go/no-go answered yes)                                                                                                | Not started                                                                                                                                                                                                                                                         |
   | T2 | The cordis Go port ships a stable v1-class release                                                                                                                                                                       | Pre-stability: tag `v4.0.0-rc.9` + 48 commits, CHANGELOG latest `0.1.0`                                                                                                                                                                                             |
   | T3 | Maturity verified locally: its test suite run in the fork, coverage/race recorded                                                                                                                                        | DONE 2026-09-08: at `61ec9f9`, race-clean (0 warnings), 86.2% statements (atomic), 176 PASS / 0 FAIL / 0 SKIP — the README claim confirms; caveat: `loader` at 74.6% (watch/resolver edges) — `docs/planning/archived/2026-09-08_cordis-test-suite-verification.md` |
   | T4 | An integration-cost prototype behind a build tag proves the drain-deadline invariant (task context survives pool shutdown, bounded only by `--task-timeout`, ADR-0002 decision 6) maps onto fiber semantics structurally | Known to map BADLY onto `fiber.StdContext()` cancellation today                                                                                                                                                                                                     |
   | T5 | Exit plan: the plugin API itself stays framework-independent (typed config, verify command, lifecycle hooks); cordis may power OUR composition root, never the third-party contract                                      | No plugin API yet                                                                                                                                                                                                                                                   |

   Until all five hold: framework-free. Not "framework-free forever" —
   that is dogma, and T1–T5 make "when yes" concrete — but the default
   answer to "adopt a lifecycle/streaming framework?" is no.
5. **Re-evaluate when the gates move.** When cordis or ro reaches v1, or
   the plugin-era go/no-go lands, re-run the comparison and AMEND this
   ADR — never silently obsolete it.

**Consequences:** Zero new dependencies result from this decision. The
05:24 task list resolves: fact-stream and actor work are unblocked NOW
(no framework needed); the cordis criteria/prototype/track items are
planning inputs for the plugin era, not waste. The comparison matrix's
three unlabeled claims (ro "~200 operators" was an estimate; cordis
coverage was a vendor claim, since verified — see T3; the ro GitHub
fetch was degraded) do not
change the verdict — the ranking was structural (ordered at-least-once
cursor streams fit channels, not operator algebra), not
maturity-score-driven — but T2 stays open precisely because cordis
stability is unproven, and T3 existed as an open gate until the
2026-09-08 local run (race-clean, 86.2%) closed it. The papdashboard missed-incident gap is
cross-process and framework-independent: no library fixes it; only
persisted watermarks do (separate design doc, TODO-listed).

**Alternatives rejected:** Adopt cordis now (pays integration cost
before a composition root exists, and the drain invariant maps badly
today — T4); adopt do now (the wiring counts are the problem, not the
wiring mechanism; a container adds indirection without removing lines);
adopt ro now (drop-on-overflow is only ever correct for projection
consumers like the hub, which channels already serve; exact-delivery
consumers need bridge-style cursors); framework-free forever (rejected
as unfalsifiable — the five triggers replace "never" with conditions).

**Cross-check (evidence ledger):** Verified this session against HEAD:
the line/signal/defer counts above (`wc -l`, `rg -c`), no errgroup or
oklog/run usage, cordis fork state (`git describe
v4.0.0-rc.9-48-g61ec9f9`, CHANGELOG `0.1.0`, README:37 claim). Carried
from the 05:24 session with its own verification labels: samber/ro
v0.4.1 (2026-08-23, Apache-2.0, 47 importers — verified via pkg.go.dev
in that session); samber/do v2 API via the loaded skill's verification
block. The Subscribe seam correction comes from
`docs/planning/2026-09-08_journal-subscription-surface-inventory.md`
(verified against code); the watermark gap analysis from
`docs/planning/archived/2026-09-08_persisted-bridge-watermark-design.md`.
Amendment 2026-09-08: the cordis suite was run in the fork that day
(`GOCACHE=/tmp/gocache`, `go1.27.1`, `-race -count=1 -coverprofile`) —
race-clean, 86.2% total statements at the same `61ec9f9` recorded
above; full per-package table and reproducibility commands in
`docs/planning/archived/2026-09-08_cordis-test-suite-verification.md`.

# Status Report — 2026-09-11 14:22 CEST — Task Detail Page Redesign

Session scope: redesign the `/task/{id}` detail page (`tq serve` webui) — user verdict on the
old page: "sucks ass a bit — especially 'payload'". The payload (the task's CONTENT: work
item, prompt contract, verify gate) rendered as one break-all escaped-JSON line inside a
definition-list row; the fact timeline stuttered the same requeue reason twice per line, and
fifteen identical requeues read as thirty noise lines.

Everything below is judged against THIS session only (no other research). Concurrent-agent
activity is reported where it touched this session's work.

---

## a) FULLY DONE

| Work | Evidence |
| --- | --- |
| **Payload view model** (`internal/webui/payload.go`, new): `payloadView` with closed `payloadKind` enum (agent/review/status/sh/raw); parses with the executor's OWN payload structs (`AgentPayload`, `ReviewPayload`, `StatusPayload`) so field renames can never drift; leads with the work item (or shell command), executor contract as a spec grid, prompt + raw JSON folded; unknown/unparseable payloads degrade to raw — never fabricate structure; pretty-prints JSON; zero information loss (raw pane always available). | `go test ./internal/webui/` green; 10 new test funcs in `payload_test.go` |
| **Payload section template** (`fragments.templ`): `payloadSection` replaces `payloadCode`; renders between the two live fragments, deliberately STATIC (a payload is immutable after enqueue) so an open prompt/raw pane survives every SSE swap; `payload · <kind>` ledger eyebrow. | Visual-verified on scratch DB: agent page + sh page both correct |
| **Payload removed from the record definition list** (`components.go` `detailItems`) with an anti-regression comment; `TestDetailItems` now pins it must NEVER re-enter the dl. | updated test green |
| **`CommandFromPayload` exported from executor** (`command.go`): the webui displays commands via the executor's own unwrap (raw line / JSON string / `{"cmd":…}`) — no split-brain copy of the contract. | executor module `build + vet + test` green |
| **Fact-line stutter fix** (`components.go`): new `factLineText` merges `Error` + `Detail.reason` into ONE honest line (requeues carried the same text in both fields — the rendered line used to repeat it); `attempt N` annotation added from `Fact.Attempt`. | `TestDetailFactsSurfacesCancelReason` extended: requeue reason appears exactly once |
| **Retry trail strip** (`payload.go` `retryTrail` + `retryStrip` template + amber CSS): aggregates requeued/failed/released facts into distinct reasons with ×counts, loudest first; stays nil until ≥2 occurrences (a single refusal is already readable in the trail). Answers "why does this keep coming back?" in one glance — the 15-requeue case from the user's paste. | Visual-verified: `retry trail ×2` strip on the scratch agent page; unit-tested incl. ordering + reasonless fallback |
| **theme.css**: payload spec grid (hairline rows, mono labels), `.tq-fold` native `<details>` (reduced-motion safe), `.tq-payload-pre` (pre-wrap, scroll-capped), amber retry strip, cyan-leaded work-item lede. Rebuilt committed `static/app.css` via the canonical `nix run .#webui-css`. | css rebuilt; all dark-mode variants written |
| **Visual verification (scratch DB, the known `TQ_DB` gotcha honored)**: built `/tmp/tq-ui`, seeded `exit 1` sh task + dirty-repo agent task, drove 3 real worker passes (1 failed→dead sh; 2 preflight requeues on agent), served on 127.0.0.1:8111, fetched both detail pages: payload section, folded prompt (newlines preserved), pretty raw JSON, spec fields, retry strip ×2, attempt annotations all render. | fetch transcripts in session; scratch artifacts in `/tmp/tq-ui-demo`, `/tmp/tq-ui-repo` |
| **Gates**: root `go build ./...` + `go vet` green; `go test ./internal/webui/ -count=1` green; executor module build/vet/test green. templ regenerated with pinned v0.3.1020. | session transcripts |

## b) PARTIALLY DONE

1. **Full-suite verification** — webui + executor packages verified; the FULL root-module
   `go test ./... -race` and `./scripts/ci-local.sh` were NOT run this session.
2. **Doc sync for the new UI surface** — CHANGELOG entry not yet written; FEATURES.md/AGENTS.md
   don't yet mention the payload section/retry trail (AGENTS.md's templ-components adoption
   table unaffected — no new library components adopted; everything is bespoke CSS, matching
   the existing custom rows).
3. **Concurrent-session CSS breakage** — commit 04aace4 (36 files, round-11 pool-revival
   session) shipped an UNMINIFIED `static/app.css` (+6299 lines, spaced
   `prefers-reduced-motion: reduce`) that broke `TestPageA11yAffordances` (pins minified
   `:reduce`). Root-caused and fixed THIS session by re-running the canonical
   `nix run .#webui-css` (121 KB minified, `:reduce` present, tests green) — but the other
   session's build pipeline may regenerate the unminified form again; see question 3.

## c) NOT STARTED

1. CHANGELOG entry for the detail-page redesign.
2. FEATURES.md line for the payload section + retry trail.
3. Full `./scripts/ci-local.sh` gate (vet/build/race/smokes/nix) on the final tree.
4. Full multi-module test loop (`for m in internal/*/…`) — only executor + webui ran.
5. Board view / dashboard table pages were NOT touched (out of scope; not regressed — full
   webui suite passes).
6. PR/announcement material: no before/after screenshots archived (scratch DB is in /tmp,
   not `docs/status/assets/`).

## d) TOTALLY FUCKED UP (own mistakes this session, all recovered)

1. **`curl` in the bash tool** — banned command; wasted a round trip. Should have used the
   `fetch` tool for the page inspection from the start.
2. **Server bind race** — started the first serve with `(timeout 90 … &)` inside the same
   command as the (banned) curl; then the background job lost the bind race against the
   zombie of that first server ("address already in use") and the first fetch hit a dead
   port. Two round trips burned on process hygiene that one clean `run_in_background` +
   `sleep 2` would have avoided.
3. **Wrong test expectation** — `TestPayloadViewAgentAutoDetectAndNoItem` asserted the raw
   pane should hide; in fact the JSON object carries fields (repo) the lede doesn't, so the
   pane legitimately shows. Tests caught it; fixed the expectation, not the code.
4. **Dead field shipped-then-caught** — `payloadView.RawJSON` was written, never rendered;
   self-caught during review and removed before commit. Should have checked template usage
   before writing the struct.
5. **Module-path test invocation** — ran `go test ./internal/executor/` from the root module
   (fails by design, ADR-0011); re-ran inside the module. Known repo rule, still tripped once.

## e) WHAT WE SHOULD IMPROVE

1. **The prompt fold is still one `<pre>` blob** — a structured prompt (Contract section /
   numbered items) could render as definition rows instead of monospace soup.
2. **Timeline grouping** — the retry strip summarizes, but the full trail still prints all
   15 pairs; grouping consecutive claim→requeue pairs per attempt (collapsible) would halve
   the noise without rewriting history.
3. **`hasRaw` heuristic is string-equality** — a payload whose pretty JSON happens to equal
   its lede hides the pane; harmless but a structural rule (kind-based) would be cleaner.
4. **Retry strip caps nothing** — a task with 50 distinct reasons renders 50 rows; a "+N
   more" overflow would bound it.
5. **app.css freshness gate is fragile** — the test pins a minified-form substring, so any
   parallel unminified build silently breaks the suite (happened today); a build-script
   contract test or a `check-webui-css.sh` gate in ci-local would catch the class, not the
   instance.
6. **Visual regression coverage** — detail-page HTML is only string-probed; a small golden
   fragment test for `payloadSection` would pin the section against template refactors.
7. **`tq serve` has no `--preflight-backoff`-style knob** — verifying retry ladders against a
   scratch DB required waiting out the 2-minute rung; a flag (or TQ_ env) would make such
   verifications fast and hermetic.
8. **`Fact.Attempt` is not rendered on the dashboard's fact FEED** (only the per-task
   timeline got it) — parity is cheap and useful when scanning bursts.

## f) UP TO 50 THINGS TO GET DONE NEXT

Near-term, high-confidence (TODO_LIST material):

1. Write the CHANGELOG entry for the detail-page redesign (payload section, retry trail,
   fact-line merge, attempt annotations, CommandFromPayload export).
2. Add the FEATURES.md line for the type-aware payload section + retry trail.
3. Run `./scripts/ci-local.sh` end-to-end on the settled tree (concurrent round-11 session
   is still landing files).
4. Full multi-module loop: every `internal/*` sub-module build+vet+test.
5. Full root `go test ./... -race`.
6. Update AGENTS.md webui notes: payload section is STATIC (outside #frag-detail) — the SSE
   contract for /task/{id}/events must never re-absorb it.
7. Golden/snapshot test for `payloadSection` (pin against template refactor rot).
8. Bound the retry strip at ~5 reasons with a "+N more" tail.
9. Render the agent prompt's numbered contract items as structured rows instead of one `<pre>`.
10. Group consecutive claim→requeue pairs in the fact timeline (collapsible per attempt).
11. Render `Fact.Attempt` in the dashboard fact feed for parity with the detail timeline.
12. Add a `tq serve --demo-preflight` (or env) knob to shorten the preflight backoff for
    scratch-DB verification runs.
13. Replace `hasRaw` string-equality with a kind-based rule (`payloadSh` raw text only).
14. CSS-build contract gate: assert `static/app.css` is the minified pipeline output
    (byte-length ceiling or `:reduce` form) in ci-local, not just a unit test.
15. Coordinate with the round-11 session on the app.css artifact (see question 3) before
    more parallel rebuilds fight each other.
16. Archive before/after screenshots of the detail page under `docs/status/assets/` (with a
    README so the ghost-archive gate passes).
17. Add the payload section to `tq top`'s task hover/preview? (evaluate — YAGNI check first).
18. Review executor `CommandFromPayload` doc example (`example_test.go` pattern) so the new
    export shows in pkg.go.dev.
19. Consider surfacing `RequeueEvidence.retry_in_ms` on the timeline line ("retry in 2m") —
    the data is already in the fact detail.
20. Dark-mode visual pass on the new sections in a real browser (CSS is written; not
    eyeballed in dark).

Mid-term (ROADMAP fuel — apply HARVEST routing rigor):

21. Webui: keyboard navigation on the detail page (j/k between tasks from the table).
22. Webui: copy buttons (copy task id, copy payload JSON, copy verify command) — CSP-safe
    pattern exists in templ-components (`display.CopyButton`).
23. Webui: filter the fact timeline by fact type on the detail page.
24. Retry-cause analytics: a dashboard card aggregating preflight refusals across ALL tasks
    ("5 tasks blocked on dirty repos") — turns per-task forensics into pool-level signal.
25. `tq show` CLI parity: render the same type-aware payload view in the terminal.
26. Attempt-heatmap on the detail page (timeline density per attempt).
27. Payload diff view when a task is re-enqueued with the same dedup key but edited text.
28. Webui i18n/labels audit: section copy ("retry trail", "verify gate") into
    docs/DOMAIN_LANGUAGE.md so UI vocabulary stays canonical.
29. Performance: payload view parse per render is fine, but cache the pretty-JSON for huge
    prompts (>64 KB) if profiles ever show it.
30. Consider a "what would happen on rescue" hint block on dead tasks (attempts budget,
    verify command preview).
31. Board cards could show the retry count badge (×N) for pending tasks with requeue
    history — same retryTrail helper.
32. Make the record-card definition list two-column responsive on narrow screens (it is a
    `<dl>`; verify wrap on mobile).
33. Accessibility pass on `.tq-fold` with screen-reader announce of expand state.
34. Add `prefers-contrast` review for the amber retry strip.
35. Fuzz `payloadViewFor` against arbitrary payload bytes (it must NEVER panic; property:
    kind ∈ closed set, Raw present).
36. Property test: retryTrail is deterministic under fact-order shuffles (stable sort key).
37. Rate-limit UI: round-11 added executor rate limiting — surface any rate-limit facts in
    the webui once their fact shape lands (coordinate).
38. Document the static-payload-section rule in ADR-0003 (SSE fragment contract) as an
    amendment rather than a comment.
39. `scripts/smoke/webui.sh`: add an agent-payload fixture page assertion (payload eyebrow,
    prompt fold) so the section can't silently regress in CI.
40. Consider `View Transition`-free skeleton for #frag-detail swaps (already cheap; only if
    jank is observed).
41. Evaluate `display.CollapsibleSection` from templ-components vs bespoke `.tq-fold`
    (adoption-table honesty; likely stays bespoke for the ledger look — document why).
42. Trim `detailFactsLimit` (500) interaction with the retry strip: strip derives from ALL
    facts, timeline shows the last 500 — state the cap in the UI when truncated.
43. Add the task-type glyph to the payload eyebrow (icon from templ-components icons set).
44. Lint: the exhaustive-warning on `factTone`'s switch (journal.Orphaned missing) — add the
    case or a default comment; advisory-baseline, one-liner.
45. Delete or use `factLines` (gopls reports it unused since the linked-feed variant landed).
46. webui package doc: list the fragment containers and their swap semantics in one table.
47. Consider moving `retryReasonLen`/`errorPreviewLen` truncation policy into one place.
48. `tq serve --verbose` request log includes the payload length — check no payload bodies
    leak into logs (security pass).
49. Update `docs/DOMAIN_LANGUAGE.md` with "work item", "verify gate", "retry trail" terms.
50. Post-redesign dogfood check: enqueue one real pool task and read its detail page on
    tq.home.lan to confirm the production journal renders the new section (read-only).

## g) QUESTIONS I CANNOT FIGURE OUT MYSELF

1. **Prompt fold default**: for agent payloads WITH a harvested item I collapse the prompt
   (item leads). For agent tasks WITHOUT an item I made the prompt the lede (always
   visible). Is that the split you want, or should the prompt always stay folded with the
   item-less case showing a "no work item — see prompt" hint instead?
2. **Retry strip scope**: should dead-lettered reasons count into the retry trail
   (currently excluded — the dead task's "last error" alert already carries them), or is
   the strip meant to be strictly about "came back and will come again"?
3. **app.css pipeline conflict**: commit 04aace4 (the concurrent round-11 session) shipped a
   6300-line unminified `static/app.css` that broke the pinned minified-CSS test; I
   regenerated the canonical minified artifact. Was that unminified build intentional (new
   pipeline I should adopt + test change), or an accidental dev-build commit — i.e. should
   the OTHER session be told to run `nix run .#webui-css`, or should I expect another fight
   over this file?

---

*Point-in-time snapshot. Verify claims against the tree before acting on them — the
round-11 session was actively landing files while this report was written (uncommitted
diffs in cmd/tq, internal/bridge, examples/ at 14:22).*

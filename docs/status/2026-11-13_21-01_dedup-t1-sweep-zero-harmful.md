# 2026-11-13 21-01 — art-dupl `-t 1` dedup sweep: zero harmful clones

Interactive sweep (no task footer; owner invoked
`art-dupl --sort total-tokens -t 1 --suggest-generics --timing --html`
directly and mandated "deduplicate to zero"). Code rode footerless daemon
sweeps per the interactive-sweep precedent.

## a) Invocation + governed surface

- Discovery scan: user's exact flags at `-t 1` → 176 actionable groups /
  458 clones (2522 detected, 2346 suppressed by the actionability filter).
- Governed scan: canonical `-c scripts/art-dupl.json` (threshold 3,
  total-tokens, typeAware) → 14 groups / 43 clones at sweep start. The
  skill's contract governs the outcome: zero HARMFUL clones, every
  survivor a verdicted accept — not zero report lines.
- Every one of the 176 groups was triaged (full inventory + verdict table
  rendered from `--json` into `.crush/shell-output/t1-tail.txt`). The
  `-t 1`-only tail (162 groups below the governed threshold) is 2-6-token
  idiom classes (`stats.X++` fold tails, `if err != nil { continue }`,
  `case <-ctx.Done()`, CLI `mustOpenDB` prologs, templ formatInt call
  sites, conform assertion shapes) — accepted by class, not per-site.

## b) What shipped — 9 extraction seams

1. `internal/slogcapture.Handler` (new package + guard test): the
   byte-identical `captureHandler` slog test double in
   dlqfix/harvest/review/status sweep tests (4 copies, canonical group
   79626f35) collapsed onto one name; `Records()` returns a copy
   (`slices.Clone`).
2. Harvest `runOverRepos` fixture (harvest_test.go): the 9-line
   drift/prune prolog ×4 collapsed (the 5th twin at
   prune_test.go TestPruneStaleNeverTouchesDlqfixAutopsies keeps its
   prolog deliberately — it needs the raw queue to mint the autopsy).
3. `openDoctorStore` (cmd/tq/doctor_test.go): the
   doctorTestStore+Open+defer-Close+ctx prolog ×9 collapsed; close moved
   to `t.Cleanup` (runs after function defers — strictly safer).
4. Executor `bufferedProcessGroup(cmd)` (verifygate.go): the
   buffer+process-group+10s-kill-die probe convention (agent.go run +
   verifygate probe) — one name for a 5-statement policy.
5. Conform `requeueEvidenceOf` (companion/conform/tests.go): the
   trail-fetch+decode+loop seam ×3; returns `(evidence, rawDetail)` so
   the leak-diagnostic pin keeps its detail; plus `trailOf` (the shared
   FactsForTask fetch) — minted AFTER my own requeueEvidenceOf
   immediately twinned `assertAnsweredFact`'s fetch prolog (the
   extraction created a37293bf; trailOf dissolved it — see §e2).
6. Templ `fullOutputLine(logPath)` (fragments.templ): the four
   byte-identical `full agent output` blocks (826/844/882/899) — TODO
   row 535 executed (line numbers had drifted from the row's
   701/719/746/763); the ADR-0003 path-not-link rationale moved into the
   component comment; `templ generate` run, app.css regenerated minified
   (the committed convention; the first regen raced the daemon sweep and
   was re-run on the tracked tree).
7. Webui `openSSEStream(w, r)`: the flusher-gate + X-Accel-Buffering +
   NewStream prolog ×2 (handleAgentLive + runEventStream).
8. Readmodel `statusCounts` now delegates both loops to the existing
   `matchesFilter` instead of hand-rolling the filter rule twice (the
   project branch's private copy was a live drift hazard — `projectMatrix`
   already used the shared predicate).
9. slogcapture guard test pins capture/Records/mutation-isolation.

## c) Accept verdicts (canonical survivors, final scan)

Final canonical scan: **8 groups / 22 clones**, every survivor verdicted:

| group | class | ruling |
| --- | --- | --- |
| fff31d74 | worker test 3-line setup prolog ×8 | idiomatic divergent-body setup; abstraction ≈ duplication |
| 418ee1e5 | prioritize onClaimed/onEnqueued fold twin | sweeper-shell mirror class; extracting delegates mints a smaller clone |
| 7c199632 + 979d46ed | seqid.go ↔ readmodel/host.go seq-codec mirror | deliberate parity mirror pinned by TestSeqIDCodecParity (docs/review/libs/go-cqrs-lite.md) |
| 3e0fac97 | cmd/tq FlagSet+Bool registration shape ×2 | CLI flag idiom; literals differ (alpha-normalized group) |
| f1507bd0 + d2c3383a | fragments.templ card-badge row twins | templ view twins — directives cannot live in templ markup (art-dupl#9 open) |
| e3ae1d68 | readmodel loop skeletons ×2 | the drift hazard (the filter rule) is now ONE copy; further folding takes more params than lines |

Died on read (NOT extracted, deliberately): the "TQ_DB twin" (park.go is
`parkBy()` — USER-env fallback, a different policy), the excerpt/no-output
vs payloadOrEmpty pair ("(no output)" vs "{}" — different domain rules
behind one shape), the incident mint-guard pair (prolog shared but the
following semantics differ — extraction would change clearPending's
behavior), doctor's buffered-cmd third site (gofmt probe: read-only, no
process group — different contract).

Trigger audit (AGENTS.md extraction triggers): the reads.go
`scanInto` trigger is DISSOLVED in-source — reads.go:150-152 documents
why the query/defer prolog stays inline (a helper would defer Close
before the caller's scan loop) and the scan loops already delegate to
scanTask/scanFacts. The doctor pending-listing and review/status
payload-validator triggers: no third site appeared in either scan.

## d) Verification (rc captured)

- Touched-module gates: slogcapture + dlqfix + harvest + review + status
  `go test -count=1` → ok; executor module build+vet+test → ok 36.4s;
  companion + conform via sqlitev4 driver → ok 5.0s + postgresv4 build ok;
  readmodel → ok; webui (post templ generate) → ok 15.5s; cmd/tq full
  suite via tracked go.work → ok 7.4s; `scripts/test-cmd-tq.sh` rc=0
  (re-run rc-to-file after one piped-masked rc — §e4).
- Root battery: build + vet + `go test ./... -race -count=1` → rc=0.
- gofmt -l over all 15 touched files → clean (one fix: the LSP symbol
  insert needed a comment separator + blank-line squeeze in verifygate.go).
- `check-mirror-clones.sh` rc=0 (2 NEW HIDDEN [return] mirrors flagged in
  sqlitev4/postgresv4 adapter.go+archive.go — PRE-EXISTING, none of this
  sweep's files; minted as a TODO row per the accepts-≈-0 ruling).
- `check-dead-exports.sh`: slogcapture not flagged; advisory summary
  unchanged.
- `check-todo-list.sh` → gate ok (21 advisory warns, 0 structural).
- AGENTS.md size guard: 27,190 / 27,200 B (TestAgentsDocSizeGuard green
  inside the cmd/tq run).
- Final canonical art-dupl: 14→8 groups, 43→22 clones.
- Final `-t 1` scan: 176→171 groups / 458→440 clones (after-picture —
  the extraction seams removed 18 clones; the remaining surface is the
  below-threshold idiom tail (accepted by class) + the 8 canonical
  accept classes; one -t 1 hash set shifts as line numbers move).

## e) Problems + hazards hit

1. templ regen + `nix run .#webui-css` raced the daemon sweep: the css
   build saw a partially-tracked tree and emitted a stale artifact;
   re-run on the tracked tree produced the correct minified form
   (164,601 B vs the previously-committed expanded 206,988 B — the
   minified form is the committed convention; key-class spot checks
   passed). Lesson: run the css step AFTER the daemon has folded the
   template edits, or `git add` first (flakes see tracked files only).
2. My own extraction minted a clone (a37293bf): requeueEvidenceOf's fetch
   prolog twinned assertAnsweredFact's. Fixed by the trailOf seam — the
   iterate-to-zero loop applies to extractions too.
3. One piped rc (`./scripts/test-cmd-tq.sh | tail`) masked the script's
   true exit — the exact PIPESTATUS-trap class the tq-bash-guard denies;
   re-run rc-to-file.
4. jq on this host lacks `--slurpfile`; set-diff done via sort/comm.

## f) TODO_LIST deltas

- Row 535 (four full-output templ blocks) → [x] DONE with pointer.
- Config row (`scripts/art-dupl.json` canonical invocation) → [x] DONE
  (config exists at HEAD; this sweep ran against it).
- Minted: home-or-baseline the 2 NEW HIDDEN cross-backend mirrors
  ([return] class, adapter.go + archive.go) per the accepts-≈-0 ruling.

## g) Verdict

Zero harmful duplication at the canonical threshold; the `-t 1`
discovery surface is fully triaged with every survivor carrying a class
verdict (this report §c). AGENTS.md ledger updated in place (templ-twin
naming corrected, sweep verdict pointer, upstream-#9 wording tightened;
27,190/27,200 B). CHANGELOG Internal row recorded. No operator-visible
behavior change.

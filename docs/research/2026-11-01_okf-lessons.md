# OKF (Open Knowledge Format) lessons for go-taskqueue

**Date:** 2026-11-01 (source-verification window).
**Source:** Google Cloud's Open Knowledge Format. Announcements
[`how-the-open-knowledge-format-can-improve-data-sharing`](https://cloud.google.com/blog/products/data-analytics/how-the-open-knowledge-format-can-improve-data-sharing)
(2026-06-12, OKF v0.1) and
[`okf-v0-2-adds-trust-signals`](https://cloud.google.com/blog/products/data-analytics/okf-v0-2-adds-trust-signals)
(2026-07-25, OKF v0.2); normative text from
[`GoogleCloudPlatform/open-knowledge-format@main/SPEC.md`](https://github.com/GoogleCloudPlatform/open-knowledge-format)
(v0.2) plus the
[`knowledge-catalog/okf` reference implementation](https://github.com/GoogleCloudPlatform/knowledge-catalog/tree/main/okf).
Spec read at source (the §-numbered quotes in the citations table); the two
articles are publisher summaries of it and are never the authority here.
**Licensing/provenance note:** OKF is a published open specification, not a
dependency. Nothing in this note proposes importing code.

**Companion notes:** `docs/research/2026-10-09_turnstone-lessons.md` is the
closest precedent (a layer-adjacent source whose written invariants became a
tq checklist) and supplies the shape of this note.

## Why OKF is worth reading

OKF is a **format for knowledge that agents write and other agents read**. It
starts from the observation that a human-authored wiki page carries an implicit
guarantee ("a person wrote this and is accountable"), and that guarantee
evaporates the moment agents generate ten thousand concepts overnight — so
consumers must instead judge each concept on **explicit, cheaply-filterable
signals**. v0.2 names the five questions such a consumer asks: what was this
created from (provenance), how much should I trust it (trust), is it still true
(freshness), is it the current version (lifecycle), and was this number
produced the way we said it must be (attestation).

That is not adjacent to tq. It is tq's exact disease, one abstraction down. A tq
run is written by an agent and read by other agents: closeout reports are
consumed by reviewers, `docs/status/**` by humans, `TODO_LIST.md` by
`tq harvest`, journal facts by the fold, `$TQ_RESULT_FILE` verdicts by
sweepers. tq already asks all five of OKF's questions — but answers them with
**prose conventions in AGENTS.md** where OKF answers them with **typed,
optional, additive frontmatter**. OKF's contribution to tq is therefore not a
feature but a *form*: it shows what the typed version of rules tq already
believes in looks like, and proves that adding those fields can be done without
breaking a single existing consumer.

The second, less obvious contribution is OKF's **compatibility discipline**.
tq pays a continuous tax for lockstep version movement (facade floor pins,
`cmd/tq` pinning the release floor, `--dep-sweep`, `check-gomod-vendor-sync`).
OKF ships a minor version that adds six field families and *nothing* is forced:
`type` stays the only required key, every addition is opt-in, a v0.1 bundle is
still conformant, and a v0.2 consumer falls back gracefully on the two renamed
v0.1 shapes. That is a worked example of additive contract growth, and it is
the pattern tq's journal detail structs should follow when they grow.

## Source-verified citations

| #  | Path (spec `@main`, articles as published; verified 2026-11-01)                       | What it actually says                                                                                                                                                                                                                                                                                    |
| -- | ------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1  | `SPEC.md` §4.1                                                                        | `type` is "the only always-required key"; `title`/`description`/`resource`/`tags` are recommended. "Type values are **not** registered centrally… consumers MUST tolerate unknown types gracefully, typically by treating them as generic concepts."                                                       |
| 2  | `SPEC.md` §4.1 "Extensions"                                                           | "Producers MAY include any additional keys. Consumers SHOULD preserve unknown keys when round-tripping and MUST NOT reject documents with unrecognized fields."                                                                                                                                          |
| 3  | `SPEC.md` §5.5                                                                        | `stale_after` is "an absolute instant. A concept is stale when `now >= stale_after`." The spec gives the rationale directly: "An absolute instant, not a relative TTL, keeps the staleness decision a plain comparison with no reference to when the concept was read."  |
| 4  | `SPEC.md` §5.4                                                                        | `status: draft \| stable \| deprecated`; "Absent `status` ⇒ `stable`." The article adds the reason deprecated concepts are *kept*: the legacy definition "is preserved for historical query reproducibility but not surfaced to new work".                                                                 |
| 5  | `SPEC.md` §5.1 + v0.2 article                                                         | `sources` records the materials a concept derives from, each optionally carrying objective credibility signals (`author`, `usage_count` + a `usage_window`, `last_modified`). Credibility **scores** are deliberately omitted — the spec's words are that a score "is subjective, unportable across consumers, and goes stale"; the consumer infers. Per-claim attribution is an ordinary footnote keyed to the source `id`. |
| 6  | `SPEC.md` §5.2 + v0.2 article                                                         | `generated: {by, at}` and `verified: [{by, at}]` are deliberately distinct because "who wrote something need not be who confirmed it". Trust tiers are **derived** from `verified` (no key ⇒ unverified; machine-only ⇒ machine-confirmed; a `human:<id>` ⇒ human-reviewed) and are "advisory signals, not access control". A bare `verified` mapping MUST be read as a one-element list. |
| 7  | `SPEC.md` §11                                                                         | Conformance needs three things only (parseable frontmatter, non-empty `type`, well-formed `index.md`/`log.md`). Consumers "MUST NOT reject a bundle because of: missing optional frontmatter fields, unknown `type` values, unknown additional frontmatter keys, broken cross-links, missing `index.md` files." Broken links "may simply represent not-yet-written knowledge". |
| 8  | `SPEC.md` §3.1, §8, §9                                                                | `index.md` and `log.md` are reserved filenames "at any level of the hierarchy and MUST NOT be used for concept documents"; everything else is a concept. `index.md` carries no frontmatter except an optional root `okf_version`. `log.md` dates MUST be ISO `YYYY-MM-DD`.                                   |
| 9  | `SPEC.md` §6.1–6.3                                                                    | The file path **is** the concept's identity; links are ordinary markdown, with the bundle-relative `/`-form recommended ("stable when documents are moved within their subdirectory"). A link's *kind* is carried by surrounding prose, not the link, so graph consumers "treat all links as directed edges of an untyped relationship". |
| 10 | `SPEC.md` §10.2, §10.3                                                                | An `Attested Computation` declares `runtime` (REQUIRED for the type), typed `parameters`, a `computation` path, an `executor` whose `receipt` "declares the fields a run must return", and an `attester` ("deterministic… no LLM") that takes a receipt and returns a verdict. "The agent MAY only supply *values* for the declared `parameters`; it MUST NOT author or edit the computation." |
| 11 | `SPEC.md` §10.5, §10.6 + v0.2 article                                                 | Attestation is **not** verification: "`verified` confirms the *definition* still matches policy. It is doc-level, slow, and recorded in the bundle. Attestation confirms a single *run* produced the value the sanctioned way. It is per-call, runtime, and not stored in the bundle." A false verdict means the consumer "refuses to display the value". |
| 12 | v0.2 article, "Design philosophy"                                                     | "Vocabulary, not rules": `type` remains the only always-required field, every new field is opt-in, custom keys are still "preserved, not rejected", a bundle adopting none of the additions is exactly as valid as v0.1, and "absence now carries meaning (unverified vs. verified) but never causes rejection."                                                       |
| 13 | v0.2 article, "From describing to deciding"                                           | Most interactions never reach the body. "Frontmatter elevates exactly those signals so decisions are cheap and token-free; full body reads happen only after a concept is chosen." Trust becomes something you filter on before committing to read.                                                       |
| 14 | v0.1 article, "Format, not service" + "Three design principles"                        | A format is producible "without an SDK", consumable "without an integration", survives moving between systems, lives in version control beside the code it describes, and is "readable by humans and parseable by agents — same file, no translation layer". Producer/consumer independence: "the format is the contract; tooling at each end is swappable." |
| 15 | v0.2 article, "Compatibility"                                                         | The bump is "minor, additive, backward-compatible" with two deliberate renames (`timestamp` → superseded by `generated.at`; body `# Citations` → superseded by `sources`), and "in both cases a v0.2 consumer can fall back to the v0.1 form. A v0.1 bundle drops in unchanged."                            |

## Lesson table — what OKF does, where tq stands

| #  | Lesson                                                                                     | OKF (verified)                                                                                                        | tq before                                                                                                                                                                        | tq now / action                                                                                                                                                                                                                                       |
| -- | ------------------------------------------------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| 1  | **Unknown types degrade, never fail the consumer**                                          | §4.1 + §11: unregistered `type` values are legitimate; a consumer "MUST tolerate unknown types gracefully" as generic concepts | the fold already skips them: `eventFor`'s `default: return nil, false, nil` (`internal/readmodel/events.go:223`) after explicitly naming the skip families; the web UI degrades an unknown payload type to `payloadRaw` (`internal/webui/payload.go:25`) | **SHIPPED, pinned 2026-11-01.** `eventFor` had no direct test file at all; `internal/readmodel/forwardcompat_test.go` now pins that an unregistered `FactType` yields `(nil, false, nil)` and that a truncated detail still returns a corruption-class error, so tolerance never widens into swallowing corruption.                                                                                                                            |
| 2  | **Unknown keys are ignored; producers may add fields freely**                               | §4.1 Extensions + §11: "MUST NOT reject documents with unrecognized fields"; unknown keys preserved on round-trip        | every tq machine boundary uses plain `json.Unmarshal` with no `DisallowUnknownFields` anywhere in the repo (grep: zero hits), and the skip-don't-fail policy is stated for detail decoders (`internal/queue/queue.go:443`, `internal/readmodel/events.go:273`) | **SHIPPED, pinned 2026-11-01.** `decodePayload` was untested; `internal/executor/payload_test.go` pins that flat and nested unknown keys decode to the known fields, keeps empty/undecodable/wrong-shape payloads as permanent contract misses, and carries a counterfactual (`TestDecodePayloadToleranceIsNotVacuous`) proving the pin bites: the same bytes under `jsonv2.RejectUnknownMembers(true)` do fail.                                                                     |
| 3  | **Absent optional field carries meaning without invalidating the document**                 | §5.4: absent `status` ⇒ `stable`; §5.2: absent `verified` ⇒ unverified; §12: absence "never causes rejection"           | legacy detail shapes are read as their pre-field meaning rather than rejected: `requeuedEvent` decodes a missing `class` as `RequeueClassUnknown` (`internal/readmodel/events.go:273`); absent `OldPriority` decodes 0 with a key-parity pin (`internal/readmodel/detailkeys_test.go`) | **ALIGNED.** The pattern is already tq's; OKF supplies the vocabulary ("absence is a tier") worth using in the code comments.                                                                                                                            |
| 4  | **Provenance is signals, never a score**                                                    | §5.1: objective `author`/`usage_count`/`last_modified`; scores are "subjective, don't port, go stale immediately"        | `--prioritize` stores an AI score per task in a `priority_scores` cache and ADR-0015 makes stored priority + aging the one claim ladder; the score is scheduling-only, never authority | **ALIGNED — and OKF gives the exact argument for why it must stay that way.** A trust tier must never be computed from an LLM score; only from `verified`-shaped facts. State this (see TODO).                                                            |
| 5  | **`generated` ≠ `verified`; trust is a derived tier, not a stored judgement**               | §5.2 + article: two distinct fields; tiers derived (unverified / machine-confirmed / human-reviewed); "advisory, not access control" | tq derives what a run *did* (commits via the `Task-Queue-ID` footer, files via `git diff-tree`, usage via go-crush-data) but has no `verified` twin and no derived tier on a task or claim | **ADOPT (design).** The gap is real: a "human-reviewed" marker on a closeout claim is exactly the signal a reviewer sweeper wants to filter on before reading the report. Tracked as a TODO; do not build until a consumer exists (OKF's own "model what you know"). |
| 6  | **Freshness is an absolute anchor, not an elapsed window**                                  | §5.3: `now >= stale_after`; an absolute instant so the comparison is read-time independent                              | tq anchors evidence to commits, not clocks: verify-window batteries "inherit a same-HEAD report"; `lint-baseline` guards a stale cache; `prune-stale` keys on the item text        | **ALIGNED (independent convergence).** Cite as corroboration for the same-HEAD rule. Explicitly do **not** convert the `.tq/claimed/<id>` mtime window (`internal/harvest/verdicts.go:138`) — a claim genuinely is a refreshed lease, and mtime+window is equivalent to an absolute `expires_at` for a mutating file. |
| 7  | **Deprecate, do not delete: historical definitions stay readable**                          | §5.4 + article: `status: deprecated` keeps a superseded definition "for historical query reproducibility"             | tq keeps legacy read paths rather than migrating them away: the facade `Open` auto-upgrades legacy DBs, `decodePayload` reads pre-field shapes, `requeuedEvent` reads legacy facts | **ALIGNED.** Note the deliberate exception: TODO_LIST.md deletes DONE rows (owner ruling 2026-09-14). The OKF rule applies to *fact/definition* schemas, not to the task backlog. Recorded so nobody "fixes" the backlog to match OKF.                |
| 8  | **Path is identity; no separate ID to mint**                                                | §9: the file path is the concept's identity; links are untyped edges whose kind lives in prose                          | tq's identity is a content hash, not an allocated key: `DedupKey` re-enqueue returns the stored task, batched harvest hashes `batch:` + the sorted item keys                  | **ALIGNED.** Same trade (no registry, no allocation step, moves are free) with the same cost (renaming re-identifies).                                                                                                                                |
| 9  | **Attested computation: the agent fills parameters and may not author the computation**     | §10.3: the agent "MAY only supply values for the declared parameters; it MUST NOT author or edit the computation"      | agent payloads are minted queue-side (`tq enqueue` is forbidden to dispatched agents), the verify command comes from the payload and not from the agent, and verifies carry `GOEXPERIMENT=jsonv2` because "agents never edit their own gate" (`internal/executor/agent.go:1294`) | **ALIGNED — by policy, not by type**, with one deliberate hole: `--reresolve-verify` (`internal/executor/agent.go:124`, wired at `cmd/tq/agentpool.go:123` / `cmd/tq/main.go:631`) ignores the enqueue-time pin and resolves the gate fresh from the repo's `.tq-verify`, i.e. from content an agent may edit. The default keeps the pin, so the sanctioned-computation rule holds by default; OKF lesson 6 shows the *better* cure for the staleness this flag exists to beat (timestamp the pin and flag it stale, which `tq doctor --hygiene` already audits at `cmd/tq/doctor.go:1096`), so the two lessons compose instead of trading against each other. Tracked as a TODO.                                        |
| 10 | **A run yields a *receipt* with a declared field set; a no-LLM attester grades it**        | §10.2/§10.5: `executor.receipt` declares the fields a run must return; `attester` is deterministic and LLM-free; a false verdict means the consumer refuses the value | tq has the *convention* but not the artifact: "a report's 'N ok / M fail' count is `grep -c '^ok:'` over a CAPTURED gate-output file — never counted in context" (AGENTS.md §Conventions) | **ADOPT (highest value).** This is the load-bearing gap: tq's gate evidence is a text file counted by hand in a paid session. OKF's answer is a typed receipt plus a deterministic comparison against the sanctioned definition. Tracked as a TODO, scoped to the verify stage first. |
| 11 | **`verified` (definition-level, slow, stored) is not attestation (per-run, runtime, never stored)** | §10.6                                                                                                          | tq deliberately separates `recordRunOutcome` (what happened) from `IsGateArtifactDeath` (a judgement over that outcome), and keeps auto-dismiss deterministic                        | **ALIGNED, and the separation is enforced by the same reasoning.** OKF's vocabulary makes the boundary nameable: attempt deaths are attestation, `--review-autofix` findings are verification.                                                                        |
| 12 | **Elevate decision fields so the decision is token-free**                                   | article: most interactions never reach the body; filter on frontmatter before reading                                 | `tq dlq --evidence-stage` filters the dead-letter surface by the failed fact's Stage/VerifyStage/Class with `!`-negation, and `tq tasks --parked` renders requeue evidence inline | **ALIGNED — tq independently built the same idea.** The transferable rule: any new trust signal belongs on the *row* surfaces (`tq tasks`, `tq dlq`, webui timeline), never only inside a report body.                                                     |
| 13 | **Additive version growth is the compatibility strategy**                                  | §Compatibility: minor + opt-in + two documented renames with documented fallbacks; a v0.1 bundle "drops in unchanged"   | tq's flag additions are additive with byte-identical behaviour when unset (e.g. `tq verdict --file`, `internal/executor/agent.go`); but module floors are pinned and force lockstep  | **ADOPT the pattern where it is free.** When a journal detail gains a field, make it optional with a documented absence meaning and a fallback read of the old shape — never a required field and never a second writer. See TODO.                       |
| 14 | **Format, not service: consumers must not need the producer's code**                        | v0.1 article: producible without an SDK, consumable without an integration                                            | tq's machine contracts are already file/JSON: the verdict channel is a JSON file (`cmd/tq/verdict.go`), markers are files under `.tq/` (`internal/harvest/verdicts.go:20`), the journal is SQLite | **ALIGNED — worth stating as a constraint.** Any future "trust signal" must land in a file or a journal fact a non-Go consumer can read, not in a Go-only struct.                                                                                        |
| 15 | **Tolerance is for knowledge surfaces, not execution contracts**                            | §11's tolerance is scoped to *consumers of a knowledge bundle*: a concept that fails to parse costs a missing answer, not a wrong action | tq already draws that line: the fold, the web UI and the detail decoders tolerate unknown types and keys (lessons 1-2), but an `agent` payload whose `v` exceeds what this binary understands fails fast as a permanent error (`internal/executor/agent.go:273`), as does `depbump` (`internal/executor/depbump.go:177`) | **ALIGNED — and this is the boundary OKF does not state.** Do not "make it tolerant" to match §11: a payload drives execution, so misparsing it is a wrong action, not a missing answer. Recorded so the guard work below cannot be over-applied. |

## What actually transfers, in one paragraph

tq has discovered all five of OKF's trust questions on its own and answered each
one with a **prose convention**: citations for provenance, "captured gate-output
file" for receipts, same-HEAD for freshness, dated DONE re-verified notes for
lifecycle, "agents never edit their own gate" for attestation. Prose conventions
fail exactly where OKF says they fail — a rule in AGENTS.md is not filterable
before the read, not checkable by a deterministic program, and not preserved
across a producer/consumer swap. The valuable move is therefore **not** to adopt
OKF (tq is not a knowledge catalog) but to adopt its **shape**: for every trust
rule tq already believes, ask whether the signal can be (a) optional, (b)
additive, (c) carried on the row surface a consumer already filters on, and (d)
graded by a deterministic checker instead of a paid reader.

The clearest instance of that composition is `--reresolve-verify`. The flag exists because a
pinned gate goes stale; it trades the pin (OKF's "the agent MUST NOT author the computation")
for freshness (lesson 6). OKF's two lessons together say that trade is unnecessary: freshness is
an *absolute timestamp on the pin*, not a re-read of mutable repo content, and tq already audits
stale pins in `tq doctor --hygiene`. Adopting lesson 6 properly removes the reason lesson 9 was
weakened.

## Rejected / not applicable (so nobody re-litigates blind)

- **Adopt OKF as a tq knowledge format.** tq is a task queue, not a knowledge
  catalog. Its corpus is the journal plus a handful of coordination markers;
  wrapping those in a markdown bundle would add a translation layer OKF itself
  argues against. **Rejected.**
- **Make every JSON boundary tolerant of unknown fields, per §11.** Rejected for
  *execution* contracts. `agent` and `depbump` payloads gate on an explicit contract
  version and reject anything newer (`internal/executor/agent.go:273`,
  `internal/executor/depbump.go:177`) because a misparsed execution payload runs the
  wrong action, while a misparsed knowledge row merely renders blank or skips. OKF's
  rule is aimed at the observation surfaces (the fold, the web UI, detail decoders),
  and that is exactly where it is adopted here.
- **Sync OKF bundles as a harvest source.** Tempting (a bundle of concepts
  looks like a TODO list) but `tq harvest` is deliberately bound to one
  machine-consumed shape with strong invariants (one `- [ ]` per line, no
  tables, `— BLOCKED:` semantics, `check-todo-list.sh`). A second source format
  doubles the gate surface for no current consumer. **Rejected as premature**;
  revisit only if an external bundle is actually offered.
- **Replace the mtime-based `.tq/claimed/<id>` liveness window with an absolute
  `expires_at`.** Algebraically equivalent for a file that is rewritten on
  heartbeat, and the change would touch the marker protocol shared with
  `tq-claim.sh` / `tq-preflight`. **Rejected (churn without a symptom).**
- **Store a credibility score for sources/gates.** OKF rejects this explicitly
  (§5.1). tq's AI priority score is a *scheduling* signal and must not be
  promoted into a trust or authority signal. **Rejected.**
- **Delete `status: deprecated` analogues to keep surfaces small.** The
  opposite: tq's legacy read paths are commitments to already-written journals.
  **Rejected**, and recorded because OKF's `deprecated` is often misread as
  "safe to remove".
- **Use OKF's reserved filenames as a docs convention.** tq's docs already have
  a working split (`docs/status/` reports, `CHANGELOG.md`, per-directory
  indexes enforced by `check-status-index.sh`). Renaming to `index.md`/`log.md`
  buys nothing. **Rejected.**

## Verification trail

- 2026-11-01, read in TWO passes, and the shape of the passes matters:
  1. *First pass was subagent-mediated.* The `fetch` tool returned an empty body
     for both blog URLs (tried twice, markdown and text), so the initial reading
     came from `agentic_fetch`, an AI extraction sub-agent, including the
     `SPEC.md` text.
  2. *Second pass byte-verified the originals.* `download` retrieved the raw
     files (`SPEC.md`, 1006 lines; both blog HTML pages), and every load-bearing
     phrase in the citations table was then grepped against those bytes: the
     §4.1 "only always-required key" and "tolerate unknown types" wording, the
     §4.1 Extensions "MUST NOT reject documents with unrecognized fields"
     wording, §5.5's "An absolute instant, not a relative TTL…", §5.4's
     "Absent `status` ⇒ `stable`", §5.2's "bare mapping as a one-element list",
     §11's "MUST NOT reject a bundle because of" list, §10.3's "MAY only supply
     *values*", §10.6's "doc-level, slow, and recorded in the bundle", §6.1's
     "directed edges of an untyped" relationship, plus the article dates
     (2026-06-12 / 2026-07-25), both author names, and the phrases "vocabulary,
     not rules", "unverified / machine-confirmed / human-reviewed",
     "Minimally opinionated", "Producer/consumer independence" and "Format, not
     platform". All matched.
- Residual caveat, stated rather than papered over: the reference
  implementation (`reference_agent`, the static visualizer, the four sample
  bundles, `attesters/sql_equality.py`) was **not** read at source. This note
  makes no claim about their code beyond what the article's prose states, and
  the row-10 lesson is drawn from the SPEC's declared field set, not from that
  example attester.
- tq-side claims verified against the working tree the same window:
  - `internal/readmodel/events.go:223` (unknown fact types skipped by the fold's
    `default` branch), `:271` (legacy requeue detail tolerance).
  - `internal/webui/payload.go:25` (unknown payload type degrades to raw).
  - `internal/executor/payload.go:14` (`decodePayload`; plain `json.Unmarshal`,
    no strict mode).
  - `grep -rn "DisallowUnknownFields" --include=*.go .` → **zero hits**
    repo-wide (excluding nothing; vendor included).
  - `internal/executor/agent.go:620`/`:635` (`envDenylistForAgents` = `TQ_DB`)
    and `:1294` (`GoEnvExperiment`, the "agents never edit their own gate"
    constant).
  - `internal/harvest/verdicts.go:20` (`.tq/verdicts` / `.tq/claimed` marker
    vocabulary) and `:138` (`claimLive`, mtime + window).
  - `internal/queue/queue.go:443` (`ParseReprioritizeEvidence`, skip-don't-fail),
    `:463` (`ValidQuestionType`, a closed enum that exists *because* an upstream
    ingest route does reject unknown types).
  - `internal/readmodel/detailkeys_test.go` (the key-parity pin discipline this
    note's guard work extends).
  - Landed in this window: `internal/readmodel/forwardcompat_test.go` (3 tests) and
    `internal/executor/payload_test.go` (3 tests). Verified green: `internal/readmodel`
    `GOWORK=off go test ./... -count=1` rc=0, `internal/executor` `GOWORK=off go test ./... -count=1`
    rc=0 (27.7s full module suite), root `go build ./...` rc=0 and `go vet ./...` rc=0,
    `check-todo-list.sh` rc=0 (0 structural, advisories pre-existing), `check-doc-refs.sh` rc=0.
  - `AGENTS.md` §Conventions ("Gate receipts", "Claims carry citations",
    "Verify-window battery") and §Operational contracts (the `tq verdict`
    channel).

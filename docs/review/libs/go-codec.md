# go-codec — Library Utilization Review

**Verdict:** ADOPTED — high (2.5/3 applicable capability rows leveraged; published as 63 before two corrections) — deep and test-pinned at exactly one seam (the cqrs fact→event JSON boundary); the library's wider surface is legitimately N/A for a JSON-wire facts journal, and the remaining bare `encoding/json` sites are deliberate (F3).

**2026-11-22 refresh:** every pin, cite, and count re-verified against HEAD `8395a969` (343 commits past the 11-11 tree `60c19070`; `git diff --name-only 1c8219e6..HEAD | grep -v '^docs/'` prints only `internal/webui/theme.css`, so every Go cite is identical at `1c8219e6`). Library-side: tag v0.3.3 still latest; checkout `v0.3.3-3-g759d752` → `v0.3.3-7-g98bb1bc` (2026-11-15) — the four new commits are CI/lint/flake chores with **zero `.go` delta since the tag** (`git diff --stat v0.3.3..HEAD -- '*.go'` → empty); [Unreleased] Added stays "Nothing yet" (one Changed line: CI lint stack → v2.15.0 with an errorfamily wrapcheck exemption). tq-side: pins unchanged — 16 require rows, 3 direct at the same lines (cqrs go.mod:6, cmd/tq/go.mod:7, executor go.mod:7). The seam itself is intact but several 11-11 cites were **stale at publication**: `git show 60c19070:internal/executor/payload.go` already read var `:15` / Decode `:28` (grep → 10, 15, 28), not the printed 15-24/:34 — payload.go has been byte-identical since 2026-11-09 (`git diff 932c7efc..HEAD -- internal/executor/payload.go` empty); corrected here together with facts_cqrs_test.go 131-139 → 130-141 and journal.go 181-188 → 181-189. Two tq deltas checked, **neither moves a row**: `d43f3a6c` (2026-11-18, `tasks --json` emits `[]` not `null`) touches only cmd/tq/tasks.go + tasks_test.go — a nil-slice init at the CLI output edge, zero codec contact; `e7e10de4` (2026-11-13, calibration.jsonl retention cap) adds a second append writer (calibration.go:82) and `TrimCalibrationDataset` (:194-243) — a byte-boundary keep-the-tail cut (`bytes.IndexByte` newline search, no JSON decode), so the Streaming row stays N/A with updated evidence (:82,163). Guard tests run in-tree this refresh (the 11-11 edition ran none): cqrs `TestPayload*` ok 0.004s, executor `TestDecodePayload*` ok 0.004s. **Score: unchanged 83 — high** (N=16, N/A=13, A=3; FULL=2, PARTIAL=1 → Σ=2.5/3 — no row moved).

**2026-11-11 refresh:** (1) F2 LANDED — "fleet review f7": `decodePayload[T]` now routes through `payloadCodec = codec.JSONCodec{}` (internal/executor/payload.go:15-35), exactly the recommended shape, with `DecodeEnvelopeOrLegacy` named as the forward path; go-codec is now a DIRECT require of the executor module (internal/executor/go.mod:7, from the M6 facade promotion). (2) DENOMINATOR CORRECTION — the §2 inventory has N=16 rows with 13 N/A, so A=3 (not 4 as published); the corrected pre-landing score is round(100×2.5/3) = 83, not 63. Row 3 stays PARTIAL (the readmodel detail reads remain deliberately outside the codec per F3), so the current score is **83 — high**. §1 cites re-verified against HEAD 2026-11-11 by grep; checkout now `v0.3.3-3-g759d752`, still pinned v0.3.3 = latest tag.

**Pinned:** v0.3.3 — direct requires: `internal/journal/cqrs/go.mod:6`, `cmd/tq/go.mod:7`, `internal/executor/go.mod:7` (new direct require since the M6 facade promotion); `// indirect` in 13 more module graphs (via go-cqrs-lite `event/v4` and the worker facade). Verified 2026-11-22 by `grep -rn 'go-codec' --include=go.mod . | grep -v vendor` (16 require rows, 3 direct — same count and line numbers as 2026-11-11).

**Latest:** v0.3.3 (local checkout /home/lars/projects/go-codec, re-checked 2026-11-22 by `git -C … describe --tags` → `v0.3.3-7-g98bb1bc`; Unreleased Added empty — one Changed line, the CI lint bump).

**Prior coverage:** none (first libs-review report).

**Reviewed:** 2026-11-08 by reading /home/lars/projects/go-codec/{README.md,FEATURES.md,CHANGELOG.md,json.go,codec.go,envelope.go,streaming.go} + repo greps (`grep -rl --include='*.go' 'larsartmann/go-codec' . | grep -v '^./vendor/'` → 3 files) + reading vendor/github.com/larsartmann/go-cqrs-lite/event/v4/codec.go (the decode side of the seam). Re-verified 2026-11-11 at HEAD `60c19070`. **Re-verified 2026-11-22** at HEAD `8395a969` by the same greps, checkout tag/HEAD checks + a tag→HEAD `*.go` diff (empty), fresh line-number sweeps of payload.go / journal.go / journal_test.go / facts_cqrs_test.go / calibration.go / events.go / host.go / redact.go, reads of the `d43f3a6c` and `e7e10de4` diffs, and the two seam guard tests in-tree (ok 0.004s / ok 0.004s).

## 1. Usage inventory

| File | Line | Use |
| --- | --- | --- |
| internal/journal/cqrs/journal.go | 164 | `event.WithCodec(codec.JSONCodec{})` — the production call: `factEvent` marshals each journal fact's payload through the codec and stamps the encoding (comment at 147-150: unstamped payloads are refused by `DecodePayloadAuto`) |
| internal/executor/payload.go | 10-15 | `payloadCodec = codec.JSONCodec{}` — "the ONE JSON door" for executor payload decoding (2026-11-09/10 landing, fleet review f7; file byte-identical since 2026-11-09 — the 11-11 edition's 15-24 cite was stale at publication, corrected 2026-11-22) |
| internal/executor/payload.go | 28 | `payloadCodec.Decode(t.Payload, &payload)` — `decodePayload[T]` (:21-33) routes through the codec |
| internal/journal/cqrs/journal_test.go | 148-151 | pins every mapped event carries `codec.EncodingJSON` ("codec.ForEncoding refuses an empty stamp") |
| cmd/tq/facts_cqrs_test.go | 130-141 | `assertEventsEncodingStamped` — same contract pinned through the real store (the CLI-level gate) |

Decode side (cqrs-lite's, on our live path): internal/readmodel/host.go:201 `event.DecodePayloadAuto[foldPayload](evt)` → vendor event/v4/codec.go:45 `codec.ForEncoding(evt.Encoding())` → `JSONCodec.Decode`. So both halves of the codec contract — encode-with-stamp and resolve-by-stamp — run on every folded fact. The repo itself never constructs another codec, wraps one, or calls `Encode`/`Decode` directly.

## 2. Capability inventory

| Capability | Status | Evidence |
| --- | --- | --- |
| `Codec` interface via `JSONCodec` at the event boundary | 🟢 FULL | journal.go:164; round-tripped through the platform on every fold |
| Encoding stamp + `ForEncoding` resolution | 🟢 FULL | stamped at journal.go:164, resolved at vendor codec.go:45, pinned by both test files (§1) |
| Repo-side JSON routed through the codec | 🟡 PARTIAL | the shared executor seam now routes through `codec.JSONCodec` (payload.go:15, :28 — F2 landed); the readmodel detail reads stay deliberately outside (events.go:251,277,291 — F3: shape-tolerant, explicit classification) |
| `CBORCodec` (canonical, smaller/faster) | ⚪ N/A | journal wire contract is JSON for operator inspectability — `fact.Detail` is `jsontext.Value` (journal.go:181-189) rendered by `tq audit`/`tq show`; a CBOR flip is a journal format break with no consumer demand |
| `CBORCompactCodec` (unknown-field rejection) | ⚪ N/A | would REFUSE unknown detail keys — the opposite of the OKF v0.2 §11 additive-growth contract (AGENTS.md: unknown detail keys are ignored, legacy fallback reads) |
| `RawCodec` passthrough | ⚪ N/A | every payload is typed at construction (`factPayload`); no pre-encoded byte path exists |
| `BufferEncoder`/`GetBuffer`/`EncodePooled` | ⚪ N/A | no in-repo hot encode loop calls the codec directly; per-fact encode buffering is owned by cqrs-lite's `event.New` |
| `ObservableCodec` + metrics | ⚪ N/A | tq has no metrics stack; health surfaces are domain-level (`pool-health`, `--read-model` S4 root) |
| `AutoDetect`/`AutoDetectDebug` | ⚪ N/A | every event is stamped; the one mixed-format hazard (legacy pre-flip DBs) is healed by the facade in-place auto-upgrade (ADR-0019), not format sniffing |
| `TranscodeToJSON` | ⚪ N/A | no CBOR estate to transcode (see CBOR row) |
| Envelope `WrapEncode`/`UnwrapDecode`/`DecodeEnvelopeOrLegacy` | ⚪ N/A | self-description already exists one level up: the event carries the encoding stamp the envelope would add |
| Streaming (`NewJSONEncoder`/`NewJSONDecoder`, NDJSON) | ⚪ N/A | calibration.jsonl is write-only: two append writers, one `json.Marshal` per row (calibration.go:82,163 — the second writer plus a 64 MiB keep-the-tail trim, `TrimCalibrationDataset` :194-243, landed 2026-11-13 in `e7e10de4`); the trim cuts at a newline via `bytes.IndexByte` and decodes no JSON — no bulk JSONL reader exists in the repo |
| `Size`/`SizeResult` | ⚪ N/A | no payload-size budget surface (no enqueue size gate exists) |
| COSE structure codec + base64 JSON helpers | ⚪ N/A | journal is local-trust; no signing/encryption consumers |
| `DeterministicCodec` marker | ⚪ N/A | determinism assertion is a signing-module concern (checkout FEATURES "Signing safety"); no signer here |
| CBOR modes/`Diagnose`/`toarray`/`keyasint` tags | ⚪ N/A | CBOR absent from the wire (see CBOR row) |

## 3. Findings

**F1 — The seam is the right one and is double-pinned.** lib: `event.New`-style constructors marshal "the payload through the codec and stamp the encoding" — the contract go-cqrs-lite builds on (checkout README "Architecture"; codec.go `ForEncoding`). repo: journal.go:147-164 does exactly this, with the unstamped-payload failure mode documented at the call site, and the stamp contract pinned at BOTH the adapter unit level (journal_test.go:148-151) and through the real store at the CLI level (facts_cqrs_test.go:130-141 — "the bug the 2026-09-13 lint pass fixed"). This is max-depth usage of the applicable surface.

**F2 — ~~`decodePayload[T]` bypasses the codec~~ RESOLVED (landed 2026-11-09/10, "fleet review f7").** lib: `JSONCodec` implements `Codec` by delegating to `encoding/json` (checkout json.go:8-23 — `jsonMarshal`/`jsonUnmarshal`), dual-build so it rides `encoding/json` v2 under `GOEXPERIMENT=jsonv2`. repo: internal/executor/payload.go:10-15 declares `payloadCodec = codec.JSONCodec{}` — commented as "the ONE JSON door executor payload decoding goes through… so a future switch to stamped/enveloped payloads (codec.DecodeEnvelopeOrLegacy) is a seam change, not a hunt across executors" — and `decodePayload` (:21-33) calls `payloadCodec.Decode` (payload.go:28). go-codec became a DIRECT require of the executor module (go.mod:7). Recommendation satisfied in full; the readmodel bypass remains deliberate (F3). (Cite correction 2026-11-22: the 11-11 edition printed 15-24/:34, but the file has been byte-identical since 2026-11-09 — `git diff 932c7efc..HEAD -- internal/executor/payload.go` is empty — so those numbers never matched the landed file.)

**F3 — Readmodel detail unmarshals are shape-tolerant by design; the codec adds nothing.** lib: strict decode variants exist (`CBORCompactCodec` rejects unknown fields — N/A above). repo: events.go:277 (`requeuedEvent`) deliberately IGNORES malformed detail — "legacy requeue facts predate the detail entirely… never fails on detail noise" — while :251/:291 turn malformed enqueue/repri details into error-family Corruption (see the go-error-family report). Both behaviors are contract-level choices (AGENTS.md additive-growth + legacy fallback); a codec swap cannot express them better than `json.Unmarshal` + explicit error classification already does.

**F4 — Redaction is orthogonal.** `internal/executor/redact.go` redacts output tails as strings before they ever reach a JSON encoder; no JSON-aware redaction need exists (verified 2026-11-08 by `grep -n 'json' internal/executor/redact.go` → no matches). No codec capability (envelope, transcode, observe) touches this path.

**F5 — If the journal ever changes wire format, the migration path is already in the lib.** lib: `DecodeEnvelopeOrLegacy[T]` does "one-call envelope/raw decode" with a JSON↔CBOR cross-retry, built for mixed-format streams (checkout envelope.go; FEATURES "Self-describing envelopes"). repo: not needed while facts are JSON-only, but it is the designed tool if `factPayload` ever gains a v2 encoding — record it as the migration lever, not a current gap.

## 4. Score + top opportunities

Inputs (re-derived 2026-11-22; denominator first corrected 2026-11-11): N=16 rows, ⚪ N/A=13, A=3. FULL=2, PARTIAL=1 (0.5), MISSED=0, MISUSED=0 → Σ=2.5. No row moved since 11-11: `d43f3a6c` touches only the CLI output edge (a nil-slice init in cmd/tq/tasks.go), `e7e10de4` only the calibration dataset writers — neither crosses the codec seam.
Score = round(100 × 2.5/3) = **83 — high**. (The originally published 63 divided by A=4 — a 12-vs-13 N/A miscount; the corrected inputs above are the published numbers' replacement.)

Top opportunities: (1) F5 — if journal payload versioning is ever proposed, spec it as envelope + `DecodeEnvelopeOrLegacy` from day one (the lib lever is now one seam away, per the landed payload.go comment); (2) if the readmodel detail reads ever need stamped payloads, route them through a shared codec door too — today their bare `json.Unmarshal` is F3-documented design, not debt. Everything else is genuinely N/A for a JSON-wire, operator-inspectable journal.

## 5. Version currency + risk

Pinned v0.3.3 == latest tag (checkout 7 commits past tag, `v0.3.3-7-g98bb1bc` re-checked 2026-11-22; the four commits past `759d752` are CI/lint/flake chores — `git diff --stat v0.3.3..HEAD -- '*.go'` is empty; Unreleased Added stays "Nothing yet", Changed carries only the CI lint bump to v2.15.0) — no action. v0.3.3 (2026-10-29, CHANGELOG) is a decode-only additive fix: CBOR decode accepts text strings for `encoding.TextUnmarshaler` types (branded-ID replay fix) — irrelevant to the JSON seam but signals the maintainer's compat posture: "encode output is byte-identical; the only behavioral change is on DECODE, and it is purely additive". Risk: module declares `go 1.27` (checkout go.mod:3), tq floor is 1.27.3 — compatible. The dual-build v1/v2 JSON contract (FEATURES "Dual-build infrastructure") means the pinned codec keeps working under ci-local's `GOEXPERIMENT=jsonv2` exactly as the repo's direct `encoding/json` sites do.

## 6. Conclusion

go-codec is adopted as the serialization contract at precisely the boundary it was built for: the cqrs fact→event adapter stamps JSON via `JSONCodec`, cqrs-lite resolves it back via `ForEncoding` on every fold, two test layers pin the stamp, and — since the f7 landing — the executor's shared payload seam decodes through the same codec door. The wide CBOR/COSE/streaming surface is honestly N/A — this repo's journal is a human-inspectable JSON wire with an additive-growth detail contract that the strict codecs would violate. The remaining bare `encoding/json` sites are documented design (F3), and the forward-looking lever is the envelope/legacy decoder if payload versioning ever lands.

## How to verify

```bash
cd /home/lars/projects/go-taskqueue
grep -rl --include='*.go' 'larsartmann/go-codec' . | grep -v '^./vendor/'
# → cmd/tq/facts_cqrs_test.go, internal/executor/payload.go, internal/journal/cqrs/journal_test.go, internal/journal/cqrs/journal.go
grep -n 'codec\.' internal/journal/cqrs/journal.go internal/journal/cqrs/journal_test.go cmd/tq/facts_cqrs_test.go
# → journal.go:164 WithCodec(JSONCodec{}); journal_test.go:150-151 + facts_cqrs_test.go:137-138 EncodingJSON
grep -n 'payloadCodec' internal/executor/payload.go    # → 10, 15, 28 (the f7 landing; file unchanged since 2026-11-09)
grep -rn 'go-codec' --include=go.mod . | grep -v vendor | grep -cv indirect   # → 3 (cqrs, cmd/tq, executor)
sed -n '149,164p;181,189p' internal/journal/cqrs/journal.go      # stamp rationale + factPayload (jsontext detail)
sed -n '199,205p' internal/readmodel/host.go              # DecodePayloadAuto consumer
sed -n '37,56p' vendor/github.com/larsartmann/go-cqrs-lite/event/v4/codec.go  # ForEncoding on the fold path
sed -n '10,28p' internal/executor/payload.go              # codec-routed decodePayload (F2, landed)
sed -n '82p;163p' internal/executor/calibration.go        # the two JSONL row writers (streaming N/A)
sed -n '187,198p' internal/executor/calibration.go        # the 64 MiB keep-the-tail trim — byte cut, no JSON decode
sed -n '251p;277p;291p' internal/readmodel/events.go      # detail unmarshal sites (F3)
grep -n 'json' internal/executor/redact.go                # → no matches (F4)
git -C /home/lars/projects/go-codec describe --tags       # → v0.3.3-7-g98bb1bc
git -C /home/lars/projects/go-codec diff --stat v0.3.3..HEAD -- '*.go'   # empty — no Go delta since the tag
cd internal/journal/cqrs && GOEXPERIMENT=jsonv2 GOWORK=off go test -run 'TestPayload' -count=1        # ok (stamp + round-trip pins; 0.003–0.141s across this session's runs)
cd /home/lars/projects/go-taskqueue/internal/executor && GOEXPERIMENT=jsonv2 GOWORK=off go test -run 'TestDecodePayload' -count=1 .   # ok 0.004s (the seam's own tests)
```

All commands above were run literally on 2026-11-22 against HEAD `8395a969` (go1.27.4; `GOEXPERIMENT=jsonv2 GOWORK=off` on the two test runs; the `cd` lines make the block replay sequentially in one shell) and produced the cited outputs. Tree note: at the time of the replay the non-docs diff `git diff --name-only 1c8219e6..HEAD | grep -v '^docs/'` printed only `internal/webui/theme.css`, so every Go cite above is identical at `1c8219e6` and `8395a969`; a later daemon commit (`42194237`) reworked cmd/tq worker-flag plumbing (main.go + flag_parity_test.go) — neither file is cited here, and facts_cqrs_test.go / the seam files are untouched. Repairs this refresh: the payload.go and calibration.go line cites and their `sed` ranges (the 11-11 numbers were stale at publication — see the refresh note above), the `describe` output (`v0.3.3-3-g759d752` → `v0.3.3-7-g98bb1bc`), a tag→HEAD `*.go`-diff line added, and the two guard-test commands added (the 11-11 edition ran no tests).

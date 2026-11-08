# go-codec — Library Utilization Review

**Verdict:** ADOPTED — moderate (2.5/4 capability rows leveraged) — deep and test-pinned at exactly one seam (the cqrs fact→event JSON boundary); the library's wider surface is legitimately N/A for a JSON-wire facts journal, and the repo's other JSON sites bypass it with semantics-identical `encoding/json` calls.

**Pinned:** v0.3.3 — direct requires: `internal/journal/cqrs/go.mod:6`, `cmd/tq/go.mod:7`; `// indirect` in 11 more modules (via go-cqrs-lite `event/v4`). Verified 2026-11-08 by `grep -rn 'go-codec' --include=go.mod . | grep -v vendor`.

**Latest:** v0.3.3 (local checkout /home/lars/projects/go-codec, checked 2026-11-08 by `git -C … describe --tags` → `v0.3.3-2-g7d02310`; Unreleased empty).

**Prior coverage:** none (first libs-review report).

**Reviewed:** 2026-11-08 by reading /home/lars/projects/go-codec/{README.md,FEATURES.md,CHANGELOG.md,json.go,codec.go,envelope.go,streaming.go} + repo greps (`grep -rl --include='*.go' 'larsartmann/go-codec' . | grep -v '^./vendor/'` → 3 files) + reading vendor/github.com/larsartmann/go-cqrs-lite/event/v4/codec.go (the decode side of the seam).

## 1. Usage inventory

| File | Line | Use |
| --- | --- | --- |
| internal/journal/cqrs/journal.go | 164 | `event.WithCodec(codec.JSONCodec{})` — the ONLY production call: `factEvent` marshals each journal fact's payload through the codec and stamps the encoding (comment at 149-153: unstamped payloads are refused by `DecodePayloadAuto`) |
| internal/journal/cqrs/journal_test.go | 148-151 | pins every mapped event carries `codec.EncodingJSON` ("codec.ForEncoding refuses an empty stamp") |
| cmd/tq/facts_cqrs_test.go | 131-139 | `assertEventsEncodingStamped` — same contract pinned through the real store (the CLI-level gate) |

Decode side (cqrs-lite's, on our live path): internal/readmodel/host.go:201 `event.DecodePayloadAuto[foldPayload](evt)` → vendor event/v4/codec.go:45 `codec.ForEncoding(evt.Encoding())` → `JSONCodec.Decode`. So both halves of the codec contract — encode-with-stamp and resolve-by-stamp — run on every folded fact. The repo itself never constructs another codec, wraps one, or calls `Encode`/`Decode` directly.

## 2. Capability inventory

| Capability | Status | Evidence |
| --- | --- | --- |
| `Codec` interface via `JSONCodec` at the event boundary | 🟢 FULL | journal.go:164; round-tripped through the platform on every fold |
| Encoding stamp + `ForEncoding` resolution | 🟢 FULL | stamped at journal.go:164, resolved at vendor codec.go:45, pinned by both test files (§1) |
| Repo-side JSON routed through the codec | 🟡 PARTIAL | `decodePayload[T]` uses bare `encoding/json.Unmarshal` (internal/executor/payload.go:21); readmodel detail unmarshals too (events.go:251,277,291); `JSONCodec.Decode` is the identical call (checkout json.go:21-23) |
| `CBORCodec` (canonical, smaller/faster) | ⚪ N/A | journal wire contract is JSON for operator inspectability — `fact.Detail` is `jsontext.Value` (journal.go:170-177) rendered by `tq audit`/`tq show`; a CBOR flip is a journal format break with no consumer demand |
| `CBORCompactCodec` (unknown-field rejection) | ⚪ N/A | would REFUSE unknown detail keys — the opposite of the OKF v0.2 §11 additive-growth contract (AGENTS.md: unknown detail keys are ignored, legacy fallback reads) |
| `RawCodec` passthrough | ⚪ N/A | every payload is typed at construction (`factPayload`); no pre-encoded byte path exists |
| `BufferEncoder`/`GetBuffer`/`EncodePooled` | ⚪ N/A | no in-repo hot encode loop calls the codec directly; per-fact encode buffering is owned by cqrs-lite's `event.New` |
| `ObservableCodec` + metrics | ⚪ N/A | tq has no metrics stack; health surfaces are domain-level (`pool-health`, `--read-model` S4 root) |
| `AutoDetect`/`AutoDetectDebug` | ⚪ N/A | every event is stamped; the one mixed-format hazard (legacy pre-flip DBs) is healed by the facade in-place auto-upgrade (ADR-0019), not format sniffing |
| `TranscodeToJSON` | ⚪ N/A | no CBOR estate to transcode (see CBOR row) |
| Envelope `WrapEncode`/`UnwrapDecode`/`DecodeEnvelopeOrLegacy` | ⚪ N/A | self-description already exists one level up: the event carries the encoding stamp the envelope would add |
| Streaming (`NewJSONEncoder`/`NewJSONDecoder`, NDJSON) | ⚪ N/A | calibration.jsonl is write-append only (internal/executor/calibration.go:84,121 — one `json.Marshal` per row); no bulk JSONL reader exists in the repo |
| `Size`/`SizeResult` | ⚪ N/A | no payload-size budget surface (no enqueue size gate exists) |
| COSE structure codec + base64 JSON helpers | ⚪ N/A | journal is local-trust; no signing/encryption consumers |
| `DeterministicCodec` marker | ⚪ N/A | determinism assertion is a signing-module concern (checkout FEATURES "Signing safety"); no signer here |
| CBOR modes/`Diagnose`/`toarray`/`keyasint` tags | ⚪ N/A | CBOR absent from the wire (see CBOR row) |

## 3. Findings

**F1 — The seam is the right one and is double-pinned.** lib: `event.New`-style constructors marshal "the payload through the codec and stamp the encoding" — the contract go-cqrs-lite builds on (checkout README "Architecture"; codec.go `ForEncoding`). repo: journal.go:149-164 does exactly this, with the unstamped-payload failure mode documented at the call site, and the stamp contract pinned at BOTH the adapter unit level (journal_test.go:148-151) and through the real store at the CLI level (facts_cqrs_test.go:131-139 — "the bug the 2026-09-13 lint pass fixed"). This is max-depth usage of the applicable surface.

**F2 — `decodePayload[T]` bypasses the codec, deliberately or not.** lib: `JSONCodec` implements `Codec` by delegating to `encoding/json` (checkout json.go:8-23 — `jsonMarshal`/`jsonUnmarshal`), dual-build so it rides `encoding/json` v2 under `GOEXPERIMENT=jsonv2`. repo: internal/executor/payload.go:21 calls `json.Unmarshal` directly inside the shared executor seam (AGENTS.md "Executor shared seams — never hand-roll" names `decodePayload[T]` itself). Impact: zero behavioral difference today (same stdlib call, and ci-local already exports `GOEXPERIMENT=jsonv2`); routing through `codec.JSONCodec{}.Decode` would buy only seam uniformity at the cost of a new direct dep in the executor module. Recommendation: adopt only if the executor module ever needs stamped/auto-detected payloads; otherwise the bypass is fine and this row stays PARTIAL.

```go
// payload.go — semantics-identical codec routing (optional):
if err := (codec.JSONCodec{}).Decode(t.Payload, &payload); err != nil {
	return payload, Permanent(fmt.Errorf("%s: decode payload: %w", kind, err))
}
```

**F3 — Readmodel detail unmarshals are shape-tolerant by design; the codec adds nothing.** lib: strict decode variants exist (`CBORCompactCodec` rejects unknown fields — N/A above). repo: events.go:277 (`requeuedEvent`) deliberately IGNORES malformed detail — "legacy requeue facts predate the detail entirely… never fails on detail noise" — while :251/:291 turn malformed enqueue/repri details into error-family Corruption (see the go-error-family report). Both behaviors are contract-level choices (AGENTS.md additive-growth + legacy fallback); a codec swap cannot express them better than `json.Unmarshal` + explicit error classification already does.

**F4 — Redaction is orthogonal.** `internal/executor/redact.go` redacts output tails as strings before they ever reach a JSON encoder; no JSON-aware redaction need exists (verified 2026-11-08 by `grep -n 'json' internal/executor/redact.go` → no matches). No codec capability (envelope, transcode, observe) touches this path.

**F5 — If the journal ever changes wire format, the migration path is already in the lib.** lib: `DecodeEnvelopeOrLegacy[T]` does "one-call envelope/raw decode" with a JSON↔CBOR cross-retry, built for mixed-format streams (checkout envelope.go; FEATURES "Self-describing envelopes"). repo: not needed while facts are JSON-only, but it is the designed tool if `factPayload` ever gains a v2 encoding — record it as the migration lever, not a current gap.

## 4. Score + top opportunities

Inputs: N=16 rows, N/A=12, A=4. FULL=2, PARTIAL=1 (0.5), MISSED=0, MISUSED=0 → Σ=2.5.
Score = round(100 × 2.5/4) = **63 — moderate**.

Top opportunities: (1) F2 — optionally route `decodePayload[T]` through `codec.JSONCodec{}.Decode` for one JSON door (low value: zero behavior change; do it with the next executor-module dep change, not alone); (2) F5 — if journal payload versioning is ever proposed, spec it as envelope + `DecodeEnvelopeOrLegacy` from day one. Everything else is genuinely N/A for a JSON-wire, operator-inspectable journal.

## 5. Version currency + risk

Pinned v0.3.3 == latest tag (checkout 2 commits past tag; Unreleased empty) — no action. v0.3.3 (2026-10-29, CHANGELOG) is a decode-only additive fix: CBOR decode accepts text strings for `encoding.TextUnmarshaler` types (branded-ID replay fix) — irrelevant to the JSON seam but signals the maintainer's compat posture: "encode output is byte-identical; the only behavioral change is on DECODE, and it is purely additive". Risk: module declares `go 1.27` (checkout go.mod:3), tq floor is 1.27.3 — compatible. The dual-build v1/v2 JSON contract (FEATURES "Dual-build infrastructure") means the pinned codec keeps working under ci-local's `GOEXPERIMENT=jsonv2` exactly as the repo's direct `encoding/json` sites do.

## 6. Conclusion

go-codec is adopted as the serialization contract at precisely the boundary it was built for: the cqrs fact→event adapter stamps JSON via `JSONCodec`, cqrs-lite resolves it back via `ForEncoding` on every fold, and two test layers pin the stamp. The wide CBOR/COSE/streaming surface is honestly N/A — this repo's journal is a human-inspectable JSON wire with an additive-growth detail contract that the strict codecs would violate. The only genuine gap is cosmetic seam uniformity (`decodePayload`), and the only forward-looking lever is the envelope/legacy decoder if payload versioning ever lands.

## How to verify

```bash
cd /home/lars/projects/go-taskqueue
grep -rl --include='*.go' 'larsartmann/go-codec' . | grep -v '^./vendor/'
# → cmd/tq/facts_cqrs_test.go, internal/journal/cqrs/journal.go, internal/journal/cqrs/journal_test.go
grep -n 'codec\.' internal/journal/cqrs/journal.go internal/journal/cqrs/journal_test.go cmd/tq/facts_cqrs_test.go
# → journal.go:164 WithCodec(JSONCodec{}); journal_test.go:150-151 + facts_cqrs_test.go:137-138 EncodingJSON
grep -rn 'go-codec' --include=go.mod . | grep -v vendor   # → v0.3.3; direct only in cqrs + cmd/tq
sed -n '149,177p' internal/journal/cqrs/journal.go        # stamp rationale + factPayload (jsontext detail)
sed -n '199,205p' internal/readmodel/host.go              # DecodePayloadAuto consumer
sed -n '37,56p' vendor/github.com/larsartmann/go-cqrs-lite/event/v4/codec.go  # ForEncoding on the fold path
sed -n '14,26p' internal/executor/payload.go              # hand-rolled json.Unmarshal seam (F2)
sed -n '251p;277p;291p' internal/readmodel/events.go      # detail unmarshal sites (F3)
grep -n 'json' internal/executor/redact.go                # → no matches (F4)
sed -n '84p;121p' internal/executor/calibration.go        # JSONL write-append only (streaming N/A)
git -C /home/lars/projects/go-codec describe --tags       # → v0.3.3-2-g7d02310
```

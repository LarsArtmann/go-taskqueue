# Feedback

External feedback (adoption attempts, evaluator reports, user asks) lands
here — raw and verbatim. The file is the record; sessions act on it and
cite it, never rewrite it (metadata fixes only).

## Lifecycle

1. **`new/`** — a feedback file waits here until a session triages it:
   execute the asks (with a status report citing the feedback file),
   answer them with reasons, or explicitly defer with a TODO_LIST item.
   A feedback file is never left untriaged silently.
2. **`done/`** — once every ask is executed or answered, `git mv` the file
   here. Repoint citations FIRST (status reports, CHANGELOG, ADRs cite
   these paths — `scripts/check-doc-refs.sh` fails the move otherwise),
   then update any index rows, same rule as the status archive.
3. **Nothing is deleted.** Superseded or rejected feedback moves to
   `done/` with a trailing disposition note appended by the closing
   session (what was done, where the evidence lives).

The one file so far — `new/2026-09-13_external-adoption-blocked-by-internal-paths.md`
(MTU Help Centre) — is fully executed except its offer half (the
conformance loop + reply): ADR-0016 facades, `postgres.OpenWithPool`, the
references doc, and the README consumer-status line all answer it; the
reply + tracking issue wait on the owner's channel decision (facade plan
T4).

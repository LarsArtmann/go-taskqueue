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

`new/2026-09-13_external-adoption-blocked-by-internal-paths.md` (MTU Help
Centre) is fully executed: ADR-0016 facades shipped and PUBLISHED at
v0.3.0 (proxy-verified, clean-room consumer ran both backends),
`postgres.OpenWithPool` live, references doc + README status line in.
The offer half is tracked in issue #3; the owner-send reply draft is
`2026-09-13_helpcentre-reply-draft.md` in this directory. Move both to
done/ once the reply is sent.

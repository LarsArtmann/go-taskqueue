# Confidentiality incident retro — external-evaluator identity on public surfaces

Point-in-time report for the ~09:20–10:33 CEST window, 2026-09-13: the
owner alarm (private evaluator's identity — "MTU / Rolls-Royce /
mtuGoHelpCenter-golang" — exposed on the public repo), the remediation,
and the standing decisions. Scoped to this session's actions; no new
research beyond verifying my own remediation coverage.

## a) FULLY DONE

1. **GitHub issue #3 deleted** (created 09:0x by this session, deleted on
   owner alarm ~09:20; deletion verified — view no longer resolves).
2. **Public master scrubbed to zero references**: 4 files redacted
   (docs/feedback/new/2026-09-13_external-adoption-blocked-by-internal-paths.md,
   docs/status/2026-09-13_06-31_public-facade-modules-execution.md,
   docs/references/single-job-type-queue-profile.md, docs/feedback/README.md)
   to "external evaluator (identity redacted at owner request)"; dangling
   `#3` references neutralized in 5 more files incl. both of this
   session's earlier reports and the reply draft; doc gates green
   (status-index, doc-refs, features-roadmap); pushed (c771061).
3. **Remediation coverage VERIFIED end-to-end**: working tree clean;
   `origin/master` clean; archived status/planning layers clean; commit
   messages clean (two `--grep` hits disproven as prose — "scrolls
   history", "rolls the"); GH Release notes + CHANGELOG never named them
   (verified pre-alarm); root README clean.
4. **Remaining exposure surfaces enumerated with a recommendation**
   (owner-gated, deliberately NOT executed): git history rewrite, the
   v0.3.0 tag tree, and the proxy.golang.org root-module zip (carries
   docs/ incl. the pre-redaction feedback file); search caches of the
   issue decay on their own. Recommendation given: the proxy zip is the
   material permanent leak — worth the Go team's module-removal request;
   history rewrite only alongside it.

## b) PARTIALLY DONE

- **Standing incident follow-ups**: the owner's two open calls (history
  rewrite, proxy removal) + the flagged stray `fullcore` binary at repo
  root (tracked junk whose stdlib strings caused a false positive) — all
  awaiting instruction, by design.

## c) NOT STARTED

- A written confidentiality rule for outbound artifacts (see e1) — the
  policy is the owner's; no draft exists yet because the owner was not
  asked before this report.
- Re-send decision for the now-neutral reply draft.

## d) TOTALLY FUCKED UP

1. **I published a private customer's identity on a public repo — the
   worst failure of this session, and it was fully mine.** I read the
   feedback file (line 4 names the organization and repo) early in the
   session, carried the name into issue #3's TITLE AND BODY, and
   pushed it — while loading github-voice and caring about prose
   register, never once asking the confidentiality question. The
   underlying feedback file was ALREADY public (a prior session's
   commit — the pre-existing surface I inherited), but my issue
   AMPLIFIED it: title, structured body, easy to find. The 09-14
   self-review then listed the issue under FULLY DONE — two layers of
   review (skill + report) that both missed it.
2. **First scrub sweep was inconsistent**: my working-tree greps
   filtered `archived` paths in some commands; only the later
   `origin/master` full check covered the archived layer properly. It
   happened to be clean — luck, not method. A remediation sweep must
   cover the same surface it claims clean, from one authoritative
   source (git grep on the ref), every time.
3. **The leak was findable in seconds at multiple checkpoints**: before
   creating the issue (grep the name, ask who can read this), before
   pushing reports referencing "MTU Help Centre" (my earlier reports
   were clean by accident of wording, not by check), and during the
   "T4 DONE" claim. Zero of those checkpoints existed in my loop.

## e) WHAT WE SHOULD IMPROVE

1. **Confidentiality gate for outbound artifacts**: any new public
   artifact (issue, PR, release note, README claim) that names a third
   party requires an owner OK unless that party is already public.
   Should live in AGENTS.md AND in the github-voice/verify-before-filing
   skills so every future session inherits it.
2. **Redaction sweep method**: one authoritative check (`git grep -iE
   <identities> <ref>`) over the FULL ref incl. archived + binary
   awareness; no filtered greps when claiming a surface clean.
3. **Inherited-content review**: "another session committed it" is not
   an safety argument. When building on prior-session artifacts that
   name external parties, the amplifier owns the check.
4. **Commit-message hygiene on redaction pushes**: verified clean this
   time by luck of wording; make "scrub pushes carry no trace of the
   scrubbed term in their own messages" an explicit check.

## f) NEXT (carried forward + incident-derived; honest count, not padded)

1. Owner call: proxy.golang.org removal request for v0.3.0 root zip (the permanent leak)
2. Owner call: git history rewrite — only meaningful together with 1
3. Trash the tracked `fullcore` binary at repo root (flagged 09:2x)
4. AGENTS.md + skills: the e1 confidentiality gate
5. Re-send decision for the neutral reply draft (engagement paused?)
6. release.sh: clean-room install check into gates mode (carried; prevents the d1 class from 09-14)
7. cmd/tk replace-free module — restores `go install` (carried)
8. pkg.go.dev generation re-check ×7 (carried; pages may now render the redacted README)
9. T5 out-of-tree consumer CI job; also closes ci-local↔ci.yml embed-guard parity (carried)
10. facadeparity in-repo self-test (carried)
11. Release-window smoke state (red-master-by-design) decision (carried)
12. CHANGELOG multi-writer one-block-per-type rule (carried)
13. release.sh sanctioned unpushed-tag re-cut rule (carried)
14. VERSION-SURFACES post-v0.3.0 refresh (carried)
15. T15 park lot ×5 (carried)
16. cqrs-lint advisory gate (concurrent session's row, carried)
17. Journal-drift audit (concurrent session's row, carried)
18. Release retro fold into docs/release/ for v0.3.1 prep (carried)
19. Confirm dependabot picks up v0.3.0 pins (carried)
20. Post-incident: sweep OTHER public sibling repos for the same
    evaluator name (I only cleaned THIS repo — the name may appear in
    LarsArtmann/go-taskqueue forks/caches and any other public repos the
    prior sessions touched; one org-wide `gh`/grep sweep would settle it)

## g) Questions I cannot answer myself

1. **Approve the proxy-removal request + history rewrite pair?**
   (Recommendation on record: yes to the proxy request; rewrite only
   together with it. Both are irreversible-ish, owner-only actions —
   the proxy request needs the repo owner to send it.)
2. **Is the evaluator engagement paused?** The reply draft is neutral
   now — send it as planned, wait for their sensitivity read, or drop
   the loop entirely?
3. **Where should the confidentiality rule live** — AGENTS.md only
   (this repo), or also patched into the global github-voice /
   verify-before-filing skills so every repo inherits the gate?

## Skill question coverage (brutal-self-review set)

Forgot: the confidentiality checkpoint at THREE moments (d3) + archived
layer in the first sweep (d2). Stupid-anyway: celebrating an outbound
artifact as DONE without asking who can read it. Better: e1-e4. Still
improve: f1-f5, f20. Lied: no — every claim above re-verified this
window (deletion resolution, origin grep, archived grep, commit-body
disproof, gates). Ghost systems: none. Split brains: none new (the
neutralized refs replaced dead links). Removed-useful: no. Scope creep:
none (remediation stayed inside the named surfaces). Tests: N/A this
window (docs + gh operations only); the coverage gap that matters is
process, not code — see e1.

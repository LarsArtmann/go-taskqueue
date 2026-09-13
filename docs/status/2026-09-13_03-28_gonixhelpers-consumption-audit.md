# go-nix-helpers consumption audit — status report

**Date:** 2026-09-13 03:28 CEST
**Session type:** interactive owner session (not a pool task — no Task-Queue-ID, no `TQ_RESULT`, no code changes)
**Scope:** ONLY this session's work: the owner question "Are we using `/home/lars/projects/go-nix-helpers/`?" and this close-out. No unrelated research performed.

---

## What the session was

One question, answered with a three-source verification chain: does go-taskqueue (and the wider fleet) consume go-nix-helpers, and does the build consume the **local checkout** at `~/projects/go-nix-helpers` or the **GitHub-pinned flake input**?

## The answer delivered (the deliverable)

| Question                | Verdict                          | Evidence                                                                                                                                                                                                                                                                                              |
| ----------------------- | -------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Dependency at all?      | **YES**                          | `flake.nix:11-15` input `github:LarsArtmann/go-nix-helpers/master` (HTTPS — keyless CI runners can't fetch git+ssh); `flake.nix:24` imports `flakeModules.go-standard`                                                                                                                                |
| What it provides        | The whole Go flake scaffolding   | `go-standard.*` option surface (pname/version/vendorHash/subPackages/enableTempl/extraBuildAttrs/shellExtraEnv/devShellExtraPackages, `flake.nix:33-78`) + derived packages/apps/fmt/lint/devShell; go-taskqueue only `mkForce`-overrides `apps.test` for the multi-module loop (`flake.nix:318-340`) |
| Local path consumed?    | **NO**                           | `nix flake metadata . --json` resolves the node as `type: github`, rev `16c3184`; `nix registry list` + `~/.config/nix/registry.json` contain no override for it                                                                                                                                      |
| Local == pin right now? | **YES (coincidental)**           | helpers HEAD = `16c3184262c55377aba2126dc20e028637f58aa0` = locked rev; tree proven clean untruncated afterwards (porcelain=0), 0 unpushed; pin `lastModified` 1789008741 = **2026-09-10 04:52 CEST = the helpers master tip**, so the pin is fresh and nothing needs updating                        |
| Fleet usage             | **~40 consumer flakes + itself** | `rg -l 'go-nix-helpers' ~/projects/*/flake.nix` → 41 files (SystemNix, CV, PapDashboard, bank-sync, go-cqrs-lite, overview, …)                                                                                                                                                                        |

Bottom line given to the owner: yes it's a hard dependency, but via the GitHub-pinned flake input; the local checkout only matters when it is pushed and the consumer lock is updated.

---

## a) FULLY DONE

1. **Consumption map** of go-taskqueue → go-nix-helpers: input URL + HTTPS rationale + `nixpkgs.follows` (flake.nix:11-15), module import (flake.nix:24), config surface (flake.nix:33-78), the single `apps.test` override (flake.nix:312-340).
2. **Pin verification**: flake.lock locked node read + `nix flake metadata --json` cross-check; rev, lastModified (converted to human time during close-out), resolved source type all first-hand.
3. **Local-vs-pin equality**: helpers `git rev-parse HEAD` == locked rev; sync state and cleanliness established (untruncated re-check: 0 dirty, 0 unpushed).
4. **No-override proof**: both the system and user nix registries checked — nothing redirects `github:LarsArtmann/go-nix-helpers` to a local path.
5. **Fleet survey**: 41 matching flakes enumerated (40 consumers + the helpers repo itself).
6. **Verdict delivered** with file:line citations, correct on every claim.
7. **Close-out hygiene** (late, see §d): CONTRIBUTING.md read (its binding gates for this report: check-doc-refs, status index — both honored); helpers cleanliness re-proven untruncated; AGENTS.md memory note added (`Relation to other projects`, end of file); this report indexed.

## b) PARTIALLY DONE

1. **Fleet survey depth**: 41 files matched, zero classified by consumption mode (`flakeModules.go-standard` vs `mkPreparedSource` vs `mkGoFlake` vs `nixosModules` vs templates). A one-line `rg` per repo would have done it; skipped as out of the immediate question's scope.
2. **Feature surface described from the consumer side only**: I never read the helpers repo's own README, AGENTS.md, or `docs/flake-standard.md` — what `go-standard` authoritatively provides is inferred from go-taskqueue's usage, not from the provider's docs.
3. **"Byte-identical" claimed from rev equality, not from a hash**: HEAD equality + clean status implies identical content by git semantics, but I never compared the local tree's hash against the lock's `narHash`. The claim is correct; its evidence chain is one link short.
4. **Pin freshness computed after the answer shipped**: the delivered verdict didn't state that the pin (2026-09-10 04:52) is the helpers master tip — "3 days old AND current" changes the interpretation, and I only computed it while preparing this report.
5. **flake.lock twin `flake-parts` node noticed, not analyzed**: the lock carries `flake-parts` twice (root + helpers' own) at the same rev — harmless duplication because only `nixpkgs` is followed, but I flagged it without quantifying or resolving it.
6. **Memory protocol honored at close-out, not at discovery**: the AGENTS.md note is in now, but the global config mandates writing at the moment of discovery.

## c) NOT STARTED

Identified during the session, deliberately not begun (all §f seeds):

- **Split-brain audit inside go-nix-helpers**: it carries `mkGoFlake.nix`, `modules/go-standard.nix`, `flakeModules.go-standard`, AND `templates/go-standard/` — four surfaces that may overlap (its 2026-06-23 status report documents a "mkGoFlake extraction"; whether mkGoFlake is now a wrapper or a parallel entry point is unverified).
- **Consumer inventory/classification doc** for the fleet.
- **Local-checkout testing recipe** (`--override-input`) — nothing documents how to validate a helpers change against a consumer without push.
- **Pin-skew tooling** across the 40 consumers.
- **`nix build` / `nix flake check` of go-taskqueue against this pin** — this session verified metadata only, never built.

## d) TOTALLY FUCKED UP

Honest grading: **nothing catastrophic** — read-only session, zero code touched, every claim in the delivered answer verified and true. But three sin-class violations, two of them documented recurrences I repeated anyway:

1. **CONTRIBUTING.md was not read at turn 1** — the index's most-repeated sin (rows 03-20, 02-58, 02-47, 02-31, 02-19 document the 3rd-7th consecutive occurrences); this is the 8th. A read-only Q&A happened to have no binding gates — that is luck, not discipline. Read at close-out.
2. **Session-start ritual skipped**: no `git log`/`git status`/`git stash list` before investigating. Discovered only at close-out that **master is ahead 14 unpushed commits** after a busy concurrent-agent night — context that should have framed the session from minute one.
3. **Verification by truncated read**: the helpers cleanliness check ran `git status -sb | head -5` — a >4-file dirty tree would have been reported "clean" from truncated output. The masked-capture genus from tonight's index rows, in miniature. It did not bite (re-proven: porcelain=0), but the _pattern_ is the sin, not the outcome.

## e) WHAT WE SHOULD IMPROVE

1. **Mechanical turn-1 checklist** (CONTRIBUTING.md + git log/status/stash) that fires before ANY task, interactive or pool-dispatched — the interactive-session loophole is why the sin streak hit 8.
2. **Never truncate verification output**: claims of "clean/synced/green" get full, untruncated evidence (`porcelain | wc -l`, exit codes read raw).
3. **Prove equality by hash when the word "identical" is used**: `narHash` comparison, not rev-equality inference.
4. **Authoritative-source habit**: when answering "what does X provide", read X's own docs, not only its consumers.
5. **Ship freshness metadata with pin facts**: a rev without a date is half an answer.
6. **Write memory at discovery** — the AGENTS.md protocol exists precisely for the "I'll remember" anti-pattern I then committed.

## f) Up to 50 things we should get done next

**BRAINSTORM, not a commitment list** — per the status-report skill, N>25 is ROADMAP fuel; docs-health HARVEST must apply extra routing rigor. Items 1-8 are near-term; the rest fuel.

**Near-term**

1. Owner: push the 14-commit unpushed master stack (every recent index row gates on it; nothing else can be CI-proven until it ships).
2. go-nix-helpers split-brain audit: `mkGoFlake.nix` vs `flakeModules.go-standard` vs `templates/go-standard/` vs `modules/go-standard.nix` — one canonical entry point or four? Read helpers' 2026-06-23 mkGoFlake-extraction report first.
3. Classify all 40 consumer flakes by consumption mode (flakeModules.go-standard / mkPreparedSource / mkGoFlake / nixosModules) and pin rev — one jq loop over the fleet's flake.locks; publish as `consumers.md` in the helpers repo (its `docs/consumer-audit-checklist.md` from the 2026-08-10 fleet audit is the precedent — execute its checklist for the 2026-09-10→13 window instead of reinventing).
4. Document the local-checkout validation loop in helpers' `docs/migration-guide.md`: `nix build --override-input go-nix-helpers <path>` recipe so helpers changes are testable against consumers WITHOUT push.
5. Upstream fix candidates inside go-standard so 40 consumers stop carrying workarounds: (a) drop `x86_64-darwin` from the default systems list (go-taskqueue pins 3 systems at flake.nix:43-50 because of it); (b) a first-class `goExperiment` option instead of every consumer stuffing `extraBuildAttrs.env.GOEXPERIMENT`; count override-frequency across the fleet first — override-frequency = missing-option signal.
6. Prove pin equality by hash where it matters: compare local helpers tree hash against the lock's `narHash` once, so AGENTS.md's "coincidental freshness" line rests on a hash, not an inference.
7. Read helpers' own README/AGENTS.md/`docs/flake-standard.md` and reconcile go-taskqueue's `go-standard` block against the documented standard (deprecated fields? vendorHash interface still current under nixpkgs 26.11?).
8. Run helpers' own gates once before further fleet reliance: its `test.nix` / `test-pure-functions.nix` / `test-module.nix` and CI — this session's consumers trust it unverified.

**Pin/lifecycle policy**

9. Ruling (→ §g1): master-tracking vs tagged releases for go-nix-helpers consumers; if tags, cut one and migrate the fleet gradually.
10. `nix flake update go-nix-helpers` cadence or event trigger (currently ad-hoc; pin happens to be the master tip).
11. Advisory CI lint: warn when a consumer's locked helpers rev is behind helpers' origin/master by > N days (`nix flake metadata --json | jq`) — offline-safe, CI-side only (a pre-push gate would false-alarm offline).
12. Diff helpers' CHANGELOG on every future consumer-side update (process note; today lock == tip so no diff exists).
13. Sweep the fleet for URL drift (`git+ssh` vs `github:` HTTPS for the helpers input) — one `rg` rule in the fleet audit; go-taskqueue's HTTPS rationale comment (flake.nix:12) should be the fleet norm.

**Lock/build hygiene (go-taskqueue)**

14. Evaluate deduping the twin `flake-parts` lock nodes by adding `flake-parts.follows` on the helpers input — ONLY after re-reading the bank-sync FOD-mismatch ruling (planning doc round-8 B1: `nixpkgs.follows` chains through helpers were the trap; establish whether flake-parts follows is the same class).
15. Run `nix build && nix flake check` against the current pin (metadata-only session; never built).
16. Mention the helpers input in README's build/docs section — dependency provenance is currently invisible to readers.
17. Record the B1 planning row's claim (flake.nix:276-280 trap) against current SystemNix reality — planning-doc truth pass.

**Helpers repo docs/health**

18. Helpers docs-health pass: newest status report is 2026-09-07 — six days of fleet evolution unannotated.
19. Harvest helpers' TODO_LIST.md (unread this session — flagged, not read) into its own backlog; same for FEATURES/ROADMAP.
20. Check `docs/man/go-standard.5` + `mkPreparedSource.5` against the actual option surfaces (man-page drift).
21. Verify `templates/` still build (template rot; `test-assets/mock-*` fixtures exist — confirm CI actually exercises them, esp. the templ-committed/templ-missing pair).
22. Confirm helpers CI eval-tests a mock CONSUMER of `flakeModules.go-standard` — today consumers learn breakage at their own build time; a mock-consumer eval in helpers CI catches it upstream (item 5's data feeds this).
23. Confirm helpers AGENTS.md documents the pinned-rev consumption contract its consumers rely on.
24. `docs/reviews/2026-06-19_full-code-review.html` — three months old; is its findings list resolved or rotting?

**Process/memory (this repo)**

25. Encode the turn-1 checklist as an AGENTS.md ritual line covering INTERACTIVE sessions too (all 8 CONTRIBUTING-miss occurrences were dispatch/interactive, none pool-gated — the gap is structural).
26. Add "pin facts need freshness metadata" to the review checklist (one line, no new script).
27. Naming-convention ruling: interactive-session reports have no task-id in the filename — confirm `2026-09-13_03-28_gonixhelpers-consumption-audit.md` (this report) is the accepted pattern vs task-id naming.
28. Fleet-wide: forbid local-absolute-path inputs to go-nix-helpers in committed flakes (would break every other machine/CI) — a one-`rg` guard if the fleet ever grows one.
29. Cross-check which consumers carry the keyless-CI HTTPS comment and which silently use `github:` — comment parity is free documentation.
30. Quantify the store-cost of the twin flake-parts node before deciding item 14 is worth touching the lock at all.

**Exploration/ideas (ROADMAP fuel)**

31. `nix flake update` automation: a scheduled job opening a PR with the lock bump + CHANGELOG diff in the body.
32. Fleet rev-uniformity enforcement: a check that all consumer locks pin the SAME helpers rev (vs documented drift — see §g3).
33. Helpers: expose `nixosModules` conventions if consumers (go-taskqueue flake.nix:31, SystemNix) are re-inventing NixOS module patterns per repo.
34. Evaluate helpers' `pure-functions.nix` for consumers that hand-roll similar option plumbing (YAGNI check first).
35. Upstream go-taskqueue's multi-module `apps.test` loop as a go-standard option (`subModuleTest = true`?) — likely duplicated across the fleet.
36. Add helpers consumer smoke: `nix flake check` one real consumer flake in its CI matrix.
37. Verify zero Go-level dependency on go-nix-helpers is a documented invariant (it is a Nix-only dep today; a guard note prevents accidental require creep).
38. Document in go-taskqueue AGENTS.md Known Issues: none found this session — deliberately no entry (keep the section signal, not noise).
39. Fleet audit script: single command emitting consumer | mode | rev | lastModified table (items 3+12+13 combined).
40. Consider `npins`/vendored-lock alternatives ONLY if flake.lock pinning proves operationally painful — otherwise don't re-litigate the input mechanism.
41. Helpers: seed-corpus style growth for its `test-assets/` (each consumer quirk becomes a fixture).
42. Post-item-4: add an integration test that builds a mock consumer against a LOCAL helpers path (proves the override-input loop actually works, not just documented).
43. Decision record: whether `apps.test` mkForce + `shellExtraEnv`/`devShellExtraPackages` are the intended public extension surface or accidental API.
44. Check whether any consumer still references the pre-extraction `mkGoFlake` path (dead-surface detection after the split-brain audit).
45. Helpers status reports: archive fully-resolved ones per its own docs-health convention (2026-06/07/08 backlog visible in its docs/status/).
46. Align the fleet's GOEXPERIMENT story once item 5b lands: audit which repos still need the env-var workaround.
47. Consider a `go-nix-helpers` release notes digest the fleet consumers can subscribe to (currently CHANGELOG-only).
48. Add the helpers input + lock rev to go-taskqueue's `tq doctor`-style output? (probably NO — scope creep; record the rejection).
49. Cross-train: ensure the nix-private-go-repos skill mentions the master-pinning + override-input loop (it's the closest skill; keep skills and repo docs non-divergent).
50. Meta: schedule the next fleet-wide consumer audit (last: 2026-08-10; ~25 new/changed repos since, incl. go-taskqueue's own adoption).

## g) Questions I cannot figure out myself

1. **Pin policy**: Is tracking `master` (ref = master + flake.lock) the deliberate fleet policy for go-nix-helpers — vs cutting tags and tracking those? And is there an intended update cadence, or is "update when something forces it" acceptable?
2. **Helpers-change validation loop**: When you (or an agent) edit go-nix-helpers, what is the intended way to prove a consumer still builds BEFORE pushing — commit→push→`nix flake update` per consumer, or should item 4's `--override-input` recipe become the documented loop?
3. **Skew tolerance**: Must all ~40 consumers pin the SAME go-nix-helpers rev at any point in time (audit + enforce), or is per-repo drift acceptable as long as each consumer's own gate is green?

---

_Point-in-time snapshot — re-verify claims against current state before relying on them. The three-source verification chain (flake.nix, flake.lock, live git/registry/metadata checks) was re-run in full during this session; the §b partial items mark where the chain is thinner than it looks._

# Status Reports — Archive Manifest

Fully-resolved reports (every forward-looking item struck inline with a
verdict). One line per file for every bulk archive sweep; single-file
archives use the commit message as the manifest. Classification keys:
ARCHIVE (all items resolved at annotation time), verdict forms
`done at <sha>` / `routed — <where>` / `subsumed — <row>` / `Won't
implement — <reason>`.

## 2026-10-07 archive sweep batch 1 (four files)

- 2026-09-10_03-05_httputil-assessment-and-permissions-policy.md → ARCHIVE: §f3/§g3 routed (TODO_LIST header-map row; ROADMAP cookie question), rest struck in the 09-14 sweep era
- 2026-09-11_23-16_docs-health-full-sweep-annotate-archive-harvest.md → ARCHIVE: zero bare items at sweep time (9 strikethroughs from its own pass)
- 2026-09-14_02-38_task-000001a09d5933c12b269647d86935d688c1.md → ARCHIVE: §b1/§b5 routed (TODO_LIST rewrite-guard + §d-citation rows), rest struck earlier
- 2026-09-14_17-16_batched-harvest-agent-power.md → ARCHIVE: §f1 Won't implement (rejected alternative), §f2 subsumed (batch-default row), §f3 routed (status batch members row)

## 2026-10-07 archive sweep batch 2 (2026-09-15 set, file 1 of 4)

- 2026-09-15_00-12_task-000001a0a1f149ed96eb7fa452a984dd257c.md → ARCHIVE: all 53 forward items resolved (§f1 done at c9a31765 — the fail→complete attempts pin minted BY this sweep; §f22 routed to the forensics-filters row; §f12/§f46 subsumed by the upstream-enrich and fold-marker rows; §g1-3 answered: Attempts vocab entry + green status test + M4 fold machinery)

## 2026-10-12 docs-health sweep 6 (23 files; owner-directed per-item strike pass)

- 2026-09-08 (none archived this sweep; 04-19 stays KEEP-OPEN — BerryBig/repo-routing asks open)
- 2026-09-14_12-39_claiming-arc-brutal-status.md → ARCHIVE: queue/ arc ruled+executed (ADR-0019 S1→S4, O4/O12); 28 strikes
- 2026-09-14_14-17_docs-health-archive-sweep-self-review-and-status.md → ARCHIVE: every minted row landed/shipped/ruled (O4/O5/O11/O13/O14/O16/O17); 60 strikes
- 2026-09-15_01-01_task-000001a0a223a4728f43bb54d054ba8fc987.md → ARCHIVE: help-text window; shipped/routed (rows 648/228/117/36/131/199/392/416)/ruled; 58 strikes
- 2026-09-18_00-14_task-000001a0b16663570cfda9c08e1f4915f2a5.md → ARCHIVE: session-usage window; rows 232/496/511/661 + O1/O4/O5/O6; 29 strikes
- 2026-09-18_01-47_task-000001a0b1c1f0d7891f4cb992fa6b954597.md → ARCHIVE: pin routed (row 496), ritual shipped, deferral ruled; 16 strikes
- 2026-09-18_02-25_task-000001a0b1e1fb89071038b7b9e57614b3e5.md → ARCHIVE: gosec-window; rows 123/229/652 + O14; 17 strikes
- 2026-09-19_00-10_task-000001a0b68a0a70be1e9eb8ecdbb32cff59.md → ARCHIVE: wrapper legs shipped into session-close.sh; rows 122/123/229/240/496/512/552; 30 strikes
- 2026-09-19_00-49_task-000001a0b6aeadf36b9a84212007800f3bf1.md → ARCHIVE: README/FEATURES nits routed (row 661), wrapper smoke shipped; 22 strikes
- 2026-09-19_01-22_task-000001a0b6ceb2a69a870f8855c3b7f7536b.md → ARCHIVE: positive wrapper leg shipped, exit contracts routed (row 496); 22 strikes
- 2026-09-19_02-38_task-000001a0b70ec95eddf878d93e9fdf2bbd35.md → ARCHIVE: check-gosec hardening family shipped; rows 123/124/229/547 + O5; 23 strikes
- 2026-09-20_01-36_task-000001a0bc02cdfc623998c6e61b0c16b94f.md → ARCHIVE: rows 653/699/117 + O1/O5; session-start probe + guard-wiring shipped; 17 strikes
- 2026-09-28_00-12_task-000001a0e4e5a90d36d988fe0bf57eb32de2.md → ARCHIVE: badge code + verify-battery + env scrub shipped; rows 297/312/324/348/352/499/506/553/680; 28 strikes
- 2026-09-29_22-37_task-000001a0eed25c041f1b181c395800000000.md → ARCHIVE: security-batch asks live in rows 378-383; 15 strikes
- 2026-09-29_23-40_task-000001a0ef17063a91104056774a00000000.md → ARCHIVE: probe/README/mvdan/heal-tooling shipped; rows 220/364/395/438/504/664/680; 23 strikes
- 2026-09-29_23-57_task-000001a0ef24c1fed92f0294f9ed00000000.md → ARCHIVE: both NEW rows shipped (commit-task.sh, probe pin); rows 220/666; 17 strikes
- 2026-09-30_04-34_archive-00-43-report-window.md → ARCHIVE: ordering/ritual shipped (AGENTS:193-196, commit-task.sh); rows 220/239/299/312/383/438 + O1/O5/O15; 18 strikes
- 2026-09-30_08-43_task-000001a0f10a0062ddb3ce70f71700000000.md → ARCHIVE: rows 367/508/664; AGENTS:196 pointer + reminders shipped; 13 strikes
- 2026-09-30_08-57_task-000001a0f117b845c364ec6f8cbe00000000.md → ARCHIVE: cancelled-neutral + ScopedGofmtStage shipped; rows 156/216/368/504/508/664; 21 strikes
- docs/planning/2026-09-12_worktree-per-agent-design.md → EXECUTED: O11 merge-policy ruling; 9 questions mirrored in ROADMAP; implementation rows 143/437; 13 strikes
- docs/planning/2026-09-13_verification-claims-guidance.md → ARCHIVE: narrative-only policy deliverable, zero numbered items (disposition note is the resolution)
- docs/planning/2026-09-15_ci-topology-one-pager.md → ARCHIVE: rows 590/591 + AGENTS:255 toolchain entry; parent row closed per O9; 3 strikes
- docs/feedback/2026-09-13_helpcentre-reply-draft.md → ARCHIVE: all 5 asks struck shipped (09-30 sweep); ADR-0017 cured the go-install limitation
- docs/feedback/new/2026-09-13_external-adoption-blocked-by-internal-paths.md → ARCHIVE: all 5 asks struck shipped (09-30 sweep); Offer section is external prose

## 2026-11-09 libs-fleet M19 digest ritual (5 files; 2026-10-09 release-hardening done-prompt passes 12–16)

- 2026-10-09_01-45_release-hardening-done-prompt-window-twelfth-pass.md → ARCHIVE: verify-only DONE-ON-ARRIVAL layer (pass 12 of the 000001a11d8b… re-dispatch chain; zero appends, no new §g)
- 2026-10-09_01-47_release-hardening-done-prompt-window-thirteenth-pass.md → ARCHIVE: verify-only DONE-ON-ARRIVAL layer (pass 13; zero appends, no new §g)
- 2026-10-09_01-52_release-hardening-done-prompt-window-fourteenth-pass.md → ARCHIVE: verify-only DONE-ON-ARRIVAL layer (pass 14; zero appends, no new §g)
- 2026-10-09_02-18_release-hardening-done-prompt-window-fifteenth-pass.md → ARCHIVE: verify-only DONE-ON-ARRIVAL layer (pass 15; zero appends, no new §g)
- 2026-10-09_02-40_release-hardening-done-prompt-window-sixteenth-pass.md → ARCHIVE: verify-only DONE-ON-ARRIVAL layer (pass 16; zero appends, no new §g; canonical sixth pass 2026-10-09_00-40 stays live)
- 2026-09-08_04-19_agent-pool-e2e-self-review.md → ARCHIVE: monthly digest sweep 2026-11-12 (point-in-time snapshot; open items in TODO_LIST)
- 2026-09-10_09-44_red-master-repaired-closeout-proven-self-review.md → ARCHIVE: monthly digest sweep 2026-11-12 (point-in-time snapshot; open items in TODO_LIST)
- 2026-09-12_01-09_round12-ci-red-fix-t14-t17-release-lint-hardening.md → ARCHIVE: monthly digest sweep 2026-11-12 (point-in-time snapshot; open items in TODO_LIST)
- 2026-09-12_01-47_per-project-ui-question.md → ARCHIVE: monthly digest sweep 2026-11-12 (point-in-time snapshot; open items in TODO_LIST)
- 2026-09-12_02-12_review-id-stamp-fix-and-crush-agent-optimization.md → ARCHIVE: monthly digest sweep 2026-11-12 (point-in-time snapshot; open items in TODO_LIST)
- 2026-09-12_14-03_priority-projection-design-research.md → ARCHIVE: monthly digest sweep 2026-11-12 (point-in-time snapshot; open items in TODO_LIST)
- 2026-09-13_09-14_brutal-self-review-v030-release.md → ARCHIVE: monthly digest sweep 2026-11-12 (point-in-time snapshot; open items in TODO_LIST)
- 2026-09-13_10-27_release-tail-closeout-pareto-m4m5m6m8.md → ARCHIVE: monthly digest sweep 2026-11-12 (point-in-time snapshot; open items in TODO_LIST)
- 2026-09-13_10-33_confidentiality-incident-retro.md → ARCHIVE: monthly digest sweep 2026-11-12 (point-in-time snapshot; open items in TODO_LIST)
- 2026-09-14_11-41_templ-components-deep-dive-webui-ux-audit.md → ARCHIVE: monthly digest sweep 2026-11-12 (point-in-time snapshot; open items in TODO_LIST)
- 2026-09-14_12-23_crush-releases-integration-assessment.md → ARCHIVE: monthly digest sweep 2026-11-12 (point-in-time snapshot; open items in TODO_LIST)
- 2026-09-14_12-45_cmd-tq-installable-module.md → ARCHIVE: monthly digest sweep 2026-11-12 (point-in-time snapshot; open items in TODO_LIST)
- 2026-09-14_12-52_derived-outcomes-verdict-channel.md → ARCHIVE: monthly digest sweep 2026-11-12 (point-in-time snapshot; open items in TODO_LIST)
- 2026-09-14_17-15_todo-purge-and-per-item-strikethrough-pass.md → ARCHIVE: monthly digest sweep 2026-11-12 (point-in-time snapshot; open items in TODO_LIST)
- 2026-09-14_17-18_batched-harvest-closeout-self-review.md → ARCHIVE: monthly digest sweep 2026-11-12 (point-in-time snapshot; open items in TODO_LIST)
- 2026-09-14_18-14_ui-overhaul-self-review.md → ARCHIVE: monthly digest sweep 2026-11-12 (point-in-time snapshot; open items in TODO_LIST)
- 2026-09-16_12-11_instrument-pass-redesign.md → ARCHIVE: monthly digest sweep 2026-11-12 (point-in-time snapshot; open items in TODO_LIST)
- 2026-09-16_18-12_csp-noindex-baseline-instrument-shots.md → ARCHIVE: monthly digest sweep 2026-11-12 (point-in-time snapshot; open items in TODO_LIST)
- 2026-09-18_05-45_papdashboard-questions-arc-gates-closeout.md → ARCHIVE: monthly digest sweep 2026-11-12 (point-in-time snapshot; open items in TODO_LIST)
- 2026-09-21_13-31_webui-design-pass.md → ARCHIVE: monthly digest sweep 2026-11-12 (point-in-time snapshot; open items in TODO_LIST)
- 2026-09-26_20-01_session-status-and-self-review.md → ARCHIVE: monthly digest sweep 2026-11-12 (point-in-time snapshot; open items in TODO_LIST)
- 2026-09-27_06-41_artdupl-t2-sweep-self-review.md → ARCHIVE: monthly digest sweep 2026-11-12 (point-in-time snapshot; open items in TODO_LIST)
- 2026-09-27_15-29_dedup-sweep-t5-type-aware-and-mirror-gate-blind-spot.md → ARCHIVE: monthly digest sweep 2026-11-12 (point-in-time snapshot; open items in TODO_LIST)
- 2026-09-28_18-18_art-dupl-t2-dedup-sweep-verdict.md → ARCHIVE: monthly digest sweep 2026-11-12 (point-in-time snapshot; open items in TODO_LIST)
- 2026-09-29_23-37_agent-autonomy-rows-and-crush-channels-deep-review.md → ARCHIVE: monthly digest sweep 2026-11-12 (point-in-time snapshot; open items in TODO_LIST)
- 2026-09-30_01-24_statix-w20-flake-fixes-and-nix-hash-heal.md → ARCHIVE: monthly digest sweep 2026-11-12 (point-in-time snapshot; open items in TODO_LIST)
- 2026-09-30_04-32_log-path-xdg-state-session-status.md → ARCHIVE: monthly digest sweep 2026-11-12 (point-in-time snapshot; open items in TODO_LIST)
- 2026-09-30_09-15_art-dupl-t3-suggest-generics-sweep.md → ARCHIVE: monthly digest sweep 2026-11-12 (point-in-time snapshot; open items in TODO_LIST)

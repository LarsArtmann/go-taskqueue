# Daemon doc-gate wiring ruling artifact (2026-10-07 04-25)

**Question:** where does `scripts/check-daemon-sweep-docs.sh` run so that
footer-less daemon sweeps over doc-gated files stop costing multi-hour red
windows (~18h doc-refs red; ~26h e2e red — both from sweeps bypassing
every hook)?

## Options considered

| Wiring | Catches sweep… | Verdict |
| --- | --- | --- |
| Git `post-commit` hook (.githooks) | instantly | **Unusable**: the daemon commits with hooks bypassed by design (AGENTS: "daemon bypasses hooks"); it would gate only human/agent commits, which already run the gates |
| ci-local start-of-run | before a push | **Shipped** (below): every pre-push run now pays ~1s when clean, runs the cheap doc gates when a gated sweep is in `origin/master..HEAD` |
| Host cron / systemd timer (evo-x2) | within minutes, always | **Proposed to owner** (see request): `*/5` systemd timer or cron over the go-taskqueue checkout running `scripts/check-daemon-sweep-docs.sh --since origin/master`; needs the host-side unit the agent cannot install (sudo) |
| CI job on master | at push time | Covered by the ci-local wiring once pushed (CI runs the same gates); adds nothing for the mid-window red |

## Shipped in this change

1. `scripts/check-daemon-sweep-docs.sh` — detector + gate runner, self-tested
   (`DAEMON_DOC_GATE_SELF_TEST=1`): daemon subject shape (regex mirror of
   the queue's `daemonCommitSubject`, row 243) × footer-less × doc-gated
   path set → run check-doc-refs + check-todo-list + check-status-index.
2. ci-local early step (after the self-test block): catches sweeps that
   landed mid-window before any expensive step pays for a red tree.

## Owner request (host side, sudo)

```ini
# /etc/systemd/system/tq-daemon-doc-gate.timer (proposal)
[Timer]
OnCalendar=*:0/5
Unit=tq-daemon-doc-gate.service

# service: WorkingDirectory=<go-taskqueue checkout>, ExecStart=scripts/check-daemon-sweep-docs.sh
```

Failure routing (email/PapDashboard alert) rides the owner's existing
timer-alert convention. Until the owner installs it, the ci-local wiring
is the only automated catch — pushes remain the last line of defense.

## Row closures

- TODO row "Daemon post-sweep cheap doc/config gate" (02-02 §g3): the
  script half is DONE; the owner-policy call is narrowed to "install the
  timer unit" (this doc).
- TODO row 206 (fold-marker.sh): DONE — script shipped with rails
  (unpushed-only, footer-less-only, exactly-mine file claim).
- TODO row "commit-task sweep detection" (02-35 §f): DONE —
  commit-task.sh detects the same-path daemon commit and prints the two
  sanctioned heals (heal-daemon-sweep / fold-marker).

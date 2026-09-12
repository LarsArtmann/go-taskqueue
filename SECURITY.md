# Security Policy

## Trust model

go-taskqueue is a local, single-node system: the operator who starts
`tq worker` or `tq agent-pool` is the owner of the machine it runs on. There
is no multi-tenant boundary — everyone who can write the database file or
start processes on the host is inside the trust boundary.

## What is dangerous here, honestly

1. **The `sh` executor runs arbitrary shell commands.** Anyone who can
   enqueue into your database can execute any command as the worker's user.
   The database file (`$TQ_DB` or `./tasks.db`) is therefore the real
   capability grant: protect it like a shell prompt (filesystem permissions,
   full-disk encryption, no shared writable mounts).
2. **Agent autonomy's trust root is the filesystem** (ADR-0002). `--yolo`
   only _requests_ autonomy; the actual grant is per-repo — a `.crushrc` in
   the repo (or a user-global crush config) that lists `bash` lets an agent
   run unsandboxed shell commands **in that repo's checkout**. Every repo
   under `--projects-dir` containing a TODO_LIST.md is a candidate for
   harvest; any of them can enqueue billable work into the shared queue.
   Review committed `.crushrc` files like you review CI workflows.
3. **The blast radius of one agent task** is: arbitrary commands in the repo
   directory, the model/API spend of one run (bounded by `--task-timeout`),
   and a git commit it makes (it is instructed never to push). Across a day
   the spend is bounded by `--daily-budget` / `--budget-cmd`, and the
   per-tick fan-out by `--max-per-tick`. These are damage caps, not
   sandboxing — they bound cost and concurrency, not file access.
4. **No network sandbox.** Agents and the `http` executor make outbound
   network calls. Verify commands (`.tq-verify`) also run with full user
   privileges — a malicious repo can lie about its verify command. Do not
   point `--projects-dir` at repositories you do not trust.
5. **Status agents mint future autonomous work.** The status executor's
   done-prompt agent appends `- [ ]` items to the repo's TODO_LIST.md, and
   the next harvest tick turns those into real agent tasks — a status run
   can legally enqueue up to ~50 new billable tasks per report (its prompt
   hard-caps the count and confines writes to the report, TODO_LIST.md, and
   its own commit; the repo verify gate must still pass). The loop's growth
   is bounded by the same damage caps as everything else
   (`--daily-budget` / `--max-per-tick`), which apply to EVERY enqueue —
   harvested and status-minted alike. If the loop misbehaves: stop the
   pool, review `tq dlq` and the latest `docs/status/*` report, cancel or
   rescue. Whether status reports themselves should be reviewed is a
   deliberate open trust-policy question, not an oversight.

6. **The prioritize scorer reads your repos, and its verdicts re-rank the
   queue.** `--prioritize` (default OFF) spawns a READ-ONLY scorer agent
   per repo holding unscored backlog items: it may read any file in the
   repo (same trust boundary as every agent turn) but its prompt forbids
   writes, and its only powers are cached scores and PENDING-task
   priority changes clamped to the backlog band — hot/machine priorities
   and marker items are structurally protected. Every batch mint counts
   against `--daily-budget` / `--budget-cmd` like any other enqueue; the
   dedup key (hash of the batch's key set) means an unchanged backlog
   never re-scores.

## UI-originated writes (`tq serve --allow-writes`)

The dashboard is read-only by construction (ADR-0003) EXCEPT for exactly
two admin routes, which exist only when `--allow-writes` / `$TQ_SERVE_WRITES=1`
is passed:

| Route                    | Effect                                                                                         | Blast radius                                             |
| ------------------------ | ---------------------------------------------------------------------------------------------- | -------------------------------------------------------- |
| `POST /task/{id}/cancel` | withdraw a pending task, or request a cooperative stop of a running one                        | one task withdrawn / stopped                             |
| `POST /task/{id}/rescue` | re-queue a dead-lettered task with fresh attempts (the agent may spend money running it again) | one task re-run → its model spend + a commit in its repo |

No other write exists: adding one requires the same flag + CSRF treatment
(repo guardrail, ADR-0003 amendment 2026-09-08). The routes never edit
payloads, facts, or the journal directly — they call the same store
methods as `tq cancel` / `tq dlq --rescue`.

**Defense layers, in order:**

1. **Off by default** — without the flag the routes are not registered
   (404, not 403).
2. **Loopback vs token matrix:**

   | Bind                              | Token                                     | Writes | Result                                                 |
   | --------------------------------- | ----------------------------------------- | ------ | ------------------------------------------------------ |
   | `127.0.0.1:port`                  | —                                         | off    | read-only dashboard                                    |
   | `127.0.0.1:port`                  | —                                         | on     | writes, CSRF-guarded                                   |
   | non-loopback / `:port` / hostname | **required** (refuses to start otherwise) | either | every route behind constant-time bearer/`?token=` auth |

3. **CSRF** — every write POST must carry the form field matching the
   browser's `tq_csrf` cookie (double-submit, constant-time compare,
   `HttpOnly`+`SameSite=Lax`, `Secure` on TLS). A cross-site form can make
   the browser SEND the cookie but cannot read it, so the field is
   unforgeable; CSP restricts same-site injection to server-rendered forms.
4. **Rate limit** — three failed CSRF tokens from one client IP lock that
   client out of BOTH write routes for 60 s (429, checked before CSRF);
   a successful write resets the strikes. Reads are never limited.

**Residual risks, honestly:** a compromised loopback process can CSRF-fetch
a page and then post valid writes (it is inside the trust boundary anyway);
the lockout is per-IP, so a distributed attacker is only slowed by the
128-bit CSRF token itself (which is the real barrier — the lockout is
hygiene, not the wall); writes can rescue a dead task whose re-run costs
one agent's spend.

## Data handling

- The journal (`tasks.db`) stores task payloads and prompts verbatim —
  including anything typed into them. Treat prompts as non-secret, or
  protect the file accordingly.
- The PapDashboard bridge posts fact metadata to `--alert-url`; the CQA
  bridge reads findings from `--cqa-url`. Both are plain HTTP by default —
  put them behind TLS if they leave the host. Alert auth uses a bearer-ish
  key passed via `--alert-api-key`.
- Completion facts record the agent session id and the verify output tail;
  verify tails can contain repository paths and test output.

## Hardening checklist for unattended pools

- [ ] `--daily-budget` (or `--budget-cmd`) and `--max-per-tick` set (this is
      ALSO the cap on the status loop's self-minted work)
- [ ] `--status-every` consciously chosen (0 = loop off) — each report can
      enqueue up to ~50 follow-up tasks
- [ ] `--project-exclusive` on every pool sharing a database
- [ ] `--repos` used instead of a broad `--projects-dir` where possible
- [ ] `tq serve`: `--allow-writes` only where an operator actually uses the
      admin buttons; non-loopback binds always carry `--auth-token`
- [ ] `.crushrc` files audited: only repos that need `bash` have it
- [ ] database file owned by the pool user, 0600, not on a shared mount
- [ ] bridges behind TLS when not on localhost

## Automated defense layers (CI)

Two advisory scanners run on every push (job-level `continue-on-error`;
triage before assuming baseline — see AGENTS.md):

- **govulncheck** — known-vulnerability scan over the root module and every
  sub-module (`govulncheck ./...` per module, GOWORK=off).
- **gosec** — static security findings over the same surface; the
  2026-09-10 triage found every finding a false positive or by-design
  (per-rule rationale in AGENTS.md's gosec baseline note).

## Reporting a vulnerability

Open a private GitHub security advisory on
[github.com/LarsArtmann/go-taskqueue](https://github.com/LarsArtmann/go-taskqueue/security/advisories/new)
rather than a public issue.

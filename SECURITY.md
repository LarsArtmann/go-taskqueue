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

## Production write API (`tq api`)

The HTTP API is the one surface DESIGNED for non-loopback exposure
(ADR-0008): it writes tasks, so the bearer token (`--auth-token`) is
mandatory on every bind and guards every route — there is no loopback
exemption and no unauthenticated read surface.

**Defense layers:**

1. **Mandatory token** — the API refuses to start without one; every
   request is checked with a constant-time compare (header or `?token=`).
2. **Auth lockout** — three failed auths from one client IP lock that
   client out of ALL routes for 60 s (429 + `Retry-After`, checked before
   the token compare); any successful auth resets the strikes. Same
   per-IP tradeoff as the dashboard's CSRF lockout: behind a NAT all
   clients share one key, and a distributed attacker is only slowed —
   the token entropy is the real barrier, the lockout is hygiene.
3. **Response headers** — every response carries
   `X-Content-Type-Options: nosniff`; authenticated responses carry
   `Cache-Control: no-store`. The API returns JSON only and renders
   nothing, so the dashboard's full CSP set does not apply here.

**Residual risks, honestly:** enqueue over plain HTTP sends the token in
cleartext — put the API behind TLS (or a tunnel) when it leaves the host;
the lockout window is per-IP, so one noisy misconfigured producer can
briefly lock out its NAT neighbors.

## Response headers, per surface

Every surface ships its headers from one reviewed site each
(`internal/webui` `securityHeaders`, `internal/httpapi` `guard`); the
matrix is the operator-visible summary. A new header, route, or surface
adds a row here AND its pinning test
(`TestSecurityHeadersOnEveryResponse` / `TestNosniffOnEveryResponse`)
in the same change.

| Header                    | `tq serve` (dashboard + `/health*`)                                                                                                                                                                                                                                                                             | `tq api`                                        |
| ------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------- |
| `Content-Security-Policy` | every route: nonce'd `default-src 'none'` (scripts same-origin + per-request nonce, `frame-ancestors 'none'`, `form-action 'none'`); `/health*` overrides to the health library's policy (`unsafe-eval` for the same-origin Datastar SDK, re-hardened with `base-uri 'none'` + `frame-ancestors`/`form-action`) | none — the API returns JSON and renders nothing |
| `X-Content-Type-Options`  | `nosniff` (every response)                                                                                                                                                                                                                                                                                      | `nosniff` (every response, including 401/429)   |
| `X-Frame-Options`         | `DENY`                                                                                                                                                                                                                                                                                                          | —                                               |
| `Referrer-Policy`         | `no-referrer`                                                                                                                                                                                                                                                                                                   | —                                               |
| `Permissions-Policy`      | camera, microphone, geolocation denied                                                                                                                                                                                                                                                                          | —                                               |
| `X-Robots-Tag`            | `noindex` on `/health*` (task pages carry a robots meta instead)                                                                                                                                                                                                                                                | —                                               |
| `Cache-Control`           | `public, max-age=86400` on the embedded health SDK bundle only; HTML and SSE unset                                                                                                                                                                                                                              | `no-store` on authenticated responses           |
| `WWW-Authenticate`        | `Bearer realm="tq dashboard"` on 401                                                                                                                                                                                                                                                                            | `Bearer realm="tq-api"` on 401                  |
| `Retry-After`             | 429 from the write-route CSRF lockout                                                                                                                                                                                                                                                                           | 429 from the bearer-auth lockout                |

Token presentation channels (how auth travels, not a header): both
surfaces accept `Authorization: Bearer` (scheme case-insensitive) and
`?token=` via the shared `internal/httpauth` primitives; the dashboard
adds a browser session-cookie channel on top. Comparison is constant-time
over SHA-256 digests on both surfaces, so the token's length never leaks
through timing.

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
- Questions (`tq ask`) carry AGENT-AUTHORED text toward PapDashboard
  (`task.question-asked` fact + the bridge forwarding). The redaction pass
  covers the fact error/detail fields, but the question body itself is
  model output — treat the PapDashboard endpoint as a disclosure boundary
  for whatever the agent chose to write, not as owner-authored prose.

Reads are not free. The alert/CQA bridge fetches and the PapDashboard
question forwarding are not passive: an outbound POST (`--alert-url`)
discloses fact metadata to a third party, and a `--cqa-url` read trusts
whatever that endpoint returns. Treat every bridge endpoint as an
untrusted boundary — it must never influence which task runs next, widen
an agent grant, or clear a deterministic finding. Content fetched from a
bridge is DATA (like repo/tool content), never control.

## Agent environment: injection vs redaction

Security at the agent boundary runs in BOTH directions. REDACTION
(default ON) scrubs secrets out of what agents EMIT — every output tail
passes the redaction table before it lands in a fact, a log, or a
dashboard. INJECTION bounds what agents RECEIVE: the agent process
inherits the pool environment minus an exact-key denylist (`TQ_DB` — the
queue's own journal path — today), so an autonomous shell cannot read or
mutate the live journal by shelling out to `tq`. The denylist is
deliberately narrow; the successor design (minted per-run allowlist
environments, flag-gated strict mode, managed HOME) is specified in
`docs/planning/2026-10-05_secret-injection-seam-design.md` and widens
only by owner ruling — pattern-matched key stripping would break the
provider access agents legitimately need.

## Learned checks may only narrow (harness rule)

The queue's automated judgements are of two kinds, and their authority
is not symmetric. A DETERMINISTIC check owns its verdict outright: the
verify gate's exit code, the redaction table (`internal/executor/redact.go`),
and the gate-artifact classifier (`executor.IsGateArtifactDeath`) decide by
code that does the same thing every run. A LEARNED check — an LLM
reviewer/autofixer, the AI priority scorer, a future risk tier — may only
TIGHTEN the deterministic admissible set, never widen it:

- A learned verdict may veto or reorder; it may not grant a capability,
  approve spend, or auto-dismiss a deterministic gate failure. If it did,
  a tricked judge would be a tricked lock — the exact failure class a
  deterministic gate exists to prevent.
- When deterministic and learned verdicts are combined, merge as
  `max(deterministic, learned)` with a union of flags: a learned finding
  may RAISE the alarm but must never LOWER or erase a deterministic one
  (defeating the judge cannot erase the tripwire), and the model is never
  told a deterministic finding was cleared.
- `IsGateArtifactDeath` auto-dismiss must stay deterministic for this
  reason; keep the classifier's inputs code-derivable.

The merge rule is code, not just prose. `executor.MergeFindings` in
`internal/executor/finding.go` combines a deterministic and a learned
finding as `max(deterministic, learned)` with a union of flags, and
`internal/executor/finding_test.go` pins that a learned "benign" verdict
cannot clear a deterministic one. `scripts/check-security-invariants.sh`
gates both this wording and the helper against drift.

This rule is adopted from the agent-harness invariant "a learned check may
narrow the deterministic admissible set; it must never widen it"
(`turnstonelabs/turnstone` `HYPOTHESIS.md`, gate-placement appendix;
comparison and citations in `docs/research/2026-10-09_turnstone-lessons.md`).

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

# `--reresolve-verify`: retire, or narrow?

**Created:** 2026-11-01 20:49 (CET). **Author:** undispatched agent session.
**Status:** design note + recommendation; the ruling is the owner's (Q2 of
`docs/planning/2026-11-01_20-17_SUPERB-okf-receipt-attester-pareto-plan.md`).
**Predecessors:** `docs/research/2026-11-01_okf-lessons.md` lessons 6/9/10;
`docs/planning/2026-11-01_20-17_SUPERB-okf-receipt-attester-pareto-plan.md` V1.

---

## 1. What the flag does today (traced, not assumed)

One flag, four hops, one decision point:

| Hop | Location |
| --- | --- |
| flag registration (worker + agent pool) | `cmd/tq/main.go:630`, `cmd/tq/agentpool.go:122` |
| into the executor | `cmd/tq/main.go:696`, `cmd/tq/agentpool.go:423`, `cmd/tq/main.go:1114` |
| the field + its documented contract | `internal/executor/agent.go:124-130` |
| the ONLY behavioural fork | `internal/executor/agent.go:399` → `runVerify` → `verifyFor` |

`verifyFor` (`internal/executor/agent.go`) is the single decision:

```go
// file > pin > detect   (flag OFF — the payload contract)
// file > detect         (flag ON  — the enqueue-time pin is dropped)
```

Nothing else in the codebase reads the flag. So the flag's entire footprint is:
**does `AgentPayload.Verify` participate in gate resolution, yes or no.**

## 2. Why it exists (the incident it answered)

The flag shipped in the 2026-09-27 `doctor --hygiene` window (commits
`d0e2a62` / `1da0ee5`, cited by
`docs/status/archived/2026-09-15_06-43_task-000001a0a2f63b92bdc1a0d85ef9935cb821.md`)
for report 09-39 §f3: **a verify contract that moved on kept firing the old
pinned command at tasks queued before the move.** The pin is written at
enqueue time by the harvester; the gate it names may have been superseded
hours later.

Its behaviour is pinned by
`TestReresolveVerifyClaimsNewGateAfterFlip` (executor module): flip
`.tq-verify` after enqueue, then flag-off fires the stale pin while flag-on
gates on the new command — an outcome flip, not a config assertion.

## 3. Why it is a trust hole (the OKF lesson)

OKF v0.2 §10.3 is explicit about the same asymmetry in a different domain: the
agent "MAY only supply *values* for the declared `parameters`; it MUST NOT
author or edit the computation." tq's gate is the computation. With the flag
on, the gate is read fresh from `.tq-verify` — a file **the agent has write
access to during its own run**.

That is a real integrity gap and not a theoretical one: an agent that cannot
make its work pass the pinned gate can, in principle, widen the file. The repo
already refuses this shape elsewhere (`GoEnvExperiment`,
`internal/executor/agent.go:1294` — "agents never edit their own gate"), so the
flag contradicts a rule the repo states elsewhere.

## 4. What has changed since the flag shipped

**The hole is now observable, which it was not this morning.** As of the
receipt work in this session (`internal/executor/receipt.go`):

- every gate run records `verify.command` — the command that ACTUALLY ran;
- the write-time attester (`executor.AttestVerify`) compares it against the
  declarations the run saw (`.tq-verify` at gate time and
  `AgentPayload.Verify`) and stamps a **refusal** when the executed command is
  neither;
- `TestAttestationRefusesReresolvedGate` pins exactly this: flag on + a
  payload pin + an auto-detected gate ⇒ `command-mismatch`, with the violated
  pin named in the detail;
- `tq show` renders the refusal; `tq dlq` rows carry `verify=command-mismatch`.

So "a gate silently stopped being the pinned one" is no longer silent.

## 5. The two options, with cost

### Option A — RETIRE the flag

Delete `ReresolveVerify`, `verifyFor`'s `reresolve` parameter, both flag
registrations, and the doctor remedy text that advertises it.

- **Cost:** low code churn, but it removes the only remedy for genuinely stale
  pins. `tq doctor --hygiene`'s `verify-pins` audit (`cmd/tq/doctor.go:990-1105`)
  reports three staleness classes (known-stale mint pattern via
  `executor.StaleVerifyReasons`, `.tq-verify currently overrides it`, and
  `STALE PIN WILL FIRE`), and today the ONLY documented cure it names is this
  flag.
- **Risk:** tasks whose pin names a superseded gate would have no operator
  remedy short of cancelling and re-harvesting.

### Option B — KEEP and NARROW

Keep the flag as an escape hatch, and close the two ways it can lie:

1. **Observability — already done.** The attester refuses the receipt when the
   executed gate was not the pinned one (§4). Nothing further to build.
2. **The agent-authored-gate case.** Re-resolution should not accept a
   `.tq-verify` the run itself modified. Cheap check at gate time: if
   `git diff --quiet <baseRev> -- .tq-verify` fails (the file differs from the
   pre-attempt baseline), do not resolve from it — fall back to the pin and
   stamp the divergence. `baseRev` is already threaded into `runVerify` for the
   gate-dead classifier, so this is one `gitRun` call, no new plumbing.
3. **Document the contract** where the flag is registered: "re-resolution is
   for pins, not for gate edits."

- **Cost:** one `git diff` per re-resolved run plus a doc line — well under an
  hour, and all of it rides machinery that already exists.
- **Risk:** a repo that legitimately rewrites `.tq-verify` inside a task
  (rare, and arguably a task-design smell) would be judged on its old pin.

## 6. Recommendation

**Option B — keep and narrow**, with step 1 already landed.

Rationale, shortest form: retiring the flag removes the only cure for a class
the doctor actively reports, and the reason to retire it — silence — is gone
now that the receipt refuses a diverged gate. Narrowing costs one git call and
converts "trust me" into "the receipt says so".

Deeper alternative, recorded but NOT recommended now: make pin staleness a
first-class stored signal (an absolute `verify_pinned_at` on the payload, the
OKF freshness lesson's "absolute instant, not a relative TTL"), and let the
claim path refuse a pin older than the repo's `.tq-verify` mtime. That is the
architecturally satisfying answer and it would let the flag retire honestly —
but it adds a payload field, a migration story and a doctor class, and it has
no consumer until the staleness audit stops being a WARN. That is an owner
product call, not an agent one.

## 7. What remains open (owner)

- **Q:** retire (Option A) or narrow (Option B)? The recommendation is B, and
  B's step 1 is already shipped, so the ruling only decides whether step 2 is
  worth an hour.
- Nothing in R1–R5 depends on the answer: the receipt and its attestation are
  useful either way, and they are what makes the question decidable at all.

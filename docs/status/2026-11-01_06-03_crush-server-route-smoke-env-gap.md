# Session status — crush server/client route: L1.8 smoke SHIPPED, M8 env gap DISPROVEN, E7 false-negative corrected

**2026-11-01 06:03 CET · Undispatched owner-directed session ("do we 100% use
SERVER/CLIENT mode for Crush v0.99 to the FULLEST MAX?"). Scope: answer the
question by FINISHING row 1450's wire legs — the prior session (01:54 report)
left the smoke uncut with an unproven mock tool-call leg. This session: triaged
two daemon sweeps, proved v0.99.0 routing + captured M5 shapes live, DISPROVED
M8 (env propagation) with upstream root cause, shipped
`scripts/smoke/crush-server-route.sh` (7 legs, green ×2), corrected a parallel
agent's E7 false negative, annotated rows 1446/1450 + minted the upstream-issue
row. No pushes; production journal untouched (reads only). NOTE at report
time (06:03): two LATER session reports exist (02-24, 04-33) whose claims
intersect this session's leftovers — the 04-33 row says row 1450 was CLOSED
as two smokes + row 1446 SHIPPED (crushserver.go enricher), yet the 06:03
tree shows `scripts/smoke/crush-pool-supervisor.sh` UNTRACKED and the
smoke-coverage gate red on it (no COVERAGE row) — either a parallel session
is re-landing it right now or it is a casualty of the 02-24 twin-reconcile
rebase; NOT adjudicated here (this report covers MY session's run only).**

---

## a) FULLY DONE (this session; each verified at HEAD)

1. **Daemon-sweep triage at session open** — 187438d7 (a baseline-append
   recursion sweep) + aa043e21 (the executor facade alias pair
   KnownEvidenceTerms/UnknownEvidenceTerms behind footer-less 14424324)
   grandfathered with per-sha notes; attribution gate green. My own commit
   first carried a bogus placeholder footer — amended to the footer-less
   gates-commit convention (7597d9fc precedent) while unpushed.
2. **Parallel-work absorption, not reversion** — the L1 seam (executor
   `CrushServer` + `TestAgentCrushServerSeam`, pool `runCrushServerActor` +
   keeper + supervisor ladder in cmd/tq/agentpool.go) landed via daemon
   sweeps mid-session; verified `cmd/tq` builds green through its tracked
   dev workspace (my first check wrongly used `GOWORK=off`, which ADR-0017
   exempts cmd/tq from).
3. **Routing PROVEN on v0.99.0** (the E-gap core the prior session could not
   close): `CRUSH_CLIENT_SERVER=1` + `-H <sock>` routes server-side and
   `GET /v1/workspaces` lists the live workspace for the whole run — pinned
   repeatedly, including inside the smoke. M4 holds (`Created session …
   session_id=` line); the tool-call → file-write loop works under routing.
4. **M5 shapes captured live mid-run** — workspaces entry
   (id/path/data_dir/version/full resolved config), sessions array
   (id/title/message_count/prompt_tokens/is_busy/attached_clients…), messages
   arrays (parts: text / tool_call with JSON-string input / tool_result with
   metadata / finish reasons stop|tool_use|canceled).
5. **M8 DISPROVEN for tool exec — the session's headline finding**: the
   client DOES forward its env with workspace creation, but the server-side
   bash TOOL never sees it. Root cause pinned in upstream source:
   `internal/shell/background.go` builds the tool shell without workspace
   Env; `internal/shell/shell.go` falls back to the server's
   `os.Environ()`. Only `RunShellCommand` (interactive `$` shell,
   backend/agent.go:315) merges `ws.Env`. Two-sided empirical pin: server
   env vars ARE visible in the routed bash tool; client `$TQ_RESULT_FILE`
   expands EMPTY — 3× on fresh stacks + repeat runs. Consequence: the
   `--crush-server` verdict/question env channels BREAK under routing.
6. **M7 amendment (new hazard)**: a client that finds a dead socket
   auto-respawns the server (ensureServer, proven live: kill -9 → next run
   rc 0 + health 200) — and the respawned server INHERITS THAT CLIENT'S
   env, including its `$TQ_RESULT_FILE`. Under the pool this is a
   cross-task env-bleed vector (task B's bash could see task A's result
   path after a client-led respawn). The keeper exists to prevent exactly
   the idle deaths that trigger it; the supervisor restarts use the pool's
   env (safe).
7. **R4 trap discovered**: `-H` WITHOUT `CRUSH_CLIENT_SERVER=1` silently
   executes LOCALLY — rc 0, no connection, no error, even with the socket
   missing. This invalidated the prior session's 02:12 "routed" verdict
   round-trip (its launcher never set the env gate) and is why their
   workspace listing looked quiet.
8. **`scripts/smoke/crush-server-route.sh` SHIPPED (L1.8)** — 7 legs, green
   end-to-end twice at HEAD (13 PASS lines): isolation gate (`crush dirs`
   on stderr, config parses, no run before the gate) → boot + `/v1/health`
   → routed verdict round-trip (rc 0 + session-id line + every provider
   call on the mock = zero-spend receipt + verdict file written) → live
   workspace + sessions/messages shape asserts DURING a stalled run →
   two-sided env-gap CANARY (flips red when upstream fixes propagation) →
   server-side hook fire + exit-2 deny (side-effect ground truth: hook
   log written, blocked command's file absent) → server-kill respawn.
   Embeds the promoted mock provider (content-keyed scripting, OpenAI
   incremental tool_call streaming, loop guard, port-0 allocation,
   request log). COVERAGE.md carries the leg table + fleet row;
   CHANGELOG [Unreleased]/Internal row (O14-gated, check-release-docs
   green).
9. **Planning doc updated** (`2026-10-30_04-43_crush-client-server-relations.md`):
   status line re-verified, M8 row rewritten as DISPROVEN with the source
   cite, §v0.99 re-verification R1–R6 appended (routing, env gap, respawn
   bleed, `-H`-alone trap, probe gotchas, L1.8 shipped).
10. **E7 false negative corrected** — a parallel agent's 02:44 E7 section
    claimed "v0.99.0 workspaces listing is quiet for routed runs"; their
    probe recipe omitted `CRUSH_CLIENT_SERVER=1`, so their run was LOCAL
    (local runs also write `<repo>/.crush/crush.db`, which fed the
    misread). Corrected in place with counter-evidence; their M5 wire
    shapes and openapi.json route authority kept (valuable, verified
    against my captures).
11. **TODO_LIST honest state** — rows 1446 (enricher: shapes now fully
    captured, must not rely on client env) and 1450 (smoke shipped +
    env-gap discovery; the three pool E-gaps BLOCKED upstream) annotated;
    NEW row minted: file the upstream crush issue (bash tool drops
    workspace Env) + executor bridge decision. check-todo-list green.
12. **Session hygiene** — my five mid-edit daemon sweeps (the smoke's debug
    iterations + docs) grandfathered with verified notes (syntax 122/122,
    smoke green, todo/release-docs gates) in 583a6846; attribution gate
    green (1525 baselined, 0 new at my exit).

## b) PARTIALLY DONE

1. **Row 1450 (E-gap proofs under routed runs)** — the wire half is DONE
   (smoke above), but the row's own trio stays open: verdict file via the
   executor's `$TQ_RESULT_FILE` channel and question parking are BLOCKED on
   the upstream env gap (b5); the 429 fast-refuse leg is being built by a
   PARALLEL agent right now (`scripts/smoke/crush-pool-supervisor.sh`,
   untracked at report time, correctly cross-references my sibling smoke —
   its COVERAGE row is still missing, which leaves the smoke-coverage gate
   red on THEIR file, not mine).
2. **The executor bridge for env channels under `--crush-server`** — row
   minted (prompt-baked result paths vs waiting upstream) but no decision
   and no implementation; the L1 seam is shipped with a latent contract
   break for paid turns if anyone sets the flag today. (The 04-33 session
   claims row 1446's enricher shipped messages-API-based — consistent with
   the bridge direction, but not verified by this session.)
3. **The window's exit artifact** — a full clean `ci-local.sh` receipt
   before push (carried from the 01:54 report §f1) still does not exist;
   I ran targeted gates only (attribution, script-syntax, todo-list,
   release-docs, doc-refs, the new smoke ×2) and deliberately skipped the
   full battery while parallel agents had in-flight trees.

## c) NOT STARTED (this session's scope)

- Filing the upstream crush issue (evidence is ready in the planning doc
  §R2; held for owner visibility — see g1).
- AGENTS.md §Known Issues rows for the newly proven traps (`-H`-alone
  silent local; crush dirs prints on stderr; zombie-mock fixed ports) —
  they live in the planning doc R5 + smoke comments only.
- Scratch cleanup: /tmp/tq-crush2, /tmp/tq-crush3, /tmp/tq-crush4,
  /tmp/tq-dbg*, /tmp/tq-crush-route-smoke-run, and the now-promoted
  /tmp/tq-crush-recipe still exist (the prior session deliberately kept
  the recipe for the next session; that next session was THIS one).
- `tq verdict` close-exit recording (undispatched session — no task to
  carry a verdict; noted here per the half-closeout convention).

## d) TOTALLY FUCKED UP (worst first)

1. **I probed against a hijacked provider for two full runs without
   noticing.** My mock restarts died instantly on `Address already in use`
   (a zombie 02:12 mock held port 39321); the mock's own crash log said so
   and I did not read it until after misdiagnosing routing twice. The
   port-0 + pidfile + kill-by-pidfile pattern only entered when CUTTING
   the smoke — it should have been the probe's first design (the class is
   even in the prior report's §d, and I repeated it).
2. **Two "routing proofs" with non-discriminating evidence.** I first
   argued from session-storage location (`<repo>/.crush/crush.db`) — which
   is identical for LOCAL and ROUTED runs — and from a `provider=mock`
   log line that turned out to be unreliable. The workspaces-during-run
   probe was ALWAYS the one true discriminator; I should have designed
   the discriminating observation before running anything.
3. **Assertion-first smoke authoring — five debug iterations**, the exact
   class AGENTS.md warns about ("observe-first for pinning work"):
   `2>/dev/null` swallowing crush-dir's stderr output (read as "prints
   nothing"), pipefail+SIGPIPE silently killing the script at `head -1`,
   an 8192 mock context_window triggering an endless summarize/compact
   cascade (needed 200k), `rm` unlinking the mock's open append-handle
   (log "went empty"), and single-quoted `$TMP` in the canary command
   landing server-side as an unwritable path. Every one was preventable
   by capturing one live transcript before writing asserts.
4. **A near-miss on a false conclusion**: mid-session I briefly announced
   "M7 respawn proven, run green" without checking WHAT env the respawned
   server adopted — one smoke iteration later the canary returned the
   CLIENT's `$TQ_RESULT_FILE` and I nearly recorded "upstream fixed env
   propagation". Only fresh-stack control runs exposed the
   respawn-inheritance confound; the smoke now guards it
   (`ensure_our_server`). The false "fix" claim would have un-caveated
   the L1 seam on a coincidence.
5. **Tool-call sloppiness**: one multiedit corrupted the smoke heredoc with
   literal NUL escapes; my python fix then used an outer `EOF` heredoc
   delimiter that collided with the inner one, executing stray shell
   fragments (harmless — both failed closed). Two wasted rounds on
   self-inflicted quoting.

## e) WHAT TO IMPROVE

1. **Observe-first applies to smokes double** — the mock/stall/probe
   recipe existed at 01:54; running ONE captured transcript before
   writing leg asserts would have saved all five iterations of d3.
2. **Design the discriminator before the experiment** — write down which
   observation separates the hypotheses (routed vs local) FIRST; never
   "verify" with evidence that both outcomes produce.
3. **Every scratch server/prover from minute one**: port 0, pidfile,
   kill-by-pidfile, request log to the scratch, size watch. The zombie
   class keeps re-occurring (prior session §d2, mine §d1).
4. **Coordinate with parallel agents on shared surfaces** — E7 landed on
   the SAME planning doc 30 min into my session with a claim my data
   contradicted; reading in-flight files before writing mine (or a
   heads-up row) beats after-the-fact correction.
5. **Heredoc discipline**: distinct delimiters per level, or the write
   tool for file bodies; never NUL-escape through multiedit JSON.
6. **Trash my own /tmp debug dirs at session end** (still pending — c).

## f) NEXT (ranked)

1. Owner ruling g1: file the upstream crush issue myself or owner-files
   (evidence ready: planning doc §R2 + the smoke's canary repro).
2. Executor bridge decision for `$TQ_RESULT_FILE`/`$TQ_QUESTION_FILE`
   under `--crush-server` (prompt-baked paths vs messages-API derivation
   vs keep the flag off for paid turns until upstream).
3. **Re-verify the 04-33 session's row-1450/1446 closure claims against the
   06:03 tree**: crush-pool-supervisor.sh untracked + COVERAGE-missing
   (re-land it or index it — possible rebase casualty of the twin
   reconcile); my crush-server-route.sh IS tracked+green at HEAD.
4. Full quiet-tree `ci-local.sh` receipt — the window's missing exit
   artifact (~65 commits unpushed across sessions now).
5. Push-state verification before any further claims (02-24 row says the
   daemon PUSHED origin to 4a69bf9f; later sessions' pushes unverified —
   the ~65-unpushed figure below was true at MY session's exit, not
   necessarily now).
6. Row 1446: build the structured-closeout enricher on the messages API
   (shapes verified; register-own-workspace doorway per E7; no client env
   dependence).
7. L2 live agent pane (SSE attach per active workspace → hub) — now
   unblocked by the verified event/workspace surface.
8. AGENTS.md §Known Issues: one row for the `-H`-alone silent-local trap +
   crush-dirs-stderr + zombie-port lessons (pointer to planning doc R5).
9. Trash /tmp probe remnants (tq-crush2/3/4, dbg*, route-smoke-run;
   decide keeping tq-crush-recipe — its content is promoted into the
   smoke).
10. Consider a `--crush-server` guard in the pool: refuse the flag for
    agent tasks until the env bridge exists (today it silently breaks
    verdict collection for routed paid turns).
11. L4 pilot (row 1448): re-scope the risk section with R3 (client-respawn
    env bleed) before any fleet routing.
12. v0.4.3 tag-wave evaluation (carried 01:54 §g3: evidence UX, pane,
    KnownEvidenceTerms ride it; now + the smoke + planning corrections).
13. Harvest this report's f-items into TODO_LIST per docs-health rigor if
    the owner ratifies (rows 3/6/8/10 already exist in some form).

## g) QUESTIONS (cannot figure out myself)

1. **Upstream filing authority for charmbracelet/crush**: the env-gap issue
   (bash tool drops workspace Env; `RunShellCommand` already merges it —
   agent tools should too) is fully evidenced and reproducible. File it
   myself under the repo's GitHub identity, or do you want to file/review
   first? (Standing limits don't state crush-upstream filing either way.)
2. **If upstream is slow**: which bridge for verdict/question collection
   under `--crush-server` — bake the result-file path into the payload
   (prompt contract change), derive outcomes from the messages API after
   the run (row 1446's enricher, no env), or keep the flag experimental
   and off for paid turns until upstream lands?
3. **Push/push-state authority**: my session exited with ~65 unpushed
   commits; the 02-24 report claims the daemon later PUSHED part of that
   window, and twin-reconcile rebases happened after my commits landed. Do
   you want a push-state audit + clean-battery receipt before the next
   wave, and does the standing "no push past a report without §g" rule
   still apply given the daemon pushes?

— reported 2026-11-01 06:03 CET; production journal untouched (reads only);
no pushes this session; scratch probe stacks still in /tmp (see c/f9);
parallel agents active throughout (E7, pool-supervisor smoke, gate commits
0859f7b6/a05c9539 noticed and left alone).

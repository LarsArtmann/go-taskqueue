# go-credentials-store — Library Utilization Review

**Verdict:** NOT ADOPTED — KEEP OUT — tq's entire secret surface is one operator-provided `--auth-token` compared constant-time in `internal/httpauth` and never persisted; an encrypted event-sourced credential vault with an admin UI solves a problem go-taskqueue does not have.

> **2026-11-11 refresh:** go-credentials-store shipped **v0.4.0/v0.4.1/v0.4.2** (all 2026-11-10) after this review's v0.3.1 snapshot. What shipped (CHANGELOG [0.4.0]–[0.4.2]): admin-UI mounting work (`adminui.Mount` `WithHostLinks`, CSP-nonce bridging for mounted hosts, mobile-menu nonce fix), release-engineering hardening (a generator-pinned `templ-generate.sh` wrapper, a pre-tag consumer-simulation gate), and one in-scope credentials fix — password resolves now carry the stored username (`Material.PasswordUser`, commit 7f56ab1). Also visible: the templ v0.3.1100 dep wave left master uncompilable and a go-snaps residue broke consuming hosts' tidy gates — the dependency-churn cost this review already weighted. **Nothing here changes the keep-out calculus**: it is still a secrets vault, while tq's surface re-verified unchanged 2026-11-11 (single `--auth-token`/`$TQ_SERVE_TOKEN`, constant-time compare in `internal/httpauth`, no persistence site — greps in §How to verify). One cross-ecosystem note: the store's HEAD (2026-11-11) bumps its cmdguard/v5 pin to v5.1.0; irrelevant to tq, which depends on neither.

**Pinned:** none (not a dependency; verified 2026-11-11 by `rg -i 'go-credentials-store' -g 'go.mod' -g 'go.sum' -g '!vendor/**' .` → no output, rc=1; same for `--glob '*.go'` import greps)

**Latest:** v0.4.2 (2026-11-10, 771b3c6 — local checkout /home/lars/projects/go-credentials-store, checked 2026-11-11; HEAD = v0.4.2-25-gc423a30, 2026-11-11: bumps the cmdguard/v5 pin to v5.1.0)

**Prior coverage:** none (docs/review/libs/ has no credentials entry; AGENTS.md records no secrets-store decision beyond the redaction contract).

**Reviewed:** 2026-11-08 by reading /home/lars/projects/go-credentials-store/{README.md,go.mod} plus internal/httpauth/httpauth.go, internal/webui/{auth.go,webui.go}, internal/httpapi/httpapi.go, internal/lockout/lockout.go, cmd/tq/bootstrap.go (renderPoolConfig), internal/bridge wiring greps; greps in §How to verify.

## 1. Where the need exists today

go-credentials-store is a full secrets product: XChaCha20-Poly1305-encrypted-at-rest credentials (API keys, passwords, Playwright session blobs) with KeyID rotation, lifecycle audit (created/rotated/probed/used/revealed), a loopback admin UI (templ + SSE + one-time reveal + Prometheus), built on the go-cqrs-lite `system` composition root (README:3-13; now pinned at v0.4.2, README:21).

The complete inventory of secrets handling in go-taskqueue:

- **`--auth-token` / `$TQ_SERVE_TOKEN`** — operator-provided bearer token for the two HTTP surfaces. Presentation policy + constant-time comparison over SHA-256 digests live in `internal/httpauth` (httpauth.go:1-14 package doc: "the secret's length never leaks through timing"; `TokenMatches`); the webui requires it only for non-loopback binds (internal/webui/auth.go:41, config field webui.go:81-84, guard applied webui.go:408-409), the API always (internal/httpapi/httpapi.go:77). The token lives in process memory only — no persistence site exists (`rg 'AuthToken'` over `*.go`: flags, config struct, guard).
- **`--alert-api-key` / `--cqa-token`** — operator-provided bridge credentials passed flag→config struct→HTTP header for PapDashboard/CQA (cmd/tq usage const, main.go:64 and :74); forwarded, never stored.
- **The generated pool config** (`~/.config/tq/pool.conf`, `tq bootstrap --install`) carries NO secrets — verified: renderPoolConfig writes projects-dir/repos/concurrency/budget/toggles only, under the comment "edit freely, flags on the CLI win" (cmd/tq/bootstrap.go:795-815).
- **Secrets redaction** is an OUTPUT concern — every output tail passes `internal/executor/redact.go`; `tq audit --journal` reports SECRET EVIDENCE rows (AGENTS.md §Secrets redaction). tq scrubs secrets; it does not keep them.

## 2. Capability inventory (fit)

| Capability | Status | Evidence |
| --- | --- | --- |
| Encrypted-at-rest secret storage (XChaCha20-Poly1305, master keys) | ⚪ N/A — tq stores zero secrets at rest; the only credential is a runtime flag | rg over `*.go` in §How to verify |
| `Resolve(ctx, platform, kind)` runtime resolution | ⚪ N/A — provider keys are consumed by the child `crush` process from its own environment; the executor deliberately does NOT inject them (envDenylistForAgents is TQ_DB only, agent.go:617-640) | internal/executor/agent.go:617-640 |
| Key rotation / KeyID versioning | ⚪ N/A — no keys to rotate; token rotation = operator restarts with a new flag | — |
| Lifecycle audit trail (created/used/revealed) | ⚪ N/A — auth attempts are audited already: lockout 3-strikes + `tq audit` | internal/lockout/lockout.go:22-28 |
| Admin UI (list/reveal/SSE) | ⚪ N/A — tq's admin surface is the dashboard, whose write plane is CSRF+lockout-guarded (AGENTS.md: tq serve/tq api security) | — |
| Master-key management (`CREDENTIALS_MASTER_KEY_V1`) | ⚪ N/A — would ADD the very secret-management problem tq lacks | README:54 |
| Dependency weight | ⚪ N/A — 32 direct requires (cqrs-lite stack, templ + templ-components, prometheus, cobra, samber/do, go-sse, cmdguard/v5 v5.1.0 — go.mod:8); adopting would roughly double the curated vendor tree | go-credentials-store/go.mod require block |
| Pure Go, `CGO_ENABLED=0` | compatible in principle (modernc sqlite, pure-Go stack) | go.mod |

## 3. Findings (buy vs cost)

1. **Buy:** effectively none for tq. The library's value proposition — durable, rotated, audited secrets — presumes services that must remember credentials between runs. tq's threat model points the other way: the token is an OPERATOR-held shared secret used to *gate* tq, not a secret tq holds on anyone's behalf.
2. **Cost (dominant):** a credentials vault in the binary would create a highest-value target inside the queue process (the thing that shells out to agents across ~/projects), plus a master-key operational burden (`CREDENTIALS_MASTER_KEY_V1`), plus 32 direct dependencies through the vendor tree of every module graph loader — the exact class of churn AGENTS.md warns about ("New module + unpushed tag = replace in EVERY graph loader"). The store's own v0.4.x changelog (2026-11-10) demonstrates the cost from the inside: a templ dep wave that left master uncompilable and a test-dep residue that broke consuming hosts' tidy gates.
3. **The adjacent real problem is already solved better locally:** secrets LEAKING through outputs — handled by redact.go + audit rows; and secrets reaching the wrong process — handled by the env denylist + the planned per-run allowlist (docs/planning/2026-10-05_secret-injection-seam-design.md). Neither is a storage problem.
4. **Adoption site if the need ever appeared** (a hypothetical future where tq brokers provider keys for agents):

```go
container, _, err := credentials.Build(ctx, credentials.Options{
    MasterKeys: map[encryption.KeyID][]byte{"v1": key}, ...
})
key, err := container.Resolver().Resolve(ctx, "groq", domain.KindAPIKey)
```

Nothing in cmd/tq, internal/executor, or ROADMAP.md wants this.

## 4. Fit assessment

go-credentials-store is a well-engineered product (event-sourced, encrypted by construction, operational drill proven, per its README) aimed at personal infrastructure that must hold API keys and session blobs for third parties. go-taskqueue is not that: it holds one operator gate token in memory, redacts secrets from everything it emits, and deliberately refuses to become a secret-injection path for its agents. Adopting would import a vault, its master-key ceremony, and a ~30-dep stack to solve a non-existent storage need while adding a new attack surface precisely where the security posture (loopback default, read-only default, redaction) has been carefully minimized.

## 5. Maturity + version currency

Latest tag v0.4.2 (2026-11-10; v0.4.0/v0.4.1/v0.4.2 all cut the same day); 11 root tags since v0.1.0 (2026-10-13) — the youngest library in this batch; last commit c423a30 2026-11-11 (bumps the cmdguard/v5 pin to v5.1.0); checkout v0.4.2-25-gc423a30. The README now records production adoption by CV (README:19-21) and the admin-UI mounting/CSP work of v0.4.0-0.4.2. Actively maintained; same-owner ecosystem (consumes cmdguard/v5, go-daemon, go-atomic-write, httputil — several reviewed as siblings here). MIT.

## 6. Conclusion

Keep out, and expect this to stay settled: tq's security contracts (constant-time token compare, loopback-default, redaction, env denylist) cover its actual secrets story without storing anything — re-verified 2026-11-11 against v0.4.2, whose changes are admin-UI/CSP, release-engineering, and a password-resolve fix, none of which touch the token-only surface. Revisit only if tq ever becomes a credential broker for its agent fleet — a direction the secret-injection-seam design doc explicitly routes AWAY from (per-run allowlists of operator-env, not a vault).

## How to verify

```bash
cd /home/lars/projects/go-taskqueue
rg -i 'go-credentials-store' -g 'go.mod' -g 'go.sum' -g '!vendor/**' .   # → no output (rc=1)
rg -i 'go-credentials-store' --glob '*.go' --glob '!vendor/**' .         # → no output (rc=1)
rg -n 'AuthToken' --glob '*.go' --glob '!vendor/**' --glob '!*_test.go' internal/ cmd/
# → flags, config fields, guard reads only — no persistence site
sed -n '1,14p' internal/httpauth/httpauth.go   # constant-time compare contract
sed -n '795,815p' cmd/tq/bootstrap.go          # pool.conf carries no secrets
rg -n 'MasterKeys' /home/lars/projects/go-credentials-store/README.md    # :54 — the ceremony tq would inherit
git -C /home/lars/projects/go-credentials-store describe --tags  # → v0.4.2-25-gc423a30
git -C /home/lars/projects/go-credentials-store tag -l | grep -v / | wc -l  # → 11
git -C /home/lars/projects/go-credentials-store log -1 --format='%ad %h %s' --date=short  # 2026-11-11 c423a30 bump cmdguard/v5 to v5.1.0
grep -n 'cmdguard' /home/lars/projects/go-credentials-store/go.mod       # :8 — cmdguard/v5 v5.1.0
grep -c $'\t' <(awk '/^require \(/{f=1} f&&/^\)/{exit} f' /home/lars/projects/go-credentials-store/go.mod)  # → 32 direct requires
```

# go-output — Library Utilization Review

**Verdict:** NOT ADOPTED — KEEP OUT — the CLI needs exactly the two output modes it has (human table, JSON), both pinned by parser contracts and guard tests; a 16-format render kit would be dead weight in a vendored, stdlib-flag CLI.

**Pinned:** none (not a dependency; verified 2026-11-08 by `grep -rn 'larsartmann/go-output' --include=go.mod . | grep -v vendor` → no output, same for import greps; re-verified 2026-11-11 by the same greps — still 0 hits in go.mod and *.go, vendor excluded)

**Latest:** v0.38.7 (local checkout /home/lars/projects/go-output, checked 2026-11-08; root series — the repo carries 666 total tags across its format sub-modules)

**2026-11-11 refresh:** versions unchanged (still v0.38.7, 666 tags, HEAD ba50334 2026-11-02); output surface re-verified unchanged (10 `--json` flags across the same 7 files, still no tabwriter, still exactly one ANSI escape at top.go:221); fixed one stale cite — the `tq tasks` json bool is at cmd/tq/tasks.go:51, previously cited as :3.

**Prior coverage:** none; cmd/tq output rendering has never been reviewed against a library.

**Reviewed:** 2026-11-08 by reading /home/lars/projects/go-output/{README.md,go.mod,format.go} plus a survey of cmd/tq rendering (tasks.go, main.go, top.go, doctor.go) and greps for tabwriter/ANSI/humanize; re-verified 2026-11-11 (all greps and line cites below re-run; one stale cite fixed, see the refresh note).

## 1. Where the need exists today

go-output is a render-anywhere kit: define a `Table`/`Tree`/`Graph` once, render to 16 formats (JSON, CSV, TSV, Markdown, XML, HTML, D2, Mermaid, DOT, YAML, AsciiDoc, TOML, PlantUML, JSONL, plain tree, lipgloss tables), with a `Format` enum for CLI flags, zero-config color (`ColorModeAuto` honors NO_COLOR/CI/FORCE_COLOR), and NOM-style live progress in an opt-in sub-module (README:1–26; format.go:8–23). Root module pulls only `golang.org/x/term` + go-branded-id (go.mod).

What cmd/tq hand-rolls today (all stdlib):

- **Fixed-width tables**: `printTaskListTo` renders the task-list header+rows with `fmt.Sprintf("%-36s %-14s …")` (cmd/tq/tasks.go:284–351, header at 298–308); `tq stats` status counts (cmd/tq/main.go:2547–2551); per-project grid (main.go:2560–2571); DLQ rows (main.go:3497); facts tail (main.go:3825). No `text/tabwriter` anywhere (grep → zero hits).
- **JSON modes**: 10 `fs.Bool("json", …)` flags across 7 files (audit.go, doctor.go, main.go×4, poolhealth.go, reprioritize.go, tasks.go, top.go — grep, re-verified 2026-11-11), each with its own `json.NewEncoder(os.Stdout)` block (e.g. main.go:918, 2230, 3688).
- **Terminal control**: exactly ONE ANSI escape in the tree — the screen clear in `tq top` (`fmt.Print("\x1b[2J\x1b[H")`, cmd/tq/top.go:221), guarded by `isTerminal(os.Stdout)`; no colors, no spinners, no progress bars (grep `\x1b` → that single line).
- **Sizing**: `dustin/go-humanize` for byte/time formatting in doctor.go:22 and poolhealth.go:22 (a direct dep of cmd/tq/go.mod:6).

## 2. Capability inventory (fit)

| Capability | Status | Evidence |
| --- | --- | --- |
| Multi-format table render (CSV/Markdown/HTML/…) | 🔴 MISSING-BUT-RELEVANT (low: no consumer has asked; JSON covers scripts) | 10 hand-rolled `--json` flags, zero `--format` flags |
| Zero-config color + NO_COLOR/CI detection | 🔴 MISSING-BUT-RELEVANT (low: monochrome output is a deliberate lean style) | top.go:221 is the only ANSI in cmd/tq |
| `Format` enum for CLI flag parsing | ⚪ N/A (tq's dual mode is a boolean, pinned by tests) | tasks.go:51 etc. |
| JSON serialization of tables | 🟡 partial overlap (per-command encoders, shape-stable wire contracts) | main.go statsPayload:2295, dlqJSONRow:3423, factJSONView:3605 |
| NOM-style live progress | ⚪ N/A (tq top's interval redraw + screen clear is sufficient; serve has SSE) | top.go:195–240 |
| Trees/graphs/D2/Mermaid | ⚪ N/A (no graph output exists or is requested) | — |

## 3. Findings (buy vs cost)

1. **Buy:** a `--format csv|markdown` on `tq tasks`/`tq dlq` would be nearly free — the row data already exists as structs, and the kit's builder/snapshot/renderer split means the JSON paths stay untouched; color with NO_COLOR compliance would arrive without hand-detecting TTY per site.
2. **Cost (dominant) — output shapes are contracts, not styling:** the serve banner's exact string is shared by the CLI print, e2e and smoke parsers ("nearly broke twice over string drift" — internal/webui/webui.go:41–44); row shapes are pinned by tests (cmd/tq/tasks_test.go, dlq_test.go, stats_json_test.go, budget_json_test.go) plus the stdoutcapture guard (cmd/tq/stdoutcapture_guard_test.go). Re-rendering through a library re-risks the exact drift class those guards were built for, across ~10 commands, for zero requested capability.
3. **Cost — dependency shape:** cmd/tq is deliberately replace-free stdlib-flag CLI (ADR-0017; AGENTS.md §Commands multi-module note). The root module is light (x/term + go-branded-id — the latter already an indirect dep at cmd/tq/go.mod:53), but the interesting renderers live in sub-modules whose intended consumption path is a cloned workspace with committed `replace` directives (README "Installation", Pattern B / ADR 009) — awkward inside tq's curated, vendored module graph (root ./vendor/, cmd/tq pinned floor).
4. **Cost — floor:** go-output requires Go 1.26+; tq is on 1.27.3 (go.mod:3) so no conflict, but the kit's API churns fast (v0.30 "CQRS" rebuild; 666 tags) relative to a CLI whose output is frozen by tests.
5. **Adoption site if a format ask ever lands** (one command, additive, JSON untouched):

```go
// cmd/tq/tasks.go — a --format flag layered over the existing row structs
tbl := output.NewTable([]string{"ID", "STATUS", "PROJECT", "TYPE", "ATT"})
for _, t := range tasks {
    tbl.AddRow([]string{t.ID.String(), string(t.Status), t.Project, t.Type,
        strconv.Itoa(t.Attempts)})
}
_, _ = output.RenderTable(tbl, format, output.RenderOptions{ColorMode: output.ColorModeAuto})
// default stays the hand-rolled printTaskListTo — parser contracts untouched
```

## 4. Fit assessment

The kit is impressive and would fit a CLI that owes nobody a stable output shape. cmd/tq owes many: e2e/smoke parsers, ~15 output-pinning test files, and human operators whose muscle memory reads the fixed-width columns. The two modes tq needs (human table, JSON) are implemented, tested, and cheaper than a migration; the modes tq lacks (CSV, Markdown, color, NOM progress) have no requester. The honest fit verdict: wrong decade of the CLI's life — a greenfield tq might start on go-output; a 4,400-line main.go with pinned output contracts should not switch. The natural trial, if ever requested, is one additive `--format` flag on `tq tasks` leaving the default path byte-identical.

## 5. Maturity + version currency

Latest root-series tag v0.38.7 (checked 2026-11-08: `git -C /home/lars/projects/go-output tag -l 'v0.*' | sort -V | tail -1`; re-verified 2026-11-11 — unchanged); 666 total tags (root + sub-modules; includes retracted v0.33.0/v0.34.0 — go.mod retract block); last commit ba50334 2026-11-02 (still HEAD on 2026-11-11). MIT (LICENSE). Root deps: go-branded-id v0.7.0, x/term, plus a testhelpers pin. Heavy same-author maintenance cadence — an API-stability ADR exists ("ADR 006 locks core interfaces") but the 0.x versioning says expect movement.

## 6. Conclusion

Keep out. The CLI's output layer is contract surface, not presentation debt; adopting a render kit buys formats nobody asked for at the price of re-risking pinned parser contracts across ten commands. Revisit only on a concrete ask (CSV export for a spreadsheet workflow is the most plausible) and then as one additive flag, default path untouched.

## How to verify

```bash
cd /home/lars/projects/go-taskqueue
grep -rn 'larsartmann/go-output' --include=go.mod . | grep -v vendor       # → no output
grep -rn 'larsartmann/go-output' --include='*.go' . | grep -v vendor       # → no output
grep -rn 'tabwriter' --include='*.go' cmd internal | grep -v vendor        # → no output (no tabwriter today)
grep -rn '\\x1b\|\\033' --include='*.go' cmd | grep -v vendor              # exactly one: top.go:221 screen clear
grep -rn 'fs.Bool("json"' cmd/tq/*.go | wc -l                              # 10 JSON flags across 7 files
sed -n '284,351p' cmd/tq/tasks.go                                          # the hand-rolled task table
sed -n '40,49p' internal/webui/webui.go                                     # banner-as-contract note
git -C /home/lars/projects/go-output tag -l 'v0.*' | sort -V | tail -1     # v0.38.7
head -3 /home/lars/projects/go-output/LICENSE                              # MIT
```

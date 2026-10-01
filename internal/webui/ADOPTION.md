# templ-components adoption

Documentation of WHICH library parts the Web UI is allowed to lean on.
Guarded bidirectionally by `TestAdoptionTableCoversTemplates` (every
template-invoked component must be listed; every listed component must
still be used) and `TestAdoptionTablePinsCustomRows` (the custom row is
contractual — hand-rolled surfaces are not silently "migrated" to library
components). Moved here from AGENTS.md 2026-10-01 (plan M87: table → link;
AGENTS.md is size-budgeted at ≤15,000 B by cmd/tq's
`TestAgentsDocSizeGuard`).

### templ-components adoption

| Identifier                                                                                                                                          | Status  | Where                                                                                            |
| --------------------------------------------------------------------------------------------------------------------------------------------------- | ------- | ------------------------------------------------------------------------------------------------ |
| `layout.Base/ThemeToggle`                                                                                                                           | adopted | `layout.templ`                                                                                   |
| `display.AreaChart/Badge/Button/Card/CopyButton/DefinitionList/EmptyState/ListNote/Scrollback/Table`                                                | adopted | `fragments.templ`                                                                                |
| `feedback.Alert`                                                                                                                                    | adopted | `fragments.templ` (task detail, last error)                                                      |
| `icons.ArchiveBox/CircleStack/Filter/Inbox`                                                                                                         | adopted | `fragments.templ` (empty states + filter)                                                        |
| status nowband (tq-seg), board columns/cards, filter inputs, page header/lamp, section hairlines, instrument panels (tq-topbar/tq-panel/tq-label/tq-fault) | custom  | `fragments.templ`/`layout.templ`/`theme.css` (StatCard retired for the nowband)                  |

Go-invoked, OUTSIDE the template-scoped table by decision:
`display.RelativeTime` renders the detail definition list's
created/updated/completed rows (AutoRefresh + CSP nonce via
`relativeTimeComponent` in components.go). The fact feed keeps wall-clock
timestamps + the client `data-age` ticker — a terminal tail reads absolute.

REJECTED, do not re-litigate (full verdicts in git history of AGENTS.md,
2026-09): `display.Eyebrow` (templated-chrome tell; replaced by custom
`tq-label`), `KanbanBoard` (custom board), errorpage module (library error
pages render their own chrome and would strand the operator mid-session;
tq stays in-chrome via `layout.Base`), `icons.Render`.

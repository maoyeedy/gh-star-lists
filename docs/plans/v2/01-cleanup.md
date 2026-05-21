# Plan: TUI v2 - Default experience and cleanup

## Context

Make the TUI the default interactive experience while keeping the CLI scripting surface intact. This plan consolidates the v2 baseline work: keep `gh star-lists` launching the browser in a TTY with no output flags, preserve all non-interactive commands, and split the current monolithic TUI files into focused components that can support follow-up UX and performance work without destabilizing behavior.

**Prerequisite:** TUI v1.7 ships the stable two-pane browser, preview pane, modals, bulk actions, session cache, search, sort, mouse support, and rendering polish.

## Scope

**In** | **Out**
---|---
Default TUI launch for interactive no-arg use | Deprecating or changing CLI subcommands
Focused TUI file/component split | YAML config, themes, or configurable keybindings
Per-pane independent search state | Multiple accounts or runtime host switching
Contextual footer/help cleanup | Plugin hooks or custom command scripting
Preserve current CLI machine output contracts | Broad GitHub dashboard features

## Current state

- `gh star-lists` already opens TUI in a TTY when no output flags are present.
- TUI state lives mostly in one `model` with shared `searchQuery` / `searchActive` across both panes.
- Help content is manually rendered in a modal and can drift from `keys.go`.
- Layout, cache/preload, modals, renderers, input handlers, and root update logic are split by file but still share one large state shape.

## Design

- Keep the product focused on Star Lists: lists, repositories, preview, and list membership mutations.
- Split code by UI responsibility without changing public CLI/API behavior.
- Use generated contextual help from key bindings where practical; avoid hand-maintained duplicate key text.
- Keep rendering ownership in `internal/tui`; do not import `internal/format`.

## Phases

| Phase | Goal | Parallel-with | Depends-on | Files | Subagent |
|---|---|---|---|---|---|
| P1 - State split | Separate pane-owned search/cursor state while preserving behavior | none | none | `internal/tui/model.go`, `internal/tui/search.go`, `internal/tui/input.go` | general-purpose |
| P2 - Component boundaries | Split root model logic into focused pane, preview, footer/help, modal, and cache/preload units | none | P1 | `internal/tui/*.go` | general-purpose |
| P3 - Contextual help | Replace duplicate help text with keymap-backed contextual short/full help | none | P2 | `internal/tui/keys.go`, `internal/tui/render_footer.go`, `internal/tui/modal_help.go` | general-purpose |
| P4 - Default flow audit | Verify TTY/default launch, CLI output paths, and help text remain consistent | none | P1, P2, P3 | `internal/command/run.go`, `internal/command/help.go`, TUI tests | general-purpose |

Phase order: P1 -> P2 -> P3 -> P4.

---

### P1 - State split

Separate list-pane and repo-pane search state. Replace shared `searchQuery` and `searchActive` with pane-specific state so a list search does not overwrite a repo search. Cursor and offset resets must apply only to the pane whose query changed, except when changing the focused list invalidates the repo pane.

```text
Exit criteria:
- List search and repo search can each retain independent query text.
- Clearing search in one pane does not clear the other pane.
- Existing navigation, search, and render tests pass.
```

### P2 - Component boundaries

Refactor without changing behavior. Keep the root model responsible for orchestration, but move pane rendering/input helpers, preview rendering/scrolling, footer/help rendering, modal internals, and cache/preload logic behind narrow helper types or files. Do not introduce config files or reusable rendering shared with CLI output.

```text
Exit criteria:
- TUI tests still describe the same behavior.
- No GitHub API calls are introduced outside `githubapi.Service`.
- No CLI formatter imports appear in `internal/tui`.
```

### P3 - Contextual help

Use the Bubbles key binding/help pattern so help rows are derived from `key.Binding` definitions. Short footer help should show the active pane's next useful actions. Full help should group navigation, list actions, repo actions, selection/bulk actions, preview, refresh, and quit.

```text
Exit criteria:
- Adding or changing a key binding requires updating `keys.go` only for the binding text.
- Footer help differs between list pane, repo pane, search, modal, and selection states.
- Help modal/full help remains readable at narrow widths.
```

### P4 - Default flow audit

Confirm `gh star-lists` still opens TUI only for interactive no-arg/no-output-flag use, while `list`, `repos`, machine output, output-file, template, jq, search, filters, and sort remain CLI-first and script-safe.

```text
Exit criteria:
- `go test ./internal/command/... ./internal/tui/...` passes.
- `make check` passes.
- `make ascii-check` passes.
```

---

## Tests

| Test | What it covers |
|---|---|
| `TestPerPaneSearchState` | List and repo queries persist independently |
| `TestSearchClearOnlyActivePane` | Clearing search only resets the active pane |
| `TestContextualFooterHelp` | Footer help reflects active pane and selection state |
| `TestHelpUsesKeyBindings` | Full help includes keymap-backed bindings |
| Existing command run tests | Default TUI launch does not affect script output paths |

## Verification

```text
go test ./internal/command/... ./internal/tui/...
make check
make ascii-check
```

Manual smoke:

```text
go run . browse
# - Search lists, enter a list, search repos, return to lists; both queries remain scoped.
# - Press ? from list pane, repo pane, search state, and selection state; help is contextual.
# - Run `go run . --json`; output remains machine-readable JSON, not TUI.
```

## Critical files

- `internal/tui/model.go` - root state and pane/search fields
- `internal/tui/input.go` - key handling, pane focus, search entry points
- `internal/tui/keys.go` - key bindings and help metadata
- `internal/command/run.go` - default TUI launch guard

## Reused utilities

- `key.NewBinding` / `key.Matches` - key binding source of truth
- `lipgloss.Width` - visual width for truncation and alignment
- `clampInt`, `slideListOffset`, `slideRepoOffset` - cursor and viewport bounds
- `githubapi.Service` - only API boundary consumed by TUI

## Out of scope
- Any change to JSON field names, TSV order, or command semantics

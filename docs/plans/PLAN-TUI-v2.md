# Plan: TUI v2 -- Default experience and architecture

## Goal

Make the TUI the default experience for interactive use while keeping the CLI
scripting surface intact. Split the TUI code into focused components after the
v1 polish and async cache stages are stable.

**Prerequisite:** TUI v1.4 shipped with narrow-layout polish, async repo cache,
mouse, search, multi-select, and viewport behavior stable.

## Scope

| In | Out |
|---|---|
| Bare `gh star-lists` on TTY opens TUI | New GitHub API surface |
| Architecture cleanup: component split and layout engine | Electron / web UI |
| Default help / README updates for bare TUI | Removing CLI scripting commands |

## Default bare-command switch

After v1.4 validates real-world use, flip the default:

```go
// run.go, in the ActionList case (or before the switch):
if parsed.Action == ActionList && len(parsed.RawArgs) == 0 && canPrompt() {
    // no flags that imply scripted use
    if !parsed.hasOutputFlags() {
        return runBrowse(ctx, parsed, service, stdout, stderr)
    }
}
```

`hasOutputFlags()` returns true if any of `--json`, `--tsv`, `--plain`,
`--fzf`, `--template`, `--jq`, `--output`, `--limit`, `--filter`, `--sort`,
`--search` were given. When true, fall through to CLI list output as today.

**Backward compatibility:** any script that calls `gh star-lists` without flags
in a TTY would now open the TUI. This is intentional. Document in README and
changelog. Provide `--no-tui` flag (maps to `ActionList` directly) as an
escape hatch for scripts that inadvertently run in a TTY.

**Help update:** bare `gh star-lists` help text gains a note:
`"Tip: in a terminal, running with no arguments opens the interactive browser."`

## Architecture: component split

By v2, `model.go` will be large. Split into focused files:

```
internal/tui/
  app.go          Options, Run
  model.go        root model struct, Init, Update, View
  layout.go       renderLayout, renderHeader, renderFooter, padRight
  pane_lists.go   renderListPane, list-pane key handlers
  pane_repos.go   renderRepoPane, repo-pane key handlers
  pane_preview.go renderPreviewPane
  modal.go        modal struct, all modal types and rendering
  messages.go     all Msg types and Cmd constructors
  sort.go         sort comparators
  keys.go         key bindings
  styles.go       shared styles
  search.go       (or import internal/search)
  model_test.go   root model tests
  layout_test.go  layout geometry tests
  modal_test.go   modal interaction tests
```

This split is mechanical -- no behavior change. Do it as the first PR of v2.

## Non-goals for v2

- YAML config. Do not add `config.yaml`, `--no-config`, or config loading.
- Themes. Do not add light/dark theme selection or theme-aware rendering.
- Configurable keybindings. Keep keybindings in code and document them in help.
- Disk cache. Defer until the v1.4 runtime async cache proves insufficient.
- Repo prefetch/performance work. This is owned by v1.4, not v2.
- Multiple GitHub accounts or `--host` switching within a running TUI session.
- Plugin system or scripting hooks.
- Removing or deprecating any CLI subcommand.

## Test additions

- Default-switch: bare `gh star-lists` on TTY calls `runTUI`; with `--no-tui`
  falls through to CLI list output; with `--json` falls through regardless.
- Component split: layout, pane, modal, and root model tests keep existing
  behavior unchanged.

## Verification

```
make check
make ascii-check
# Default switch: gh star-lists (TTY) opens TUI; gh star-lists --json still
#   prints JSON.
```

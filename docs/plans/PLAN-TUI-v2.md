# Plan: TUI v2 — Default experience + config

## Goal

Make the TUI the default experience for interactive use while keeping the CLI
scripting surface intact. Add user-configurable theme and behavior. Polish
everything that accumulated as tech debt across v1–v1.2.

**Prerequisite:** TUI v1.2 shipped (search, multi-select, mouse, viewport).

## Scope

| In | Out |
|---|---|
| Bare `gh star-lists` on TTY opens TUI | New GitHub API surface |
| YAML config: theme, defaults, keybind overrides | Electron / web UI |
| Disk cache (optional, off by default) | Multi-account support |
| Performance: parallel repo fetching for visible lists | Removing CLI scripting commands |
| Architecture cleanup: component split, layout engine | |

## Default bare-command switch

After v1.2 validates real-world use, flip the default:

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

## YAML config

Location: `${XDG_CONFIG_HOME:-~/.config}/gh-star-lists/config.yaml`

```yaml
tui:
  theme: dark          # dark | light | none
  default_sort_lists: name   # github | name | repos | added
  default_sort_repos: stars  # github | name | stars | pushed | language | starred
  mouse: true
  preview: false       # start with preview pane open

# Future: keybind overrides (v2.1+)
```

Loading:
- Parse at TUI startup only; not on every keypress.
- Missing file = all defaults; unknown keys = warn once then ignore.
- `--no-config` flag bypasses config file entirely.
- Config is never written back by the TUI (read-only at runtime).

Use `gopkg.in/yaml.v3` (already in `go.mod`). Define a `Config` struct in
`internal/tui/config.go`. `LoadConfig() (Config, error)` is called from
`app.go` before constructing the model.

**Files:**
- `internal/tui/config.go` — `Config` struct, `LoadConfig`, `defaultConfig`.
- `internal/tui/app.go` — call `LoadConfig`, merge with `Options`, pass to `newModel`.
- `internal/command/parse.go` — add `--no-config` flag for `browse`.

## Light theme

Add a second built-in theme. `styles.go` becomes theme-aware:

```go
type theme struct {
    cursor   lipgloss.Style
    selected lipgloss.Style
    faint    lipgloss.Style
    title    lipgloss.Style
    footer   lipgloss.Style
    error    lipgloss.Style
    success  lipgloss.Style
    modal    lipgloss.Style
}

var darkTheme  theme = ...  // current styles
var lightTheme theme = ...  // inverted palette
var noTheme    theme = ...  // no color, bold only
```

Active theme set once at model init from config. All rendering functions
receive `m.theme.*` instead of package-level style vars. This also fixes
the current coupling where `styles.go` is a file-level global.

`lipgloss.HasDarkBackground()` can auto-select theme when `config.tui.theme`
is omitted.

## Disk cache (optional, off by default)

Currently the in-memory `cacheService` (5-minute TTL) is the only cache layer.
Between TUI sessions everything is re-fetched from the network.

Add an optional disk cache backed by a simple JSON file per list ID:

```
${XDG_CACHE_HOME:-~/.cache}/gh-star-lists/lists.json
${XDG_CACHE_HOME:-~/.cache}/gh-star-lists/repos/<listID>.json
```

Enable via config (`tui.disk_cache: true`) or `--disk-cache` flag.
TTL configurable (default 10 minutes for disk, same as `--cache-ttl` floor).

**Architecture:** implement as another `Service` wrapper (`diskCacheService`)
in `internal/githubapi/`, similar to `cacheService`. Stack:
`productionService → diskCacheService → memoryCacheService`.
`diskCacheService` reads from disk on miss, writes on fetch, respects TTL via
mtime. `ctrl+r` in TUI calls `Invalidate()` which clears both layers.

This keeps the `Service` boundary intact. TUI never reads disk directly.

## Performance: parallel repo fetch

When the TUI opens, it shows the list pane immediately. On drill-in it fetches
repos for that one list. With many lists, navigating one-by-one is slow.

**Speculative prefetch:** after `listsLoadedMsg`, start background fetches for
the first 3 visible lists (likely to be drilled into). Use a bounded
`errgroup` (limit 3). Results stored in a `prefetchCache map[string][]Repository`
on the model. On drill-in, check `prefetchCache` before issuing a new
`loadReposCmd`.

Keep prefetch transparent — it never blocks the UI and its results are
discarded if the in-memory `cacheService` invalidated.

**Files:**
- `internal/tui/model.go`: add `prefetchCache`, `prefetchCmd(svc, listIDs)`.
- `internal/tui/messages.go`: add `prefetchedMsg`.

## Architecture: component split

By v2, `model.go` will be large. Split into focused files:

```
internal/tui/
  app.go          Options, Run
  config.go       Config, LoadConfig
  model.go        root model struct, Init, Update, View
  layout.go       renderLayout, renderHeader, renderFooter, padRight
  pane_lists.go   renderListPane, list-pane key handlers
  pane_repos.go   renderRepoPane, repo-pane key handlers
  pane_preview.go renderPreviewPane
  modal.go        modal struct, all modal types and rendering
  messages.go     all Msg types and Cmd constructors
  sort.go         sort comparators
  keys.go         key bindings
  styles.go       theme struct + built-in themes
  search.go       (or import internal/search)
  model_test.go   root model tests
  layout_test.go  layout geometry tests
  modal_test.go   modal interaction tests
```

This split is mechanical — no behavior change. Do it as the first PR of v2.

## Non-goals for v2

- Keybind customization in config (v2.1+, blocked on upstream bubbles `key`
  package making binding definitions configurable at runtime).
- Multiple GitHub accounts or `--host` switching within a running TUI session.
- Plugin system or scripting hooks.
- Removing or deprecating any CLI subcommand.

## Test additions

- Config loading: missing file returns defaults; unknown key is ignored;
  `--no-config` bypasses file.
- Theme: dark/light/none produce different rendered output (snapshot the
  footer in each theme with color disabled so the test is deterministic).
- Disk cache: cache hit skips network; cache miss fetches and writes; `Invalidate`
  deletes on-disk files.
- Prefetch: after `listsLoadedMsg`, prefetch commands are issued for first N lists.
- Default-switch: bare `gh star-lists` on TTY calls `runTUI`; with `--no-tui`
  falls through to CLI list output; with `--json` falls through regardless.

## Verification

```
make check
make ascii-check
# Config: create ~/.config/gh-star-lists/config.yaml with theme: light,
#   open TUI, confirm light palette.
# Disk cache: open TUI, quit, reopen, verify list pane loads instantly.
# Default switch: gh star-lists (TTY) opens TUI; gh star-lists --json still
#   prints JSON.
# Prefetch: open TUI with network throttled, arrow to second list,
#   confirm it loads faster than first.
```

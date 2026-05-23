# Plan: TUI v2 - Nice-to-have UX

## Context

Focused, Star Lists-specific UX improvements that make the TUI faster to scan and harder to misuse - without growing it into a general GitHub dashboard. Every item reuses an existing `githubapi.Service` method or an existing CLI semantic.

**Prerequisite:** TUI v2 cleanup and performance plans (stable component boundaries, per-pane state, preload/cache behavior).

## Scope

**In** | **Out**
---|---
Pane counts and repo header row | New chrome (separators, faint dividers, full-row highlights)
List detail via existing modal pattern | Permanent preview pane, adaptive (wide/narrow) layout
Searchable list pickers, clearer destructive copy | YAML config, themes, configurable keybindings
"Unlisted" as a virtual TUI list | "All Starred" virtual list, plugin hooks, scripting surface
TUI-only affordances backed by existing CLI actions | New GitHub domain features outside Star Lists
Safer modal confirmations | Changing machine output contracts

## Current state

- Two-pane TUI: list pane, repo pane. `modalRepoDetail` shows repo info on demand.
- Search bar already shows `displayed/total` count when active (`render_list.go`); empty states and loading spinner exist.
- Pickers (`modal_list.go`) are j/k lists capped at 8 visible rows, no filter.
- `parse.go` explicitly rejects `--all` and `--unlisted` for `browse` (deliberate exclusion to date).
- Bulk ops run via `errgroup.SetLimit(5)` with `atomic.Int64` tracking.
- `internal/humanize.ShortAge` shared by TUI and CLI.
- Sort enums use `<Prefix>End` sentinels; cycle via `% int(...End)`.

## Design

- Reuse the modal pattern where a permanent pane is tempting - it already works.
- Add at most one new dimension (`isVirtual bool` on list rows) gated by one predicate (`canMutate(list)`); do not scatter ad-hoc checks across input/modals.
- No layout-mode toggles, no adaptive geometry. The current two-pane split stays.
- No new config; conservative defaults in code.
- Follow CLAUDE.md "Code Review Checklist": shared before specialized, sentinel iotas, concurrency-safe tests, no speculative cache infra.

## Phases

| Phase | Goal | Parallel-with | Depends-on | Files |
|---|---|---|---|---|
| P1 - Pane counts + repo header | Two highest-impact scan cues, nothing else | P2, P3 | none | `render_header.go`, `render_repo.go`, `render_list.go` |
| P2 - List detail modal | Extend `keys.Preview` to lists; mirror `modalRepoDetail` | P1, P3 | none | `modal_repo.go`, `modal_view.go`, `modal.go`, `input.go` |
| P3 - Picker search + safer confirms | Filterable destination pickers, explicit consequences in destructive copy | P1, P2 | none | `modal_list.go`, `modal_view.go`, `modal_update.go` |
| P4 - Unlisted virtual list | One virtual entry; one `isVirtual` flag; one `canMutate` predicate | none | P1, P2, P3 | `cache.go`, `render_list.go`, `input.go`, `messages.go` |

Phase order: (P1 | P2 | P3) → P4.

---

### P1 - Pane counts + repo header

Add pane titles with `(N)` counts on each pane, and a one-line repo column header (name / stars / language) above the repo list. Skip everything else considered for scan polish - separators, full-row highlights, faint dividers - until evidence says they're needed.

```text
Exit criteria:
- List pane shows "Lists (N)" title; repo pane shows "Repos (N)" title.
- Repo pane shows a one-line column header that stays aligned at narrow and wide widths.
- Header band (app name, list name, sort suffix, counts) never wraps.
- Existing render tests updated for the new lines.
```

### P2 - List detail modal

Today: `keys.Preview` opens `modalRepoDetail` only when `active == paneRepo` (`input.go:142-146`). Lists have no equivalent - description, privacy, URL, and last-added are invisible inside the TUI.

All fields already exist on `domain.StarList` (`Name`, `Description`, `IsPrivate`, `RepoCount`, `URL`, `LastAddedAt`), so the modal needs no extra fetch and no GraphQL change.

The work is exactly three things:

1. Add `modalListDetail` to the `kind` iota in `modal.go` (before the `modalEnd` sentinel).
2. Add `newListDetailModal(list domain.StarList)` in `modal_repo.go` (or a new `modal_list_detail.go` if `modal_repo.go` grows past ~150 lines) and a `viewListDetail()` case in `modal_view.go` mirroring `viewRepoDetail`'s structure: title (name + privacy badge), URL, blank, repo count, blank, description, last-added.
3. Drop the `m.active == paneRepo` guard in `input.go:142`; branch on active pane, open the matching modal.

Use `humanize.ShortAge` for `LastAddedAt`. Reuse `styleRepoBadge` for the privacy badge (`private` / `public`).

Do **not** add j/k scrolling - neither detail modal needs it. Content is bounded, `truncateToWidth` already handles long URLs/descriptions. Revisit only if a real list shows up that doesn't fit.

```text
Exit criteria:
- `p` on a focused list opens modalListDetail; `p` on a focused repo still opens modalRepoDetail.
- modalListDetail uses already-loaded StarList fields; no new API call.
- No scrolling code added; long description/URL truncate as in modalRepoDetail.
- Test asserts privacy badge text differs for IsPrivate true vs false.
```

### P3 - Picker search + safer confirms

Add a text-input filter to `modalPickList` (used by add/move/copy/merge destinations). Filter clamps cursor to filtered length. Rewrite destructive modal copy so the user sees the target name and the count of affected repos in one sentence (e.g., `Merge into "Rust" - "Go" will be deleted, 14 repos moved`).

```text
Exit criteria:
- Picker `/` toggles a filter; arrow keys keep working with cursor clamped to filtered subset.
- Delete, merge, copy modals state target name and repo count explicitly.
- Modal tests cover: filter narrowing, empty filter results, cursor clamp, copy strings.
```

### P4 - Unlisted virtual list

Add a single virtual entry "Unlisted" at the top of the list pane, backed by the existing `--unlisted` CLI semantic. Skip "All Starred" - it's better served by `gh star-lists repos --all` piping. Introduce one `isVirtual` field on the list-row model and one `canMutate(list) bool` predicate; every action site that mutates a list calls `canMutate` once. Repo-level actions (open, add-to-list, unstar, preview, sort, search, selection) work as on real lists.

```text
Exit criteria:
- Exactly one virtual entry exists; "All Starred" is not added.
- `canMutate` is the only branch that distinguishes virtual from real (no scattered isVirtual checks).
- Edit/delete/copy/merge are unavailable on the virtual entry and announce why via the status line.
- Real-list behavior is byte-identical to before.
- Loading the virtual list reuses the existing repo-cache path (no parallel cache type).
```

---

## Tests

| Test | What it covers |
|---|---|
| `TestPaneTitlesShowCounts` | Counts in titles fit across widths without wrapping |
| `TestRepoHeaderAligns` | Repo column header aligns with rows at narrow + wide widths |
| `TestListDetailModal` | `p` on a focused list opens modalListDetail with already-loaded fields |
| `TestListDetailPrivacyBadge` | Privacy badge text reflects `IsPrivate` |
| `TestPickerFilter` | Picker filter narrows choices and clamps cursor |
| `TestDestructiveModalCopy` | Delete/merge/copy modals include target name + repo count |
| `TestUnlistedVirtualEntry` | Virtual entry rejects list-mutation actions via `canMutate` |

## Verification

```text
go test ./internal/tui/...
make check
make ascii-check
```

Manual smoke:

```text
go run . browse --mouse
# - Pane titles + repo header visible; widths stable.
# - `p` on a list shows list detail modal; `p` on a repo shows repo detail modal.
# - Add/move/copy pickers filter with `/`; cursor stays in bounds.
# - Delete/merge/copy modals show explicit target + count.
# - "Unlisted" virtual entry exists; edit/delete/copy/merge are unavailable; add-to-list / unstar / open work.
```

## Critical files

- `internal/tui/render_header.go`, `render_repo.go`, `render_list.go` - counts and repo header
- `internal/tui/modal.go`, `modal_repo.go`, `modal_view.go`, `input.go` - list detail modal + pane-aware Preview key
- `internal/tui/modal_list.go` - picker filter
- `internal/tui/input.go` - single `canMutate` gate for the virtual entry

## Reused utilities

- `truncateToWidth`, `padRight`, `padLeft` - text fitting (TUI-side)
- `humanize.ShortAge(value, now)` - relative age, shared with CLI
- `githubapi.Service` - all data and mutations behind the existing boundary
- `bubbles/v2/textinput` - already imported in `modal_list.go`; reuse for picker filter
- Existing CLI semantics: `--unlisted`, add/remove/unstar

## Explicitly out of scope

- "All Starred" virtual entry - use `repos --all` from the CLI
- Permanent preview pane and adaptive (wide/narrow) layout - modal pattern stays
- Visual chrome beyond P1: full-row cursor highlight, separators between rows, faint dividers
- User-defined layouts, themes, keybindings
- Any `gh-dash` style PR/issue/notification surface

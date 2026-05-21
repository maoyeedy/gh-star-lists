# Plan: TUI v1.7 — UI Tweaks

## Goal

Fix three real visual problems in the main repo table — ragged repo names, weak cursor visibility, and the raw `|` pane separator — plus add a faint header row for self-documenting columns. No column reordering, no age column, no structural changes.

**Prerequisite:** TUI v1.6 shipped with cleaned-up cursor motion, search, and navigation-cache tests.

## Current state

Verified against `internal/tui/render_repo.go`, `render_list.go`, `render.go`, `styles.go`, `geometry.go`:

- **Repo names are ragged.** `owner/repo` is rendered as a single styled string (`faint(owner) + faint("/") + normal(repo)`, lines 235–254 of `render_repo.go`). Variable owner length means repo names start at different columns, creating visual jitter when scanning.
- **No full-row cursor.** Active repo row uses a 2-char `> ` prefix (cyan/bold or faint). No background highlight or other unmistakable focus indicator (`render_repo.go:133–142`).
- **Pane separator `|` is unstyled.** The `|` between sidebar and repo pane is a plain string literal (`render.go:82`), not faint. It draws the eye more than the data.
- **No column headers.** The repo table has no header row. Stars have a `★` glyph but no label; no column names exist (`render_header.go` only shows app name + list name).
- **Owner dim is ANSI faint only.** `styleRepoOwner = lipgloss.NewStyle().Faint(true)` (`styles.go:17`). No color fallback for terminals where faint renders near-invisible.
- **Language is far right.** Right-aligned in a 20-char field at row end (`render_repo.go:167–185`). On wide terminals it feels detached from its repo.

### What the review got wrong (excluded)

| Claim | Reality |
|-------|---------|
| "First numeric column is ambiguous" | No age column exists in the main table. Stars have `★` glyph. False. |
| "Sidebar lacks counts" | Counts are already shown right-aligned (`render_list.go:99–101`). False. |
| "Main table starts too far right" | Sidebar is padded to its full width; repo pane starts immediately after `\|` with no gap. Geometry is proportional (30% / 22%). Not observed. |
| "Repeated `\|` creates a visual wall between columns" | `\|` is a pane separator, not between data columns. Each row has exactly one `\|`. Inaccurate description. |
| Review's recommended final design includes an "age" column | v1.4 explicitly removed age from the repo table ("cluttered the right edge without adding scanning value"). Not adding back. |

## Scope

**In** | **Out**
---|---
Fixed-width dim owner prefix for aligned repo names | Full column split (owner as separate column)
Full-row background highlight for active repo cursor | Column reordering
Faint pane separator `|` | Age column in repo table
Faint column header row | Sidebar count changes (already works)
Owner color fallback beyond ANSI faint | Language column repositioning (deferred)
Language column: reduce right-alignment field width from 20 to tighter heuristic | New sort/filter keys

## Design

### W1 — Fixed-width dim owner prefix

Replace the current `faint(owner) + faint("/") + normal(repo)` concatenation with a fixed-width owner field:

```
dim(padRight(owner + "/", ownerColW)) + normal(repo)
```

- `ownerColW` = min(max visible owner length, 22). Computed dynamically from `m.displayedRepos` similar to `ensureRepoWidths()`.
- Owners shorter than `ownerColW` get right-padded with spaces so repo names align.
- Owners longer than `ownerColW` are truncated with `"..."` before the `/`.
- Owner and `/` remain faint. Repo is normal (or bold+cyan when focused).
- This keeps `owner/repo` as one semantic column but aligns repo names.

Visual result:
```
  stars   owner/                 repo                         lang
  1.4k ★  yasirkula/             UnityBezierSolution           C#
  5.9k ★  VeriorPies/            ParrelSync                    C#
  1.1k ★  methusalah/            SplineMesh                    C#
   148 ★  Unity-Technologies/    DynamicResolutionSample       C#
```

### W2 — Full-row cursor highlight

Add a subtle background color to the active repo row instead of relying solely on the `> ` prefix:

- `styleCursorRow = lipgloss.NewStyle().Background(lipgloss.Color("236"))` — dark gray background (adaptable to light/dark themes).
- The entire repo row gets this style when `isCursor && m.active == paneRepo`.
- The `> ` prefix remains but now cues the eye even without scanning to the row start.
- When `m.active != paneRepo`, the cursor row gets a lighter background or no background (existing faint `>` suffices as secondary focus signal).

Implementation approach: wrap the assembled `row` string with the cursor row style, or use lipgloss background on the row. Avoid per-field background styling — style the whole row string.

### W3 — Faint pane separator

Change `separator := "|"` to `separator := styleSeparator.Render("|")` in `render.go:82` (two-column) and `render.go:51` (three-column). `styleSeparator` already exists with `Faint(true)`.

### W4 — Faint column header row

Add a single faint header row at the top of the repo pane (immediately after the search bar, before the first data row):

```
  stars   owner                  repo                          lang
```

- Only render when `showStars` is true (i.e., width >= 30). For narrower terminals, stars header disappears live.
- Header row is always `stylePaneSubtitle` (faint).
- Column widths in the header must match the data column widths exactly.
- No header if the list has no repos or is loading/error.

### W5 — Owner color fallback

Change `styleRepoOwner` from pure `Faint(true)` to a combination that uses a low-contrast gray foreground as fallback:

```
styleRepoOwner = lipgloss.NewStyle().Faint(true).Foreground(lipgloss.Color("8"))
```

(Color "8" is ANSI bright black / gray, which renders as a muted gray on most terminals. Combined with `Faint(true)`, terminals that ignore faint still get a low-contrast owner.)

### W6 — Tighter language column width

Reduce `langWidth` from a fixed 20 to a dynamic computed width based on visible languages (like `ensureRepoWidths()`), clamped to `[8, 16]`. On average, lang names like "C#", "Python", "JavaScript", "TypeScript" fit in 10–12 chars; the current 20 wastes right-side space.

## Work items

### W1 — Fixed-width owner prefix

- Add `cachedOwnerWidth` field to `model` (or extend `ensureRepoWidths` to compute it).
- `ensureRepoWidths()` scans `m.displayedRepos` for max owner length, caps at 22.
- In `renderRepoPane`, compute `ownerPart = padRight(owner+"/", ownerColW)` in a new output segment.
- Style ownerPart with `styleRepoOwner`, repo with existing repo style.
- Update truncation logic: clip `nameRaw` to `nameMaxW` before the `strings.Cut("/")`, but now the owner portion within the clipped width is the owner field.

### W2 — Full-row cursor highlight

- Add `styleCursorRow` style with muted background color.
- After assembling `row` (line 276 of `render_repo.go`), apply background styling when `isCursor`.
- When `m.active != paneRepo`, use lighter/no background variant.
- Test: active row has visible background indicator.

### W3 — Faint pane separator

- In `render.go`: replace `separator := "|"` with `separator := styleSeparator.Render("|")` in both layout branches.
- Verify no regression in mouse click coordinate hit-testing (`handleMouseClick` in `input.go` — uses `calcPaneGeometry` which returns column indices, not styled widths. Lipgloss styling on `|` shouldn't affect column indices since styling is zero-width by lipgloss convention with separator chars, but verify.)

### W4 — Faint column header row

- Add `renderRepoHeader(w int) string` function that outputs a faint row matching data column layout.
- Call it from `renderRepoPane` before the data loop, with matching `h--`.
- Columns: stars header, owner header, repo header, lang header (conditionally per width flags).
- The header line must be excluded from scroll calculations so it stays fixed at top (or simply treat it as reduced height for scrollable rows).

### W5 — Owner color fallback

- Update `styleRepoOwner` in `styles.go` to add `.Foreground(lipgloss.Color("8"))`.

### W6 — Tighter language column width

- Compute dynamic lang width in `ensureRepoWidths()` or similar, clamped `[8, 16]`.
- Remove or replace the hardcoded `langWidth = 20` constant.

## Test additions

| Test | What it covers |
|------|----------------|
| `TestFixedWidthOwner` | Owner field is padded to consistent width; repo names align |
| `TestCursorRowHighlight` | Active repo row has background styling; inactive does not |
| `TestFaintSeparator` | Pane separator rendered with faint style |
| `TestRepoHeaderRow` | Header row present with matching column widths |
| `TestOwnerColorFallback` | `styleRepoOwner` includes foreground color |
| `TestLangWidthDynamic` | Language column width adapts to visible content, clamped to [8,16] |
| Existing tests in `render_repo_test.go` | No regression in truncation, cursor, stars, language rendering |

## Verification

```
make check
make ascii-check
```

Manual smoke:

```
go run . browse
# Verify repo names align in a fixed-width owner column.
# Verify active repo row has visible background highlight.
# Verify pane separator `|` is faint.
# Verify column headers appear faint at top of repo pane.
# Verify owner is muted gray (not invisible) in terminals with poor faint support.
# Verify language column width adapts to content.

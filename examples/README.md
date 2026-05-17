# fzf-browse.sh

Interactive two-pane browser for GitHub Star Lists. Single fzf instance, two-mode state machine.

```
┌────────────────────────────────────────────────────────────────────────────────────────┐
│ List>                                                                                  │
│                                                                       ┌─ Repositories ─┐
│ > Game Dev        12                                                  │                │
│   CLI Tools        5                                                  │ Repo    go-git │
│   Rust libs        8                                                  │ Stars   1.2K  │
│   Go tools        14                                                  │ Lang    Go    │
│                                                                       │ Pushed  3d    │
│                                                                       │ URL     …     │
│                                                                       └────────────────┘
├─ Keys ─────────────────────────────────────────────────────────────────────────────────┤
│ Enter: repos  |  Esc: quit  |  Alt-S: sort (name)                                    │
└────────────────────────────────────────────────────────────────────────────────────────┘
```

## Requirements

- `fzf` with footer support, `gh` (authenticated), `gh star-lists` extension

```sh
bash examples/fzf-browse.sh
# or: chmod +x examples/fzf-browse.sh && ./examples/fzf-browse.sh
```

## Key bindings

**List view** (`List>` prompt)

| Key | Action |
|-----|--------|
| `Enter` | Drill into focused list |
| `Esc` | Quit |
| `Alt-S` | Cycle list sort: name, repo count, recently added, GitHub order |
| `Ctrl-R` | Refresh cached lists and previews |
| `n` | Create a Star List |
| `e` | Edit focused Star List name/description |
| `D` | Delete focused Star List after typed confirmation |
| `c` | Copy focused Star List contents to another list |
| `?` | Toggle preview pane |
| Type | Fuzzy-match list names |

**Repo view** (`Repo>` prompt)

| Key | Action |
|-----|--------|
| `Enter` | Open focused repo in browser, stay in fzf |
| `Esc` | Back to list view |
| `Alt-S` | Cycle repo sort: name, stars, pushed, GitHub order |
| `Ctrl-R` | Refresh cached repos and previews |
| `a` | Add focused repo to another Star List |
| `x` | Remove focused repo from the current Star List |
| `m` | Move focused repo from the current Star List to another |
| `u` | Unstar focused repo after typed confirmation |
| `?` | Toggle preview pane |
| Type | Fuzzy-match repo names |

## Cache

```
$XDG_CACHE_HOME/gh-star-lists/fzf/   (if XDG_CACHE_HOME set)
$HOME/.cache/gh-star-lists/fzf/      (default)
```

- `_lists.<sort>.tsv` — cached list index per sort mode
- `<list-ID>.<sort>.tsv` — repos per list and sort mode, fetched on first focus
- Mutating fzf actions clear the cache automatically. `Ctrl-R` also clears and reloads.

## How it works

Single fzf instance, two-mode state machine tracked via `$FZF_PROMPT`:

| Prompt | Items | Preview |
|--------|-------|---------|
| `List> ` | Star lists sorted by name by default | Repos in focused list (cached `gsl_repos`) |
| `Repo> ` | Repos of drilled list, sorted by full `owner/repo` name by default | Detail for focused repo (ANSI-formatted) |

Mode switch: `Enter` sends `change-prompt(...)+reload(...)+transform-footer(...)` via fzf `transform` action. `Esc` sends reverse from repo view or quits from list view.

Bash functions (`gsl_*`) `export -f`'d. Script forces `SHELL=bash` so functions survive into fzf subshells regardless of login shell.

**TSV columns consumed:**

| Command | Col 1 | Col 4 | Col 5 |
|---------|-------|-------|-------|
| `gh star-lists list --tsv` | Name | LastAddedAt | ID |

| Command | Col 1 | Col 4 | Col 6 | Col 7 |
|---------|-------|-------|-------|-------|
| `gh star-lists repos <id> --tsv` | NameWithOwner | Stars | URL | Language |

## Customization

Edit script directly. Key areas:

### Layout & appearance

fzf invocation at bottom of script. Current defaults:

```sh
  --height=95% --layout=reverse --border --margin=1 --padding=1,2 --info=inline \
  --list-border=none --input-border=line --input-label=' Search ' \
  --footer="$LIST_FOOTER" --footer-border=line --footer-label=' Keys ' \
  --pointer='▸ ' --marker='★ ' --ellipsis='…' \
  --preview-window='right,55%,border-left,wrap,~3' \
  --color='fg:#d7dae0,fg+:#ffffff,bg:#1f2329,bg+:#2b3139' \
  --color='hl:#f2cc60,hl+:#f2cc60' \
  --color='border:#4b5563,label:#b8c0cc,input-border:#5c6674,input-label:#9fb7ff' \
  --color='prompt:#9fb7ff,pointer:#f28fad,marker:#8bd5a9' \
  --color='preview-border:#4b5563,preview-label:#8bd5a9,footer:#8b949e,footer-border:#4b5563,footer-label:#b8c0cc,spinner:#f2cc60,info:#8b949e'
```

Smaller `--height` for less obtrusive overlay. Drop `--color` lines for terminal theme. Tweak `--preview-window` size/position:

```sh
--preview-window='right,70%,border-left,wrap,~3'   # wider preview
--preview-window='left,50%,border-right,wrap,~3'    # left side
--preview-window='down,40%,border-top,wrap,~3'      # below
```

### Preview content

`gsl_preview_list` — formats repo rows in list-mode right pane. It intentionally omits descriptions to keep browsing quiet:

```sh
# Current: Stars  Language  NameWithOwner  Fork marker
printf "\033[33m%7s \342\230\205\033[0m  \033[36m%-12s\033[0m  \033[37m%s\033[0m\033[2m%s\033[0m\n", fmtstars($4), lang, name, fork

# Add description back if you prefer a denser preview:
printf "%-45s  %6s \342\230\205  %s\n", name, fmtstars($4), d
```

`gsl_preview_repo` — formats repo detail in repo-mode right pane. Emits ANSI for hierarchy and label coloring.

TSV repo columns: `1=NameWithOwner 2=Desc 3=IsFork 4=Stars 5=PushedAt 6=URL 7=Language`

### Sort lists and repos

The browser sorts Star Lists by name by default. In list view, `Alt-S` cycles through:

| Mode | Command behavior |
|------|------------------|
| `name` | `gh star-lists list --tsv --sort name` |
| `repos` | `gh star-lists list --tsv --sort repos --desc` |
| `added` | `gh star-lists list --tsv --sort added --desc` |
| `github` | `gh star-lists list --tsv` |

Repos are sorted by full `owner/repo` name by default. In repo view, `Alt-S` cycles through:

| Mode | Command behavior |
|------|------------------|
| `name` | `gh star-lists repos <id> --tsv --sort name` |
| `stars` | `gh star-lists repos <id> --tsv --sort stars --desc` |
| `pushed` | `gh star-lists repos <id> --tsv --sort pushed --desc` |
| `github` | `gh star-lists repos <id> --tsv` |

To change the repo default, edit `gsl_repo_sort_mode`:

```sh
gsl_repo_sort_mode() {
  local mode
  mode=$(cat "$REPO_SORT_FILE" 2>/dev/null || true)
  case "$mode" in
    name|stars|pushed|github) printf '%s' "$mode" ;;
    *) printf 'stars' ;;
  esac
}
```

### Filter repos by language on drill

```sh
gsl_drill() {
  local id="$1" f="$CACHE_DIR/$id.tsv"
  printf '%s' "$id" >"$STATE_FILE"
  [[ -s "$f" ]] || gh star-lists repos "$id" --tsv >"$f" 2>/dev/null
  awk -F'\t' 'tolower($7) == "go"' "$f"
}
```

### Change cache location

```sh
CACHE_DIR="$HOME/.local/share/gh-star-lists-fzf"
```

### Disable disk cache (always fetch fresh)

```sh
gsl_repos() { gh star-lists repos "$1" --tsv; }
gsl_lists() { gh star-lists list --tsv; }
```

### Cap large lists with --limit

```sh
gsl_repos() {
  local id="$1" f="$CACHE_DIR/$id.tsv"
  [[ -s "$f" ]] || gh star-lists repos "$id" --tsv --limit 100 >"$f" 2>/dev/null
  cat "$f"
}
```

### Custom preview: bat / glow for README

```sh
gsl_preview_repo() {
  local name="$1"
  gh repo view "$name" --json description,stargazerCount,primaryLanguage,url \
    --template '{{.stargazerCount}} ★  {{.primaryLanguage.name}}  {{.url}}
{{.description}}' 2>/dev/null
}
```

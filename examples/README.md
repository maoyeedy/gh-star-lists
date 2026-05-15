# fzf-browse.sh

Interactive two-pane browser for GitHub Star Lists. Single fzf instance, two-mode state machine.

```
┌─ Lists ── Enter: drill ── Alt-W: open in browser ── Alt-R: refresh ── Alt-H: quit ────┐
│ List>                                                                                  │
│                                                                       ┌─ Repositories ─┐
│ > Game Dev        12                                                  │                │
│   CLI Tools        5                                                  │ Repo    go-git │
│   Rust libs        8                                                  │ Stars   1.2K  │
│   Go tools        14                                                  │ Lang    Go    │
│                                                                       │ Pushed  3d    │
│                                                                       │ URL     …     │
│                                                                       └────────────────┘
└────────────────────────────────────────────────────────────────────────────────────────┘
  ▸ Lists  |  Enter: drill  |  Alt-W: open in browser  |  Alt-R: refresh  |  Alt-H: quit
```

## Requirements

- `fzf` (recent), `gh` (authenticated), `gh star-lists` extension

```sh
bash examples/fzf-browse.sh
# or: chmod +x examples/fzf-browse.sh && ./examples/fzf-browse.sh
```

## Key bindings

**List view** (`List>` prompt)

| Key | Action |
|-----|--------|
| `Enter` | Drill into focused list |
| `Alt-W` | Open focused list page in browser |
| `Alt-R` | Clear cache, reload lists from GitHub |
| `Alt-H` / `Esc` | Quit |
| `?` | Toggle preview pane |
| Type | Fuzzy-match list names |

**Repo view** (`Repo>` prompt)

| Key | Action |
|-----|--------|
| `Enter` | Open focused repo in browser, exit fzf |
| `Alt-O` | Open focused repo in browser, stay in fzf |
| `Alt-R` | Clear cache, reload repos from GitHub |
| `Alt-H` / `Esc` | Back to list view |
| `?` | Toggle preview pane |
| Type | Fuzzy-match repo names |

## Cache

```
$XDG_CACHE_HOME/gh-star-lists/fzf/   (if XDG_CACHE_HOME set)
$HOME/.cache/gh-star-lists/fzf/      (default)
```

- `_lists.tsv` — cached list index
- `<list-ID>.tsv` — repos per list, fetched on first focus
- `Ctrl-R` clears all, or `rm -rf ~/.cache/gh-star-lists/fzf`

## How it works

Single fzf instance, two-mode state machine tracked via `$FZF_PROMPT`:

| Prompt | Items | Preview |
|--------|-------|---------|
| `List> ` | Star lists from `gsl_lists` | Repos in focused list (cached `gsl_repos`) |
| `Repo> ` | Repos of drilled list | Detail for focused repo (ANSI-formatted) |

Mode switch: `Enter` sends `change-prompt(...)+reload(...)+transform-header(...)` via fzf `transform` action. `Alt-H` sends reverse.

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
  --pointer='▸ ' --marker='★ ' --ellipsis='…' \
  --preview-window='right,60%,border-left,nowrap,~3' \
  --color='fg:#d0d0d0,fg+:#ffffff,bg:#1e1e2e,bg+:#313244' \
  --color='hl:#f9e2af,hl+:#f9e2af' \
  --color='border:#45475a,label:#a6adc8' \
  --color='prompt:#89b4fa,pointer:#f38ba8,marker:#a6e3a1' \
  --color='header:#6c7086,spinner:#f9e2af,info:#6c7086'
```

Smaller `--height` for less obtrusive overlay. Drop `--color` lines for terminal theme. Tweak `--preview-window` size/position:

```sh
--preview-window='right,70%,border-left,nowrap,~3'   # wider preview
--preview-window='left,50%,border-right,nowrap,~3'    # left side
--preview-window='down,40%,border-top,nowrap,~3'      # below
```

### Preview content

`gsl_preview_list` — formats repo rows in list-mode right pane:

```sh
# Current: NameWithOwner  Stars  Description
printf "%-45s  %6s \342\230\205  %s\n", name, fmtstars($4), d

# Add language and fork:
printf "%-45s  %6s \342\230\205  %-12s  %s  %s\n", name, fmtstars($4), $7, $3, d
```

`gsl_preview_repo` — formats repo detail in repo-mode right pane. Emits ANSI for label coloring.

TSV repo columns: `1=NameWithOwner 2=Desc 3=IsFork 4=Stars 5=PushedAt 6=URL 7=Language`

### Sort repos in preview

Override `gsl_preview_list` or `gsl_drill`:

```sh
gsl_drill() {
  printf '%s' "$1" >"$STATE_FILE"
  gh star-lists repos "$1" --tsv --sort stars --desc | tee "$CACHE_DIR/$1.tsv"
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

### Enter stays in fzf (swap Enter / Alt-O behavior)

In `ENTER` variable, change:

```sh
  echo "execute-silent(gsl_open_repo {1})+accept"
# to:
  echo "execute-silent(gsl_open_repo {1})"
```

### Rebind keys

```sh
--bind="ctrl-o:transform:$ALT_O"   # move "open+stay" to Ctrl-O
--bind="ctrl-l:transform:$ALT_W"   # move "list page" to Ctrl-L
--bind="alt-o:ignore"              # remove old binding
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

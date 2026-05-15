#!/usr/bin/env bash
# Interactive fzf browser for gh-star-lists.
#
# Left pane: star lists. Right pane: repos in focused list (live on arrow-key,
# cached on disk after first fetch). Enter drills into a list; enter on a repo
# opens it in the browser; esc goes back to lists.
#
# Requires: fzf, gh (authenticated as a gh extension via `gh star-lists`)
#
# Keys
#   Enter    drill into list / open repo in browser (exits fzf)
#   Alt-O    open repo in browser, stay in fzf
#   Alt-W    open focused list page in browser
#   Alt-R    clear on-disk cache and refresh current view
#   Alt-H    back (repo mode) / quit (list mode)
#   ?        toggle preview pane

set -euo pipefail

# Sandbox: ignore user's fzf config to avoid binding/preview collisions.
unset FZF_DEFAULT_COMMAND FZF_DEFAULT_OPTS _ZO_FZF_OPTS

# --- pre-flight -----------------------------------------------------------------

for cmd in fzf gh awk; do
  command -v "$cmd" >/dev/null || { printf 'missing dependency: %s\n' "$cmd" >&2; exit 1; }
done
gh auth status >/dev/null 2>&1 || { printf 'not authenticated — run: gh auth login\n' >&2; exit 1; }

# Force fzf to use bash so export -f functions survive into fzf subshells
# regardless of the user's login shell ($SHELL is the login shell, not this script's).
SHELL=$(command -v bash)
export SHELL

# --- disk cache -----------------------------------------------------------------
# Repos for each list are cached by list ID. Survives across arrow-key navigations
# (only the first focus hits the network). The list index itself is also cached.
# alt-r clears all caches and re-fetches.

CACHE_DIR="${XDG_CACHE_HOME:-$HOME/.cache}/gh-star-lists/fzf"
STATE_FILE="$CACHE_DIR/.state"   # holds current list ID while in repo view
LISTS_CACHE="$CACHE_DIR/_lists.tsv"
mkdir -p "$CACHE_DIR"
export CACHE_DIR STATE_FILE LISTS_CACHE

# --- helper functions (exported for fzf subshells) ------------------------------

# Fetch repos for a list ID, writing to cache on first call.
gsl_repos() {
  local id="$1" f
  f="$CACHE_DIR/$id.tsv"
  if [[ ! -s "$f" ]]; then
    gh star-lists repos "$id" --tsv >"$f" 2>/dev/null || { rm -f "$f"; return 1; }
  fi
  cat "$f"
}

# Fetch the list index, writing to cache on first call.
gsl_lists() {
  if [[ ! -s "$LISTS_CACHE" ]]; then
    gh star-lists list --tsv >"$LISTS_CACHE" 2>/dev/null || { rm -f "$LISTS_CACHE"; return 1; }
  fi
  cat "$LISTS_CACHE"
}

# Switch to repo view: record current list ID, stream repos to fzf.
gsl_drill() {
  printf '%s' "$1" >"$STATE_FILE"
  gsl_repos "$1"
}

# Switch back to list view.
gsl_back() {
  printf '' >"$STATE_FILE"
  gsl_lists
}

# Open a repo by NameWithOwner (col 1 of repos TSV).
gsl_open_repo() {
  gh repo view "$1" --web
}

# Preview: repos inside a focused list (col 5 of lists TSV = list ID).
# TSV columns for lists: 1=Name 2=Desc 3=RepoCount 4=LastAddedAt 5=ID 6=URL
gsl_preview_list() {
  gsl_repos "$1" | awk -F'\t' '
    function fmtstars(n) {
      if (n >= 1000000) return sprintf("%.1fM", n / 1000000)
      if (n >= 1000)    return sprintf("%.1fK", n / 1000)
      return sprintf("%d", n)
    }
    {
      name = $1
      if (length(name) > 42) name = substr(name, 1, 39) "..."
      d = $2
      if (length(d) > 50) d = substr(d, 1, 47) "..."
      printf "%-45s  %6s \342\230\205  %s\n", name, fmtstars($4), d
    }
    END { if (NR == 0) print "(empty list)" }'
}

# Preview: detail block for a focused repo (col 1 of repos TSV = NameWithOwner).
# TSV columns for repos: 1=NameWithOwner 2=Desc 3=IsFork 4=Stars 5=PushedAt 6=URL 7=Lang
gsl_preview_repo() {
  local name="$1" id f
  id=$(cat "$STATE_FILE" 2>/dev/null) || { printf '\033[2m(no state)\033[0m\n'; return; }
  f="$CACHE_DIR/$id.tsv"
  [[ -s "$f" ]] || { printf '\033[2m(cache miss \342\200\224 press Alt-R)\033[0m\n'; return; }
  awk -F'\t' -v n="$name" '
    function fmtstars(n) {
      if (n >= 1000000) return sprintf("%.1fM", n / 1000000)
      if (n >= 1000)    return sprintf("%.1fK", n / 1000)
      return sprintf("%d", n)
    }
    $1 == n {
      desc = $2
      if (length(desc) > 70) desc = substr(desc, 1, 67) "..."
      printf "\033[1;34m%-8s\033[0m \033[37m%s\033[0m\n", "Repo",   $1
      printf "\033[1;34m%-8s\033[0m %s\n",                "Desc",   desc
      printf "\033[1;34m%-8s\033[0m %s\n",                "Stars",  fmtstars($4)
      printf "\033[1;34m%-8s\033[0m %s\n",                "Lang",   $7
      printf "\033[1;34m%-8s\033[0m %s\n",                "Pushed", $5
      printf "\033[1;34m%-8s\033[0m %s\n",                "Fork",   $3
      printf "\033[1;34m%-8s\033[0m \033[2m%s\033[0m\n",  "URL",    $6
      exit
    }' "$f"
}

# Cleanup on exit.
cleanup() { rm -f "$STATE_FILE"; }
trap cleanup EXIT

export -f gsl_repos gsl_lists gsl_drill gsl_back gsl_open_repo gsl_preview_list gsl_preview_repo

# --- fzf prompt names -----------------------------------------------------------
# Trailing space is intentional: fzf includes it in $FZF_PROMPT.

P_LIST='List> '
P_REPO='Repo> '
export P_LIST P_REPO

# --- fzf action strings ---------------------------------------------------------
# Single-quoted so $P_LIST/$P_REPO/$CACHE_DIR/$STATE_FILE are NOT expanded here;
# they expand at runtime inside fzf's bash subshell (where they are env-exported).
# fzf substitutes {1} / {5} / {} BEFORE the subshell runs, so they arrive as
# literal values.

# Preview panel: repos of focused list (list mode) or repo detail (repo mode).
PREVIEW='
if [[ "$FZF_PROMPT" == "$P_LIST" ]]; then
  gsl_preview_list {5}
else
  gsl_preview_repo {1}
fi
'

# Enter: drill in list mode; open repo URL and exit in repo mode.
ENTER='
if [[ "$FZF_PROMPT" == "$P_LIST" ]]; then
  echo "change-prompt($P_REPO)+reload(gsl_drill {5})+transform-header(printf \"%s\" \"  Repo mode  |  Enter: open in browser  |  Alt-O: open + stay  |  Alt-R: refresh  |  Alt-H: back  |  ?: toggle preview  \")"
else
  echo "execute-silent(gsl_open_repo {1})+accept"
fi
'

# Alt-H: quit in list mode; back to lists in repo mode (same role as Esc).
ESC='
if [[ "$FZF_PROMPT" == "$P_LIST" ]]; then
  echo "abort"
else
  echo "change-prompt($P_LIST)+reload(gsl_back)+transform-header(printf \"%s\" \"  Lists  |  Enter: drill  |  Alt-W: open in browser  |  Alt-R: refresh  |  Alt-H: quit  |  ?: toggle preview  \")"
fi
'

# Alt-R: clear cache and re-fetch current view.
RELOAD='
if [[ "$FZF_PROMPT" == "$P_LIST" ]]; then
  rm -f "$CACHE_DIR"/*.tsv
  echo "reload(gsl_lists)"
else
  id=$(cat "$STATE_FILE" 2>/dev/null)
  [[ -n "$id" ]] || exit 0
  rm -f "$CACHE_DIR"/*.tsv
  echo "reload(gsl_repos $id)"
fi
'

# Alt-W: open focused list page in browser (list mode only; noop in repo mode).
ALT_W='
if [[ "$FZF_PROMPT" == "$P_LIST" ]]; then
  echo "execute-silent(gh star-lists repos {5} --web)"
fi
'

# Alt-O: open focused repo in browser, stay in fzf (repo mode only).
ALT_O='
if [[ "$FZF_PROMPT" == "$P_REPO" ]]; then
  echo "execute-silent(gsl_open_repo {1})"
fi
'

# --- launch ---------------------------------------------------------------------

fzf \
  --delimiter $'\t' \
  --with-nth=1 \
  --prompt="$P_LIST" \
  --header='  Lists  |  Enter: drill  |  Alt-W: open in browser  |  Alt-R: refresh  |  Alt-H: quit  |  ?: toggle preview  ' \
  --height=95% \
  --layout=reverse \
  --border \
  --margin=1 \
  --padding=1,2 \
  --info=inline \
  --pointer='▸ ' \
  --marker='★ ' \
  --ellipsis='…' \
  --color='fg:#d0d0d0,fg+:#ffffff,bg:#1e1e2e,bg+:#313244' \
  --color='hl:#f9e2af,hl+:#f9e2af' \
  --color='border:#45475a,label:#a6adc8' \
  --color='prompt:#89b4fa,pointer:#f38ba8,marker:#a6e3a1' \
  --color='header:#6c7086,spinner:#f9e2af,info:#6c7086' \
  --preview="$PREVIEW" \
  --preview-window='right,60%,border-left,nowrap,~3' \
  --preview-label=' Repositories ' \
  --bind="focus:transform-preview-label:[[ \$FZF_PROMPT == \"\$P_LIST\" ]] && printf ' Repositories ' || printf ' Repository detail '" \
  --bind="enter:transform:$ENTER" \
  --bind="alt-h:transform:$ESC" \
  --bind="start:reload(gsl_lists)" \
  --bind="alt-r:transform:$RELOAD" \
  --bind="alt-w:transform:$ALT_W" \
  --bind="alt-o:transform:$ALT_O" \
  --bind='?:toggle-preview'

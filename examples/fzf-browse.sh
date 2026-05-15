#!/usr/bin/env bash
# Interactive fzf browser for gh-star-lists.
#
# Left pane: star lists. Right pane: repos in focused list (live on arrow-key,
# cached on disk after first fetch). Enter drills into a list; enter on a repo
# opens it in the browser while staying in fzf; esc goes back to lists.
#
# Requires: fzf, gh (authenticated as a gh extension via `gh star-lists`)
#
# Keys
#   Enter    drill into list / open repo in browser
#   Esc      back (repo mode) / quit (list mode)
#   Alt-S    cycle sort mode
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

CACHE_DIR="${XDG_CACHE_HOME:-$HOME/.cache}/gh-star-lists/fzf"
STATE_FILE="$CACHE_DIR/.state"   # holds current list ID while in repo view
LIST_SORT_FILE="$CACHE_DIR/.list-sort"
REPO_SORT_FILE="$CACHE_DIR/.repo-sort"
mkdir -p "$CACHE_DIR"
export CACHE_DIR STATE_FILE LIST_SORT_FILE REPO_SORT_FILE

# --- helper functions (exported for fzf subshells) ------------------------------

# Current list sort mode. Defaults to Star List name ordering.
gsl_list_sort_mode() {
  local mode
  mode=$(cat "$LIST_SORT_FILE" 2>/dev/null || true)
  case "$mode" in
    name|repos|added|github) printf '%s' "$mode" ;;
    *) printf 'name' ;;
  esac
}

# Current repo sort mode. Defaults to full owner/repo name ordering.
gsl_repo_sort_mode() {
  local mode
  mode=$(cat "$REPO_SORT_FILE" 2>/dev/null || true)
  case "$mode" in
    name|stars|pushed|github) printf '%s' "$mode" ;;
    *) printf 'name' ;;
  esac
}

# Fetch repos for a list ID and sort mode, writing to cache on first call.
gsl_repos() {
  local id="$1" mode="${2:-}" f
  [[ -n "$mode" ]] || mode=$(gsl_repo_sort_mode)
  f="$CACHE_DIR/$id.$mode.tsv"
  if [[ ! -s "$f" ]]; then
    case "$mode" in
      name)
        gh star-lists repos "$id" --tsv --sort name >"$f" 2>/dev/null || { rm -f "$f"; return 1; }
        ;;
      stars)
        gh star-lists repos "$id" --tsv --sort stars --desc >"$f" 2>/dev/null || { rm -f "$f"; return 1; }
        ;;
      pushed)
        gh star-lists repos "$id" --tsv --sort pushed --desc >"$f" 2>/dev/null || { rm -f "$f"; return 1; }
        ;;
      github)
        gh star-lists repos "$id" --tsv >"$f" 2>/dev/null || { rm -f "$f"; return 1; }
        ;;
      *)
        return 1
        ;;
    esac
  fi
  cat "$f"
}

# Fetch the list index, writing to cache on first call.
gsl_lists() {
  local mode="${1:-}" f
  [[ -n "$mode" ]] || mode=$(gsl_list_sort_mode)
  f="$CACHE_DIR/_lists.$mode.tsv"
  if [[ ! -s "$f" ]]; then
    case "$mode" in
      name)
        gh star-lists list --tsv --sort name >"$f" 2>/dev/null || { rm -f "$f"; return 1; }
        ;;
      repos)
        gh star-lists list --tsv --sort repos --desc >"$f" 2>/dev/null || { rm -f "$f"; return 1; }
        ;;
      added)
        gh star-lists list --tsv --sort added --desc >"$f" 2>/dev/null || { rm -f "$f"; return 1; }
        ;;
      github)
        gh star-lists list --tsv >"$f" 2>/dev/null || { rm -f "$f"; return 1; }
        ;;
      *)
        return 1
        ;;
    esac
  fi
  cat "$f"
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

# Footer: repo mode shows the active domain sort, not fzf's fuzzy ranking.
gsl_list_footer() {
  printf ' Enter: repos  |  Esc: quit  |  Alt-S: sort (%s) ' "$(gsl_list_sort_mode)"
}

gsl_repo_footer() {
  printf ' Enter: open  |  Esc: lists  |  Alt-S: sort (%s) ' "$(gsl_repo_sort_mode)"
}

# Cycle list sort modes while staying in list view.
gsl_cycle_list_sort() {
  local current next
  current=$(gsl_list_sort_mode)
  case "$current" in
    name) next="repos" ;;
    repos) next="added" ;;
    added) next="github" ;;
    *) next="name" ;;
  esac
  printf '%s' "$next" >"$LIST_SORT_FILE"
  printf 'reload(gsl_lists %s)+transform-footer(gsl_list_footer)+first' "$next"
}

# Cycle repo sort modes while staying in repo view.
gsl_cycle_sort() {
  local id current next
  id=$(cat "$STATE_FILE" 2>/dev/null) || return 0
  [[ -n "$id" ]] || return 0
  current=$(gsl_repo_sort_mode)
  case "$current" in
    name) next="stars" ;;
    stars) next="pushed" ;;
    pushed) next="github" ;;
    *) next="name" ;;
  esac
  printf '%s' "$next" >"$REPO_SORT_FILE"
  printf 'reload(gsl_repos %s %s)+transform-footer(gsl_repo_footer)+first' "$id" "$next"
}

# Preview: repos inside a focused list (col 5 of lists TSV = list ID).
# TSV columns for lists: 1=Name 2=Desc 3=RepoCount 4=LastAddedAt 5=ID 6=URL
gsl_preview_list() {
  gsl_repos "$1" | awk -F'\t' -v title="$2" -v repo_count="$3" '
    function fmtstars(n) {
      if (n >= 1000000) return sprintf("%.1fM", n / 1000000)
      if (n >= 1000)    return sprintf("%.1fK", n / 1000)
      return sprintf("%d", n)
    }
    BEGIN {
      if (title != "") {
        if (length(title) > 56) title = substr(title, 1, 53) "..."
        printf "\033[1;36m%s\033[0m\n", title
        if (repo_count != "") {
          printf "\033[2mRepos in this list: %s\033[0m\n\n", repo_count
        } else {
          printf "\033[2mRepos in this list\033[0m\n\n"
        }
      }
    }
    {
      name = $1
      if (length(name) > 48) name = substr(name, 1, 45) "..."
      lang = $7
      if (lang == "") lang = "-"
      if (length(lang) > 12) lang = substr(lang, 1, 9) "..."
      fork = ($3 == "true") ? " fork" : ""
      printf "\033[33m%7s \342\230\205\033[0m  \033[36m%-12s\033[0m  \033[37m%s\033[0m\033[2m%s\033[0m\n", fmtstars($4), lang, name, fork
    }
    END { if (NR == 0) print "\033[2m(empty list)\033[0m" }'
}

# Preview: detail block for a focused repo (col 1 of repos TSV = NameWithOwner).
# TSV columns for repos: 1=NameWithOwner 2=Desc 3=IsFork 4=Stars 5=PushedAt 6=URL 7=Lang
gsl_preview_repo() {
  local name="$1" id mode f
  id=$(cat "$STATE_FILE" 2>/dev/null) || { printf '\033[2m(no state)\033[0m\n'; return; }
  mode=$(gsl_repo_sort_mode)
  f="$CACHE_DIR/$id.$mode.tsv"
  [[ -s "$f" ]] || { printf '\033[2m(cache miss)\033[0m\n'; return; }
  awk -F'\t' -v n="$name" '
    function fmtstars(n) {
      if (n >= 1000000) return sprintf("%.1fM", n / 1000000)
      if (n >= 1000)    return sprintf("%.1fK", n / 1000)
      return sprintf("%d", n)
    }
    $1 == n {
      desc = $2
      if (desc == "") desc = "(no description)"
      if (length(desc) > 100) desc = substr(desc, 1, 97) "..."
      lang = $7
      if (lang == "") lang = "-"
      fork = ($3 == "true") ? "fork" : "source"
      printf "\033[1;36m%s\033[0m\n", $1
      printf "\033[2m%s\033[0m\n\n", $6
      printf "\033[33m%s \342\230\205\033[0m  \033[36m%s\033[0m  \033[2m%s  pushed %s\033[0m\n\n", fmtstars($4), lang, fork, $5
      printf "\033[1;34mDescription\033[0m\n%s\n", desc
      exit
    }' "$f"
}

# Cleanup on exit.
cleanup() { rm -f "$STATE_FILE" "$LIST_SORT_FILE" "$REPO_SORT_FILE"; }
trap cleanup EXIT

export -f gsl_list_sort_mode gsl_repo_sort_mode gsl_repos gsl_lists gsl_drill gsl_back gsl_open_repo gsl_list_footer gsl_repo_footer gsl_cycle_list_sort gsl_cycle_sort gsl_preview_list gsl_preview_repo

# --- fzf prompt names -----------------------------------------------------------
# Trailing space is intentional: fzf includes it in $FZF_PROMPT.

P_LIST='List> '
P_REPO='Repo> '
LIST_FOOTER=$(gsl_list_footer)
export P_LIST P_REPO LIST_FOOTER

# --- fzf action strings ---------------------------------------------------------
# Single-quoted so $P_LIST/$P_REPO/$CACHE_DIR/$STATE_FILE are NOT expanded here;
# they expand at runtime inside fzf's bash subshell (where they are env-exported).
# fzf substitutes {1} / {5} / {} BEFORE the subshell runs, so they arrive as
# literal values.

# Preview panel: repos of focused list (list mode) or repo detail (repo mode).
PREVIEW='
if [[ "$FZF_PROMPT" == "$P_LIST" ]]; then
  gsl_preview_list {5} {1} {3}
else
  gsl_preview_repo {1}
fi
'

# Enter: drill in list mode; open repo URL in repo mode.
ENTER='
if [[ "$FZF_PROMPT" == "$P_LIST" ]]; then
  echo "change-prompt($P_REPO)+reload(gsl_drill {5})+transform-footer(gsl_repo_footer)"
else
  echo "execute-silent(gsl_open_repo {1})"
fi
'

# Esc: quit in list mode; back to lists in repo mode.
BACK_OR_QUIT='
if [[ "$FZF_PROMPT" == "$P_LIST" ]]; then
  echo "abort"
else
  echo "change-prompt($P_LIST)+reload(gsl_back)+transform-footer(gsl_list_footer)"
fi
'

# Alt-S: cycle domain sort order in the current mode.
SORT='
if [[ "$FZF_PROMPT" == "$P_LIST" ]]; then
  gsl_cycle_list_sort
elif [[ "$FZF_PROMPT" == "$P_REPO" ]]; then
  gsl_cycle_sort
fi
'

# --- launch ---------------------------------------------------------------------

fzf \
  --delimiter $'\t' \
  --with-nth=1 \
  --prompt="$P_LIST" \
  --footer="$LIST_FOOTER" \
  --footer-border=line \
  --footer-label=' Keys ' \
  --height=95% \
  --layout=reverse \
  --border \
  --list-border=none \
  --input-border=line \
  --input-label=' Search ' \
  --margin=1 \
  --padding=1,2 \
  --info=inline \
  --pointer='▸ ' \
  --marker='★ ' \
  --ellipsis='…' \
  --color='fg:#d7dae0,fg+:#ffffff,bg:#1f2329,bg+:#2b3139' \
  --color='hl:#f2cc60,hl+:#f2cc60' \
  --color='border:#4b5563,label:#b8c0cc,input-border:#5c6674,input-label:#9fb7ff' \
  --color='prompt:#9fb7ff,pointer:#f28fad,marker:#8bd5a9' \
  --color='preview-border:#4b5563,preview-label:#8bd5a9,footer:#8b949e,footer-border:#4b5563,footer-label:#b8c0cc,spinner:#f2cc60,info:#8b949e' \
  --preview="$PREVIEW" \
  --preview-window='right,55%,border-left,wrap,wrap-word,~3' \
  --preview-wrap-sign='' \
  --preview-label=' Repositories ' \
  --bind="focus:transform-preview-label:[[ \$FZF_PROMPT == \"\$P_LIST\" ]] && printf ' Repositories ' || printf ' Repository detail '" \
  --bind="enter:transform:$ENTER" \
  --bind="esc:transform:$BACK_OR_QUIT" \
  --bind="alt-s:transform:$SORT" \
  --bind="start:reload(gsl_lists)" \
  --bind='?:toggle-preview'

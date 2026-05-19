# gh-star-lists

Explore GitHub Star Lists from the terminal: inspect lists, sort/filter repositories, export scriptable output, and safely manage list membership through your existing `gh` authentication.

## Install

```sh
gh extension install maoyeedy/gh-star-lists
```

## Prerequisites

- GitHub CLI (`gh`) installed and authenticated
- A GitHub account with Star Lists enabled

```sh
gh auth status
gh auth login
```

The extension uses the user's existing `gh` authentication context. It does not store tokens.

## Commands

```
gh star-lists [list] [FLAGS]
gh star-lists repos <LIST_ID_OR_NAME> [FLAGS]
```

```sh
gh star-lists                                      # human table (default)
gh star-lists --sort name --desc --limit 10
gh star-lists list --sort repos --desc             # sort by repo count
gh star-lists repos "Game Dev"                     # by name (case-insensitive)
gh star-lists repos                                # interactive list picker in a TTY
gh star-lists repos UL_kwDOExample --sort stars --desc
gh star-lists repos UL_kwDOExample --search "github cli"
gh star-lists repos UL_kwDOExample --filter name:go --filter fork:false
gh star-lists repos UL_kwDOExample --filter language:Go
gh star-lists repos UL_kwDOExample --filter license:mit --filter min-stars:100
gh star-lists repos UL_kwDOExample --web           # open in browser
gh star-lists repos --all --sort starred --desc    # all starred repos
gh star-lists repos --unlisted --sort starred      # starred not in any list
gh star-lists --host ghe.example.com               # use a specific gh-authenticated host
gh star-lists create "Go Tools" --description "Useful Go repos"
gh star-lists create                               # interactive create wizard in a TTY
gh star-lists add cli/cli --to "CLI Tools"
gh star-lists add cli/cli                          # interactive target list picker in a TTY
gh star-lists move cli/cli --from Inbox --to "CLI Tools" --yes
gh star-lists mv cli/cli --from Inbox --to "CLI Tools" --yes
gh star-lists copy --from Inbox --to Archive
gh star-lists delete Inbox --yes

## Interactive Use

Omit optional inputs in a TTY for interactive prompts:

```
gh star-lists repos                  # pick a list interactively
gh star-lists create                 # prompted: name → description → visibility
gh star-lists edit <LIST>            # multi-select fields to update
gh star-lists add <REPO>             # pick target list
gh star-lists move <REPO>            # pick source and target lists
```

## Default Output

**Lists** (`gh star-lists`)

```
NAME      REPOS  ADDED    ID                   URL
Game Dev  12     6d ago   UL_kwDOExample_AA    https://github.com/stars/username/lists/game-dev
CLI Tools 5      1mo ago  UL_kwDOExample_BB    https://github.com/stars/username/lists/cli-tools
```

**Repos** (`gh star-lists repos <ID>`)

```
REPOSITORY                          STARS  LANG    FORK  PUSHED   URL
RichysHub/MagicaVoxel-VOX-importer  233    Python  no    4y ago   https://github.com/RichysHub/MagicaVoxel-VOX-importer
Naxela/The_Lightmapper              797    Python  no    23d ago  https://github.com/Naxela/The_Lightmapper
```

## Flags

**Sort**

- `-s`, `--sort <KEY[:asc|desc]>` — comma-separated for multi-key; repeatable
- `-d`, `--desc` — descending order for sort keys without an explicit direction

| Context | Keys |
|---------|------|
| lists | `added` `name` `repos` |
| repos | `name` `stars` `pushed` `language` `starred` |

**Filter**

- `-f`, `--filter <KEY:VALUE>` — repeatable; all keys except `name` are repos-only
- `-S`, `--search <QUERY>` — repos-only fuzzy ranking across repository name, description, and language

| Key | Accepts |
|-----|---------|
| `name` | case-insensitive substring |
| `fork` | `true` / `false` |
| `language` | case-insensitive |
| `archived` | `true` / `false` |
| `license` | SPDX key, case-insensitive |
| `min-stars` / `max-stars` | integer |
| `topic` | exact topic name, case-insensitive |

**Output**

- `--json` — JSON
- `--jq <EXPR>` — filter JSON output with a jq expression
- `--tsv` — TSV
- `--plain` — multiline detail view
- `--template <STR>` — Go template
- `--output <FILE>` — write to file
- `--no-color` — disable color

**Other**

- `-l`, `--limit <N>` — max results
- `--cache-ttl <DURATION>` — override the in-memory cache TTL (default: 5m); pass `0` or use `--no-cache` to disable
- `--no-cache` — disable response caching for this invocation
- `-w`, `--web` — open in browser
- `--all` — all starred repositories
- `--unlisted` — starred repos not in any list
- `--host <HOST>` — use a specific GitHub host from `gh auth`

**Write commands**

- `create <NAME> [-D, --description <TEXT>] [--private]`; omit `<NAME>` in a TTY for prompts
- `edit <LIST> [-n, --name <NAME>] [-D, --description <TEXT>] [--private|--public]`; omit edit flags in a TTY for prompts
- `delete <LIST> --yes`
- `add <OWNER/REPO> --to <LIST>`
- `remove <OWNER/REPO> --from <LIST> --yes`
- `move <OWNER/REPO> --from <LIST> --to <LIST> --yes`
- `copy --from <LIST> --to <LIST>`
- `merge --from <LIST> --to <LIST> --yes [--delete-source]`
- `unstar <OWNER/REPO> --yes`
- `--dry-run` previews write commands without mutating GitHub

Aliases: `ls=list`, `rm=remove`, `mv=move`, `cp=copy`.

GitHub currently limits accounts to 20 Star Lists; `create` surfaces GitHub's API rejection if the account is already at the limit.

## Output Formats

```sh
gh star-lists --json
gh star-lists --tsv
gh star-lists --plain                              # multiline detail view
gh star-lists --output stars.json
gh star-lists --jq '.[].name'
gh star-lists --template '{{range .}}{{.name}} {{.id}}\n{{end}}'
```

**JSON/TSV fields**

| Context | Fields |
|---------|--------|
| lists JSON | `name` `description` `lastAddedAt` `id` `repoCount` `url` |
| lists TSV | `name` `description` `repoCount` `lastAddedAt` `id` `url` |
| repos JSON | `nameWithOwner` `description` `isFork` `stargazerCount` `pushedAt` `url` `language` `starredAt` |
| repos TSV | `nameWithOwner` `description` `isFork` `stargazerCount` `pushedAt` `url` `language` |

Responses are cached in memory for 5 minutes by default. Override with `--cache-ttl <DURATION>` or disable with `--no-cache`. Caching is especially useful when chaining multiple calls with `--unlisted`.

## Recipes

```sh
# Star List names
gh star-lists --json | jq -r '.[].name'

# First Star List ID
LIST_ID="$(gh star-lists --tsv | awk -F '\t' 'NR == 1 { print $5 }')"

# Repo names and URLs
gh star-lists repos "$LIST_ID" --json | jq -r '.[] | "\(.nameWithOwner)\t\(.url)"'
gh star-lists repos "$LIST_ID" --jq '.[] | .nameWithOwner'

# Top 5 repos by stars
gh star-lists repos "$LIST_ID" --sort stars --desc --limit 5
gh star-lists repos "$LIST_ID" --sort language:asc,stars:desc --limit 10

# Non-fork repos matching a keyword
gh star-lists repos "$LIST_ID" --filter name:golang --filter fork:false --json

# Fuzzy-ranked search by repository name or description
gh star-lists repos "$LIST_ID" --search "github cli" --limit 10

# Repos by language
gh star-lists repos "$LIST_ID" --filter language:Go --sort language

# Star count via template
gh star-lists repos "$LIST_ID" --sort stars --desc --limit 10 \
  --template '{{range .}}{{.stargazerCount}} {{.nameWithOwner}}\n{{end}}'

# Unlisted starred repos
gh star-lists repos --unlisted --sort starred --desc

# Open in browser
gh star-lists repos "$LIST_ID" --web
```

### Interactive browser (fzf)

See [`examples/README.md`](examples/README.md) — two-pane fzf browser with live repo preview, disk cache, and customization guide. Requires `fzf`.

```sh
bash examples/fzf-browse.sh
```

## Troubleshooting

```sh
gh auth status
gh auth login
gh star-lists --help
```

- Authentication errors: refresh `gh` auth with `gh auth login`.
- Inaccessible list errors: pass a list name from `gh star-lists`, or an ID that starts with `UL_`.
- Slow `--unlisted` runs: cache is on by default; add `--cache-ttl 10m` to extend it, or avoid `--no-cache` for these queries.
- Write command uncertainty: add `--dry-run` to preview supported write actions before mutating GitHub.

## Development

```sh
make check   # scripts/check.sh: test, vet, build
make lint    # go vet
make smoke   # scripts/smoke-local.sh
scripts/smoke-gh-extension.sh
```

```sh
pre-commit install   # enable pre-commit hooks (requires pre-commit)
```

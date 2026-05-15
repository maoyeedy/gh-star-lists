# gh-star-lists

Explore GitHub Star Lists from terminal.

## Install

```sh
gh extension install maoyeedy/gh-star-lists
```

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
gh star-lists repos UL_kwDOExample --sort stars --desc
gh star-lists repos UL_kwDOExample --filter name:go --filter fork:false
gh star-lists repos UL_kwDOExample --filter language:Go
gh star-lists repos UL_kwDOExample --web           # open in browser
gh star-lists repos --unlisted --sort starred      # starred not in any list
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

- `--sort <KEY>` — comma-separated for multi-key; repeatable
- `--desc` — descending order

| Context | Keys |
|---------|------|
| lists | `added` `name` `repos` |
| repos | `name` `stars` `pushed` `language` `starred` |

**Filter**

- `--filter <KEY:VALUE>` — repeatable; `fork` and `language` are repos-only

| Key | Accepts |
|-----|---------|
| `name` | case-insensitive substring |
| `fork` | `true` / `false` |
| `language` | case-insensitive |

**Output**

- `--json` — JSON
- `--tsv` — TSV
- `--plain` — multiline detail view
- `--template <STR>` — Go template
- `--output <FILE>` — write to file
- `--no-color` — disable color

**Other**

- `--limit <N>` — max results
- `--cache` — cache API responses 5 min; recommended with `--unlisted`
- `--web` — open in browser
- `--unlisted` — starred repos not in any list

## Output Formats

```sh
gh star-lists --json
gh star-lists --tsv
gh star-lists --plain                              # multiline detail view
gh star-lists --output stars.json
gh star-lists --template '{{range .}}{{.name}} {{.id}}\n{{end}}'
```

**JSON/TSV fields**

| Context | Fields |
|---------|--------|
| lists | `name` `description` `lastAddedAt` `id` |
| repos | `nameWithOwner` `description` `isFork` `stargazerCount` `pushedAt` `url` `language` `starredAt` |

`--cache` caches API responses in memory for 5 min. Useful when chaining multiple calls. Recommended with `--unlisted`.

## Recipes

```sh
# Star List names
gh star-lists --json | jq -r '.[].name'

# First Star List ID
LIST_ID="$(gh star-lists --tsv | awk -F '\t' 'NR == 1 { print $4 }')"

# Repo names and URLs
gh star-lists repos "$LIST_ID" --json | jq -r '.[] | "\(.nameWithOwner)\t\(.url)"'

# Top 5 repos by stars
gh star-lists repos "$LIST_ID" --sort stars --desc --limit 5

# Non-fork repos matching a keyword
gh star-lists repos "$LIST_ID" --filter name:golang --filter fork:false --json

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

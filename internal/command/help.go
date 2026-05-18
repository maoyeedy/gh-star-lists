package command

import (
	"strings"

	"github.com/maoyeedy/gh-star-lists/internal/format"
)

const helpText = `gh star-lists

Query and manage GitHub Star Lists through the GitHub CLI authentication context.

Usage:
  gh star-lists [list] [--sort <KEY>] [--desc] [--limit <N>] [--filter <KEY:VALUE> ...] [--cache] [--output <FILE>] [--template <STR>] [--jq <QUERY>] [--no-color] [--plain | --json | --tsv]
  gh star-lists repos <LIST_ID_OR_NAME> [--sort <KEY>] [--desc] [--limit <N>] [--filter <KEY:VALUE> ...] [--cache] [--output <FILE>] [--template <STR>] [--jq <QUERY>] [--no-color] [--plain | --json | --tsv] [--web] [--unlisted] [--all] [--search <STR>]
  gh star-lists create <NAME> [--description <STR>] [--private | --public] [--dry-run]
  gh star-lists edit <LIST_ID_OR_NAME> [--name <STR>] [--description <STR>] [--private | --public] [--dry-run]
  gh star-lists delete <LIST_ID_OR_NAME> [--yes] [--dry-run]
  gh star-lists add <REPO> --to <LIST_ID_OR_NAME> [--dry-run]
  gh star-lists remove <REPO> --from <LIST_ID_OR_NAME> [--yes] [--dry-run]
  gh star-lists move <REPO> --from <LIST_ID_OR_NAME> --to <LIST_ID_OR_NAME> [--yes] [--dry-run]
  gh star-lists copy --from <LIST_ID_OR_NAME> --to <LIST_ID_OR_NAME> [--delete-source] [--yes] [--dry-run]
  gh star-lists merge --from <LIST_ID_OR_NAME> --to <LIST_ID_OR_NAME> [--yes] [--dry-run]
  gh star-lists unstar <REPO> [--yes] [--dry-run]
  gh star-lists --help

Commands:
  list              List your Star Lists. This is the default command.
  repos <LIST_ID_OR_NAME>   List repositories in a Star List. Accepts a list ID or
                    a case-insensitive name (fetches lists to resolve).
  create <NAME>     Create a new Star List with the given name.
  edit <LIST_ID_OR_NAME>    Update a Star List's name, description, or visibility.
  delete <LIST_ID_OR_NAME>  Delete a Star List (removes all repos from it).
  add <REPO>        Add a starred repository to a Star List.
  remove <REPO>     Remove a repository from a Star List.
  move <REPO>       Move a repository between Star Lists.
  copy              Copy all repositories from one Star List to another.
  merge             Merge all repositories from one Star List into another,
                    then delete the source list.
  unstar <REPO>     Unstar a repository.

Output:
  human             Table output optimized for reading (default).
  --plain           Detailed plain text output.
  --json            Machine-readable JSON output.
  --tsv             Tab-separated output for scripts.
  --output <FILE>   Write output to a file instead of stdout. Prompts before overwriting an existing file; pass --yes to skip the prompt.
  --template <STR>  Go template string applied to JSON data (implies --json data model).
  --jq <QUERY>      jq-style query applied to JSON output (implies --json).
  --no-color        Disable ANSI color in human output.
  --web             Open the Star List in a browser (repos only; no output flags).
  --unlisted        Show starred repos not in any Star List (repos only; N+1 API calls).
  --all             Show all starred repositories (repos only).

Sorting:
  --sort <KEY1,KEY2>  Sort results locally after fetching all pages.
                    Comma-separate or repeat for secondary sort.
                    Use "key:asc" or "key:desc" for per-key direction.
                    List keys: added, name, repos.
                    Repository keys: name, stars, pushed, language, starred.
  --desc            Reverse an explicit --sort order.
  --limit <N>       Show only the first N results (applied server-side when possible).

Filtering:
  --filter <KEY:VALUE>  Filter results after fetching all pages (repeatable).
                    Keys: name (case-insensitive contains), fork, language, archived,
                    license, min-stars, max-stars, topic (repos only).
  --search <STR>   Fuzzy-match repositories by name, description, and language.

Caching:
  --cache           Cache API responses in memory (TTL: 5 min).

Examples:
  gh star-lists
  gh star-lists --sort name
  gh star-lists --plain
  gh star-lists list --tsv --sort repos --desc
  gh star-lists repos UL_kwDOExample --sort stars --desc
  gh star-lists repos UL_kwDOExample --json
  gh star-lists repos UL_kwDOExample --filter language:Go --sort language
  gh star-lists repos UL_kwDOExample --web
  gh star-lists repos --unlisted --sort starred
  gh star-lists repos --all --json
  gh star-lists create "My List" --description "Tools I use"
  gh star-lists edit "My List" --name "Better Name" --private
  gh star-lists delete "Old List" --yes
  gh star-lists add cli/cli --to "My List"
  gh star-lists remove cli/cli --from "My List"
  gh star-lists move cli/cli --from "Old" --to "New"
  gh star-lists copy --from tools --to backup
  gh star-lists merge --from old --to new

Authentication:
  Uses the user's existing gh authentication. This command does not store tokens.

Host:
  --host <HOST>     GitHub Enterprise hostname (default: github.com).
`

func HelpText() string {
	return helpText
}

func HelpTextWithOptions(options format.Options) string {
	if !options.Color {
		return helpText
	}
	var b strings.Builder
	bold := boldAnsi
	lines := strings.Split(helpText, "\n")
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "gh star-lists"):
			b.WriteString(bold(line))
		case strings.HasPrefix(line, "  gh star-lists"):
			b.WriteString(bold(line))
		default:
			b.WriteString(line)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func boldAnsi(s string) string {
	return "\x1b[1m" + s + "\x1b[0m"
}

func UsageText() string {
	return `Usage:
  gh star-lists [list] [--sort <KEY>] [--desc] [--limit <N>] [--filter <KEY:VALUE> ...] [--cache] [--output <FILE>] [--template <STR>] [--jq <QUERY>] [--no-color] [--plain | --json | --tsv]
  gh star-lists repos <LIST_ID_OR_NAME> [--sort <KEY>] [--desc] [--limit <N>] [--filter <KEY:VALUE> ...] [--cache] [--output <FILE>] [--template <STR>] [--jq <QUERY>] [--no-color] [--plain | --json | --tsv] [--web] [--unlisted] [--all] [--search <STR>]
  gh star-lists create <NAME> [--description <STR>] [--private | --public] [--dry-run]
  gh star-lists edit <LIST_ID_OR_NAME> [--name <STR>] [--description <STR>] [--private | --public] [--dry-run]
  gh star-lists delete <LIST_ID_OR_NAME> [--yes] [--dry-run]
  gh star-lists add <REPO> --to <LIST_ID_OR_NAME> [--dry-run]
  gh star-lists remove <REPO> --from <LIST_ID_OR_NAME> [--yes] [--dry-run]
  gh star-lists move <REPO> --from <LIST_ID_OR_NAME> --to <LIST_ID_OR_NAME> [--yes] [--dry-run]
  gh star-lists copy --from <LIST_ID_OR_NAME> --to <LIST_ID_OR_NAME> [--delete-source] [--yes] [--dry-run]
  gh star-lists merge --from <LIST_ID_OR_NAME> --to <LIST_ID_OR_NAME> [--yes] [--dry-run]
  gh star-lists unstar <REPO> [--yes] [--dry-run]
  gh star-lists --help
`
}

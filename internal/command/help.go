package command

const helpText = `gh star-lists

Query GitHub Star Lists through the GitHub CLI authentication context.

Usage:
  gh star-lists [list] [--sort <KEY>] [--desc] [--limit <N>] [--filter <KEY:VALUE> ...] [--cache] [--output <FILE>] [--template <STR>] [--no-color] [--plain | --json | --tsv]
  gh star-lists repos <LIST_ID_OR_NAME> [--sort <KEY>] [--desc] [--limit <N>] [--filter <KEY:VALUE> ...] [--cache] [--output <FILE>] [--template <STR>] [--no-color] [--plain | --json | --tsv]
  gh star-lists --help

Commands:
  list              List your Star Lists. This is the default command.
  repos <LIST_ID_OR_NAME>   List repositories in a Star List. Accepts a list ID or
	                    a case-insensitive name (fetches lists to resolve).

Output:
  human             Table output optimized for reading (default).
  --plain           Detailed plain text output.
  --json            Machine-readable JSON output.
  --tsv             Tab-separated output for scripts.
  --output <FILE>   Write output to a file instead of stdout.
  --template <STR>  Go template string applied to JSON data (implies --json data model).
  --no-color        Disable ANSI color in human output.

Sorting:
  --sort <KEY1,KEY2>  Sort results locally after fetching all pages.
                    Comma-separate or repeat for secondary sort.
                    List keys: added, name.
                    Repository keys: name, stars, pushed.
  --desc            Reverse an explicit --sort order.
  --limit <N>       Show only the first N results.

Filtering:
  --filter <KEY:VALUE>  Filter results after fetching all pages (repeatable).
                        Keys: name (case-insensitive contains), fork (true/false).

Caching:
  --cache           Cache API responses in memory (TTL: 5 min).

Examples:
  gh star-lists
  gh star-lists --sort name
  gh star-lists --plain
  gh star-lists list --tsv
  gh star-lists repos UL_kwDOExample --sort stars --desc
  gh star-lists repos UL_kwDOExample --json

Authentication:
  Uses the user's existing gh authentication. This command does not store tokens.
`

// HelpText returns the normal stdout help for gh star-lists.
func HelpText() string {
	return helpText
}

// UsageText returns the short usage block appended to invalid-usage diagnostics.
func UsageText() string {
	return `Usage:
  gh star-lists [list] [--sort <KEY>] [--desc] [--limit <N>] [--filter <KEY:VALUE> ...] [--cache] [--output <FILE>] [--template <STR>] [--no-color] [--plain | --json | --tsv]
  gh star-lists repos <LIST_ID_OR_NAME> [--sort <KEY>] [--desc] [--limit <N>] [--filter <KEY:VALUE> ...] [--cache] [--output <FILE>] [--template <STR>] [--no-color] [--plain | --json | --tsv]
  gh star-lists --help
`
}

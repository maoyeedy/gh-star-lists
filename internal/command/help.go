package command

const helpText = `gh star-lists v0.1

Query GitHub Star Lists through the GitHub CLI authentication context.

Usage:
  gh star-lists [list] [--sort <KEY>] [--desc] [--plain | --json | --tsv]
  gh star-lists repos <LIST_ID> [--sort <KEY>] [--desc] [--plain | --json | --tsv]
  gh star-lists --help

Commands:
  list              List your Star Lists. This is the default command.
  repos <LIST_ID>   List repositories in a Star List.

Output:
  human             Table output optimized for reading (default).
  --plain           Detailed plain text output.
  --json            Machine-readable JSON output.
  --tsv             Tab-separated output for scripts.

Sorting:
  --sort <KEY>      Sort results locally after fetching all pages.
                    List keys: added, name.
                    Repository keys: name, stars, pushed.
  --desc            Reverse an explicit --sort order.

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
  gh star-lists [list] [--sort <KEY>] [--desc] [--plain | --json | --tsv]
  gh star-lists repos <LIST_ID> [--sort <KEY>] [--desc] [--plain | --json | --tsv]
  gh star-lists --help
`
}

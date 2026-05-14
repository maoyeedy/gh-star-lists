package command_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/maoyeedy/gh-star-lists/internal/command"
	"github.com/maoyeedy/gh-star-lists/internal/format"
)

func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		argv []string
		want command.Parsed
	}{
		{
			name: "empty argv defaults to human list",
			argv: nil,
			want: command.Parsed{Action: command.ActionList, Mode: format.OutputHuman},
		},
		{
			name: "explicit list",
			argv: []string{"list"},
			want: command.Parsed{Action: command.ActionList, Mode: format.OutputHuman},
		},
		{
			name: "repos captures list id",
			argv: []string{"repos", "UL_kwDOExample"},
			want: command.Parsed{Action: command.ActionRepos, ListID: "UL_kwDOExample", Mode: format.OutputHuman},
		},
		{
			name: "long help short circuits",
			argv: []string{"--help"},
			want: command.Parsed{Action: command.ActionHelp, Mode: format.OutputHuman},
		},
		{
			name: "short help short circuits before missing repos id",
			argv: []string{"repos", "-h"},
			want: command.Parsed{Action: command.ActionHelp, Mode: format.OutputHuman},
		},
		{
			name: "json before subcommand",
			argv: []string{"--json", "repos", "list-id"},
			want: command.Parsed{Action: command.ActionRepos, ListID: "list-id", Mode: format.OutputJSON},
		},
		{
			name: "tsv after subcommand",
			argv: []string{"list", "--tsv"},
			want: command.Parsed{Action: command.ActionList, Mode: format.OutputTSV},
		},
		{
			name: "json after repos id",
			argv: []string{"repos", "list-id", "--json"},
			want: command.Parsed{Action: command.ActionRepos, ListID: "list-id", Mode: format.OutputJSON},
		},
		{
			name: "plain before subcommand",
			argv: []string{"--plain", "list"},
			want: command.Parsed{Action: command.ActionList, Mode: format.OutputPlain},
		},
		{
			name: "plain after repos id",
			argv: []string{"repos", "list-id", "--plain"},
			want: command.Parsed{Action: command.ActionRepos, ListID: "list-id", Mode: format.OutputPlain},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := command.Parse(tt.argv)
			if err != nil {
				t.Fatalf("Parse(%q) returned unexpected error: %v", tt.argv, err)
			}

			if got != tt.want {
				t.Fatalf("Parse(%q) = %#v, want %#v", tt.argv, got, tt.want)
			}
		})
	}
}

func TestParseUsageErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		argv        []string
		wantMessage string
	}{
		{name: "unknown command", argv: []string{"stars"}, wantMessage: "unknown command"},
		{name: "repos missing id", argv: []string{"repos"}, wantMessage: "missing list id"},
		{name: "list extra arg", argv: []string{"list", "extra"}, wantMessage: "too many arguments"},
		{name: "repos extra arg", argv: []string{"repos", "id", "extra"}, wantMessage: "too many arguments"},
		{name: "conflicting output flags", argv: []string{"--json", "--tsv"}, wantMessage: "cannot combine --plain, --json, and --tsv"},
		{name: "conflicting plain output flag", argv: []string{"--plain", "--json"}, wantMessage: "cannot combine --plain, --json, and --tsv"},
		{name: "unknown flag", argv: []string{"--xml"}, wantMessage: "unknown flag"},
		{name: "cache flag is deferred", argv: []string{"--cache"}, wantMessage: "unknown flag"},
		{name: "filter flag is deferred", argv: []string{"--filter", "fork:false"}, wantMessage: "unknown flag"},
		{name: "sort flag is deferred", argv: []string{"--sort", "stars"}, wantMessage: "unknown flag"},
		{name: "limit flag is deferred", argv: []string{"--limit", "10"}, wantMessage: "unknown flag"},
		{name: "empty flag value", argv: []string{""}, wantMessage: "empty argument"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := command.Parse(tt.argv)
			if err == nil {
				t.Fatalf("Parse(%q) returned nil error", tt.argv)
			}

			var usageErr *command.UsageError
			if !errors.As(err, &usageErr) {
				t.Fatalf("Parse(%q) error %T, want *command.UsageError", tt.argv, err)
			}

			if !strings.Contains(err.Error(), tt.wantMessage) {
				t.Fatalf("Parse(%q) error %q, want message containing %q", tt.argv, err, tt.wantMessage)
			}
		})
	}
}

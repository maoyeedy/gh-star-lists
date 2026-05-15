package format_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/maoyeedy/gh-star-lists/internal/format"
	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

func TestWriteRepositoriesTSVUsesLegacyFieldOrder(t *testing.T) {
	t.Parallel()

	repos := []githubapi.Repository{
		{
			NameWithOwner:  "cli/cli",
			Description:    "GitHub CLI",
			IsFork:         false,
			StargazerCount: 41000,
			PushedAt:       "2024-05-01T12:00:00Z",
			URL:            "https://github.com/cli/cli",
			Language:       "Go",
		},
		{
			NameWithOwner:  "fork/project",
			IsFork:         true,
			StargazerCount: 0,
			PushedAt:       "",
			URL:            "https://github.com/fork/project",
			Language:       "",
		},
	}

	var out bytes.Buffer
	if err := format.WriteRepositories(&out, format.OutputTSV, repos); err != nil {
		t.Fatalf("WriteRepositories returned unexpected error: %v", err)
	}

	want := "cli/cli\tGitHub CLI\tno\t41000\t2024-05-01T12:00:00Z\thttps://github.com/cli/cli\tGo\n" +
		"fork/project\t\tyes\t0\t\thttps://github.com/fork/project\t\n"
	if got := out.String(); got != want {
		t.Fatalf("WriteRepositories TSV output mismatch\ngot:  %q\nwant: %q", got, want)
	}
}

func TestWriteRepositoriesJSONUsesLowerCamelCaseArray(t *testing.T) {
	t.Parallel()

	repos := []githubapi.Repository{
		{
			NameWithOwner:  "cli/cli",
			Description:    "GitHub CLI",
			IsFork:         false,
			StargazerCount: 41000,
			PushedAt:       "2024-05-01T12:00:00Z",
			URL:            "https://github.com/cli/cli",
			Language:       "Go",
		},
	}

	var out bytes.Buffer
	if err := format.WriteRepositories(&out, format.OutputJSON, repos); err != nil {
		t.Fatalf("WriteRepositories returned unexpected error: %v", err)
	}

	want := "[{\"nameWithOwner\":\"cli/cli\",\"description\":\"GitHub CLI\",\"isFork\":false,\"stargazerCount\":41000,\"pushedAt\":\"2024-05-01T12:00:00Z\",\"url\":\"https://github.com/cli/cli\",\"language\":\"Go\"}]\n"
	if got := out.String(); got != want {
		t.Fatalf("WriteRepositories JSON output mismatch\ngot:  %q\nwant: %q", got, want)
	}
}

func TestWriteRepositoriesEmptyOutputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode format.OutputMode
		want string
	}{
		{name: "json", mode: format.OutputJSON, want: "[]\n"},
		{name: "tsv", mode: format.OutputTSV, want: ""},
		{name: "human", mode: format.OutputHuman, want: "No repositories found.\n"},
		{name: "plain", mode: format.OutputPlain, want: "No repositories found.\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var out bytes.Buffer
			if err := format.WriteRepositories(&out, tt.mode, nil); err != nil {
				t.Fatalf("WriteRepositories returned unexpected error: %v", err)
			}
			if got := out.String(); got != tt.want {
				t.Fatalf("WriteRepositories empty %s output = %q, want %q", tt.mode, got, tt.want)
			}
		})
	}
}

func TestWriteRepositoriesHumanIsDeterministic(t *testing.T) {
	t.Parallel()

	repos := []githubapi.Repository{
		{
			NameWithOwner:  "cli/cli",
			Description:    "GitHub CLI",
			IsFork:         false,
			StargazerCount: 41000,
			PushedAt:       "2024-05-01T12:00:00Z",
			URL:            "https://github.com/cli/cli",
			Language:       "Go",
		},
		{
			NameWithOwner:  "fork/project",
			IsFork:         true,
			StargazerCount: 0,
			URL:            "https://github.com/fork/project",
			Language:       "",
		},
	}

	var out bytes.Buffer
	options := format.Options{
		Mode:  format.OutputHuman,
		Width: 120,
		Now:   time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC),
	}
	if err := format.WriteRepositoriesWithOptions(&out, options, repos); err != nil {
		t.Fatalf("WriteRepositories returned unexpected error: %v", err)
	}

	want := "REPOSITORY    STARS  LANG  FORK  PUSHED  URL\n" +
		"cli/cli       41000  Go    no    2y ago  https://github.com/cli/cli\n" +
		"fork/project  0            yes   -       https://github.com/fork/project\n"
	if got := out.String(); got != want {
		t.Fatalf("WriteRepositories human output mismatch\ngot:  %q\nwant: %q", got, want)
	}
}

func TestWriteRepositoriesPlainPreservesDetailedOutput(t *testing.T) {
	t.Parallel()

	repos := []githubapi.Repository{
		{
			NameWithOwner:  "cli/cli",
			Description:    "GitHub CLI",
			IsFork:         false,
			StargazerCount: 41000,
			PushedAt:       "2024-05-01T12:00:00Z",
			URL:            "https://github.com/cli/cli",
			Language:       "Go",
		},
		{
			NameWithOwner:  "fork/project",
			IsFork:         true,
			StargazerCount: 0,
			URL:            "https://github.com/fork/project",
			Language:       "",
		},
	}

	var out bytes.Buffer
	if err := format.WriteRepositories(&out, format.OutputPlain, repos); err != nil {
		t.Fatalf("WriteRepositories returned unexpected error: %v", err)
	}

	want := "cli/cli\n" +
		"  Description: GitHub CLI\n" +
		"  Fork: no\n" +
		"  Stars: 41000\n" +
		"  Pushed: 2024-05-01T12:00:00Z\n" +
		"  URL: https://github.com/cli/cli\n" +
		"  Language: Go\n" +
		"fork/project\n" +
		"  Fork: yes\n" +
		"  Stars: 0\n" +
		"  URL: https://github.com/fork/project\n"
	if got := out.String(); got != want {
		t.Fatalf("WriteRepositories human output mismatch\ngot:  %q\nwant: %q", got, want)
	}
}

func TestWriteRepositoriesReturnsWriterErrors(t *testing.T) {
	t.Parallel()

	repos := []githubapi.Repository{{NameWithOwner: "cli/cli", URL: "https://github.com/cli/cli"}}
	if err := format.WriteRepositories(errWriter{}, format.OutputHuman, repos); err == nil {
		t.Fatal("WriteRepositories human returned nil error for failing writer")
	}
	if err := format.WriteRepositories(errWriter{}, format.OutputTSV, repos); err == nil {
		t.Fatal("WriteRepositories TSV returned nil error for failing writer")
	}
	if err := format.WriteRepositories(errWriter{}, format.OutputJSON, repos); err == nil {
		t.Fatal("WriteRepositories JSON returned nil error for failing writer")
	}
	if err := format.WriteRepositories(errWriter{}, format.OutputPlain, repos); err == nil {
		t.Fatal("WriteRepositories plain returned nil error for failing writer")
	}
}

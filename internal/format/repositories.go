package format

import (
	"fmt"
	"io"
	"strconv"

	"github.com/cli/go-gh/v2/pkg/tableprinter"
	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

// WriteRepositories writes repositories to w in the requested output mode.
func WriteRepositories(w io.Writer, mode OutputMode, repos []githubapi.Repository) error {
	return WriteRepositoriesWithOptions(w, Options{Mode: mode}, repos)
}

// WriteRepositoriesWithOptions writes repositories with terminal-aware human settings.
func WriteRepositoriesWithOptions(
	w io.Writer,
	options Options,
	repos []githubapi.Repository,
) error {
	options = normalizeOptions(options)
	switch options.Mode {
	case OutputJSON:
		return writeJSONSliceWithOptions(w, options, repos)
	case OutputTSV:
		return writeRepositoriesTSV(w, repos)
	case OutputFZF:
		return writeRepositoriesFZF(w, repos)
	case OutputPlain:
		return writeRepositoriesPlain(w, repos)
	case OutputTemplate:
		return writeTemplate(w, options, repos)
	case OutputHuman:
		return writeRepositoriesHuman(w, options, repos)
	default:
		return fmt.Errorf("unsupported output mode %q", options.Mode)
	}
}

func writeRepositoriesTSV(w io.Writer, repos []githubapi.Repository) error {
	for _, repo := range repos {
		if _, err := fmt.Fprintf(
			w,
			"%s\t%s\t%s\t%d\t%s\t%s\t%s\n",
			repo.NameWithOwner,
			repo.Description,
			yesNo(repo.IsFork),
			repo.StargazerCount,
			repo.PushedAt,
			repo.URL,
			repo.Language,
		); err != nil {
			return err
		}
	}
	return nil
}

func writeRepositoriesFZF(w io.Writer, repos []githubapi.Repository) error {
	for _, repo := range repos {
		if _, err := fmt.Fprintf(
			w,
			"%s\t%d\t%s\t%s\t%s\t%s\t%s\n",
			repo.NameWithOwner,
			repo.StargazerCount,
			repo.Language,
			repo.URL,
			repo.Description,
			repo.PushedAt,
			yesNo(repo.IsFork),
		); err != nil {
			return err
		}
	}
	return nil
}

func writeRepositoriesHuman(w io.Writer, options Options, repos []githubapi.Repository) error {
	if len(repos) == 0 {
		_, _ = fmt.Fprintln(w, "No repositories found.")
		_, err := fmt.Fprintln(w, "Try a different filter, --search, or --all.")
		return err
	}
	boldFn := Bold(options.Color)
	faintFn := Faint(options.Color)
	table := tableprinter.New(w, true, options.Width)
	table.AddHeader(
		[]string{"REPOSITORY", "STARS", "LANG", "FORK", "PUSHED", "URL"},
		tableprinter.WithColor(boldFn),
	)
	for _, repo := range repos {
		table.AddField(FormatNameWithOwner(repo.NameWithOwner, options.Color))
		table.AddField(strconv.Itoa(repo.StargazerCount), tableprinter.WithTruncate(nil))
		table.AddField(repo.Language, tableprinter.WithTruncate(nil))
		table.AddField(yesNo(repo.IsFork), tableprinter.WithTruncate(nil))
		table.AddField(shortAge(repo.PushedAt, options.Now), tableprinter.WithTruncate(nil))
		table.AddField(repo.URL, tableprinter.WithColor(faintFn))
		table.EndRow()
	}
	return table.Render()
}

func writeRepositoriesPlain(w io.Writer, repos []githubapi.Repository) error {
	if len(repos) == 0 {
		_, err := fmt.Fprintln(w, "No repositories found.")
		return err
	}
	for _, repo := range repos {
		if _, err := fmt.Fprintf(w, "%s\n", repo.NameWithOwner); err != nil {
			return err
		}
		if repo.Description != "" {
			if _, err := fmt.Fprintf(w, "  Description: %s\n", repo.Description); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "  Fork: %s\n", yesNo(repo.IsFork)); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "  Stars: %d\n", repo.StargazerCount); err != nil {
			return err
		}
		if repo.PushedAt != "" {
			if _, err := fmt.Fprintf(w, "  Pushed: %s\n", repo.PushedAt); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "  URL: %s\n", repo.URL); err != nil {
			return err
		}
		if repo.Language != "" {
			if _, err := fmt.Fprintf(w, "  Language: %s\n", repo.Language); err != nil {
				return err
			}
		}
	}
	return nil
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

package format

import (
	"fmt"
	"io"

	"github.com/cli/go-gh/v2/pkg/tableprinter"
	"github.com/maoyeedy/gh-star-lists/internal/domain"
)

// WriteRepositories writes repositories to w in the requested output mode.
func WriteRepositories(w io.Writer, mode OutputMode, repos []domain.Repository) error {
	return WriteRepositoriesWithOptions(w, Options{Mode: mode}, repos)
}

// WriteRepositoriesWithOptions writes repositories with terminal-aware human settings.
func WriteRepositoriesWithOptions(
	w io.Writer,
	options Options,
	repos []domain.Repository,
) error {
	options = normalizeOptions(options)
	rows := make([]domain.RepoRow, len(repos))
	for i, r := range repos {
		rows[i] = RepoRowFromDomain(r, options.Now)
	}
	switch options.Mode {
	case OutputJSON:
		return writeJSONSliceWithOptions(w, options, rows)
	case OutputTSV:
		return writeRepositoriesTSV(w, rows)
	case OutputFZF:
		return writeRepositoriesFZF(w, rows)
	case OutputPlain:
		return writeRepositoriesPlain(w, rows)
	case OutputTemplate:
		return writeTemplate(w, options, rows)
	case OutputHuman:
		return writeRepositoriesHuman(w, options, rows)
	default:
		return fmt.Errorf("unsupported output mode %q", options.Mode)
	}
}

func writeRepositoriesTSV(w io.Writer, repos []domain.RepoRow) error {
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

func writeRepositoriesFZF(w io.Writer, repos []domain.RepoRow) error {
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

func writeRepositoriesHuman(w io.Writer, options Options, repos []domain.RepoRow) error {
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
		table.AddField(formatName(repo.Owner, repo.Name, options.Color))
		table.AddField(repo.Stars, tableprinter.WithTruncate(nil))
		table.AddField(repo.Language, tableprinter.WithTruncate(nil))
		table.AddField(repo.Fork, tableprinter.WithTruncate(nil))
		table.AddField(repo.PushedAge, tableprinter.WithTruncate(nil))
		table.AddField(repo.URL, tableprinter.WithColor(faintFn))
		table.EndRow()
	}
	return table.Render()
}

func writeRepositoriesPlain(w io.Writer, repos []domain.RepoRow) error {
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
		if _, err := fmt.Fprintf(w, "  Fork: %s\n", repo.Fork); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "  Stars: %s\n", repo.Stars); err != nil {
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

func formatName(owner, name string, color bool) string {
	if name == "" {
		if !color {
			return owner
		}
		return Bold(true)(owner)
	}
	if !color {
		return owner + "/" + name
	}
	return Faint(true)(owner) + Faint(true)("/") + Bold(true)(name)
}

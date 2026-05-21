package format

import (
	"fmt"
	"io"

	"github.com/cli/go-gh/v2/pkg/tableprinter"
	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
	"github.com/maoyeedy/gh-star-lists/internal/humanize"
)

// WriteStarLists writes Star Lists to w in the requested output mode.
func WriteStarLists(w io.Writer, mode OutputMode, lists []githubapi.StarList) error {
	return WriteStarListsWithOptions(w, Options{Mode: mode}, lists)
}

// WriteStarListsWithOptions writes Star Lists with terminal-aware human settings.
func WriteStarListsWithOptions(w io.Writer, options Options, lists []githubapi.StarList) error {
	options = normalizeOptions(options)
	switch options.Mode {
	case OutputJSON:
		return writeJSONSliceWithOptions(w, options, lists)
	case OutputTSV:
		return writeStarListsTSV(w, lists)
	case OutputFZF:
		return writeStarListsFZF(w, lists)
	case OutputPlain:
		return writeStarListsPlain(w, lists)
	case OutputTemplate:
		return writeTemplate(w, options, lists)
	case OutputHuman:
		return writeStarListsHuman(w, options, lists)
	default:
		return fmt.Errorf("unsupported output mode %q", options.Mode)
	}
}

func writeStarListsTSV(w io.Writer, lists []githubapi.StarList) error {
	for _, list := range lists {
		if _, err := fmt.Fprintf(
			w,
			"%s\t%s\t%d\t%s\t%s\t%s\n",
			list.Name,
			list.Description,
			list.RepoCount,
			list.LastAddedAt,
			list.ID,
			list.URL,
		); err != nil {
			return err
		}
	}
	return nil
}

func writeStarListsFZF(w io.Writer, lists []githubapi.StarList) error {
	for _, list := range lists {
		if _, err := fmt.Fprintf(
			w,
			"%s\t%s\t%d\t%s\t%s\t%s\n",
			list.Name,
			list.ID,
			list.RepoCount,
			list.URL,
			list.Description,
			list.LastAddedAt,
		); err != nil {
			return err
		}
	}
	return nil
}

func writeStarListsHuman(w io.Writer, options Options, lists []githubapi.StarList) error {
	if len(lists) == 0 {
		_, _ = fmt.Fprintln(w, "No Star Lists found.")
		_, err := fmt.Fprintln(w, "Create one with `gh star-lists create <NAME>`.")
		return err
	}
	boldFn := Bold(options.Color)
	faintFn := Faint(options.Color)
	table := tableprinter.New(w, true, options.Width)
	table.AddHeader([]string{"NAME", "REPOS", "ADDED", "ID", "URL"}, tableprinter.WithColor(boldFn))
	for _, list := range lists {
		table.AddField(list.Name, tableprinter.WithColor(boldFn))
		table.AddField(fmt.Sprintf("%d", list.RepoCount), tableprinter.WithTruncate(nil))
		table.AddField(
			humanize.ShortAge(list.LastAddedAt, options.Now),
			tableprinter.WithTruncate(nil),
		)
		table.AddField(list.ID, tableprinter.WithColor(faintFn))
		table.AddField(list.URL, tableprinter.WithTruncate(nil))
		table.EndRow()
	}
	return table.Render()
}

func writeStarListsPlain(w io.Writer, lists []githubapi.StarList) error {
	if len(lists) == 0 {
		_, err := fmt.Fprintln(w, "No Star Lists found.")
		return err
	}
	for _, list := range lists {
		if _, err := fmt.Fprintf(w, "%s\n", list.Name); err != nil {
			return err
		}
		if list.Description != "" {
			if _, err := fmt.Fprintf(w, "  Description: %s\n", list.Description); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "  Repos: %d\n", list.RepoCount); err != nil {
			return err
		}
		if list.LastAddedAt != "" {
			if _, err := fmt.Fprintf(w, "  Last added: %s\n", list.LastAddedAt); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "  ID: %s\n", list.ID); err != nil {
			return err
		}
		if list.URL != "" {
			if _, err := fmt.Fprintf(w, "  URL: %s\n", list.URL); err != nil {
				return err
			}
		}
	}
	return nil
}

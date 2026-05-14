package format

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/cli/go-gh/v2/pkg/tableprinter"
	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
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
		return writeStarListsJSON(w, lists)
	case OutputTSV:
		return writeStarListsTSV(w, lists)
	case OutputPlain:
		return writeStarListsPlain(w, lists)
	case OutputHuman:
		return writeStarListsHuman(w, options, lists)
	default:
		return fmt.Errorf("unsupported output mode %q", options.Mode)
	}
}

func writeStarListsJSON(w io.Writer, lists []githubapi.StarList) error {
	if lists == nil {
		lists = []githubapi.StarList{}
	}
	return json.NewEncoder(w).Encode(lists)
}

func writeStarListsTSV(w io.Writer, lists []githubapi.StarList) error {
	for _, list := range lists {
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", list.Name, list.Description, list.LastAddedAt, list.ID); err != nil {
			return err
		}
	}
	return nil
}

func writeStarListsHuman(w io.Writer, options Options, lists []githubapi.StarList) error {
	if len(lists) == 0 {
		_, err := fmt.Fprintln(w, "No Star Lists found.")
		return err
	}
	table := tableprinter.New(w, true, options.Width)
	table.AddHeader([]string{"NAME", "ADDED", "ID"}, tableprinter.WithColor(bold(options.Color)))
	for _, list := range lists {
		table.AddField(list.Name, tableprinter.WithColor(bold(options.Color)))
		table.AddField(shortAge(list.LastAddedAt, options.Now), tableprinter.WithTruncate(nil))
		table.AddField(list.ID, tableprinter.WithColor(faint(options.Color)))
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
		if list.LastAddedAt != "" {
			if _, err := fmt.Fprintf(w, "  Last added: %s\n", list.LastAddedAt); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "  ID: %s\n", list.ID); err != nil {
			return err
		}
	}
	return nil
}

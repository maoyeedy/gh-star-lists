package format

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

// WriteStarLists writes Star Lists to w in the requested output mode.
func WriteStarLists(w io.Writer, mode OutputMode, lists []githubapi.StarList) error {
	switch mode {
	case OutputJSON:
		return writeStarListsJSON(w, lists)
	case OutputTSV:
		return writeStarListsTSV(w, lists)
	case OutputHuman:
		return writeStarListsHuman(w, lists)
	default:
		return fmt.Errorf("unsupported output mode %q", mode)
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

func writeStarListsHuman(w io.Writer, lists []githubapi.StarList) error {
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

package format

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

// WriteRepositories writes repositories to w in the requested output mode.
func WriteRepositories(w io.Writer, mode OutputMode, repos []githubapi.Repository) error {
	switch mode {
	case OutputJSON:
		return writeRepositoriesJSON(w, repos)
	case OutputTSV:
		return writeRepositoriesTSV(w, repos)
	case OutputHuman:
		return writeRepositoriesHuman(w, repos)
	default:
		return fmt.Errorf("unsupported output mode %q", mode)
	}
}

func writeRepositoriesJSON(w io.Writer, repos []githubapi.Repository) error {
	if repos == nil {
		repos = []githubapi.Repository{}
	}
	return json.NewEncoder(w).Encode(repos)
}

func writeRepositoriesTSV(w io.Writer, repos []githubapi.Repository) error {
	for _, repo := range repos {
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%s\n", repo.NameWithOwner, repo.Description, yesNo(repo.IsFork), repo.StargazerCount, repo.PushedAt, repo.URL); err != nil {
			return err
		}
	}
	return nil
}

func writeRepositoriesHuman(w io.Writer, repos []githubapi.Repository) error {
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
	}
	return nil
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

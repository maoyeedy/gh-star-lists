package command

import (
	"context"
	"fmt"
	"strings"

	"github.com/maoyeedy/gh-star-lists/internal/githubapi"
)

func normalizePromptError(err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "interrupt") {
		return ErrPromptCancelled
	}
	return err
}

func styleText(fn func(bool) func(string) string, enabled bool, text string) string {
	style := fn(enabled)
	if style == nil {
		return text
	}
	return style(text)
}

func ensureCreateInputs(parsed *Parsed) error {
	if parsed.Name != "" {
		return nil
	}
	if !canPrompt() {
		return usage("create requires a list name (or run in a TTY to be prompted)")
	}
	name, err := promptRequiredInput("List name:", "")
	if err != nil {
		return err
	}
	parsed.Name = name
	if !parsed.DescriptionSet {
		description, err := promptInput("Description:", "")
		if err != nil {
			return err
		}
		parsed.Description = description
		parsed.DescriptionSet = true
	}
	if !parsed.PrivateSet {
		idx, err := promptForList("Visibility:", "Public", []string{"Public", "Private"})
		if err != nil {
			return err
		}
		if idx < 0 || idx > 1 {
			return fmt.Errorf("invalid visibility selection")
		}
		parsed.Private = idx == 1
		parsed.PrivateSet = true
	}
	return nil
}

func ensureEditInputs(parsed *Parsed, current githubapi.StarList) error {
	if parsed.Name != "" || parsed.DescriptionSet || parsed.PrivateSet {
		return nil
	}
	if !canPrompt() {
		return usage(
			"edit requires --name, --description, --private, or --public (or run in a TTY to be prompted)",
		)
	}
	choices := []string{"Name", "Description", "Visibility"}
	selected, err := promptMultiSelect("Select fields to update:", nil, choices)
	if err != nil {
		return err
	}
	if len(selected) == 0 {
		return ErrPromptCancelled
	}
	for _, idx := range selected {
		if idx < 0 || idx >= len(choices) {
			return fmt.Errorf("invalid edit field selection")
		}
		switch choices[idx] {
		case "Name":
			name, err := promptRequiredInput("New name:", current.Name)
			if err != nil {
				return err
			}
			parsed.Name = name
		case "Description":
			description, err := promptInput("New description:", current.Description)
			if err != nil {
				return err
			}
			parsed.Description = description
			parsed.DescriptionSet = true
		case "Visibility":
			visibility, err := promptForList("Visibility:", "", []string{"Public", "Private"})
			if err != nil {
				return err
			}
			if visibility < 0 || visibility > 1 {
				return fmt.Errorf("invalid visibility selection")
			}
			parsed.Private = visibility == 1
			parsed.PrivateSet = true
		}
	}
	return nil
}

func ensureListSelectors(
	ctx context.Context,
	service githubapi.Service,
	parsed *Parsed,
) ([]githubapi.StarList, error) {
	needFrom := actionNeedsFrom(parsed.Action) && parsed.FromListID == ""
	needTo := actionNeedsTo(parsed.Action) && parsed.ToListID == ""
	if !needFrom && !needTo {
		return nil, nil
	}
	if !canPrompt() {
		return nil, missingSelectorError(parsed.Action, needFrom, needTo)
	}
	lists, err := service.ListStarLists(ctx)
	if err != nil {
		return nil, err
	}
	if len(lists) == 0 {
		return nil, fmt.Errorf("no Star Lists exist; create one with `gh star-lists create <NAME>`")
	}
	if needFrom {
		id, err := pickList(lists, "Select source Star List (--from):", "")
		if err != nil {
			return nil, err
		}
		parsed.FromListID = id
	}
	if needTo {
		excludeID := ""
		if actionNeedsFrom(parsed.Action) {
			excludeID = parsed.FromListID
		}
		id, err := pickList(lists, "Select target Star List (--to):", excludeID)
		if err != nil {
			return nil, err
		}
		parsed.ToListID = id
	}
	return lists, nil
}

func ensureReposListSelector(ctx context.Context, service githubapi.Service, parsed *Parsed) error {
	if parsed.Action != ActionRepos || parsed.ListID != "" || parsed.All || parsed.Unlisted {
		return nil
	}
	if !canPrompt() {
		return usage(
			"repos requires <LIST_ID_OR_NAME>, --all, or --unlisted (or run in a TTY to choose a list interactively)",
		)
	}
	lists, err := service.ListStarLists(ctx)
	if err != nil {
		return err
	}
	if len(lists) == 0 {
		return fmt.Errorf("no Star Lists exist; create one with `gh star-lists create <NAME>`")
	}
	id, err := pickList(lists, "Select Star List:", "")
	if err != nil {
		return err
	}
	parsed.ListID = id
	return nil
}

func promptRequiredInput(label, defaultValue string) (string, error) {
	value, err := promptInput(label, defaultValue)
	if err != nil {
		return "", err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", usage("%s cannot be empty", strings.TrimSuffix(label, ":"))
	}
	return value, nil
}

func requireYes(parsed Parsed, action string) error {
	if parsed.Yes || parsed.DryRun {
		return nil
	}
	if canPrompt() {
		confirmed, err := confirmAction("Confirm " + action + "?")
		if err != nil {
			return err
		}
		if confirmed {
			return nil
		}
		return usage("%s was not confirmed", action)
	}
	return usage("%s requires --yes or --dry-run (or run interactively in a TTY)", action)
}

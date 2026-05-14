package command

import (
	"fmt"
	"strings"

	"github.com/maoyeedy/gh-star-lists/internal/format"
)

// Action identifies the operation requested by the CLI arguments.
type Action string

const (
	ActionList  Action = "list"
	ActionRepos Action = "repos"
	ActionHelp  Action = "help"
)

// Sort keys accepted by --sort for list and repos subcommands.
const (
	SortKeyAdded  = "added"
	SortKeyName   = "name"
	SortKeyStars  = "stars"
	SortKeyPushed = "pushed"
)

// Parsed is the normalized command state consumed by the runner.
type Parsed struct {
	Action   Action
	ListID   string
	Mode     format.OutputMode
	SortKey  string
	SortDesc bool
}

// UsageError describes invalid CLI input. Runners should map this to exit code 2.
type UsageError struct {
	Message string
}

func (e *UsageError) Error() string {
	return e.Message
}

// Parse normalizes gh-star-lists arguments without initializing GitHub clients.
func Parse(argv []string) (Parsed, error) {
	var (
		positionals []string
		jsonFlag    bool
		tsvFlag     bool
		plainFlag   bool
		sortKey     string
		sortDesc    bool
	)

	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		switch arg {
		case "":
			return Parsed{}, usage("empty argument")
		case "--help", "-h":
			return Parsed{Action: ActionHelp, Mode: format.OutputHuman}, nil
		case "--json":
			jsonFlag = true
		case "--tsv":
			tsvFlag = true
		case "--plain":
			plainFlag = true
		case "--sort":
			if i+1 >= len(argv) {
				return Parsed{}, usage("missing value for --sort")
			}
			i++
			sortKey = argv[i]
			if sortKey == "" {
				return Parsed{}, usage("empty value for --sort")
			}
			if strings.HasPrefix(sortKey, "-") {
				return Parsed{}, usage("missing value for --sort")
			}
		case "--desc":
			sortDesc = true
		default:
			if strings.HasPrefix(arg, "-") {
				return Parsed{}, usage("unknown flag %q", arg)
			}
			positionals = append(positionals, arg)
		}
	}

	mode, err := format.SelectOutputMode(jsonFlag, tsvFlag, plainFlag)
	if err != nil {
		return Parsed{}, usage("%s", err.Error())
	}

	if len(positionals) == 0 {
		if err := validateSort(ActionList, sortKey, sortDesc); err != nil {
			return Parsed{}, err
		}
		return Parsed{Action: ActionList, Mode: mode, SortKey: sortKey, SortDesc: sortDesc}, nil
	}

	switch positionals[0] {
	case "list":
		if len(positionals) > 1 {
			return Parsed{}, usage("too many arguments for list: %s", strings.Join(positionals[1:], " "))
		}
		if err := validateSort(ActionList, sortKey, sortDesc); err != nil {
			return Parsed{}, err
		}
		return Parsed{Action: ActionList, Mode: mode, SortKey: sortKey, SortDesc: sortDesc}, nil
	case "repos":
		if len(positionals) == 1 {
			return Parsed{}, usage("missing list id for repos")
		}
		if len(positionals) > 2 {
			return Parsed{}, usage("too many arguments for repos: %s", strings.Join(positionals[2:], " "))
		}
		if err := validateSort(ActionRepos, sortKey, sortDesc); err != nil {
			return Parsed{}, err
		}
		return Parsed{Action: ActionRepos, ListID: positionals[1], Mode: mode, SortKey: sortKey, SortDesc: sortDesc}, nil
	default:
		return Parsed{}, usage("unknown command %q", positionals[0])
	}
}

func validateSort(action Action, sortKey string, sortDesc bool) error {
	if sortKey == "" {
		if sortDesc {
			return usage("--desc requires --sort")
		}
		return nil
	}

	switch action {
	case ActionList:
		switch sortKey {
		case SortKeyAdded, SortKeyName:
			return nil
		default:
			return usage("unsupported sort key %q for list; supported keys: added, name", sortKey)
		}
	case ActionRepos:
		switch sortKey {
		case SortKeyName, SortKeyStars, SortKeyPushed:
			return nil
		default:
			return usage("unsupported sort key %q for repos; supported keys: name, stars, pushed", sortKey)
		}
	default:
		return nil
	}
}

func usage(format string, args ...any) *UsageError {
	return &UsageError{Message: fmt.Sprintf(format, args...)}
}

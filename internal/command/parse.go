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

// Parsed is the normalized command state consumed by the runner.
type Parsed struct {
	Action Action
	ListID string
	Mode   format.OutputMode
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
	)

	for _, arg := range argv {
		switch arg {
		case "":
			return Parsed{}, usage("empty argument")
		case "--help", "-h":
			return Parsed{Action: ActionHelp, Mode: format.OutputHuman}, nil
		case "--json":
			jsonFlag = true
		case "--tsv":
			tsvFlag = true
		default:
			if strings.HasPrefix(arg, "-") {
				return Parsed{}, usage("unknown flag %q", arg)
			}
			positionals = append(positionals, arg)
		}
	}

	mode, err := format.SelectOutputMode(jsonFlag, tsvFlag)
	if err != nil {
		return Parsed{}, usage("%s", err.Error())
	}

	if len(positionals) == 0 {
		return Parsed{Action: ActionList, Mode: mode}, nil
	}

	switch positionals[0] {
	case "list":
		if len(positionals) > 1 {
			return Parsed{}, usage("too many arguments for list: %s", strings.Join(positionals[1:], " "))
		}
		return Parsed{Action: ActionList, Mode: mode}, nil
	case "repos":
		if len(positionals) == 1 {
			return Parsed{}, usage("missing list id for repos")
		}
		if len(positionals) > 2 {
			return Parsed{}, usage("too many arguments for repos: %s", strings.Join(positionals[2:], " "))
		}
		return Parsed{Action: ActionRepos, ListID: positionals[1], Mode: mode}, nil
	default:
		return Parsed{}, usage("unknown command %q", positionals[0])
	}
}

func usage(format string, args ...any) *UsageError {
	return &UsageError{Message: fmt.Sprintf(format, args...)}
}

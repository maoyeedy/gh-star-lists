package command

import (
	"fmt"
	"strconv"
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

// Filter keys accepted by --filter for list and repos subcommands.
const (
	FilterKeyName = "name"
	FilterKeyFork = "fork"
)

type Filter struct {
	Key   string
	Value string
}

// Parsed is the normalized command state consumed by the runner.
type Parsed struct {
	Action     Action
	ListID     string
	Mode       format.OutputMode
	SortKeys   []string
	SortDesc   bool
	NoColor    bool
	Limit      int
	Cache      bool
	Filters    []Filter
	OutputPath string
	Template   string
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
		sortKeys    []string
		sortDesc    bool
		limit       int
		cacheFlag   bool
		noColorFlag bool
		filters     []Filter
		outputPath  string
		templateStr string
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
			raw := argv[i]
			if raw == "" {
				return Parsed{}, usage("empty value for --sort")
			}
			if strings.HasPrefix(raw, "-") {
				return Parsed{}, usage("missing value for --sort")
			}
			for _, key := range strings.Split(raw, ",") {
				key = strings.TrimSpace(key)
				if key != "" {
					sortKeys = append(sortKeys, key)
				}
			}
		case "--desc":
			sortDesc = true
		case "--cache":
			cacheFlag = true
		case "--no-color":
			noColorFlag = true
		case "--limit":
			if i+1 >= len(argv) {
				return Parsed{}, usage("missing value for --limit")
			}
			i++
			v, err := strconv.Atoi(argv[i])
			if err != nil {
				return Parsed{}, usage("invalid value for --limit: %s", argv[i])
			}
			if v <= 0 {
				return Parsed{}, usage("invalid value for --limit: must be positive")
			}
			limit = v
		case "--filter":
			if i+1 >= len(argv) {
				return Parsed{}, usage("missing value for --filter")
			}
			i++
			raw := argv[i]
			key, value, ok := strings.Cut(raw, ":")
			if !ok || key == "" {
				return Parsed{}, usage("invalid filter %q: expected key:value", raw)
			}
			filters = append(
				filters,
				Filter{Key: strings.ToLower(key), Value: strings.ToLower(value)},
			)
		case "--template":
			if i+1 >= len(argv) {
				return Parsed{}, usage("missing value for --template")
			}
			i++
			templateStr = argv[i]
			if templateStr == "" {
				return Parsed{}, usage("empty value for --template")
			}
		case "--output":
			if i+1 >= len(argv) {
				return Parsed{}, usage("missing value for --output")
			}
			i++
			outputPath = argv[i]
			if outputPath == "" {
				return Parsed{}, usage("empty value for --output")
			}
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
	if templateStr != "" {
		mode = format.OutputTemplate
	}

	if len(positionals) == 0 {
		if err := validateFilters(ActionList, filters); err != nil {
			return Parsed{}, err
		}
		if err := validateSort(ActionList, sortKeys, sortDesc); err != nil {
			return Parsed{}, err
		}
		return Parsed{
			Action:     ActionList,
			Mode:       mode,
			SortKeys:   sortKeys,
			SortDesc:   sortDesc,
			Limit:      limit,
			NoColor:    noColorFlag,
			Filters:    filters,
			Cache:      cacheFlag,
			OutputPath: outputPath,
			Template:   templateStr,
		}, nil
	}

	switch positionals[0] {
	case "list":
		if len(positionals) > 1 {
			return Parsed{}, usage(
				"too many arguments for list: %s",
				strings.Join(positionals[1:], " "),
			)
		}
		if err := validateFilters(ActionList, filters); err != nil {
			return Parsed{}, err
		}
		if err := validateSort(ActionList, sortKeys, sortDesc); err != nil {
			return Parsed{}, err
		}
		return Parsed{
			Action:     ActionList,
			Mode:       mode,
			SortKeys:   sortKeys,
			SortDesc:   sortDesc,
			Limit:      limit,
			NoColor:    noColorFlag,
			Filters:    filters,
			Cache:      cacheFlag,
			OutputPath: outputPath,
			Template:   templateStr,
		}, nil
	case "repos":
		if len(positionals) == 1 {
			return Parsed{}, usage("missing list id for repos")
		}
		if len(positionals) > 2 {
			return Parsed{}, usage(
				"too many arguments for repos: %s",
				strings.Join(positionals[2:], " "),
			)
		}
		if err := validateFilters(ActionRepos, filters); err != nil {
			return Parsed{}, err
		}
		if err := validateSort(ActionRepos, sortKeys, sortDesc); err != nil {
			return Parsed{}, err
		}
		return Parsed{
			Action:     ActionRepos,
			ListID:     positionals[1],
			Mode:       mode,
			SortKeys:   sortKeys,
			SortDesc:   sortDesc,
			Limit:      limit,
			NoColor:    noColorFlag,
			Filters:    filters,
			Cache:      cacheFlag,
			OutputPath: outputPath,
			Template:   templateStr,
		}, nil
	default:
		return Parsed{}, usage("unknown command %q", positionals[0])
	}
}

func validateFilters(action Action, filters []Filter) error {
	for _, f := range filters {
		switch f.Key {
		case FilterKeyName:
		case FilterKeyFork:
			if action != ActionRepos {
				return usage("filter key %q is only supported for repos", f.Key)
			}
			if f.Value != "true" && f.Value != "false" {
				return usage(
					"invalid filter value for fork: expected true or false, got %q",
					f.Value,
				)
			}
		default:
			return usage("unknown filter key %q; supported keys: name, fork", f.Key)
		}
	}
	return nil
}

func validateSort(action Action, sortKeys []string, sortDesc bool) error {
	if len(sortKeys) == 0 {
		if sortDesc {
			return usage("--desc requires --sort")
		}
		return nil
	}

	for _, key := range sortKeys {
		switch action {
		case ActionList:
			switch key {
			case SortKeyAdded, SortKeyName:
			default:
				return usage("unsupported sort key %q for list; supported keys: added, name", key)
			}
		case ActionRepos:
			switch key {
			case SortKeyName, SortKeyStars, SortKeyPushed:
			default:
				return usage(
					"unsupported sort key %q for repos; supported keys: name, stars, pushed",
					key,
				)
			}
		default:
			return nil
		}
	}
	return nil
}

func usage(format string, args ...any) *UsageError {
	return &UsageError{Message: fmt.Sprintf(format, args...)}
}

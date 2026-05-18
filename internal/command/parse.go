package command

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/maoyeedy/gh-star-lists/internal/format"
)

type Action string

const (
	ActionList   Action = "list"
	ActionRepos  Action = "repos"
	ActionCreate Action = "create"
	ActionEdit   Action = "edit"
	ActionDelete Action = "delete"
	ActionAdd    Action = "add"
	ActionRemove Action = "remove"
	ActionMove   Action = "move"
	ActionCopy   Action = "copy"
	ActionMerge  Action = "merge"
	ActionUnstar Action = "unstar"
	ActionHelp   Action = "help"
)

const (
	SortKeyAdded     = "added"
	SortKeyName      = "name"
	SortKeyStars     = "stars"
	SortKeyPushed    = "pushed"
	SortKeyLanguage  = "language"
	SortKeyRepoCount = "repos"
	SortKeyStarred   = "starred"
)

const (
	FilterKeyName     = "name"
	FilterKeyFork     = "fork"
	FilterKeyLanguage = "language"
	FilterKeyArchived = "archived"
	FilterKeyLicense  = "license"
	FilterKeyMinStars = "min-stars"
	FilterKeyMaxStars = "max-stars"
	FilterKeyTopic    = "topic"
)

var commandAliases = map[string]string{
	"ls": "list",
	"rm": "remove",
	"mv": "move",
	"cp": "copy",
}

func canonicalCommand(token string) string {
	if c, ok := commandAliases[token]; ok {
		return c
	}
	switch token {
	case "list", "repos", "create", "edit", "delete",
		"add", "remove", "move", "copy", "merge", "unstar":
		return token
	}
	return ""
}

type Filter struct {
	Key   string
	Value string
}

type SortTerm struct {
	Key  string
	Desc bool
}

type Parsed struct {
	Action         Action
	HelpTopic      string
	FullHelp       bool
	ListID         string
	FromListID     string
	ToListID       string
	RepoName       string
	Name           string
	Description    string
	DescriptionSet bool
	Private        bool
	PrivateSet     bool
	Mode           format.OutputMode
	SortKeys       []string
	SortTerms      []SortTerm
	SortDesc       bool
	NoColor        bool
	Limit          int
	CacheTTL       *time.Duration
	Filters        []Filter
	Search         string
	OutputPath     string
	Template       string
	JQ             string
	Host           string
	Web            bool
	Unlisted       bool
	All            bool
	Yes            bool
	DryRun         bool
	DeleteSource   bool
}

type UsageError struct {
	Message string
}

func (e *UsageError) Error() string {
	return e.Message
}

type UnknownCommandError struct {
	Command string
}

func (e *UnknownCommandError) Error() string {
	return fmt.Sprintf("unknown command %q for \"gh star-lists\"", e.Command)
}

func Parse(argv []string) (Parsed, error) {
	var (
		positionals      []string
		helpFlag         bool
		fullFlag         bool
		jsonFlag         bool
		tsvFlag          bool
		plainFlag        bool
		sortKeys         []string
		sortTerms        []SortTerm
		rawSortTerms     []string
		sortDesc         bool
		limit            int
		cacheTTL         *time.Duration
		noColorFlag      bool
		filters          []Filter
		searchValue      string
		outputPath       string
		templateStr      string
		jqValue          string
		hostValue        string
		webFlag          bool
		unlistedFlag     bool
		allFlag          bool
		yesFlag          bool
		dryRunFlag       bool
		deleteSourceFlag bool
		toValue          string
		fromValue        string
		nameValue        string
		descriptionValue string
		descriptionSet   bool
		privateFlag      bool
		publicFlag       bool
	)

	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		switch arg {
		case "":
			return Parsed{}, usage("empty argument")
		case "--help", "-h":
			helpFlag = true
		case "--full":
			fullFlag = true
		case "--json":
			jsonFlag = true
		case "--tsv":
			tsvFlag = true
		case "--plain":
			plainFlag = true
		case "--sort", "-s":
			if i+1 >= len(argv) {
				return Parsed{}, usage("missing value for %s", arg)
			}
			i++
			raw := argv[i]
			if raw == "" {
				return Parsed{}, usage("empty value for %s", arg)
			}
			if strings.HasPrefix(raw, "-") {
				return Parsed{}, usage("missing value for %s", arg)
			}
			rawSortTerms = append(rawSortTerms, strings.Split(raw, ",")...)
		case "--desc", "-d":
			sortDesc = true
		case "--cache-ttl":
			if i+1 >= len(argv) {
				return Parsed{}, usage("missing value for --cache-ttl")
			}
			i++
			d, err := time.ParseDuration(argv[i])
			if err != nil {
				return Parsed{}, usage("invalid value for --cache-ttl: %v", err)
			}
			if d < 0 {
				return Parsed{}, usage("--cache-ttl must not be negative")
			}
			cacheTTL = &d
		case "--no-color":
			noColorFlag = true
		case "--host":
			if i+1 >= len(argv) {
				return Parsed{}, usage("missing value for --host")
			}
			i++
			hostValue = strings.TrimSpace(argv[i])
			if hostValue == "" {
				return Parsed{}, usage("empty value for --host")
			}
			if err := validateHost(hostValue); err != nil {
				return Parsed{}, err
			}
		case "--web", "-w":
			webFlag = true
		case "--unlisted":
			unlistedFlag = true
		case "--all":
			allFlag = true
		case "--yes", "-y":
			yesFlag = true
		case "--dry-run":
			dryRunFlag = true
		case "--delete-source":
			deleteSourceFlag = true
		case "--private":
			privateFlag = true
		case "--public":
			publicFlag = true
		case "--to":
			if i+1 >= len(argv) {
				return Parsed{}, usage("missing value for --to")
			}
			i++
			toValue = argv[i]
			if toValue == "" {
				return Parsed{}, usage("empty value for --to")
			}
		case "--from":
			if i+1 >= len(argv) {
				return Parsed{}, usage("missing value for --from")
			}
			i++
			fromValue = argv[i]
			if fromValue == "" {
				return Parsed{}, usage("empty value for --from")
			}
		case "--name", "-n":
			if i+1 >= len(argv) {
				return Parsed{}, usage("missing value for %s", arg)
			}
			i++
			nameValue = argv[i]
			if nameValue == "" {
				return Parsed{}, usage("empty value for %s", arg)
			}
		case "--description", "-D":
			if i+1 >= len(argv) {
				return Parsed{}, usage("missing value for %s", arg)
			}
			i++
			descriptionValue = argv[i]
			descriptionSet = true
		case "--limit", "-l":
			if i+1 >= len(argv) {
				return Parsed{}, usage("missing value for %s", arg)
			}
			i++
			v, err := strconv.Atoi(argv[i])
			if err != nil {
				return Parsed{}, usage("invalid value for %s: %s", arg, argv[i])
			}
			if v <= 0 {
				return Parsed{}, usage("invalid value for %s: must be positive", arg)
			}
			limit = v
		case "--filter", "-f":
			if i+1 >= len(argv) {
				return Parsed{}, usage("missing value for %s", arg)
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
		case "--search", "-S":
			if i+1 >= len(argv) {
				return Parsed{}, usage("missing value for %s", arg)
			}
			i++
			searchValue = strings.TrimSpace(argv[i])
			if searchValue == "" {
				return Parsed{}, usage("empty value for --search")
			}
			if len(searchTerms(searchValue)) == 0 {
				return Parsed{}, usage("search has no searchable terms")
			}
		case "--template":
			if i+1 >= len(argv) {
				return Parsed{}, usage("missing value for --template")
			}
			i++
			templateStr = argv[i]
			if templateStr == "" {
				return Parsed{}, usage("empty value for --template")
			}
		case "--jq":
			if i+1 >= len(argv) {
				return Parsed{}, usage("missing value for --jq")
			}
			i++
			jqValue = argv[i]
			if jqValue == "" {
				return Parsed{}, usage("empty value for --jq")
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
	if jqValue != "" {
		if tsvFlag || plainFlag || templateStr != "" {
			return Parsed{}, usage("--jq cannot be combined with --plain, --tsv, or --template")
		}
		mode = format.OutputJSON
	}
	sortKeys, sortTerms, err = parseSortTerms(rawSortTerms, sortDesc)
	if err != nil {
		return Parsed{}, err
	}
	if privateFlag && publicFlag {
		return Parsed{}, usage("cannot combine --private and --public")
	}
	privateSet := privateFlag || publicFlag
	privateValue := privateFlag

	if helpFlag || fullFlag {
		topic := ""
		if len(positionals) > 0 {
			topic = canonicalCommand(positionals[0])
		}
		return Parsed{
			Action:    ActionHelp,
			HelpTopic: topic,
			FullHelp:  fullFlag,
			Mode:      format.OutputHuman,
		}, nil
	}

	if len(positionals) > 0 {
		cmd := positionals[0]
		if c, ok := commandAliases[cmd]; ok {
			cmd = c
		}
		switch cmd {
		case "list":
			if len(positionals) > 1 {
				return Parsed{}, usage(
					"too many arguments for list: %s",
					strings.Join(positionals[1:], " "),
				)
			}
		case "repos":
			if allFlag && unlistedFlag {
				return Parsed{}, usage("cannot combine --all and --unlisted")
			}
			if unlistedFlag && len(positionals) > 1 {
				return Parsed{}, usage("--unlisted does not accept a list id")
			}
			if allFlag && len(positionals) > 1 {
				return Parsed{}, usage("--all does not accept a list id")
			}
			if !unlistedFlag && !allFlag && len(positionals) > 2 {
				return Parsed{}, usage(
					"too many arguments for repos: %s",
					strings.Join(positionals[2:], " "),
				)
			}
			if webFlag &&
				(jsonFlag || tsvFlag || plainFlag || templateStr != "" || outputPath != "" || jqValue != "") {
				return Parsed{}, usage("--web cannot be combined with output flags")
			}
			if err := validateFilters(ActionRepos, filters); err != nil {
				return Parsed{}, err
			}
			if err := validateSort(ActionRepos, sortKeys, sortDesc); err != nil {
				return Parsed{}, err
			}
			listID := ""
			if !unlistedFlag && len(positionals) > 1 {
				listID = positionals[1]
			}
			return Parsed{
				Action:     ActionRepos,
				ListID:     listID,
				Mode:       mode,
				SortKeys:   sortKeys,
				SortTerms:  sortTerms,
				SortDesc:   sortDesc,
				Limit:      limit,
				NoColor:    noColorFlag,
				Filters:    filters,
				Search:     searchValue,
				CacheTTL:   cacheTTL,
				OutputPath: outputPath,
				Template:   templateStr,
				JQ:         jqValue,
				Host:       hostValue,
				Web:        webFlag,
				Unlisted:   unlistedFlag,
				All:        allFlag,
				Yes:        yesFlag,
			}, nil
		case "create":
			if len(positionals) > 2 {
				return Parsed{}, usage("create accepts at most one list name")
			}
			if err := validateWriteOutputFlags(
				jsonFlag,
				tsvFlag,
				plainFlag,
				templateStr,
				outputPath,
				jqValue,
			); err != nil {
				return Parsed{}, err
			}
			if err := validateWriteSearchFlag(searchValue); err != nil {
				return Parsed{}, err
			}
			name := ""
			if len(positionals) == 2 {
				name = positionals[1]
			}
			return Parsed{
				Action:         ActionCreate,
				Name:           name,
				Description:    descriptionValue,
				DescriptionSet: descriptionSet,
				Private:        privateValue,
				PrivateSet:     privateSet,
				Mode:           mode,
				DryRun:         dryRunFlag,
				Host:           hostValue,
			}, nil
		case "edit":
			if len(positionals) != 2 {
				return Parsed{}, usage("edit requires exactly one list id or name")
			}
			if err := validateWriteOutputFlags(
				jsonFlag,
				tsvFlag,
				plainFlag,
				templateStr,
				outputPath,
				jqValue,
			); err != nil {
				return Parsed{}, err
			}
			if err := validateWriteSearchFlag(searchValue); err != nil {
				return Parsed{}, err
			}
			return Parsed{
				Action:         ActionEdit,
				ListID:         positionals[1],
				Name:           nameValue,
				Description:    descriptionValue,
				DescriptionSet: descriptionSet,
				Private:        privateValue,
				PrivateSet:     privateSet,
				Mode:           mode,
				DryRun:         dryRunFlag,
				Host:           hostValue,
			}, nil
		case "delete":
			if len(positionals) != 2 {
				return Parsed{}, usage("delete requires exactly one list id or name")
			}
			if err := validateWriteOutputFlags(
				jsonFlag,
				tsvFlag,
				plainFlag,
				templateStr,
				outputPath,
				jqValue,
			); err != nil {
				return Parsed{}, err
			}
			if err := validateWriteSearchFlag(searchValue); err != nil {
				return Parsed{}, err
			}
			return Parsed{
				Action: ActionDelete,
				ListID: positionals[1],
				Mode:   mode,
				Yes:    yesFlag,
				DryRun: dryRunFlag,
				Host:   hostValue,
			}, nil
		case "add":
			if len(positionals) != 2 {
				return Parsed{}, usage("add requires exactly one repository owner/name")
			}
			if err := validateWriteOutputFlags(
				jsonFlag,
				tsvFlag,
				plainFlag,
				templateStr,
				outputPath,
				jqValue,
			); err != nil {
				return Parsed{}, err
			}
			if err := validateWriteSearchFlag(searchValue); err != nil {
				return Parsed{}, err
			}
			return Parsed{
				Action:   ActionAdd,
				RepoName: positionals[1],
				ToListID: toValue,
				Mode:     mode,
				DryRun:   dryRunFlag,
				Host:     hostValue,
			}, nil
		case "remove":
			if len(positionals) != 2 {
				return Parsed{}, usage("remove requires exactly one repository owner/name")
			}
			if err := validateWriteOutputFlags(
				jsonFlag,
				tsvFlag,
				plainFlag,
				templateStr,
				outputPath,
				jqValue,
			); err != nil {
				return Parsed{}, err
			}
			if err := validateWriteSearchFlag(searchValue); err != nil {
				return Parsed{}, err
			}
			return Parsed{
				Action:     ActionRemove,
				RepoName:   positionals[1],
				FromListID: fromValue,
				Mode:       mode,
				Yes:        yesFlag,
				DryRun:     dryRunFlag,
				Host:       hostValue,
			}, nil
		case "move":
			if len(positionals) != 2 {
				return Parsed{}, usage("move requires exactly one repository owner/name")
			}
			if fromValue != "" && toValue != "" &&
				strings.EqualFold(strings.TrimSpace(fromValue), strings.TrimSpace(toValue)) {
				return Parsed{}, usage("move requires distinct --from and --to")
			}
			if err := validateWriteOutputFlags(
				jsonFlag,
				tsvFlag,
				plainFlag,
				templateStr,
				outputPath,
				jqValue,
			); err != nil {
				return Parsed{}, err
			}
			if err := validateWriteSearchFlag(searchValue); err != nil {
				return Parsed{}, err
			}
			return Parsed{
				Action:     ActionMove,
				RepoName:   positionals[1],
				FromListID: fromValue,
				ToListID:   toValue,
				Mode:       mode,
				Yes:        yesFlag,
				DryRun:     dryRunFlag,
				Host:       hostValue,
			}, nil
		case "copy", "merge":
			if len(positionals) != 1 {
				return Parsed{}, usage("%s does not accept positional arguments", cmd)
			}
			if fromValue != "" && toValue != "" &&
				strings.EqualFold(strings.TrimSpace(fromValue), strings.TrimSpace(toValue)) {
				return Parsed{}, usage("%s requires distinct --from and --to", cmd)
			}
			if err := validateWriteOutputFlags(
				jsonFlag,
				tsvFlag,
				plainFlag,
				templateStr,
				outputPath,
				jqValue,
			); err != nil {
				return Parsed{}, err
			}
			if err := validateWriteSearchFlag(searchValue); err != nil {
				return Parsed{}, err
			}
			action := ActionCopy
			if cmd == "merge" {
				action = ActionMerge
			}
			return Parsed{
				Action:       action,
				FromListID:   fromValue,
				ToListID:     toValue,
				Mode:         mode,
				Yes:          yesFlag,
				DryRun:       dryRunFlag,
				DeleteSource: deleteSourceFlag,
				Host:         hostValue,
			}, nil
		case "unstar":
			if len(positionals) != 2 {
				return Parsed{}, usage("unstar requires exactly one repository owner/name")
			}
			if err := validateWriteOutputFlags(
				jsonFlag,
				tsvFlag,
				plainFlag,
				templateStr,
				outputPath,
				jqValue,
			); err != nil {
				return Parsed{}, err
			}
			if err := validateWriteSearchFlag(searchValue); err != nil {
				return Parsed{}, err
			}
			return Parsed{
				Action:   ActionUnstar,
				RepoName: positionals[1],
				Mode:     mode,
				Yes:      yesFlag,
				DryRun:   dryRunFlag,
				Host:     hostValue,
			}, nil
		default:
			return Parsed{}, &UnknownCommandError{Command: positionals[0]}
		}
	}

	if webFlag {
		return Parsed{}, usage("--web is only supported for repos")
	}
	if unlistedFlag {
		return Parsed{}, usage("--unlisted is only supported for repos")
	}
	if allFlag {
		return Parsed{}, usage("--all is only supported for repos")
	}
	if searchValue != "" {
		return Parsed{}, usage("--search is only supported for repos")
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
		SortTerms:  sortTerms,
		SortDesc:   sortDesc,
		Limit:      limit,
		NoColor:    noColorFlag,
		Filters:    filters,
		CacheTTL:   cacheTTL,
		OutputPath: outputPath,
		Template:   templateStr,
		JQ:         jqValue,
		Host:       hostValue,
		Yes:        yesFlag,
	}, nil
}

func validateWriteOutputFlags(
	jsonFlag, tsvFlag, plainFlag bool,
	templateStr string,
	outputPath string,
	jqValue string,
) error {
	if jsonFlag || tsvFlag || plainFlag || templateStr != "" || outputPath != "" || jqValue != "" {
		return usage("output flags are not supported for write commands")
	}
	return nil
}

func validateWriteSearchFlag(searchValue string) error {
	if searchValue != "" {
		return usage("--search is only supported for repos")
	}
	return nil
}

func parseSortTerms(rawTerms []string, globalDesc bool) ([]string, []SortTerm, error) {
	keys := make([]string, 0, len(rawTerms))
	terms := make([]SortTerm, 0, len(rawTerms))
	hasDirection := false
	for _, raw := range rawTerms {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		key, direction, hasDir := strings.Cut(raw, ":")
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, nil, usage("empty sort key in %q", raw)
		}
		desc := globalDesc
		if hasDir {
			hasDirection = true
			switch strings.ToLower(strings.TrimSpace(direction)) {
			case "asc":
				desc = false
			case "desc":
				desc = true
			default:
				return nil, nil, usage(
					"invalid sort direction %q for %q: expected asc or desc",
					direction,
					key,
				)
			}
		}
		keys = append(keys, key)
		terms = append(terms, SortTerm{Key: key, Desc: desc})
	}
	if !hasDirection {
		if len(keys) == 0 {
			return nil, nil, nil
		}
		return keys, nil, nil
	}
	return keys, terms, nil
}

func validateHost(host string) error {
	if strings.Contains(host, "://") || strings.Contains(host, "/") {
		return usage("invalid value for --host: expected hostname, got %q", host)
	}
	return nil
}

var reposOnlyFilterKeys = map[string]struct{}{
	FilterKeyFork:     {},
	FilterKeyLanguage: {},
	FilterKeyArchived: {},
	FilterKeyLicense:  {},
	FilterKeyTopic:    {},
	FilterKeyMinStars: {},
	FilterKeyMaxStars: {},
}

func validateFilters(action Action, filters []Filter) error {
	for i, f := range filters {
		if _, reposOnly := reposOnlyFilterKeys[f.Key]; reposOnly && action != ActionRepos {
			return usage("filter key %q is only supported for repos", f.Key)
		}
		switch f.Key {
		case FilterKeyTopic:
			if strings.Contains(f.Value, ",") {
				return usage(
					"invalid filter value for topic: only one topic per --filter; repeat the flag for AND semantics",
				)
			}
		case FilterKeyName, FilterKeyLanguage, FilterKeyLicense:
		case FilterKeyFork, FilterKeyArchived:
			if f.Value != "true" && f.Value != "false" {
				return usage(
					"invalid filter value for %s: expected true or false, got %q",
					f.Key, f.Value,
				)
			}
		case FilterKeyMinStars, FilterKeyMaxStars:
			n, err := strconv.Atoi(f.Value)
			if err != nil {
				return usage(
					"invalid filter value for %s: expected integer, got %q",
					f.Key,
					f.Value,
				)
			}
			if n < 0 {
				filters[i].Value = "0"
			}
		default:
			return usage(
				"unknown filter key %q; supported keys: name, fork, language, archived, license, min-stars, max-stars, topic",
				f.Key,
			)
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
			case SortKeyAdded, SortKeyName, SortKeyRepoCount:
			default:
				return usage(
					"unsupported sort key %q for list; supported keys: added, name, repos",
					key,
				)
			}
		case ActionRepos:
			switch key {
			case SortKeyName, SortKeyStars, SortKeyPushed, SortKeyLanguage, SortKeyStarred:
			default:
				return usage(
					"unsupported sort key %q for repos; supported keys: name, stars, pushed, language, starred",
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

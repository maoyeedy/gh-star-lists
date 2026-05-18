package command

import (
	"fmt"
	"strconv"
	"strings"

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

type Filter struct {
	Key   string
	Value string
}

type SortTerm struct {
	Key  string
	Desc bool
}

type Parsed struct {
	Action       Action
	ListID       string
	FromListID   string
	ToListID     string
	RepoName     string
	Name         string
	Description  string
	Private      bool
	PrivateSet   bool
	Mode         format.OutputMode
	SortKeys     []string
	SortTerms    []SortTerm
	SortDesc     bool
	NoColor      bool
	Limit        int
	Cache        bool
	Filters      []Filter
	Search       string
	OutputPath   string
	Template     string
	JQ           string
	Host         string
	Web          bool
	Unlisted     bool
	All          bool
	Yes          bool
	DryRun       bool
	DeleteSource bool
}

type UsageError struct {
	Message string
}

func (e *UsageError) Error() string {
	return e.Message
}

func Parse(argv []string) (Parsed, error) {
	var (
		positionals      []string
		jsonFlag         bool
		tsvFlag          bool
		plainFlag        bool
		sortKeys         []string
		sortTerms        []SortTerm
		rawSortTerms     []string
		sortDesc         bool
		limit            int
		cacheFlag        bool
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
		privateFlag      bool
		publicFlag       bool
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
			rawSortTerms = append(rawSortTerms, strings.Split(raw, ",")...)
		case "--desc":
			sortDesc = true
		case "--cache":
			cacheFlag = true
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
		case "--web":
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
		case "--name":
			if i+1 >= len(argv) {
				return Parsed{}, usage("missing value for --name")
			}
			i++
			nameValue = argv[i]
			if nameValue == "" {
				return Parsed{}, usage("empty value for --name")
			}
		case "--description":
			if i+1 >= len(argv) {
				return Parsed{}, usage("missing value for --description")
			}
			i++
			descriptionValue = argv[i]
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
		case "--search":
			if i+1 >= len(argv) {
				return Parsed{}, usage("missing value for --search")
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

	if len(positionals) > 0 {
		switch positionals[0] {
		case "list":
			if len(positionals) > 1 {
				return Parsed{}, usage(
					"too many arguments for list: %s",
					strings.Join(positionals[1:], " "),
				)
			}
		case "repos":
			if len(positionals) == 1 {
				if !unlistedFlag && !allFlag {
					return Parsed{}, usage("missing list id for repos")
				}
			}
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
				Cache:      cacheFlag,
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
			if len(positionals) != 2 {
				return Parsed{}, usage("create requires exactly one list name")
			}
			if err := validateWriteOutputFlags(jsonFlag, tsvFlag, plainFlag, templateStr, outputPath, jqValue); err != nil {
				return Parsed{}, err
			}
			if err := validateWriteSearchFlag(searchValue); err != nil {
				return Parsed{}, err
			}
			return Parsed{Action: ActionCreate, Name: positionals[1], Description: descriptionValue, Private: privateValue, PrivateSet: privateSet, Mode: mode, DryRun: dryRunFlag, Host: hostValue}, nil
		case "edit":
			if len(positionals) != 2 {
				return Parsed{}, usage("edit requires exactly one list id or name")
			}
			if nameValue == "" && descriptionValue == "" && !privateSet {
				return Parsed{}, usage("edit requires --name, --description, --private, or --public")
			}
			if err := validateWriteOutputFlags(jsonFlag, tsvFlag, plainFlag, templateStr, outputPath, jqValue); err != nil {
				return Parsed{}, err
			}
			if err := validateWriteSearchFlag(searchValue); err != nil {
				return Parsed{}, err
			}
			return Parsed{Action: ActionEdit, ListID: positionals[1], Name: nameValue, Description: descriptionValue, Private: privateValue, PrivateSet: privateSet, Mode: mode, DryRun: dryRunFlag, Host: hostValue}, nil
		case "delete":
			if len(positionals) != 2 {
				return Parsed{}, usage("delete requires exactly one list id or name")
			}
			if err := validateWriteOutputFlags(jsonFlag, tsvFlag, plainFlag, templateStr, outputPath, jqValue); err != nil {
				return Parsed{}, err
			}
			if err := validateWriteSearchFlag(searchValue); err != nil {
				return Parsed{}, err
			}
			return Parsed{Action: ActionDelete, ListID: positionals[1], Mode: mode, Yes: yesFlag, DryRun: dryRunFlag, Host: hostValue}, nil
		case "add":
			if len(positionals) != 2 {
				return Parsed{}, usage("add requires exactly one repository owner/name")
			}
			if toValue == "" {
				return Parsed{}, usage("add requires --to <LIST_ID_OR_NAME>")
			}
			if err := validateWriteOutputFlags(jsonFlag, tsvFlag, plainFlag, templateStr, outputPath, jqValue); err != nil {
				return Parsed{}, err
			}
			if err := validateWriteSearchFlag(searchValue); err != nil {
				return Parsed{}, err
			}
			return Parsed{Action: ActionAdd, RepoName: positionals[1], ToListID: toValue, Mode: mode, DryRun: dryRunFlag, Host: hostValue}, nil
		case "remove":
			if len(positionals) != 2 {
				return Parsed{}, usage("remove requires exactly one repository owner/name")
			}
			if fromValue == "" {
				return Parsed{}, usage("remove requires --from <LIST_ID_OR_NAME>")
			}
			if err := validateWriteOutputFlags(jsonFlag, tsvFlag, plainFlag, templateStr, outputPath, jqValue); err != nil {
				return Parsed{}, err
			}
			if err := validateWriteSearchFlag(searchValue); err != nil {
				return Parsed{}, err
			}
			return Parsed{Action: ActionRemove, RepoName: positionals[1], FromListID: fromValue, Mode: mode, Yes: yesFlag, DryRun: dryRunFlag, Host: hostValue}, nil
		case "move":
			if len(positionals) != 2 {
				return Parsed{}, usage("move requires exactly one repository owner/name")
			}
			if fromValue == "" || toValue == "" {
				return Parsed{}, usage("move requires --from and --to")
			}
			if strings.EqualFold(strings.TrimSpace(fromValue), strings.TrimSpace(toValue)) {
				return Parsed{}, usage("move requires distinct --from and --to")
			}
			if err := validateWriteOutputFlags(jsonFlag, tsvFlag, plainFlag, templateStr, outputPath, jqValue); err != nil {
				return Parsed{}, err
			}
			if err := validateWriteSearchFlag(searchValue); err != nil {
				return Parsed{}, err
			}
			return Parsed{Action: ActionMove, RepoName: positionals[1], FromListID: fromValue, ToListID: toValue, Mode: mode, Yes: yesFlag, DryRun: dryRunFlag, Host: hostValue}, nil
		case "copy", "merge":
			if len(positionals) != 1 {
				return Parsed{}, usage("%s does not accept positional arguments", positionals[0])
			}
			if fromValue == "" || toValue == "" {
				return Parsed{}, usage("%s requires --from and --to", positionals[0])
			}
			if strings.EqualFold(strings.TrimSpace(fromValue), strings.TrimSpace(toValue)) {
				return Parsed{}, usage("%s requires distinct --from and --to", positionals[0])
			}
			if err := validateWriteOutputFlags(jsonFlag, tsvFlag, plainFlag, templateStr, outputPath, jqValue); err != nil {
				return Parsed{}, err
			}
			if err := validateWriteSearchFlag(searchValue); err != nil {
				return Parsed{}, err
			}
			action := ActionCopy
			if positionals[0] == "merge" {
				action = ActionMerge
			}
			return Parsed{Action: action, FromListID: fromValue, ToListID: toValue, Mode: mode, Yes: yesFlag, DryRun: dryRunFlag, DeleteSource: deleteSourceFlag, Host: hostValue}, nil
		case "unstar":
			if len(positionals) != 2 {
				return Parsed{}, usage("unstar requires exactly one repository owner/name")
			}
			if err := validateWriteOutputFlags(jsonFlag, tsvFlag, plainFlag, templateStr, outputPath, jqValue); err != nil {
				return Parsed{}, err
			}
			if err := validateWriteSearchFlag(searchValue); err != nil {
				return Parsed{}, err
			}
			return Parsed{Action: ActionUnstar, RepoName: positionals[1], Mode: mode, Yes: yesFlag, DryRun: dryRunFlag, Host: hostValue}, nil
		default:
			return Parsed{}, usage("unknown command %q", positionals[0])
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
		Cache:      cacheFlag,
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

func HostFromArgs(argv []string) string {
	for i := 0; i < len(argv); i++ {
		if argv[i] == "--host" && i+1 < len(argv) {
			return strings.TrimSpace(argv[i+1])
		}
	}
	return ""
}

func CacheFromArgs(argv []string) bool {
	for _, arg := range argv {
		if arg == "--cache" {
			return true
		}
	}
	return false
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
				return nil, nil, usage("invalid sort direction %q for %q: expected asc or desc", direction, key)
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
				return usage("invalid filter value for topic: only one topic per --filter; repeat the flag for AND semantics")
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
				return usage("invalid filter value for %s: expected integer, got %q", f.Key, f.Value)
			}
			if n < 0 {
				filters[i].Value = "0"
			}
		default:
			return usage("unknown filter key %q; supported keys: name, fork, language, archived, license, min-stars, max-stars, topic", f.Key)
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

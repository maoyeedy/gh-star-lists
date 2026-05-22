package command

import (
	"fmt"
	"strconv"
	"strings"
)

var reposOnlyFilterKeys = map[string]struct{}{
	FilterKeyFork:     {},
	FilterKeyLanguage: {},
	FilterKeyArchived: {},
	FilterKeyLicense:  {},
	FilterKeyTopic:    {},
	FilterKeyMinStars: {},
	FilterKeyMaxStars: {},
}

func validateWriteOutputFlags(
	jsonFlag, tsvFlag, plainFlag, fzfFlag bool,
	templateStr string,
	outputPath string,
	jqValue string,
) error {
	if jsonFlag || tsvFlag || plainFlag || fzfFlag || templateStr != "" || outputPath != "" ||
		jqValue != "" {
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

func validateHost(host string) error {
	if strings.Contains(host, "://") || strings.Contains(host, "/") {
		return usage("invalid value for --host: expected hostname, got %q", host)
	}
	return nil
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
			case SortKeyName, SortKeyStars, SortKeyPushed, SortKeyLanguage:
			default:
				return usage(
					"unsupported sort key %q for repos; supported keys: name, stars, pushed, language",
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

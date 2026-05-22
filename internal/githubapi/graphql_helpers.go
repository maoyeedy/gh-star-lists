package githubapi

import (
	"fmt"
	"strings"
)

func parseRepoName(nameWithOwner string) (owner, name string, err error) {
	var ok bool
	owner, name, ok = strings.Cut(nameWithOwner, "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return "", "", fmt.Errorf("invalid repository %q: expected owner/name", nameWithOwner)
	}
	return owner, name, nil
}

func limitFromOptions(options []ListOptions) int {
	if len(options) == 0 {
		return 0
	}
	return options[0].Limit
}

func withTopicsFromOptions(options []ListOptions) bool {
	if len(options) == 0 {
		return false
	}
	return options[0].WithTopics
}

func pageFirst(pageSize, limit, current int) int {
	if limit <= 0 {
		return pageSize
	}
	remaining := limit - current
	if remaining <= 0 {
		return 1
	}
	if remaining < pageSize {
		return remaining
	}
	return pageSize
}

func limitReached(limit, count int) bool {
	return limit > 0 && count >= limit
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func listURL(host, login, slug string) string {
	var b strings.Builder
	b.Grow(len("https://") + len(host) + len("/stars/") + len(login) + len("/lists/") + len(slug))
	b.WriteString("https://")
	b.WriteString(host)
	b.WriteString("/stars/")
	b.WriteString(login)
	b.WriteString("/lists/")
	b.WriteString(slug)
	return b.String()
}

package githubapi

import (
	"fmt"
	"strings"

	"github.com/maoyeedy/gh-star-lists/internal/domain"
)

func parseRepoName(nameWithOwner string) (owner, name string, err error) {
	var ok bool
	owner, name, ok = strings.Cut(nameWithOwner, "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return "", "", fmt.Errorf("invalid repository %q: expected owner/name", nameWithOwner)
	}
	return owner, name, nil
}

func limitFromOptions(options []domain.ListOptions) int {
	if len(options) == 0 {
		return 0
	}
	return options[0].Limit
}

func withTopicsFromOptions(options []domain.ListOptions) bool {
	if len(options) == 0 {
		return false
	}
	return options[0].WithTopics
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

package command

import (
	"fmt"

	"github.com/maoyeedy/gh-star-lists/internal/app"
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
	ActionUnstar Action = "unstar"
	ActionHelp   Action = "help"
	ActionTUI    Action = "tui"
)

const (
	SortKeyAdded     = app.SortKeyAdded
	SortKeyName      = app.SortKeyName
	SortKeyStars     = app.SortKeyStars
	SortKeyPushed    = app.SortKeyPushed
	SortKeyLanguage  = app.SortKeyLanguage
	SortKeyRepoCount = app.SortKeyRepoCount
	SortKeyStarred   = app.SortKeyStarred
)

const (
	FilterKeyName     = app.FilterKeyName
	FilterKeyFork     = app.FilterKeyFork
	FilterKeyLanguage = app.FilterKeyLanguage
	FilterKeyArchived = app.FilterKeyArchived
	FilterKeyLicense  = app.FilterKeyLicense
	FilterKeyMinStars = app.FilterKeyMinStars
	FilterKeyMaxStars = app.FilterKeyMaxStars
	FilterKeyTopic    = app.FilterKeyTopic
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
		"add", "remove", "move", "copy", "unstar", "tui":
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
	Filters        []Filter
	Search         string
	OutputPath     string
	Template       string
	Host           string
	Web            bool
	Unlisted       bool
	All            bool
	Yes            bool
	DryRun         bool
	DeleteSource   bool
	Mouse          bool
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

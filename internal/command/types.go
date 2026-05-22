package command

import (
	"fmt"
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
	ActionTUI    Action = "tui"
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
	"ls":     "list",
	"rm":     "remove",
	"mv":     "move",
	"cp":     "copy",
	"browse": "tui",
}

func canonicalCommand(token string) string {
	if c, ok := commandAliases[token]; ok {
		return c
	}
	switch token {
	case "list", "repos", "create", "edit", "delete",
		"add", "remove", "move", "copy", "merge", "unstar", "tui":
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

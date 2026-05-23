package tui

import "charm.land/bubbles/v2/key"

// helpGroup is a named group of key bindings for the full help view.
type helpGroup struct {
	title    string
	bindings []key.Binding
}

type keyMap struct {
	Up      key.Binding
	Down    key.Binding
	PgUp    key.Binding
	PgDn    key.Binding
	Home    key.Binding
	End     key.Binding
	Enter   key.Binding
	Back    key.Binding
	Left    key.Binding
	Right   key.Binding
	Quit    key.Binding
	Sort    key.Binding
	Open    key.Binding
	Refresh key.Binding
	Help    key.Binding

	CreateList key.Binding
	EditList   key.Binding
	DeleteList key.Binding
	AddRepo    key.Binding
	RemoveRepo key.Binding
	MoveRepo   key.Binding
	UnstarRepo key.Binding
	CopyList   key.Binding
	MergeList  key.Binding
	Preview    key.Binding
	Search     key.Binding
	Select     key.Binding
}

var keys = keyMap{
	Up:    key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("up/k", "move up")),
	Down:  key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("down/j", "move down")),
	PgUp:  key.NewBinding(key.WithKeys("pgup", "ctrl+b"), key.WithHelp("pgup", "page up")),
	PgDn:  key.NewBinding(key.WithKeys("pgdown", "ctrl+f"), key.WithHelp("pgdn", "page down")),
	Home:  key.NewBinding(key.WithKeys("home", "g"), key.WithHelp("home/g", "top")),
	End:   key.NewBinding(key.WithKeys("end", "G"), key.WithHelp("end/G", "bottom")),
	Enter: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select/open")),
	Back:  key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back/quit")),
	Left: key.NewBinding(
		key.WithKeys("left"),
		key.WithHelp("left", "focus lists"),
	),
	Right: key.NewBinding(
		key.WithKeys("right"),
		key.WithHelp("right", "focus repos"),
	),
	Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	Sort:    key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "cycle sort")),
	Open:    key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "open in browser")),
	Refresh: key.NewBinding(key.WithKeys("ctrl+r"), key.WithHelp("ctrl+r", "refresh")),
	Help:    key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "toggle help")),

	CreateList: key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new list")),
	EditList:   key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit list")),
	DeleteList: key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete list")),
	AddRepo:    key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add repo to list")),
	RemoveRepo: key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "remove repo from list")),
	MoveRepo:   key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "move repo")),
	UnstarRepo: key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "unstar repo")),
	CopyList:   key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "copy list contents")),
	MergeList:  key.NewBinding(key.WithKeys("C"), key.WithHelp("C", "merge list (destructive)")),
	Preview:    key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "repo details")),
	Search:     key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
	Select:     key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "select")),
}

// helpGroups returns all key binding groups for the full help view.
func (k keyMap) helpGroups() []helpGroup {
	return []helpGroup{
		{
			title: "Navigation",
			bindings: []key.Binding{
				k.Up, k.Down, k.PgUp, k.PgDn, k.Home, k.End,
				k.Left, k.Right, k.Enter, k.Back,
			},
		},
		{
			title: "List Actions",
			bindings: []key.Binding{
				k.CreateList, k.EditList, k.DeleteList, k.CopyList, k.MergeList,
			},
		},
		{
			title: "Repo Actions",
			bindings: []key.Binding{
				k.AddRepo, k.RemoveRepo, k.MoveRepo, k.UnstarRepo,
			},
		},
		{
			title: "Selection/Bulk",
			bindings: []key.Binding{
				k.Select,
			},
		},
		{
			title: "Details",
			bindings: []key.Binding{
				k.Preview, k.Open,
			},
		},
		{
			title: "View",
			bindings: []key.Binding{
				k.Sort, k.Search, k.Refresh, k.Help,
			},
		},
		{
			title: "Quit",
			bindings: []key.Binding{
				k.Quit,
			},
		},
	}
}

// footerBindings returns the key bindings to show in the footer short help
// for the given pane and selection state.
func (k keyMap) footerBindings(active pane, hasSelection bool) []key.Binding {
	if active == paneRepo {
		bindings := []key.Binding{k.Search, k.Select}
		if hasSelection {
			bindings = append(bindings, k.AddRepo, k.RemoveRepo, k.MoveRepo)
		}
		return append(bindings, k.Open, k.Help, k.Quit)
	}
	return []key.Binding{k.Search, k.Enter, k.Sort, k.Help, k.Quit}
}

package tui

import "charm.land/bubbles/v2/key"

type keyMap struct {
	Up      key.Binding
	Down    key.Binding
	Enter   key.Binding
	Back    key.Binding
	Quit    key.Binding
	Sort    key.Binding
	Open    key.Binding
	Refresh key.Binding
	Help    key.Binding
}

var keys = keyMap{
	Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("up/k", "move up")),
	Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("down/j", "move down")),
	Enter:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select/open")),
	Back:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back/quit")),
	Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	Sort:    key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "cycle sort")),
	Open:    key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "open in browser")),
	Refresh: key.NewBinding(key.WithKeys("ctrl+r"), key.WithHelp("ctrl+r", "refresh")),
	Help:    key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "toggle help")),
}

package ui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up           key.Binding
	Down         key.Binding
	Top          key.Binding
	Bottom       key.Binding
	PageUp       key.Binding
	PageDown     key.Binding
	Tab          key.Binding
	Enter        key.Binding
	OpenGithub   key.Binding
	CopyInstall  key.Binding
	Install      key.Binding
	Uninstall    key.Binding
	Upgrade      key.Binding
	Search       key.Binding
	Sort         key.Binding
	Categories   key.Binding
	Help         key.Binding
	Quit         key.Binding
	Escape       key.Binding
	Left         key.Binding
	Right        key.Binding
}

var keys = keyMap{
	Up:          key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑ k", "move up")),
	Down:        key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓ j", "move down")),
	Left:        key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("← h", "move left / back")),
	Right:       key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→ l", "move right")),
	Top:         key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("g", "first")),
	Bottom:      key.NewBinding(key.WithKeys("G", "end"), key.WithHelp("G", "last")),
	PageUp:      key.NewBinding(key.WithKeys("pgup", "ctrl+u"), key.WithHelp("pgup", "page up")),
	PageDown:    key.NewBinding(key.WithKeys("pgdown", "ctrl+d"), key.WithHelp("pgdn", "page down")),
	Tab:         key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch pane")),
	Enter:       key.NewBinding(key.WithKeys("enter"), key.WithHelp("⏎", "open / confirm")),
	OpenGithub:  key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "open on github")),
	CopyInstall: key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "copy install cmd")),
	Install:     key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "install")),
	Uninstall:   key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "uninstall (if installed)")),
	Upgrade:     key.NewBinding(key.WithKeys("U"), key.WithHelp("U", "update (if installed)")),
	Search:      key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
	Sort:        key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "cycle sort")),
	Categories:  key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "categories")),
	Help:        key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Quit:        key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
	Escape:      key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back / cancel")),
}

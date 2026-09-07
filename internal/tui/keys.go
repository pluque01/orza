package tui

import (
	"charm.land/bubbles/v2/key"
)

type keyMap struct {
	Up, Down, Left, Right, Home, End         key.Binding
	Open, Toggle, Back, Help, FormHelp, Quit key.Binding
	Next, Previous                           key.Binding
	New, NewFolder                           key.Binding
	Edit, Move, Delete, Connect              key.Binding
	Reload, Save, Confirm, Detail            key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Up:        key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("Up/k", "Move up")),
		Down:      key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("Down/j", "Move down")),
		Left:      key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("Left/h", "Collapse/parent")),
		Right:     key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("Right/l", "Expand/child")),
		Home:      key.NewBinding(key.WithKeys("home", "g"), key.WithHelp("Home/g", "First row")),
		End:       key.NewBinding(key.WithKeys("end", "G"), key.WithHelp("End/G", "Last row")),
		Open:      key.NewBinding(key.WithKeys("enter"), key.WithHelp("Enter", "Open/save")),
		Toggle:    key.NewBinding(key.WithKeys("enter", "space"), key.WithHelp("Enter/Space", "Toggle")),
		Back:      key.NewBinding(key.WithKeys("esc"), key.WithHelp("Esc", "Back")),
		Help:      key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "Help")),
		FormHelp:  key.NewBinding(key.WithKeys("f1"), key.WithHelp("F1", "Help")),
		Quit:      key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "Quit")),
		Next:      key.NewBinding(key.WithKeys("tab"), key.WithHelp("Tab", "Next field")),
		Previous:  key.NewBinding(key.WithKeys("shift+tab", "f2"), key.WithHelp("Shift+Tab/F2", "Previous field")),
		New:       key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "New connection")),
		NewFolder: key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "New folder")),
		Edit:      key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "Edit")),
		Move:      key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "Move")),
		Delete:    key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "Delete")),
		Connect:   key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "Connect")),
		Reload:    key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "Reload")),
		Save:      key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("Ctrl+S", "Save")),
		Confirm:   key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "Confirm")),
		Detail:    key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "Detail")),
	}
}

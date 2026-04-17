package ui

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/mestadler/deskedit/internal/desktop"
)

func (m *Model) openIconPicker() {
	names, _ := desktop.ListAvailableIcons()
	items := make([]list.Item, len(names))
	for i, n := range names {
		items[i] = iconItem(n)
	}
	delegate := list.NewDefaultDelegate()
	delegate.SetSpacing(0)
	delegate.ShowDescription = false
	l := list.New(items, delegate, m.width-2, m.height-4)
	l.Title = "Pick an icon"
	l.SetFilteringEnabled(true)
	m.iconPicker = l
	m.screen = screenIconPicker
	m.err = nil
}

func (m *Model) updateIconPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if keyMatches(msg, m.keys.IconPicker.ToggleHelp) {
		m.help.ShowAll = !m.help.ShowAll
		return m, nil
	}

	if m.iconPicker.FilterState() == list.Filtering {
		var cmd tea.Cmd
		m.iconPicker, cmd = m.iconPicker.Update(msg)
		return m, cmd
	}

	switch {
	case keyMatches(msg, m.keys.IconPicker.Cancel):
		m.screen = screenEditor
		return m, nil
	case keyMatches(msg, m.keys.IconPicker.Accept):
		if it, ok := m.iconPicker.SelectedItem().(iconItem); ok {
			m.inputs[fieldIcon].SetValue(string(it))
		}
		m.screen = screenEditor
		return m, nil
	}

	var cmd tea.Cmd
	m.iconPicker, cmd = m.iconPicker.Update(msg)
	return m, cmd
}

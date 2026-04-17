package ui

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.toggleHelpIfMatched(msg, m.keys.List.ToggleHelp) {
		return m, nil
	}

	if m.list.FilterState() == list.Filtering {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}

	switch {
	case keyMatches(msg, m.keys.List.Quit):
		m.startConfirm(confirmExit, "Exit deskedit?")
		return m, nil
	case keyMatches(msg, m.keys.List.Edit):
		sel, ok := m.list.SelectedItem().(listItem)
		if !ok {
			return m, nil
		}
		if err := m.openEditor(sel.Entry); err != nil {
			m.err = err
			return m, nil
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

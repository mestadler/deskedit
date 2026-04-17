package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/mestadler/deskedit/internal/desktop"
)

func (m *Model) openInstallPath() {
	m.ensureDefaults()
	m.install.resetForm()
	m.screen = screenInstallPath
	m.err = nil
	m.status = ""
}

func (m *Model) updateInstallPath(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.toggleHelpIfMatched(msg, m.keys.InstallPath.ToggleHelp) {
		return m, nil
	}

	switch {
	case keyMatches(msg, m.keys.InstallPath.Cancel):
		m.screen = screenEditor
		return m, nil
	case keyMatches(msg, m.keys.InstallPath.Browse):
		m.openInstallBrowse(m.install.browseStartDir())
		return m, nil
	case keyMatches(msg, m.keys.InstallPath.NextField):
		m.install.cycleFocus(+1)
		return m, nil
	case keyMatches(msg, m.keys.InstallPath.PrevField):
		m.install.cycleFocus(-1)
		return m, nil
	case keyMatches(msg, m.keys.InstallPath.Install):
		path := strings.TrimSpace(m.install.pathInput.Value())
		name := strings.TrimSpace(m.install.nameInput.Value())
		if path != "" && name == "" {
			m.install.selectFile(path)
			return m, nil
		}
		if path == "" {
			m.openInstallBrowse(m.install.browseStartDir())
			return m, nil
		}
		if name == "" {
			m.err = fmt.Errorf("icon name is required")
			return m, nil
		}
		return m, m.doInstall(path, name)
	}

	return m, m.install.updateFocusedInput(msg)
}

func (m *Model) viewInstallPath() string {
	title := titleStyle.Render("Install new icon")
	pathRow := labelStyle.Render("Source:") + " " + m.install.pathInput.View()
	nameRow := labelStyle.Render("Name:") + " " + m.install.nameInput.View()

	if m.install.focus == 0 {
		pathRow = activeBorder.Render(pathRow)
		nameRow = inactiveBorder.Render(nameRow)
	} else {
		pathRow = inactiveBorder.Render(pathRow)
		nameRow = activeBorder.Render(nameRow)
	}

	return strings.Join([]string{
		title,
		pathRow,
		nameRow,
		hintStyle.Render("PNGs >256px are resized; smaller PNGs keep their size. SVGs go to scalable/."),
	}, "\n")
}

func (m *Model) openInstallBrowse(start string) {
	if err := m.install.openBrowse(start, m.width, m.height); err != nil {
		m.err = err
		return
	}
	m.screen = screenInstallBrowse
	m.err = nil
}

func (m *Model) updateInstallBrowse(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.toggleHelpIfMatched(msg, m.keys.InstallBrowse.ToggleHelp) {
		return m, nil
	}

	if m.install.browser.FilterState() == list.Filtering {
		var cmd tea.Cmd
		m.install.browser, cmd = m.install.browser.Update(msg)
		return m, cmd
	}

	switch {
	case keyMatches(msg, m.keys.InstallBrowse.Back):
		m.screen = screenInstallPath
		return m, nil
	case keyMatches(msg, m.keys.InstallBrowse.OpenSelect):
		it, ok := m.install.browser.SelectedItem().(fsItem)
		if !ok {
			return m, nil
		}
		if it.isDir {
			if err := m.install.enterDirectory(it.path); err != nil {
				m.err = err
				return m, nil
			}
			return m, nil
		}
		m.install.selectFile(it.path)
		m.screen = screenInstallPath
		return m, nil
	}

	var cmd tea.Cmd
	m.install.browser, cmd = m.install.browser.Update(msg)
	return m, cmd
}

func (m *Model) doInstall(path, name string) tea.Cmd {
	return func() tea.Msg {
		res, err := desktop.InstallIcon(desktop.IconInstallRequest{SourcePath: path, Name: name})
		return installDoneMsg{res: res, err: err}
	}
}

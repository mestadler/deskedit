package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/mestadler/deskedit/internal/desktop"
)

func (m *Model) openInstallPath() {
	m.ensureDefaults()

	pathTI := textinput.New()
	pathTI.Placeholder = "/path/to/icon.png"
	pathTI.CharLimit = 4096
	pathTI.Width = 60
	pathTI.Focus()

	nameTI := textinput.New()
	nameTI.Placeholder = "icon name (no extension)"
	nameTI.CharLimit = 128
	nameTI.Width = 40

	m.installPathInput = pathTI
	m.installNameInput = nameTI
	m.installFocus = 0
	m.screen = screenInstallPath
	m.err = nil
	m.status = ""
}

func (m *Model) updateInstallPath(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if keyMatches(msg, m.keys.InstallPath.ToggleHelp) {
		m.help.ShowAll = !m.help.ShowAll
		return m, nil
	}

	switch {
	case keyMatches(msg, m.keys.InstallPath.Cancel):
		m.screen = screenEditor
		return m, nil
	case keyMatches(msg, m.keys.InstallPath.Browse):
		m.openInstallBrowse(m.installBrowseStartDir())
		return m, nil
	case keyMatches(msg, m.keys.InstallPath.NextField):
		m.installCycleFocus(+1)
		return m, nil
	case keyMatches(msg, m.keys.InstallPath.PrevField):
		m.installCycleFocus(-1)
		return m, nil
	case keyMatches(msg, m.keys.InstallPath.Install):
		path := strings.TrimSpace(m.installPathInput.Value())
		name := strings.TrimSpace(m.installNameInput.Value())
		if path != "" && name == "" {
			base := filepath.Base(path)
			base = strings.TrimSuffix(base, filepath.Ext(base))
			m.installNameInput.SetValue(base)
			m.installCycleFocus(+1)
			return m, nil
		}
		if path == "" {
			m.openInstallBrowse(m.installBrowseStartDir())
			return m, nil
		}
		if name == "" {
			m.err = fmt.Errorf("icon name is required")
			return m, nil
		}
		return m, m.doInstall(path, name)
	}

	var cmd tea.Cmd
	if m.installFocus == 0 {
		m.installPathInput, cmd = m.installPathInput.Update(msg)
	} else {
		m.installNameInput, cmd = m.installNameInput.Update(msg)
	}
	return m, cmd
}

func (m *Model) installCycleFocus(delta int) {
	if m.installFocus == 0 {
		m.installPathInput.Blur()
	} else {
		m.installNameInput.Blur()
	}
	m.installFocus = (m.installFocus + delta + 2) % 2
	if m.installFocus == 0 {
		m.installPathInput.Focus()
	} else {
		m.installNameInput.Focus()
	}
}

func (m *Model) viewInstallPath() string {
	title := titleStyle.Render("Install new icon")
	pathRow := labelStyle.Render("Source:") + " " + m.installPathInput.View()
	nameRow := labelStyle.Render("Name:") + " " + m.installNameInput.View()

	if m.installFocus == 0 {
		pathRow = activeBorder.Render(pathRow)
		nameRow = inactiveBorder.Render(nameRow)
	} else {
		pathRow = inactiveBorder.Render(pathRow)
		nameRow = activeBorder.Render(nameRow)
	}

	status := ""
	if m.err != nil {
		status = errorStyle.Render("error: " + m.err.Error())
	} else if m.status != "" {
		status = statusStyle.Render(m.status)
	}

	return strings.Join([]string{
		title,
		pathRow,
		nameRow,
		hintStyle.Render("PNGs >256px are resized; smaller PNGs keep their size. SVGs go to scalable/."),
		m.help.View(m.keys.InstallPath),
		status,
	}, "\n")
}

func (m *Model) openInstallBrowse(start string) {
	items, err := listDir(start)
	if err != nil {
		m.err = err
		return
	}
	delegate := list.NewDefaultDelegate()
	delegate.SetSpacing(0)
	delegate.ShowDescription = false
	l := list.New(items, delegate, m.width-2, m.height-6)
	l.Title = start
	l.SetFilteringEnabled(true)
	m.browser = l
	m.browserCWD = start
	m.lastBrowseDir = start
	m.screen = screenInstallBrowse
	m.err = nil
}

func (m *Model) updateInstallBrowse(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if keyMatches(msg, m.keys.InstallBrowse.ToggleHelp) {
		m.help.ShowAll = !m.help.ShowAll
		return m, nil
	}

	if m.browser.FilterState() == list.Filtering {
		var cmd tea.Cmd
		m.browser, cmd = m.browser.Update(msg)
		return m, cmd
	}

	switch {
	case keyMatches(msg, m.keys.InstallBrowse.Back):
		m.screen = screenInstallPath
		return m, nil
	case keyMatches(msg, m.keys.InstallBrowse.OpenSelect):
		it, ok := m.browser.SelectedItem().(fsItem)
		if !ok {
			return m, nil
		}
		if it.isDir {
			items, err := listDir(it.path)
			if err != nil {
				m.err = err
				return m, nil
			}
			m.browser.SetItems(items)
			m.browser.Title = it.path
			m.browserCWD = it.path
			m.lastBrowseDir = it.path
			m.browser.ResetSelected()
			return m, nil
		}
		m.installPathInput.SetValue(it.path)
		m.lastBrowseDir = filepath.Dir(it.path)
		if strings.TrimSpace(m.installNameInput.Value()) == "" {
			base := filepath.Base(it.path)
			base = strings.TrimSuffix(base, filepath.Ext(base))
			m.installNameInput.SetValue(base)
		}
		m.screen = screenInstallPath
		m.installFocus = 0
		m.installCycleFocus(+1)
		return m, nil
	}

	var cmd tea.Cmd
	m.browser, cmd = m.browser.Update(msg)
	return m, cmd
}

func (m *Model) doInstall(path, name string) tea.Cmd {
	return func() tea.Msg {
		res, err := desktop.InstallIcon(desktop.IconInstallRequest{SourcePath: path, Name: name})
		return installDoneMsg{res: res, err: err}
	}
}

func (m *Model) installBrowseStartDir() string {
	if strings.TrimSpace(m.lastBrowseDir) != "" {
		return m.lastBrowseDir
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "."
	}
	return home
}

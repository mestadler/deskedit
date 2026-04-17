// Package ui implements the deskedit TUI using Bubble Tea.
package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/mestadler/deskedit/internal/desktop"
	"github.com/mestadler/deskedit/internal/gpu"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			MarginBottom(1)
	labelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39")).
			Width(14)
	hintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true)
	activeBorder = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("205")).
			Padding(0, 1)
	inactiveBorder = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1)
	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("242")).
			MarginTop(1)
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)
	badgeUser   = lipgloss.NewStyle().Foreground(lipgloss.Color("40")).Render("[user]")
	badgeSystem = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("[system]")
)

type screen int

const (
	screenList screen = iota
	screenEditor
	screenIconPicker
	screenInstallPath
	screenInstallBrowse
)

type listItem struct{ desktop.Entry }

func (i listItem) Title() string {
	badge := badgeUser
	if i.Source == desktop.SourceSystem {
		badge = badgeSystem
	}
	name := i.Name
	if i.NoDisplay || i.Hidden {
		name += " (hidden)"
	}
	return fmt.Sprintf("%s %s", badge, name)
}

func (i listItem) Description() string { return i.ID }
func (i listItem) FilterValue() string { return i.Name + " " + i.ID }

type iconItem string

func (i iconItem) Title() string       { return string(i) }
func (i iconItem) Description() string { return "" }
func (i iconItem) FilterValue() string { return string(i) }

type field int

const (
	fieldName field = iota
	fieldExec
	fieldIcon
	fieldTerminal
	fieldNoDisplay
	fieldHidden
	fieldGPU
	fieldCount
)

var fieldLabels = [...]string{"Name", "Exec", "Icon", "Terminal", "NoDisplay", "Hidden", "GPU"}

type Model struct {
	screen screen

	entries []desktop.Entry
	list    list.Model

	current    *desktop.File
	currentEnt desktop.Entry
	inputs     []textinput.Model
	terminal   bool
	noDisplay  bool
	hidden     bool
	gpuMode    gpu.Mode
	gpuCaps    gpu.Capabilities
	focused    field

	iconPicker list.Model

	install installModel

	keys keyMaps
	help help.Model

	status string
	err    error

	width, height int
}

type installDoneMsg struct {
	res *desktop.IconInstallResult
	err error
}

type entriesRefreshedMsg struct {
	entries []desktop.Entry
	err     error
}

func New() (*Model, error) {
	entries, err := desktop.Discover()
	if err != nil {
		return nil, err
	}

	delegate := newPlainDelegate()
	l := list.New(nil, delegate, 0, 0)
	l.Title = "Desktop Entries"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)

	h := help.New()
	h.ShowAll = false

	m := &Model{
		screen:  screenList,
		entries: entries,
		list:    l,
		gpuCaps: gpu.Detect(),
		keys:    defaultKeyMaps(),
		help:    h,
	}
	m.setEntries(entries)
	return m, nil
}

func (m *Model) Init() tea.Cmd { return nil }

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case error:
		m.err = msg
		return m, nil
	case installDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.inputs[fieldIcon].SetValue(m.install.nameInput.Value())
		m.screen = screenEditor
		note := fmt.Sprintf("installed: %s", msg.res.InstalledPath)
		if msg.res.Resized {
			note += fmt.Sprintf(" (resized from %dx%d)", msg.res.OriginalW, msg.res.OriginalH)
		}
		if msg.res.CacheUpdateErr != nil {
			note += "  [cache update failed, icon still usable]"
		}
		m.status = note
		m.err = nil
		return m, nil
	case entriesRefreshedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.setEntries(msg.entries)
		m.err = nil
		return m, nil
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.list.SetSize(msg.Width-2, msg.Height-4)
		if m.iconPicker.Items() != nil {
			m.iconPicker.SetSize(msg.Width-2, msg.Height-4)
		}
		if m.install.browser.Items() != nil {
			m.install.browser.SetSize(msg.Width-2, msg.Height-6)
		}
		return m, nil
	case tea.KeyMsg:
		if keyMatches(msg, m.keys.Global.Quit) {
			return m, tea.Quit
		}
		switch m.screen {
		case screenList:
			return m.updateList(msg)
		case screenEditor:
			return m.updateEditor(msg)
		case screenIconPicker:
			return m.updateIconPicker(msg)
		case screenInstallPath:
			return m.updateInstallPath(msg)
		case screenInstallBrowse:
			return m.updateInstallBrowse(msg)
		}
	}

	var cmd tea.Cmd
	switch m.screen {
	case screenList:
		m.list, cmd = m.list.Update(msg)
	case screenIconPicker:
		m.iconPicker, cmd = m.iconPicker.Update(msg)
	case screenInstallBrowse:
		m.install.browser, cmd = m.install.browser.Update(msg)
	}
	return m, cmd
}

func (m *Model) View() string {
	switch m.screen {
	case screenList:
		return m.list.View() + "\n" + m.help.View(m.keys.List)
	case screenEditor:
		return m.viewEditor()
	case screenIconPicker:
		return m.iconPicker.View() + "\n" + m.help.View(m.keys.IconPicker)
	case screenInstallPath:
		return m.viewInstallPath()
	case screenInstallBrowse:
		title := titleStyle.Render("Browse: " + m.install.browserCWD)
		return title + "\n" + m.install.browser.View() + "\n" + m.help.View(m.keys.InstallBrowse)
	}
	return ""
}

func (m *Model) setEntries(entries []desktop.Entry) {
	m.entries = entries
	items := make([]list.Item, len(entries))
	for i, e := range entries {
		items[i] = listItem{e}
	}
	m.list.SetItems(items)
}

func (m *Model) ensureDefaults() {
	if len(m.keys.Global.Quit.Keys()) == 0 {
		m.keys = defaultKeyMaps()
	}
	if m.help.ShortSeparator == "" && m.help.FullSeparator == "" {
		h := help.New()
		h.ShowAll = false
		m.help = h
	}
}

func parseBoolDesktop(v string) bool {
	return strings.EqualFold(strings.TrimSpace(v), "true")
}

func formatBoolDesktop(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func renderStatus(err error, status string) string {
	if err != nil {
		return errorStyle.Render("error: " + err.Error())
	}
	if status != "" {
		return statusStyle.Render(status)
	}
	return ""
}

func activePathForView(path string) string {
	if path == "" {
		return ""
	}
	return filepath.Base(path)
}

func Run() error {
	m, err := New()
	if err != nil {
		return err
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

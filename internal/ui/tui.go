// Package ui implements the deskedit TUI using Bubble Tea.
package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/mestadler/deskedit/internal/desktop"
	"github.com/mestadler/deskedit/internal/gpu"
)

var (
	appFrameStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1)
	chromeTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("230")).
				Background(lipgloss.Color("62")).
				Padding(0, 1)
	commandBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			Background(lipgloss.Color("236")).
			Padding(0, 1)
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
	screenCommandPalette
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

type commandItem struct {
	id   string
	name string
	desc string
}

func (i commandItem) Title() string       { return i.name }
func (i commandItem) Description() string { return i.desc }
func (i commandItem) FilterValue() string { return i.name + " " + i.desc }

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

	commandPalette list.Model
	paletteReturn  screen

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
		if keyMatches(msg, m.keys.Global.CommandPalette) {
			if m.screen == screenCommandPalette {
				m.screen = m.paletteReturn
				return m, nil
			}
			m.openCommandPalette(m.screen)
			return m, nil
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
		case screenCommandPalette:
			return m.updateCommandPalette(msg)
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
	case screenCommandPalette:
		m.commandPalette, cmd = m.commandPalette.Update(msg)
	}
	return m, cmd
}

func (m *Model) View() string {
	var body string
	switch m.screen {
	case screenList:
		body = m.list.View()
	case screenEditor:
		body = m.viewEditor()
	case screenIconPicker:
		body = m.iconPicker.View()
	case screenInstallPath:
		body = m.viewInstallPath()
	case screenInstallBrowse:
		body = m.install.browser.View()
	case screenCommandPalette:
		body = m.commandPalette.View()
	default:
		body = ""
	}

	return m.renderLayout(m.screenTitle(), body, m.expandedHelp(), m.primaryCommandBar())
}

func (m *Model) renderLayout(title, body, expandedHelp, commandBar string) string {
	status := renderStatus(m.err, m.status)
	parts := []string{
		chromeTitleStyle.Render(title),
		body,
	}
	if expandedHelp != "" {
		parts = append(parts, expandedHelp)
	}
	if status != "" {
		parts = append(parts, status)
	}
	parts = append(parts, commandBarStyle.Render(commandBar))
	content := strings.Join(parts, "\n")
	return appFrameStyle.Render(content)
}

func (m *Model) screenTitle() string {
	switch m.screen {
	case screenList:
		return "deskedit  -  Desktop Entries"
	case screenEditor:
		if m.current != nil {
			return "deskedit  -  Editing " + activePathForView(m.current.Path)
		}
		return "deskedit  -  Editor"
	case screenIconPicker:
		return "deskedit  -  Pick Icon"
	case screenInstallPath:
		return "deskedit  -  Install Icon"
	case screenInstallBrowse:
		return "deskedit  -  Browse Files: " + m.install.browserCWD
	case screenCommandPalette:
		return "deskedit  -  Command Palette"
	default:
		return "deskedit"
	}
}

func (m *Model) expandedHelp() string {
	if !m.help.ShowAll {
		return ""
	}
	h := m.help
	h.ShowAll = true

	switch m.screen {
	case screenList:
		return h.View(m.keys.List)
	case screenEditor:
		return h.View(m.keys.Editor)
	case screenIconPicker:
		return h.View(m.keys.IconPicker)
	case screenInstallPath:
		return h.View(m.keys.InstallPath)
	case screenInstallBrowse:
		return h.View(m.keys.InstallBrowse)
	case screenCommandPalette:
		return h.View(m.keys.Palette)
	default:
		return ""
	}
}

func (m *Model) primaryCommandBar() string {
	bindings := m.primaryBindings()
	parts := make([]string, 0, len(bindings))
	for _, b := range bindings {
		h := b.Help()
		if strings.TrimSpace(h.Key) == "" || strings.TrimSpace(h.Desc) == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s %s", h.Key, h.Desc))
	}
	return strings.Join(parts, "   ")
}

func (m *Model) primaryBindings() []key.Binding {
	global := []key.Binding{m.keys.Global.CommandPalette}
	switch m.screen {
	case screenList:
		return append(global, m.keys.List.ShortHelp()...)
	case screenEditor:
		return append(global, m.keys.Editor.ShortHelp()...)
	case screenIconPicker:
		return append(global, m.keys.IconPicker.ShortHelp()...)
	case screenInstallPath:
		return append(global, m.keys.InstallPath.ShortHelp()...)
	case screenInstallBrowse:
		return append(global, m.keys.InstallBrowse.ShortHelp()...)
	case screenCommandPalette:
		return m.keys.Palette.ShortHelp()
	default:
		return global
	}
}

func (m *Model) openCommandPalette(from screen) {
	items := m.commandItemsFor(from)
	listItems := make([]list.Item, len(items))
	for i, it := range items {
		listItems[i] = it
	}

	delegate := list.NewDefaultDelegate()
	delegate.SetSpacing(0)
	delegate.ShowDescription = true

	l := list.New(listItems, delegate, m.width-2, m.height-6)
	if m.width <= 0 || m.height <= 0 {
		l.SetSize(80, 20)
	}
	l.Title = "Commands"
	l.SetFilteringEnabled(true)
	m.commandPalette = l
	m.paletteReturn = from
	m.screen = screenCommandPalette
	m.err = nil
}

func (m *Model) commandItemsFor(sc screen) []commandItem {
	switch sc {
	case screenList:
		return []commandItem{{id: "list_edit", name: "Edit selected entry", desc: "Open selected desktop entry"}, {id: "list_filter", name: "Filter entries", desc: "Start filtering entry list"}, {id: "list_quit", name: "Quit", desc: "Exit deskedit"}, {id: "help_toggle", name: "Toggle expanded help", desc: "Show/hide full help"}}
	case screenEditor:
		return []commandItem{{id: "editor_save", name: "Save changes", desc: "Write desktop entry"}, {id: "editor_cancel", name: "Discard and return", desc: "Return to list without saving"}, {id: "editor_next", name: "Next field", desc: "Move focus to next field"}, {id: "editor_prev", name: "Previous field", desc: "Move focus to previous field"}, {id: "editor_icon_picker", name: "Open icon picker", desc: "Pick icon from available names"}, {id: "editor_install_icon", name: "Browse/install icon", desc: "Install icon from file"}, {id: "help_toggle", name: "Toggle expanded help", desc: "Show/hide full help"}}
	case screenIconPicker:
		return []commandItem{{id: "picker_accept", name: "Accept selected icon", desc: "Use highlighted icon"}, {id: "picker_cancel", name: "Cancel", desc: "Return to editor"}, {id: "picker_filter", name: "Filter icons", desc: "Search icon names"}, {id: "help_toggle", name: "Toggle expanded help", desc: "Show/hide full help"}}
	case screenInstallPath:
		return []commandItem{{id: "install_submit", name: "Install icon", desc: "Install or browse when source empty"}, {id: "install_browse", name: "Browse files", desc: "Open file browser"}, {id: "install_next", name: "Next field", desc: "Move focus to next field"}, {id: "install_prev", name: "Previous field", desc: "Move focus to previous field"}, {id: "install_cancel", name: "Cancel", desc: "Return to editor"}, {id: "help_toggle", name: "Toggle expanded help", desc: "Show/hide full help"}}
	case screenInstallBrowse:
		return []commandItem{{id: "browse_open", name: "Open/select", desc: "Open directory or select file"}, {id: "browse_back", name: "Back to form", desc: "Return to install form"}, {id: "browse_filter", name: "Filter files", desc: "Search files"}, {id: "help_toggle", name: "Toggle expanded help", desc: "Show/hide full help"}}
	default:
		return []commandItem{{id: "help_toggle", name: "Toggle expanded help", desc: "Show/hide full help"}}
	}
}

func (m *Model) updateCommandPalette(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.toggleHelpIfMatched(msg, m.keys.Palette.ToggleHelp) {
		return m, nil
	}

	if m.commandPalette.FilterState() == list.Filtering {
		var cmd tea.Cmd
		m.commandPalette, cmd = m.commandPalette.Update(msg)
		return m, cmd
	}

	switch {
	case keyMatches(msg, m.keys.Palette.Cancel):
		m.screen = m.paletteReturn
		return m, nil
	case keyMatches(msg, m.keys.Palette.Accept):
		it, ok := m.commandPalette.SelectedItem().(commandItem)
		if !ok {
			m.screen = m.paletteReturn
			return m, nil
		}
		return m.executeCommand(it.id, m.paletteReturn)
	}

	var cmd tea.Cmd
	m.commandPalette, cmd = m.commandPalette.Update(msg)
	return m, cmd
}

func (m *Model) executeCommand(id string, target screen) (tea.Model, tea.Cmd) {
	m.screen = target

	switch id {
	case "help_toggle":
		m.help.ShowAll = !m.help.ShowAll
		return m, nil
	case "list_edit":
		return m.updateList(tea.KeyMsg{Type: tea.KeyEnter})
	case "list_filter":
		return m.updateList(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	case "list_quit":
		return m, tea.Quit
	case "editor_save":
		return m.updateEditor(tea.KeyMsg{Type: tea.KeyCtrlS})
	case "editor_cancel":
		return m.updateEditor(tea.KeyMsg{Type: tea.KeyEsc})
	case "editor_next":
		return m.updateEditor(tea.KeyMsg{Type: tea.KeyTab})
	case "editor_prev":
		return m.updateEditor(tea.KeyMsg{Type: tea.KeyShiftTab})
	case "editor_icon_picker":
		return m.updateEditor(tea.KeyMsg{Type: tea.KeyCtrlI})
	case "editor_install_icon":
		return m.updateEditor(tea.KeyMsg{Type: tea.KeyCtrlN})
	case "picker_accept":
		return m.updateIconPicker(tea.KeyMsg{Type: tea.KeyEnter})
	case "picker_cancel":
		return m.updateIconPicker(tea.KeyMsg{Type: tea.KeyEsc})
	case "picker_filter":
		return m.updateIconPicker(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	case "install_submit":
		return m.updateInstallPath(tea.KeyMsg{Type: tea.KeyEnter})
	case "install_browse":
		return m.updateInstallPath(tea.KeyMsg{Type: tea.KeyCtrlB})
	case "install_next":
		return m.updateInstallPath(tea.KeyMsg{Type: tea.KeyTab})
	case "install_prev":
		return m.updateInstallPath(tea.KeyMsg{Type: tea.KeyShiftTab})
	case "install_cancel":
		return m.updateInstallPath(tea.KeyMsg{Type: tea.KeyEsc})
	case "browse_open":
		return m.updateInstallBrowse(tea.KeyMsg{Type: tea.KeyEnter})
	case "browse_back":
		return m.updateInstallBrowse(tea.KeyMsg{Type: tea.KeyEsc})
	case "browse_filter":
		return m.updateInstallBrowse(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	default:
		return m, nil
	}
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

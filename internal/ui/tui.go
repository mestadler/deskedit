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
	headerFrameStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("63")).
				Padding(0, 1)
	headerTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("230"))
	bodyFrameStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1)
	footerFrameStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("63")).
				Padding(0, 1)
	commandBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))
	footerChipStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))
	footerChipFocusedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("230")).
				Bold(true).
				Underline(true)
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

type regionFocus int

const (
	screenList screen = iota
	screenEditor
	screenIconPicker
	screenInstallPath
	screenInstallBrowse
	screenCommandPalette
)

const (
	regionHeader regionFocus = iota
	regionBody
	regionFooter
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
	regionFocus    regionFocus
	footerAction   int

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
		screen:      screenList,
		entries:     entries,
		list:        l,
		gpuCaps:     gpu.Detect(),
		keys:        defaultKeyMaps(),
		help:        h,
		regionFocus: regionBody,
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
		m.applyLayoutSizing(msg.Width, msg.Height)
		return m, nil
	case tea.KeyMsg:
		if keyMatches(msg, m.keys.Global.Quit) {
			return m, tea.Quit
		}
		if keyMatches(msg, m.keys.Global.NextRegion) || isCtrlTab(msg) {
			m.focusNextRegion()
			return m, nil
		}
		if keyMatches(msg, m.keys.Global.PrevRegion) || isCtrlShiftTab(msg) {
			m.focusPrevRegion()
			return m, nil
		}
		if keyMatches(msg, m.keys.Global.CommandPalette) {
			if m.screen == screenCommandPalette {
				m.screen = m.paletteReturn
				m.regionFocus = regionBody
				return m, nil
			}
			m.openCommandPalette(m.screen)
			return m, nil
		}
		if m.regionFocus != regionBody {
			if m.regionFocus == regionFooter {
				if handled, model, cmd := m.updateFooterRegion(msg); handled {
					return model, cmd
				}
			}
			if msg.String() == "esc" {
				m.regionFocus = regionBody
			}
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
	availW, availH := m.layoutViewport()
	innerW := maxInt(1, availW-appFrameStyle.GetHorizontalFrameSize())
	innerH := maxInt(1, availH-appFrameStyle.GetVerticalFrameSize())

	headFrameH := 1 + headerFrameStyle.GetVerticalFrameSize()
	if strings.TrimSpace(title) != "" {
		headFrameH++
	}
	footFrameH := 1 + footerFrameStyle.GetVerticalFrameSize()
	bodyContentH := maxInt(3, innerH-headFrameH-footFrameH)

	status := renderStatus(m.err, m.status)
	bodyParts := []string{body}
	if expandedHelp != "" {
		bodyParts = append(bodyParts, expandedHelp)
	}
	if status != "" {
		bodyParts = append(bodyParts, status)
	}

	headerWidth := innerW - headerFrameStyle.GetHorizontalFrameSize()
	headerLines := []string{lipgloss.PlaceHorizontal(headerWidth, lipgloss.Center, headerTitleStyle.Render("deskedit"))}
	if strings.TrimSpace(title) != "" {
		headerLines = append(headerLines, lipgloss.PlaceHorizontal(headerWidth, lipgloss.Center, hintStyle.Render(title)))
	}
	headerText := strings.Join(headerLines, "\n")
	headerStyle := focusedFrameStyle(headerFrameStyle, m.regionFocus == regionHeader)
	bodyStyle := focusedFrameStyle(bodyFrameStyle, m.regionFocus == regionBody)
	footerStyle := focusedFrameStyle(footerFrameStyle, m.regionFocus == regionFooter)

	header := headerStyle.Width(innerW - headerStyle.GetHorizontalFrameSize()).Render(headerText)
	bodyPanel := bodyStyle.Width(innerW - bodyStyle.GetHorizontalFrameSize()).Height(bodyContentH - bodyStyle.GetVerticalFrameSize()).Render(strings.Join(bodyParts, "\n"))
	footer := footerStyle.Width(innerW - footerStyle.GetHorizontalFrameSize()).Render(commandBarStyle.Render(commandBar))

	content := lipgloss.JoinVertical(lipgloss.Top, header, bodyPanel, footer)
	return appFrameStyle.Render(content)
}

func (m *Model) screenTitle() string {
	switch m.screen {
	case screenList:
		return "Desktop Entries"
	case screenEditor:
		if m.current != nil {
			return "Editing " + activePathForView(m.current.Path)
		}
		return "Editor"
	case screenIconPicker:
		return "Pick Icon"
	case screenInstallPath:
		return "Install Icon"
	case screenInstallBrowse:
		return "Browse Files: " + m.install.browserCWD
	case screenCommandPalette:
		return "Command Palette"
	default:
		return ""
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
	actions := m.footerActionsFor(m.screen)
	parts := make([]string, 0, len(actions))
	for i, a := range actions {
		h := a.binding.Help()
		if strings.TrimSpace(h.Key) == "" || strings.TrimSpace(h.Desc) == "" {
			continue
		}
		label := fmt.Sprintf("[%s %s]", h.Key, h.Desc)
		style := footerChipStyle
		if m.regionFocus == regionFooter && i == m.footerAction {
			style = footerChipFocusedStyle
		}
		parts = append(parts, style.Render(label))
	}
	return strings.Join(parts, "   ")
}

type footerAction struct {
	binding key.Binding
	id      string
}

func (m *Model) footerActionsFor(sc screen) []footerAction {
	global := []footerAction{
		{binding: m.keys.Global.CommandPalette, id: "global_palette"},
		{binding: m.keys.Global.NextRegion, id: "global_next_region"},
		{binding: m.keys.Global.PrevRegion, id: "global_prev_region"},
	}

	switch m.screen {
	case screenList:
		return append(global,
			footerAction{binding: m.keys.List.Edit, id: "list_edit"},
			footerAction{binding: m.keys.List.Filter, id: "list_filter"},
			footerAction{binding: m.keys.List.Quit, id: "list_quit"},
			footerAction{binding: m.keys.List.ToggleHelp, id: "help_toggle"},
		)
	case screenEditor:
		return append(global,
			footerAction{binding: m.keys.Editor.Save, id: "editor_save"},
			footerAction{binding: m.keys.Editor.IconPicker, id: "editor_icon_picker"},
			footerAction{binding: m.keys.Editor.InstallIcon, id: "editor_install_icon"},
			footerAction{binding: m.keys.Editor.Cancel, id: "editor_cancel"},
			footerAction{binding: m.keys.Editor.ToggleHelp, id: "help_toggle"},
		)
	case screenIconPicker:
		return append(global,
			footerAction{binding: m.keys.IconPicker.Accept, id: "picker_accept"},
			footerAction{binding: m.keys.IconPicker.Cancel, id: "picker_cancel"},
			footerAction{binding: m.keys.IconPicker.Filter, id: "picker_filter"},
			footerAction{binding: m.keys.IconPicker.ToggleHelp, id: "help_toggle"},
		)
	case screenInstallPath:
		return append(global,
			footerAction{binding: m.keys.InstallPath.Install, id: "install_submit"},
			footerAction{binding: m.keys.InstallPath.Browse, id: "install_browse"},
			footerAction{binding: m.keys.InstallPath.Cancel, id: "install_cancel"},
			footerAction{binding: m.keys.InstallPath.ToggleHelp, id: "help_toggle"},
		)
	case screenInstallBrowse:
		return append(global,
			footerAction{binding: m.keys.InstallBrowse.OpenSelect, id: "browse_open"},
			footerAction{binding: m.keys.InstallBrowse.Back, id: "browse_back"},
			footerAction{binding: m.keys.InstallBrowse.Filter, id: "browse_filter"},
			footerAction{binding: m.keys.InstallBrowse.ToggleHelp, id: "help_toggle"},
		)
	case screenCommandPalette:
		return []footerAction{
			{binding: m.keys.Palette.Accept, id: "palette_run"},
			{binding: m.keys.Palette.Cancel, id: "palette_cancel"},
			{binding: m.keys.Palette.Filter, id: "palette_filter"},
			{binding: m.keys.Palette.ToggleHelp, id: "help_toggle"},
		}
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
	m.regionFocus = regionBody
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
	case "global_palette":
		m.openCommandPalette(target)
		return m, nil
	case "global_next_region":
		m.focusNextRegion()
		return m, nil
	case "global_prev_region":
		m.focusPrevRegion()
		return m, nil
	case "palette_run":
		return m.updateCommandPalette(tea.KeyMsg{Type: tea.KeyEnter})
	case "palette_cancel":
		return m.updateCommandPalette(tea.KeyMsg{Type: tea.KeyEsc})
	case "palette_filter":
		return m.updateCommandPalette(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
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
	if m.regionFocus < regionHeader || m.regionFocus > regionFooter {
		m.regionFocus = regionBody
	}
}

func (m *Model) layoutViewport() (int, int) {
	if m.width > 0 && m.height > 0 {
		return m.width, m.height
	}
	return 100, 30
}

func (m *Model) bodyPanelSize(totalW, totalH int) (int, int) {
	innerW := maxInt(1, totalW-appFrameStyle.GetHorizontalFrameSize())
	innerH := maxInt(1, totalH-appFrameStyle.GetVerticalFrameSize())
	headFrameH := 1 + headerFrameStyle.GetVerticalFrameSize()
	if strings.TrimSpace(m.screenTitle()) != "" {
		headFrameH++
	}
	footFrameH := 1 + footerFrameStyle.GetVerticalFrameSize()
	bodyContentH := maxInt(3, innerH-headFrameH-footFrameH)
	bodyW := maxInt(5, innerW-bodyFrameStyle.GetHorizontalFrameSize())
	bodyH := maxInt(3, bodyContentH-bodyFrameStyle.GetVerticalFrameSize())
	return bodyW, bodyH
}

func (m *Model) applyLayoutSizing(totalW, totalH int) {
	bodyW, bodyH := m.bodyPanelSize(totalW, totalH)
	m.list.SetSize(bodyW, bodyH)
	if m.iconPicker.Items() != nil {
		m.iconPicker.SetSize(bodyW, bodyH)
	}
	if m.install.browser.Items() != nil {
		m.install.browser.SetSize(bodyW, bodyH)
	}
	if m.commandPalette.Items() != nil {
		m.commandPalette.SetSize(bodyW, bodyH)
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (m *Model) updateFooterRegion(msg tea.KeyMsg) (bool, tea.Model, tea.Cmd) {
	actions := m.footerActionsFor(m.screen)
	if len(actions) == 0 {
		return false, m, nil
	}
	if m.footerAction < 0 || m.footerAction >= len(actions) {
		m.footerAction = 0
	}

	switch msg.String() {
	case "tab":
		m.footerAction = (m.footerAction + 1) % len(actions)
		return true, m, nil
	case "shift+tab":
		m.footerAction = (m.footerAction - 1 + len(actions)) % len(actions)
		return true, m, nil
	case "enter":
		selected := actions[m.footerAction]
		m.regionFocus = regionBody
		model, cmd := m.executeCommand(selected.id, m.screen)
		return true, model, cmd
	default:
		return false, m, nil
	}
}

func focusedFrameStyle(base lipgloss.Style, focused bool) lipgloss.Style {
	if !focused {
		return base
	}
	return base.BorderForeground(lipgloss.Color("205"))
}

func (m *Model) focusNextRegion() {
	m.regionFocus = (m.regionFocus + 1) % 3
	if m.regionFocus == regionFooter {
		m.footerAction = 0
	}
}

func (m *Model) focusPrevRegion() {
	m.regionFocus = (m.regionFocus - 1 + 3) % 3
	if m.regionFocus == regionFooter {
		m.footerAction = 0
	}
}

func isCtrlTab(msg tea.KeyMsg) bool {
	return msg.String() == "ctrl+tab"
}

func isCtrlShiftTab(msg tea.KeyMsg) bool {
	s := msg.String()
	return s == "ctrl+shift+tab" || s == "shift+ctrl+tab"
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

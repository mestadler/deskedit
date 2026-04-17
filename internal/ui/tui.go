// Package ui implements the deskedit TUI using Bubble Tea.
//
// The app has three screens:
//
//   - List:    browse discovered .desktop entries, filter by substring.
//   - Editor:  form with Name / Exec / Icon / GPU-mode fields.
//   - Picker:  full-screen filter list used for picking an icon.
//
// Edits to system files are written to the user override location; edits to
// user files are written in place. Saves are atomic (tmp + rename).
package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/mestadler/deskedit/internal/desktop"
	"github.com/mestadler/deskedit/internal/gpu"
)

// ---- styles ----------------------------------------------------------------

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

// ---- screens ---------------------------------------------------------------

type screen int

const (
	screenList screen = iota
	screenEditor
	screenIconPicker
	screenInstallPath   // prompt for source file path + icon name
	screenInstallBrowse // filesystem browser
)

// ---- list item wrapper -----------------------------------------------------

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

// simple string wrapper for the icon picker list
type iconItem string

func (i iconItem) Title() string       { return string(i) }
func (i iconItem) Description() string { return "" }
func (i iconItem) FilterValue() string { return string(i) }

// ---- editor fields ---------------------------------------------------------

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

// ---- model -----------------------------------------------------------------

type Model struct {
	screen screen

	// list screen
	entries []desktop.Entry
	list    list.Model

	// editor screen
	current    *desktop.File
	currentEnt desktop.Entry
	inputs     []textinput.Model
	terminal   bool
	noDisplay  bool
	hidden     bool
	gpuMode    gpu.Mode
	gpuCaps    gpu.Capabilities
	focused    field

	// icon picker
	iconPicker list.Model

	// icon install
	installPathInput textinput.Model
	installNameInput textinput.Model
	installFocus     int // 0 = path, 1 = name
	browser          list.Model
	browserCWD       string
	lastBrowseDir    string

	// status
	status string
	err    error

	width, height int
}

// New constructs the initial model with the entry list populated.
func New() (*Model, error) {
	entries, err := desktop.Discover()
	if err != nil {
		return nil, err
	}

	items := make([]list.Item, len(entries))
	for i, e := range entries {
		items[i] = listItem{e}
	}

	delegate := list.NewDefaultDelegate()
	delegate.SetSpacing(0)
	delegate.ShowDescription = false

	l := list.New(items, delegate, 0, 0)
	l.Title = "Desktop Entries"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)

	m := &Model{
		screen:  screenList,
		entries: entries,
		list:    l,
		gpuCaps: gpu.Detect(),
	}
	return m, nil
}

// ---- Init ------------------------------------------------------------------

func (m *Model) Init() tea.Cmd { return nil }

// ---- Update ----------------------------------------------------------------

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case installDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		// Success: set the Icon field, return to editor, show a status.
		m.inputs[fieldIcon].SetValue(m.installNameInput.Value())
		m.screen = screenEditor
		note := fmt.Sprintf("installed: %s", msg.res.InstalledPath)
		if msg.res.Resized {
			note += fmt.Sprintf(" (resized from %dx%d)",
				msg.res.OriginalW, msg.res.OriginalH)
		}
		if msg.res.CacheUpdateErr != nil {
			note += "  [cache update failed, icon still usable]"
		}
		m.status = note
		m.err = nil
		return m, nil

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		// Leave some room for our own chrome.
		m.list.SetSize(msg.Width-2, msg.Height-4)
		if m.iconPicker.Items() != nil {
			m.iconPicker.SetSize(msg.Width-2, msg.Height-4)
		}
		if m.browser.Items() != nil {
			m.browser.SetSize(msg.Width-2, msg.Height-6)
		}
		return m, nil

	case tea.KeyMsg:
		// Global shortcuts
		switch msg.String() {
		case "ctrl+c":
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

	// Propagate to the active sub-model
	var cmd tea.Cmd
	switch m.screen {
	case screenList:
		m.list, cmd = m.list.Update(msg)
	case screenIconPicker:
		m.iconPicker, cmd = m.iconPicker.Update(msg)
	case screenInstallBrowse:
		m.browser, cmd = m.browser.Update(msg)
	}
	return m, cmd
}

// ---- list screen -----------------------------------------------------------

func (m *Model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Let the list handle filtering normally; only intercept enter/q.
	if m.list.FilterState() == list.Filtering {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}
	switch msg.String() {
	case "q", "esc":
		return m, tea.Quit
	case "enter":
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

// ---- editor screen ---------------------------------------------------------

func (m *Model) openEditor(e desktop.Entry) error {
	f, err := desktop.Load(e.Path)
	if err != nil {
		return fmt.Errorf("loading %s: %w", e.Path, err)
	}
	m.current = f
	m.currentEnt = e
	m.screen = screenEditor
	m.focused = fieldName
	m.status = ""
	m.err = nil

	m.inputs = make([]textinput.Model, 3) // Name, Exec, Icon
	for i := range m.inputs {
		ti := textinput.New()
		ti.CharLimit = 1024
		ti.Prompt = ""
		m.inputs[i] = ti
	}
	if v, ok := f.Get("Name"); ok {
		m.inputs[fieldName].SetValue(v)
	}
	if v, ok := f.Get("Exec"); ok {
		m.inputs[fieldExec].SetValue(v)
		m.gpuMode = gpu.DetectMode(v)
	} else {
		m.gpuMode = gpu.ModeNone
	}
	if v, ok := f.Get("Icon"); ok {
		m.inputs[fieldIcon].SetValue(v)
	}
	if v, ok := f.Get("Terminal"); ok {
		m.terminal = parseBoolDesktop(v)
	} else {
		m.terminal = false
	}
	if v, ok := f.Get("NoDisplay"); ok {
		m.noDisplay = parseBoolDesktop(v)
	} else {
		m.noDisplay = false
	}
	if v, ok := f.Get("Hidden"); ok {
		m.hidden = parseBoolDesktop(v)
	} else {
		m.hidden = false
	}
	m.inputs[fieldName].Focus()
	return nil
}

func (m *Model) updateEditor(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenList
		m.current = nil
		m.status = "discarded changes"
		return m, nil
	case "tab", "down":
		m.focusNext()
		return m, nil
	case "shift+tab", "up":
		m.focusPrev()
		return m, nil
	case "ctrl+s":
		if err := m.save(); err != nil {
			m.err = err
			return m, nil
		}
		m.screen = screenList
		return m, m.refreshEntries()
	case "ctrl+i":
		// Jump to icon picker regardless of focused field
		m.openIconPicker()
		return m, nil
	case "ctrl+n":
		// Browse for an icon file first, then confirm/edit in the form.
		m.openInstallPath()
		m.openInstallBrowse(m.installBrowseStartDir())
		return m, nil
	}

	// Field-specific keys
	switch m.focused {
	case fieldTerminal:
		switch msg.String() {
		case "left", "h", "right", "l", " ", "enter":
			m.terminal = !m.terminal
			return m, nil
		}
	case fieldNoDisplay:
		switch msg.String() {
		case "left", "h", "right", "l", " ", "enter":
			m.noDisplay = !m.noDisplay
			return m, nil
		}
	case fieldHidden:
		switch msg.String() {
		case "left", "h", "right", "l", " ", "enter":
			m.hidden = !m.hidden
			return m, nil
		}
	case fieldGPU:
		switch msg.String() {
		case "left", "h":
			m.gpuMode = prevMode(m.gpuMode, m.gpuCaps)
			return m, nil
		case "right", "l", " ":
			m.gpuMode = nextMode(m.gpuMode, m.gpuCaps)
			return m, nil
		}
	case fieldIcon:
		if msg.String() == "enter" {
			m.openIconPicker()
			return m, nil
		}
	}

	// Forward to focused textinput (if any)
	if int(m.focused) < len(m.inputs) {
		var cmd tea.Cmd
		m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *Model) focusNext() {
	if int(m.focused) < len(m.inputs) {
		m.inputs[m.focused].Blur()
	}
	m.focused = (m.focused + 1) % fieldCount
	if int(m.focused) < len(m.inputs) {
		m.inputs[m.focused].Focus()
	}
}

func (m *Model) focusPrev() {
	if int(m.focused) < len(m.inputs) {
		m.inputs[m.focused].Blur()
	}
	m.focused = (m.focused - 1 + fieldCount) % fieldCount
	if int(m.focused) < len(m.inputs) {
		m.inputs[m.focused].Focus()
	}
}

// save applies the form state to the in-memory file and writes it to disk.
// System files are written to the user override location.
func (m *Model) save() error {
	if m.current == nil {
		return fmt.Errorf("nothing to save")
	}

	newName := strings.TrimSpace(m.inputs[fieldName].Value())
	newExec := strings.TrimSpace(m.inputs[fieldExec].Value())
	newIcon := strings.TrimSpace(m.inputs[fieldIcon].Value())

	// Apply GPU mode by rewriting Exec (idempotent).
	wrapped := gpu.Wrap(newExec, m.gpuMode)

	if newName != "" {
		m.current.Set("Name", newName)
	}
	m.current.Set("Exec", wrapped)
	if newIcon != "" {
		m.current.Set("Icon", newIcon)
	}
	m.current.Set("Terminal", formatBoolDesktop(m.terminal))
	m.current.Set("NoDisplay", formatBoolDesktop(m.noDisplay))
	m.current.Set("Hidden", formatBoolDesktop(m.hidden))

	// Where to write?
	target := m.current.Path
	if m.currentEnt.Source == desktop.SourceSystem {
		p, err := desktop.UserPathFor(m.currentEnt.ID)
		if err != nil {
			return err
		}
		target = p
	}
	if err := m.current.Save(target); err != nil {
		return err
	}
	m.status = fmt.Sprintf("saved: %s", target)
	return nil
}

// refreshEntries re-discovers entries after a save so user overrides appear.
func (m *Model) refreshEntries() tea.Cmd {
	return func() tea.Msg {
		entries, err := desktop.Discover()
		if err != nil {
			return err
		}
		return entriesRefreshedMsg(entries)
	}
}

type entriesRefreshedMsg []desktop.Entry

// ---- icon picker -----------------------------------------------------------

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
	l.Title = "Pick an icon (type / to filter, enter to accept, esc to cancel)"
	l.SetFilteringEnabled(true)
	m.iconPicker = l
	m.screen = screenIconPicker
}

func (m *Model) updateIconPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.iconPicker.FilterState() == list.Filtering {
		var cmd tea.Cmd
		m.iconPicker, cmd = m.iconPicker.Update(msg)
		return m, cmd
	}
	switch msg.String() {
	case "esc":
		m.screen = screenEditor
		return m, nil
	case "enter":
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

// ---- View ------------------------------------------------------------------

func (m *Model) View() string {
	switch m.screen {
	case screenList:
		hint := hintStyle.Render("enter: edit   /: filter   q: quit")
		return m.list.View() + "\n" + hint
	case screenEditor:
		return m.viewEditor()
	case screenIconPicker:
		hint := hintStyle.Render("enter: accept   esc: cancel   /: filter")
		return m.iconPicker.View() + "\n" + hint
	case screenInstallPath:
		return m.viewInstallPath()
	case screenInstallBrowse:
		hint := hintStyle.Render("enter: open/select   esc: back   /: filter")
		title := titleStyle.Render("Browse: " + m.browserCWD)
		return title + "\n" + m.browser.View() + "\n" + hint
	}
	return ""
}

func (m *Model) viewEditor() string {
	if m.current == nil {
		return "no file loaded"
	}

	title := titleStyle.Render(fmt.Sprintf("Editing: %s", filepath.Base(m.current.Path)))
	src := badgeUser
	if m.currentEnt.Source == desktop.SourceSystem {
		src = badgeSystem
	}
	pathLine := hintStyle.Render(fmt.Sprintf("%s  %s", src, m.current.Path))
	if m.currentEnt.Source == desktop.SourceSystem {
		if p, err := desktop.UserPathFor(m.currentEnt.ID); err == nil {
			pathLine += "\n" + hintStyle.Render("will save to: "+p)
		}
	}

	rows := make([]string, 0, int(fieldCount))
	for f := field(0); f < fieldCount; f++ {
		label := labelStyle.Render(fieldLabels[f] + ":")
		var val string
		if int(f) < len(m.inputs) {
			val = m.inputs[f].View()
		} else if f == fieldTerminal {
			val = renderBoolField(m.terminal)
		} else if f == fieldNoDisplay {
			val = renderBoolField(m.noDisplay)
		} else if f == fieldHidden {
			val = renderBoolField(m.hidden)
		} else if f == fieldGPU {
			val = m.renderGPUField()
		}
		row := label + " " + val
		if m.focused == f {
			row = activeBorder.Render(row)
		} else {
			row = inactiveBorder.Render(row)
		}
		rows = append(rows, row)
	}

	hint := hintStyle.Render(
		"tab: next field   space/enter: toggle bool   ctrl+s: save   ctrl+i: icon picker   ctrl+n: browse/install icon   esc: cancel",
	)

	var status string
	if m.err != nil {
		status = errorStyle.Render("error: " + m.err.Error())
	} else if m.status != "" {
		status = statusStyle.Render(m.status)
	}

	return strings.Join([]string{
		title, pathLine, strings.Join(rows, "\n"), hint, status,
	}, "\n")
}

func (m *Model) renderGPUField() string {
	modes := availableModes(m.gpuCaps)
	if len(modes) == 0 {
		return hintStyle.Render("(no offload mechanisms detected)")
	}
	parts := make([]string, 0, len(modes))
	for _, md := range modes {
		label := modeLabel(md)
		if md == m.gpuMode {
			parts = append(parts, lipgloss.NewStyle().
				Foreground(lipgloss.Color("205")).Bold(true).
				Render("◉ "+label))
		} else {
			parts = append(parts, "○ "+label)
		}
	}
	return strings.Join(parts, "   ") + "  " + hintStyle.Render("(←/→ to change)")
}

func renderBoolField(v bool) string {
	if v {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true).Render("yes") +
			" " + hintStyle.Render("(←/→/space/enter to toggle)")
	}
	return "no " + hintStyle.Render("(←/→/space/enter to toggle)")
}

// ---- mode helpers ----------------------------------------------------------

func availableModes(c gpu.Capabilities) []gpu.Mode {
	out := []gpu.Mode{gpu.ModeNone}
	if c.Switcheroo {
		out = append(out, gpu.ModeSwitcheroo)
	}
	if c.NVIDIA {
		out = append(out, gpu.ModeNVIDIA)
	}
	if c.DRIPrime {
		out = append(out, gpu.ModeDRIPrime)
	}
	return out
}

func modeLabel(m gpu.Mode) string {
	switch m {
	case gpu.ModeNone:
		return "default (iGPU)"
	case gpu.ModeSwitcheroo:
		return "switcheroo"
	case gpu.ModeNVIDIA:
		return "NVIDIA PRIME"
	case gpu.ModeDRIPrime:
		return "DRI_PRIME (Mesa)"
	}
	return string(m)
}

func nextMode(current gpu.Mode, c gpu.Capabilities) gpu.Mode {
	modes := availableModes(c)
	for i, m := range modes {
		if m == current {
			return modes[(i+1)%len(modes)]
		}
	}
	return modes[0]
}

func prevMode(current gpu.Mode, c gpu.Capabilities) gpu.Mode {
	modes := availableModes(c)
	for i, m := range modes {
		if m == current {
			return modes[(i-1+len(modes))%len(modes)]
		}
	}
	return modes[0]
}

// ---- icon install flow -----------------------------------------------------

func (m *Model) openInstallPath() {
	pathTI := textinput.New()
	pathTI.Placeholder = "/path/to/icon.png  (or press ctrl+b to browse)"
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
	switch msg.String() {
	case "esc":
		m.screen = screenEditor
		return m, nil
	case "ctrl+b":
		m.openInstallBrowse(m.installBrowseStartDir())
		return m, nil
	case "tab", "down":
		m.installCycleFocus(+1)
		return m, nil
	case "shift+tab", "up":
		m.installCycleFocus(-1)
		return m, nil
	case "enter":
		// If path has content but name is empty, auto-fill name from basename
		// of the path and move focus to name field for confirmation.
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

	// Forward to focused input
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

	pathLabel := labelStyle.Render("Source:")
	nameLabel := labelStyle.Render("Name:")

	pathRow := pathLabel + " " + m.installPathInput.View()
	nameRow := nameLabel + " " + m.installNameInput.View()

	if m.installFocus == 0 {
		pathRow = activeBorder.Render(pathRow)
		nameRow = inactiveBorder.Render(nameRow)
	} else {
		pathRow = inactiveBorder.Render(pathRow)
		nameRow = activeBorder.Render(nameRow)
	}

	hint := hintStyle.Render(
		"tab: next   enter: install (or browse if source empty)   ctrl+b: browse files   esc: cancel",
	)
	info := hintStyle.Render(
		"PNGs >256px are resized; smaller PNGs keep their size. SVGs go to scalable/.",
	)

	var status string
	if m.err != nil {
		status = errorStyle.Render("error: " + m.err.Error())
	} else if m.status != "" {
		status = statusStyle.Render(m.status)
	}

	return strings.Join([]string{title, pathRow, nameRow, info, hint, status}, "\n")
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
}

func (m *Model) updateInstallBrowse(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.browser.FilterState() == list.Filtering {
		var cmd tea.Cmd
		m.browser, cmd = m.browser.Update(msg)
		return m, cmd
	}
	switch msg.String() {
	case "esc":
		m.screen = screenInstallPath
		return m, nil
	case "enter":
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
		// File selected — populate the path input and return
		m.installPathInput.SetValue(it.path)
		m.lastBrowseDir = filepath.Dir(it.path)
		// Default the name from the basename if still empty
		if strings.TrimSpace(m.installNameInput.Value()) == "" {
			base := filepath.Base(it.path)
			base = strings.TrimSuffix(base, filepath.Ext(base))
			m.installNameInput.SetValue(base)
		}
		m.screen = screenInstallPath
		// Focus name so the user can confirm or edit
		m.installFocus = 0
		m.installCycleFocus(+1)
		return m, nil
	}
	var cmd tea.Cmd
	m.browser, cmd = m.browser.Update(msg)
	return m, cmd
}

// doInstall runs the install as a tea.Cmd so the UI stays responsive.
type installDoneMsg struct {
	res *desktop.IconInstallResult
	err error
}

func (m *Model) doInstall(path, name string) tea.Cmd {
	return func() tea.Msg {
		res, err := desktop.InstallIcon(desktop.IconInstallRequest{
			SourcePath: path,
			Name:       name,
		})
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

func parseBoolDesktop(v string) bool {
	return strings.EqualFold(strings.TrimSpace(v), "true")
}

func formatBoolDesktop(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

// Run is the entry point called from main.
func Run() error {
	m, err := New()
	if err != nil {
		return err
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

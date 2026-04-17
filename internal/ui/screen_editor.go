package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/mestadler/deskedit/internal/desktop"
	"github.com/mestadler/deskedit/internal/gpu"
)

func (m *Model) openEditor(e desktop.Entry) error {
	m.ensureDefaults()

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

	m.inputs = make([]textinput.Model, 3)
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
	m.terminal = false
	m.noDisplay = false
	m.hidden = false
	if v, ok := f.Get("Terminal"); ok {
		m.terminal = parseBoolDesktop(v)
	}
	if v, ok := f.Get("NoDisplay"); ok {
		m.noDisplay = parseBoolDesktop(v)
	}
	if v, ok := f.Get("Hidden"); ok {
		m.hidden = parseBoolDesktop(v)
	}
	m.inputs[fieldName].Focus()
	return nil
}

func (m *Model) updateEditor(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if keyMatches(msg, m.keys.Editor.ToggleHelp) {
		m.help.ShowAll = !m.help.ShowAll
		return m, nil
	}

	switch {
	case keyMatches(msg, m.keys.Editor.Cancel):
		m.screen = screenList
		m.current = nil
		m.status = "discarded changes"
		return m, nil
	case keyMatches(msg, m.keys.Editor.NextField):
		m.focusNext()
		return m, nil
	case keyMatches(msg, m.keys.Editor.PrevField):
		m.focusPrev()
		return m, nil
	case keyMatches(msg, m.keys.Editor.Save):
		if err := m.save(); err != nil {
			m.err = err
			return m, nil
		}
		m.screen = screenList
		return m, m.refreshEntries()
	case keyMatches(msg, m.keys.Editor.IconPicker):
		m.openIconPicker()
		return m, nil
	case keyMatches(msg, m.keys.Editor.InstallIcon):
		m.openInstallPath()
		m.openInstallBrowse(m.install.browseStartDir())
		return m, nil
	}

	switch m.focused {
	case fieldTerminal:
		if keyMatches(msg, m.keys.Editor.ToggleBool) || msg.Type == tea.KeySpace {
			m.terminal = !m.terminal
			return m, nil
		}
	case fieldNoDisplay:
		if keyMatches(msg, m.keys.Editor.ToggleBool) || msg.Type == tea.KeySpace {
			m.noDisplay = !m.noDisplay
			return m, nil
		}
	case fieldHidden:
		if keyMatches(msg, m.keys.Editor.ToggleBool) || msg.Type == tea.KeySpace {
			m.hidden = !m.hidden
			return m, nil
		}
	case fieldGPU:
		switch {
		case keyMatches(msg, m.keys.Editor.ToggleBool) || msg.Type == tea.KeySpace:
			m.gpuMode = nextMode(m.gpuMode, m.gpuCaps)
			return m, nil
		case msg.String() == "left" || msg.String() == "h":
			m.gpuMode = prevMode(m.gpuMode, m.gpuCaps)
			return m, nil
		}
	case fieldIcon:
		if keyMatches(msg, m.keys.Editor.OpenIconEdit) {
			m.openIconPicker()
			return m, nil
		}
	}

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

func (m *Model) save() error {
	if m.current == nil {
		return fmt.Errorf("nothing to save")
	}

	newName := strings.TrimSpace(m.inputs[fieldName].Value())
	newExec := strings.TrimSpace(m.inputs[fieldExec].Value())
	newIcon := strings.TrimSpace(m.inputs[fieldIcon].Value())

	if newName != "" {
		m.current.Set("Name", newName)
	}
	m.current.Set("Exec", gpu.Wrap(newExec, m.gpuMode))
	if newIcon != "" {
		m.current.Set("Icon", newIcon)
	}
	m.current.Set("Terminal", formatBoolDesktop(m.terminal))
	m.current.Set("NoDisplay", formatBoolDesktop(m.noDisplay))
	m.current.Set("Hidden", formatBoolDesktop(m.hidden))

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

func (m *Model) refreshEntries() tea.Cmd {
	return func() tea.Msg {
		entries, err := desktop.Discover()
		return entriesRefreshedMsg{entries: entries, err: err}
	}
}

func (m *Model) viewEditor() string {
	if m.current == nil {
		return "no file loaded"
	}

	title := titleStyle.Render(fmt.Sprintf("Editing: %s", activePathForView(m.current.Path)))
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
		switch {
		case int(f) < len(m.inputs):
			val = m.inputs[f].View()
		case f == fieldTerminal:
			val = renderBoolField(m.terminal)
		case f == fieldNoDisplay:
			val = renderBoolField(m.noDisplay)
		case f == fieldHidden:
			val = renderBoolField(m.hidden)
		case f == fieldGPU:
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

	status := ""
	if m.err != nil {
		status = errorStyle.Render("error: " + m.err.Error())
	} else if m.status != "" {
		status = statusStyle.Render(m.status)
	}

	return strings.Join([]string{
		title,
		pathLine,
		strings.Join(rows, "\n"),
		m.help.View(m.keys.Editor),
		status,
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
				Foreground(lipgloss.Color("205")).
				Bold(true).
				Render("◉ "+label))
		} else {
			parts = append(parts, "○ "+label)
		}
	}
	return strings.Join(parts, "   ")
}

func renderBoolField(v bool) string {
	if v {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true).Render("yes")
	}
	return "no"
}

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

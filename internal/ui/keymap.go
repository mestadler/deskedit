package ui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

type globalKeyMap struct {
	Quit           key.Binding
	CommandPalette key.Binding
	NextRegion     key.Binding
	PrevRegion     key.Binding
}

type listKeyMap struct {
	Edit       key.Binding
	Filter     key.Binding
	Quit       key.Binding
	ToggleHelp key.Binding
}

func (k listKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Edit, k.Filter, k.Quit, k.ToggleHelp}
}

func (k listKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Edit, k.Filter, k.Quit, k.ToggleHelp}}
}

type editorKeyMap struct {
	NextField    key.Binding
	PrevField    key.Binding
	Save         key.Binding
	Cancel       key.Binding
	IconPicker   key.Binding
	InstallIcon  key.Binding
	ToggleBool   key.Binding
	CycleGPU     key.Binding
	OpenIconEdit key.Binding
	ToggleHelp   key.Binding
}

func (k editorKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Save, k.IconPicker, k.InstallIcon, k.Cancel, k.ToggleHelp}
}

func (k editorKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.NextField, k.PrevField, k.Save, k.Cancel},
		{k.IconPicker, k.InstallIcon, k.OpenIconEdit},
		{k.ToggleBool, k.CycleGPU, k.ToggleHelp},
	}
}

type iconPickerKeyMap struct {
	Accept     key.Binding
	Cancel     key.Binding
	Filter     key.Binding
	ToggleHelp key.Binding
}

func (k iconPickerKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Accept, k.Cancel, k.Filter, k.ToggleHelp}
}

func (k iconPickerKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Accept, k.Cancel, k.Filter, k.ToggleHelp}}
}

type installPathKeyMap struct {
	NextField  key.Binding
	PrevField  key.Binding
	Browse     key.Binding
	Install    key.Binding
	Cancel     key.Binding
	ToggleHelp key.Binding
}

func (k installPathKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Install, k.Browse, k.Cancel, k.ToggleHelp}
}

func (k installPathKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.NextField, k.PrevField, k.Install, k.Browse, k.Cancel, k.ToggleHelp}}
}

type installBrowseKeyMap struct {
	OpenSelect key.Binding
	Back       key.Binding
	Filter     key.Binding
	ToggleHelp key.Binding
}

func (k installBrowseKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.OpenSelect, k.Back, k.Filter, k.ToggleHelp}
}

func (k installBrowseKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.OpenSelect, k.Back, k.Filter, k.ToggleHelp}}
}

type keyMaps struct {
	Global        globalKeyMap
	List          listKeyMap
	Editor        editorKeyMap
	IconPicker    iconPickerKeyMap
	InstallPath   installPathKeyMap
	InstallBrowse installBrowseKeyMap
	Palette       iconPickerKeyMap
}

func defaultKeyMaps() keyMaps {
	toggleHelp := key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "toggle help"))
	return keyMaps{
		Global: globalKeyMap{
			Quit:           key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "quit")),
			CommandPalette: key.NewBinding(key.WithKeys("ctrl+k"), key.WithHelp("ctrl+k", "commands")),
			NextRegion:     key.NewBinding(key.WithKeys("ctrl+tab"), key.WithHelp("ctrl+tab", "next region")),
			PrevRegion:     key.NewBinding(key.WithKeys("ctrl+shift+tab"), key.WithHelp("ctrl+shift+tab", "prev region")),
		},
		List: listKeyMap{
			Edit:       key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "edit")),
			Filter:     key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
			Quit:       key.NewBinding(key.WithKeys("q", "esc"), key.WithHelp("q/esc", "quit")),
			ToggleHelp: toggleHelp,
		},
		Editor: editorKeyMap{
			NextField:    key.NewBinding(key.WithKeys("tab", "down"), key.WithHelp("tab/down", "next field")),
			PrevField:    key.NewBinding(key.WithKeys("shift+tab", "up"), key.WithHelp("shift+tab/up", "prev field")),
			Save:         key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "save")),
			Cancel:       key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
			IconPicker:   key.NewBinding(key.WithKeys("ctrl+i"), key.WithHelp("ctrl+i", "icon picker")),
			InstallIcon:  key.NewBinding(key.WithKeys("ctrl+n"), key.WithHelp("ctrl+n", "browse/install icon")),
			ToggleBool:   key.NewBinding(key.WithKeys("left", "right", "h", "l", "space", "enter"), key.WithHelp("←/→/space", "toggle bool")),
			CycleGPU:     key.NewBinding(key.WithKeys("left", "right", "h", "l", "space"), key.WithHelp("←/→/space", "cycle GPU mode")),
			OpenIconEdit: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open icon picker")),
			ToggleHelp:   toggleHelp,
		},
		IconPicker: iconPickerKeyMap{
			Accept:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "accept")),
			Cancel:     key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
			Filter:     key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
			ToggleHelp: toggleHelp,
		},
		InstallPath: installPathKeyMap{
			NextField:  key.NewBinding(key.WithKeys("tab", "down"), key.WithHelp("tab/down", "next")),
			PrevField:  key.NewBinding(key.WithKeys("shift+tab", "up"), key.WithHelp("shift+tab/up", "prev")),
			Browse:     key.NewBinding(key.WithKeys("ctrl+b"), key.WithHelp("ctrl+b", "browse files")),
			Install:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "install/browse")),
			Cancel:     key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
			ToggleHelp: toggleHelp,
		},
		InstallBrowse: installBrowseKeyMap{
			OpenSelect: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open/select")),
			Back:       key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
			Filter:     key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
			ToggleHelp: toggleHelp,
		},
		Palette: iconPickerKeyMap{
			Accept:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "run")),
			Cancel:     key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
			Filter:     key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
			ToggleHelp: toggleHelp,
		},
	}
}

func keyMatches(msg tea.KeyMsg, bindings ...key.Binding) bool {
	for _, b := range bindings {
		if key.Matches(msg, b) {
			return true
		}
	}
	return false
}

func (m *Model) toggleHelpIfMatched(msg tea.KeyMsg, binding key.Binding) bool {
	if !key.Matches(msg, binding) {
		return false
	}
	m.help.ShowAll = !m.help.ShowAll
	return true
}

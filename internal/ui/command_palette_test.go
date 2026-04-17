package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/mestadler/deskedit/internal/desktop"
)

func TestCommandPalette_OpenAndCancelReturnsToPreviousScreen(t *testing.T) {
	path := t.TempDir() + "/applications/app.desktop"
	writeDesktopEntryFile(t, path, "[Desktop Entry]\nName=App\nExec=app\n")

	m := newModelForUXTests()
	if err := m.openEditor(desktop.Entry{Path: path, ID: "app.desktop", Source: desktop.SourceUser}); err != nil {
		t.Fatalf("openEditor: %v", err)
	}

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	if m.screen != screenCommandPalette {
		t.Fatalf("screen = %v, want command palette", m.screen)
	}

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.screen != screenEditor {
		t.Fatalf("screen = %v, want editor", m.screen)
	}
}

func TestCommandPalette_FilteringWorks(t *testing.T) {
	path := t.TempDir() + "/applications/app.desktop"
	writeDesktopEntryFile(t, path, "[Desktop Entry]\nName=App\nExec=app\n")

	m := newModelForUXTests()
	if err := m.openEditor(desktop.Entry{Path: path, ID: "app.desktop", Source: desktop.SourceUser}); err != nil {
		t.Fatalf("openEditor: %v", err)
	}

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})

	if m.commandPalette.FilterState() != list.Filtering {
		t.Fatalf("palette filter state = %v, want filtering", m.commandPalette.FilterState())
	}
}

func TestCommandPalette_ExecuteSaveAction(t *testing.T) {
	path := t.TempDir() + "/applications/app.desktop"
	writeDesktopEntryFile(t, path, "[Desktop Entry]\nName=App\nExec=app\n")

	m := newModelForUXTests()
	if err := m.openEditor(desktop.Entry{Path: path, ID: "app.desktop", Source: desktop.SourceUser}); err != nil {
		t.Fatalf("openEditor: %v", err)
	}
	m.inputs[fieldName].SetValue("Saved From Palette")

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	selectPaletteCommand(t, m, "editor_save")
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.confirmActive || m.confirmKind != confirmSave {
		t.Fatalf("expected save confirm modal after palette save action")
	}
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab}) // Yes
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.screen != screenList {
		t.Fatalf("screen = %v, want list", m.screen)
	}
	if !strings.Contains(m.status, "saved:") {
		t.Fatalf("status = %q, want save status", m.status)
	}

	f, err := desktop.Load(path)
	if err != nil {
		t.Fatalf("load saved file: %v", err)
	}
	if got, ok := f.Get("Name"); !ok || got != "Saved From Palette" {
		t.Fatalf("Name = %q, %v; want %q, true", got, ok, "Saved From Palette")
	}
}

func TestCommandPalette_ListEditActionOpensEditor(t *testing.T) {
	path := t.TempDir() + "/applications/app.desktop"
	writeDesktopEntryFile(t, path, "[Desktop Entry]\nName=List App\nExec=app\n")

	m := newModelForUXTests()
	entry := desktop.Entry{Path: path, ID: "app.desktop", Name: "List App", Source: desktop.SourceUser}
	m.setEntries([]desktop.Entry{entry})
	m.screen = screenList
	m.list.Select(0)

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	selectPaletteCommand(t, m, "list_edit")
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.screen != screenEditor {
		t.Fatalf("screen = %v, want editor", m.screen)
	}
	if m.current == nil || m.current.Path != path {
		t.Fatalf("current path = %v, want %q", m.current, path)
	}
}

func TestCommandPalette_SaveActionParityWithCtrlS(t *testing.T) {
	pathViaKey := t.TempDir() + "/applications/key.desktop"
	pathViaPalette := t.TempDir() + "/applications/palette.desktop"
	writeDesktopEntryFile(t, pathViaKey, "[Desktop Entry]\nName=App\nExec=app\n")
	writeDesktopEntryFile(t, pathViaPalette, "[Desktop Entry]\nName=App\nExec=app\n")

	byKey := newModelForUXTests()
	if err := byKey.openEditor(desktop.Entry{Path: pathViaKey, ID: "key.desktop", Source: desktop.SourceUser}); err != nil {
		t.Fatalf("openEditor key path: %v", err)
	}
	byKey.inputs[fieldName].SetValue("Parity Name")
	_, _ = byKey.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	_, _ = byKey.Update(tea.KeyMsg{Type: tea.KeyTab}) // Yes
	_, _ = byKey.Update(tea.KeyMsg{Type: tea.KeyEnter})

	byPalette := newModelForUXTests()
	if err := byPalette.openEditor(desktop.Entry{Path: pathViaPalette, ID: "palette.desktop", Source: desktop.SourceUser}); err != nil {
		t.Fatalf("openEditor palette path: %v", err)
	}
	byPalette.inputs[fieldName].SetValue("Parity Name")
	_, _ = byPalette.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	selectPaletteCommand(t, byPalette, "editor_save")
	_, _ = byPalette.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_, _ = byPalette.Update(tea.KeyMsg{Type: tea.KeyTab}) // Yes
	_, _ = byPalette.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if byKey.screen != byPalette.screen {
		t.Fatalf("screen mismatch key=%v palette=%v", byKey.screen, byPalette.screen)
	}

	fKey, err := desktop.Load(pathViaKey)
	if err != nil {
		t.Fatalf("load key file: %v", err)
	}
	fPalette, err := desktop.Load(pathViaPalette)
	if err != nil {
		t.Fatalf("load palette file: %v", err)
	}
	nameKey, _ := fKey.Get("Name")
	namePalette, _ := fPalette.Get("Name")
	if nameKey != namePalette {
		t.Fatalf("saved name mismatch key=%q palette=%q", nameKey, namePalette)
	}
}

func selectPaletteCommand(t *testing.T, m *Model, id string) {
	t.Helper()
	for i, it := range m.commandPalette.Items() {
		cmd, ok := it.(commandItem)
		if ok && cmd.id == id {
			m.commandPalette.Select(i)
			return
		}
	}
	t.Fatalf("palette command %q not found", id)
}

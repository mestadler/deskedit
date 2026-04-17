package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mestadler/deskedit/internal/desktop"
)

func TestConfirmModal_DefaultsToNoAndRendersYesNo(t *testing.T) {
	m := newModelForUXTests()
	m.startConfirm(confirmSave, "Save changes?")

	if !m.confirmActive {
		t.Fatalf("confirmActive = false, want true")
	}
	if m.confirmSelection != confirmNo {
		t.Fatalf("confirmSelection = %v, want confirmNo", m.confirmSelection)
	}

	v := m.View()
	if !strings.Contains(v, "Confirm Save") {
		t.Fatalf("view missing confirm title: %q", v)
	}
	if !strings.Contains(v, "[Yes]") || !strings.Contains(v, "[No]") {
		t.Fatalf("view missing yes/no options: %q", v)
	}
	if strings.Count(v, "[Yes]") != 1 || strings.Count(v, "[No]") != 1 {
		t.Fatalf("confirm options should be exactly one Yes and one No: %q", v)
	}

	if got := confirmTitleStyle.GetForeground(); got == nil || fmt.Sprintf("%v", got) != fmt.Sprintf("%v", lipgloss.Color("196")) {
		t.Fatalf("expected confirm title foreground to be red 196, got %v", got)
	}
}

func TestConfirmModal_KeyHandlingAndResolution(t *testing.T) {
	m := newModelForUXTests()
	m.startConfirm(confirmSave, "Save now?")

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.confirmSelection != confirmYes {
		t.Fatalf("after tab selection = %v, want confirmYes", m.confirmSelection)
	}

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.confirmActive {
		t.Fatalf("confirm should be closed after enter")
	}
	if m.confirmSelection != confirmYes {
		t.Fatalf("expected yes selection to be applied")
	}

	m.startConfirm(confirmSave, "Save now?")
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.confirmActive {
		t.Fatalf("confirm should be closed after esc")
	}
	if m.confirmSelection != confirmNo {
		t.Fatalf("esc should resolve with no selection")
	}
}

func TestExitConfirm_DefaultNoFromListQuit(t *testing.T) {
	m := newModelForUXTests()
	m.screen = screenList

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	if !m.confirmActive {
		t.Fatalf("confirmActive = false, want true")
	}
	if m.confirmKind != confirmExit {
		t.Fatalf("confirmKind = %v, want %v", m.confirmKind, confirmExit)
	}
	if m.confirmSelection != confirmNo {
		t.Fatalf("confirmSelection = %v, want confirmNo", m.confirmSelection)
	}
}

func TestExitConfirm_YesQuits(t *testing.T) {
	m := newModelForUXTests()
	m.screen = screenList

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab}) // select Yes
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.confirmActive {
		t.Fatalf("confirm should be closed after decision")
	}
	if cmd == nil {
		t.Fatalf("expected quit command, got nil")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg from confirm-yes exit")
	}
}

func TestExitConfirm_EscCancels(t *testing.T) {
	m := newModelForUXTests()
	m.screen = screenList

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if m.confirmActive {
		t.Fatalf("confirm should be closed after esc")
	}
	if cmd != nil {
		t.Fatalf("expected no quit command on cancel")
	}
}

func TestExitConfirm_GlobalQuitStartsConfirm(t *testing.T) {
	m := newModelForUXTests()

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	if !m.confirmActive {
		t.Fatalf("confirmActive = false, want true")
	}
	if m.confirmKind != confirmExit {
		t.Fatalf("confirmKind = %v, want %v", m.confirmKind, confirmExit)
	}
}

func TestSaveConfirm_DefaultNoFromEditorCtrlS_NoWrite(t *testing.T) {
	path := t.TempDir() + "/applications/app.desktop"
	writeDesktopEntryFile(t, path, "[Desktop Entry]\nName=App\nExec=app\n")

	m := newModelForUXTests()
	if err := m.openEditor(desktop.Entry{Path: path, ID: "app.desktop", Source: desktop.SourceUser}); err != nil {
		t.Fatalf("openEditor: %v", err)
	}
	m.inputs[fieldName].SetValue("Not Saved")

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if !m.confirmActive || m.confirmKind != confirmSave {
		t.Fatalf("expected save confirmation modal")
	}
	if m.confirmSelection != confirmNo {
		t.Fatalf("default confirm selection = %v, want No", m.confirmSelection)
	}

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // default No
	if m.screen != screenEditor {
		t.Fatalf("screen = %v, want editor after cancel", m.screen)
	}

	f, err := desktop.Load(path)
	if err != nil {
		t.Fatalf("load file: %v", err)
	}
	if got, _ := f.Get("Name"); got != "App" {
		t.Fatalf("Name changed unexpectedly on No: %q", got)
	}
}

func TestSaveConfirm_YesWritesAndReturnsToList(t *testing.T) {
	path := t.TempDir() + "/applications/app.desktop"
	writeDesktopEntryFile(t, path, "[Desktop Entry]\nName=App\nExec=app\n")

	m := newModelForUXTests()
	if err := m.openEditor(desktop.Entry{Path: path, ID: "app.desktop", Source: desktop.SourceUser}); err != nil {
		t.Fatalf("openEditor: %v", err)
	}
	m.inputs[fieldName].SetValue("Saved Name")

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
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
		t.Fatalf("load file: %v", err)
	}
	if got, _ := f.Get("Name"); got != "Saved Name" {
		t.Fatalf("Name = %q, want %q", got, "Saved Name")
	}
}

func TestSaveConfirm_TriggeredFromPaletteAndFooter(t *testing.T) {
	pathA := t.TempDir() + "/applications/a.desktop"
	pathB := t.TempDir() + "/applications/b.desktop"
	writeDesktopEntryFile(t, pathA, "[Desktop Entry]\nName=A\nExec=app\n")
	writeDesktopEntryFile(t, pathB, "[Desktop Entry]\nName=B\nExec=app\n")

	viaPalette := newModelForUXTests()
	if err := viaPalette.openEditor(desktop.Entry{Path: pathA, ID: "a.desktop", Source: desktop.SourceUser}); err != nil {
		t.Fatalf("openEditor palette: %v", err)
	}
	_, _ = viaPalette.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	selectPaletteCommand(t, viaPalette, "editor_save")
	_, _ = viaPalette.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !viaPalette.confirmActive || viaPalette.confirmKind != confirmSave {
		t.Fatalf("palette save should open save confirm")
	}

	viaFooter := newModelForUXTests()
	if err := viaFooter.openEditor(desktop.Entry{Path: pathB, ID: "b.desktop", Source: desktop.SourceUser}); err != nil {
		t.Fatalf("openEditor footer: %v", err)
	}
	viaFooter.regionFocus = regionFooter
	actions := viaFooter.footerActionsFor(screenEditor)
	idx := -1
	for i, a := range actions {
		if a.id == "editor_save" {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatalf("editor_save footer action not found")
	}
	viaFooter.footerAction = idx
	_, _ = viaFooter.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !viaFooter.confirmActive || viaFooter.confirmKind != confirmSave {
		t.Fatalf("footer save should open save confirm")
	}
}

func TestConfirmModal_BlocksUnderlyingInput(t *testing.T) {
	path := t.TempDir() + "/applications/app.desktop"
	writeDesktopEntryFile(t, path, "[Desktop Entry]\nName=App\nExec=app\n")

	m := newModelForUXTests()
	if err := m.openEditor(desktop.Entry{Path: path, ID: "app.desktop", Source: desktop.SourceUser}); err != nil {
		t.Fatalf("openEditor: %v", err)
	}

	if m.focused != fieldName {
		t.Fatalf("focused = %v, want fieldName", m.focused)
	}

	m.startConfirm(confirmSave, "Save changes?")
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.focused != fieldName {
		t.Fatalf("underlying editor focus changed while confirm active")
	}
	if m.confirmSelection != confirmYes {
		t.Fatalf("tab should act on confirm selection; got %v", m.confirmSelection)
	}
}

func TestConfirmModal_NarrowTerminalOverlayStaysUsable(t *testing.T) {
	m := newModelForUXTests()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 40, Height: 12})
	m.startConfirm(confirmExit, "Exit deskedit?")

	v := m.View()
	if !strings.Contains(v, "Confirm Exit") {
		t.Fatalf("missing confirm exit title on narrow terminal: %q", v)
	}
	if !strings.Contains(v, "[Yes]") || !strings.Contains(v, "[No]") {
		t.Fatalf("missing yes/no options on narrow terminal: %q", v)
	}

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.confirmActive {
		t.Fatalf("confirm should close on esc in narrow terminal")
	}
}

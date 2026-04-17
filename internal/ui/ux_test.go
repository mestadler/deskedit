package ui

import (
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/mestadler/deskedit/internal/desktop"
)

func newModelForUXTests() *Model {
	delegate := list.NewDefaultDelegate()
	delegate.SetSpacing(0)
	delegate.ShowDescription = false

	m := &Model{}
	m.ensureDefaults()
	m.list = list.New(nil, delegate, 80, 24)
	m.list.SetFilteringEnabled(true)
	return m
}

func TestUpdate_HandlesEntriesRefreshedMsg(t *testing.T) {
	m := newModelForUXTests()
	entries := []desktop.Entry{{ID: "app.desktop", Name: "App"}}

	_, _ = m.Update(entriesRefreshedMsg{entries: entries})

	if len(m.entries) != 1 {
		t.Fatalf("entries length = %d, want 1", len(m.entries))
	}
	if got := len(m.list.Items()); got != 1 {
		t.Fatalf("list items = %d, want 1", got)
	}
}

func TestUpdate_HandlesEntriesRefreshedMsgError(t *testing.T) {
	m := newModelForUXTests()
	want := errors.New("refresh failed")

	_, _ = m.Update(entriesRefreshedMsg{err: want})

	if !errors.Is(m.err, want) {
		t.Fatalf("err = %v, want %v", m.err, want)
	}
}

func TestEditorView_HelpMatchesKeyBindings(t *testing.T) {
	path := t.TempDir() + "/applications/app.desktop"
	writeDesktopEntryFile(t, path, "[Desktop Entry]\nName=App\nExec=app\n")

	m := newModelForUXTests()
	if err := m.openEditor(desktop.Entry{Path: path, ID: "app.desktop", Source: desktop.SourceUser}); err != nil {
		t.Fatalf("openEditor: %v", err)
	}

	v := m.View()
	if !strings.Contains(v, "save") {
		t.Fatalf("editor help missing save binding: %q", v)
	}
	if !strings.Contains(v, "browse/install icon") {
		t.Fatalf("editor help missing install binding: %q", v)
	}
}

func TestScreenSpecificHelp_RenderedByView(t *testing.T) {
	m := newModelForUXTests()
	m.screen = screenList
	if v := m.View(); !strings.Contains(v, "edit") {
		t.Fatalf("list help missing edit hint: %q", v)
	}

	m.openInstallPath()
	if v := m.View(); !strings.Contains(v, "install/browse") {
		t.Fatalf("install-path help missing install hint: %q", v)
	}
}

func TestOpenInstallPath_ResetsStatusAndError(t *testing.T) {
	m := newModelForUXTests()
	m.status = "old status"
	m.err = errors.New("old err")

	m.openInstallPath()

	if m.status != "" {
		t.Fatalf("status = %q, want empty", m.status)
	}
	if m.err != nil {
		t.Fatalf("err = %v, want nil", m.err)
	}
}

func TestInstallBrowse_EscReturnsToForm(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	m := newModelForUXTests()
	m.openInstallPath()
	m.openInstallBrowse(home)
	if m.screen != screenInstallBrowse {
		t.Fatalf("screen = %v, want install browse", m.screen)
	}

	_, _ = m.updateInstallBrowse(tea.KeyMsg{Type: tea.KeyEsc})
	if m.screen != screenInstallPath {
		t.Fatalf("screen = %v, want install path", m.screen)
	}
}

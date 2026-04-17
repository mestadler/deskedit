package ui

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mestadler/deskedit/internal/desktop"
)

func TestCtrlNOpensInstallBrowserFromHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	entryPath := filepath.Join(t.TempDir(), "applications", "app.desktop")
	writeDesktopEntryFile(t, entryPath, "[Desktop Entry]\nName=App\nExec=app\n")

	m := &Model{}
	err := m.openEditor(desktop.Entry{Path: entryPath, ID: "app.desktop", Source: desktop.SourceUser})
	if err != nil {
		t.Fatalf("openEditor: %v", err)
	}

	_, _ = m.updateEditor(tea.KeyMsg{Type: tea.KeyCtrlN})

	if m.screen != screenInstallBrowse {
		t.Fatalf("screen = %v, want install browse", m.screen)
	}
	if m.browserCWD != home {
		t.Fatalf("browserCWD = %q, want %q", m.browserCWD, home)
	}
}

func TestInstallBrowseSelectionPrefillsPathAndName(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	iconPath := filepath.Join(home, "logo.png")
	if err := os.WriteFile(iconPath, []byte("not-a-real-png"), 0o644); err != nil {
		t.Fatalf("write icon: %v", err)
	}

	entryPath := filepath.Join(t.TempDir(), "applications", "app.desktop")
	writeDesktopEntryFile(t, entryPath, "[Desktop Entry]\nName=App\nExec=app\n")

	m := &Model{}
	err := m.openEditor(desktop.Entry{Path: entryPath, ID: "app.desktop", Source: desktop.SourceUser})
	if err != nil {
		t.Fatalf("openEditor: %v", err)
	}

	_, _ = m.updateEditor(tea.KeyMsg{Type: tea.KeyCtrlN})
	_, _ = m.updateInstallBrowse(tea.KeyMsg{Type: tea.KeyDown}) // move from .. to first file
	_, _ = m.updateInstallBrowse(tea.KeyMsg{Type: tea.KeyEnter})

	if m.screen != screenInstallPath {
		t.Fatalf("screen = %v, want install path", m.screen)
	}
	if got := m.installPathInput.Value(); got != iconPath {
		t.Fatalf("install path = %q, want %q", got, iconPath)
	}
	if got := m.installNameInput.Value(); got != "logo" {
		t.Fatalf("install name = %q, want %q", got, "logo")
	}
	if got := m.lastBrowseDir; got != home {
		t.Fatalf("lastBrowseDir = %q, want %q", got, home)
	}
}

func TestInstallPathEnterWithEmptySourceOpensBrowser(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	m := &Model{}
	m.openInstallPath()
	if m.screen != screenInstallPath {
		t.Fatalf("screen = %v, want install path", m.screen)
	}

	_, _ = m.updateInstallPath(tea.KeyMsg{Type: tea.KeyEnter})

	if m.screen != screenInstallBrowse {
		t.Fatalf("screen = %v, want install browse", m.screen)
	}
	if m.browserCWD != home {
		t.Fatalf("browserCWD = %q, want %q", m.browserCWD, home)
	}
	if m.err != nil {
		t.Fatalf("unexpected error: %v", m.err)
	}
}

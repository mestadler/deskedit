package ui

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mestadler/deskedit/internal/desktop"
	"github.com/mestadler/deskedit/internal/gpu"
)

func writeDesktopEntryFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestSave_SystemEntryWritesUserOverride(t *testing.T) {
	sysBase := t.TempDir()
	userBase := t.TempDir()

	sysApps := filepath.Join(sysBase, "applications")
	if err := os.MkdirAll(sysApps, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", sysApps, err)
	}

	id := "app.desktop"
	sysPath := filepath.Join(sysApps, id)
	original := "[Desktop Entry]\nName=System App\nExec=system-app\nIcon=system-icon\n"
	writeDesktopEntryFile(t, sysPath, original)

	t.Setenv("XDG_DATA_HOME", userBase)

	m := &Model{}
	err := m.openEditor(desktop.Entry{Path: sysPath, ID: id, Source: desktop.SourceSystem})
	if err != nil {
		t.Fatalf("openEditor: %v", err)
	}

	m.inputs[fieldName].SetValue("User Override")
	m.inputs[fieldExec].SetValue("override-app --flag")
	m.inputs[fieldIcon].SetValue("override-icon")
	m.gpuMode = gpu.ModeNone

	if err := m.save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	userOverridePath, err := desktop.UserPathFor(id)
	if err != nil {
		t.Fatalf("UserPathFor: %v", err)
	}

	if _, err := os.Stat(userOverridePath); err != nil {
		t.Fatalf("expected user override at %s: %v", userOverridePath, err)
	}

	sysAfter, err := os.ReadFile(sysPath)
	if err != nil {
		t.Fatalf("read system desktop file: %v", err)
	}
	if string(sysAfter) != original {
		t.Fatalf("system file changed in place; got:\n%s\nwant:\n%s", string(sysAfter), original)
	}

	overrideFile, err := desktop.Load(userOverridePath)
	if err != nil {
		t.Fatalf("load override: %v", err)
	}

	if got, ok := overrideFile.Get("Name"); !ok || got != "User Override" {
		t.Fatalf("override Name = %q, %v; want %q, true", got, ok, "User Override")
	}
	if got, ok := overrideFile.Get("Exec"); !ok || got != "override-app --flag" {
		t.Fatalf("override Exec = %q, %v; want %q, true", got, ok, "override-app --flag")
	}
	if got, ok := overrideFile.Get("Icon"); !ok || got != "override-icon" {
		t.Fatalf("override Icon = %q, %v; want %q, true", got, ok, "override-icon")
	}

	wantStatus := "saved: " + userOverridePath
	if m.status != wantStatus {
		t.Fatalf("status = %q, want %q", m.status, wantStatus)
	}
}

func TestOpenEditor_LoadsBooleanFields(t *testing.T) {
	base := t.TempDir()
	path := filepath.Join(base, "applications", "bools.desktop")
	writeDesktopEntryFile(t, path, "[Desktop Entry]\nName=Bool App\nExec=app\nTerminal=TRUE\nNoDisplay=true\nHidden=false\n")

	m := &Model{}
	err := m.openEditor(desktop.Entry{Path: path, ID: "bools.desktop", Source: desktop.SourceUser})
	if err != nil {
		t.Fatalf("openEditor: %v", err)
	}

	if !m.terminal {
		t.Fatalf("terminal = %v, want true", m.terminal)
	}
	if !m.noDisplay {
		t.Fatalf("noDisplay = %v, want true", m.noDisplay)
	}
	if m.hidden {
		t.Fatalf("hidden = %v, want false", m.hidden)
	}
}

func TestSave_BooleanFieldsToggleAndNormalize(t *testing.T) {
	base := t.TempDir()
	path := filepath.Join(base, "applications", "toggle.desktop")
	writeDesktopEntryFile(t, path, "[Desktop Entry]\nName=Toggle App\nExec=app\n")

	m := &Model{}
	err := m.openEditor(desktop.Entry{Path: path, ID: "toggle.desktop", Source: desktop.SourceUser})
	if err != nil {
		t.Fatalf("openEditor: %v", err)
	}

	m.focused = fieldTerminal
	if _, _ = m.updateEditor(tea.KeyMsg{Type: tea.KeySpace}); !m.terminal {
		t.Fatalf("terminal toggle failed; got %v", m.terminal)
	}

	m.focused = fieldNoDisplay
	if _, _ = m.updateEditor(tea.KeyMsg{Type: tea.KeyRight}); !m.noDisplay {
		t.Fatalf("noDisplay toggle failed; got %v", m.noDisplay)
	}

	m.focused = fieldHidden
	if _, _ = m.updateEditor(tea.KeyMsg{Type: tea.KeyEnter}); !m.hidden {
		t.Fatalf("hidden toggle failed; got %v", m.hidden)
	}

	if err := m.save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	f, err := desktop.Load(path)
	if err != nil {
		t.Fatalf("load saved file: %v", err)
	}

	if got, ok := f.Get("Terminal"); !ok || got != "true" {
		t.Fatalf("Terminal = %q, %v; want true, true", got, ok)
	}
	if got, ok := f.Get("NoDisplay"); !ok || got != "true" {
		t.Fatalf("NoDisplay = %q, %v; want true, true", got, ok)
	}
	if got, ok := f.Get("Hidden"); !ok || got != "true" {
		t.Fatalf("Hidden = %q, %v; want true, true", got, ok)
	}
}

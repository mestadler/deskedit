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
	delegate := newPlainDelegate()
	m := &Model{}
	m.ensureDefaults()
	m.regionFocus = regionBody
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

func TestNarrowTerminal_ViewStaysUsableAcrossScreens(t *testing.T) {
	sizes := []struct {
		name   string
		width  int
		height int
	}{
		{name: "40x12", width: 40, height: 12},
		{name: "60x16", width: 60, height: 16},
	}

	for _, sz := range sizes {
		t.Run(sz.name+"_list", func(t *testing.T) {
			m := newModelForUXTests()
			assertScreenViewAtSize(t, m, sz.width, sz.height, "┌", "deskedit", "ctrl+k commands", "enter edit")
		})

		t.Run(sz.name+"_editor", func(t *testing.T) {
			path := t.TempDir() + "/applications/app.desktop"
			writeDesktopEntryFile(t, path, "[Desktop Entry]\nName=App\nExec=app\n")
			m := newModelForUXTests()
			if err := m.openEditor(desktop.Entry{Path: path, ID: "app.desktop", Source: desktop.SourceUser}); err != nil {
				t.Fatalf("openEditor: %v", err)
			}
			assertScreenViewAtSize(t, m, sz.width, sz.height, "┌", "deskedit", "ctrl+k commands", "ctrl+s save")
		})

		t.Run(sz.name+"_icon_picker", func(t *testing.T) {
			m := newModelForUXTests()
			m.openIconPicker()
			assertScreenViewAtSize(t, m, sz.width, sz.height, "┌", "deskedit", "ctrl+k commands", "enter accept")
		})

		t.Run(sz.name+"_install_path", func(t *testing.T) {
			m := newModelForUXTests()
			m.openInstallPath()
			assertScreenViewAtSize(t, m, sz.width, sz.height, "┌", "deskedit", "ctrl+k commands", "enter", "install/browse")
		})

		t.Run(sz.name+"_install_browse", func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			m := newModelForUXTests()
			m.openInstallPath()
			m.openInstallBrowse(home)
			assertScreenViewAtSize(t, m, sz.width, sz.height, "┌", "deskedit", "ctrl+k commands", "enter", "open/select")
		})
	}
}

func TestLayoutChrome_TitleBarVisibleAcrossScreens(t *testing.T) {
	path := t.TempDir() + "/applications/app.desktop"
	writeDesktopEntryFile(t, path, "[Desktop Entry]\nName=App\nExec=app\n")

	home := t.TempDir()
	t.Setenv("HOME", home)

	m := newModelForUXTests()
	m.setEntries([]desktop.Entry{{Path: path, ID: "app.desktop", Name: "App", Source: desktop.SourceUser}})
	m.openIconPicker()
	assertViewContains(t, m, "┌", "deskedit")

	if err := m.openEditor(desktop.Entry{Path: path, ID: "app.desktop", Source: desktop.SourceUser}); err != nil {
		t.Fatalf("openEditor: %v", err)
	}
	assertViewContains(t, m, "┌", "deskedit")

	m.openInstallPath()
	assertViewContains(t, m, "┌", "deskedit")

	m.openInstallBrowse(home)
	assertViewContains(t, m, "┌", "deskedit")

	m.screen = screenList
	assertViewContains(t, m, "┌", "deskedit")
}

func TestPrimaryCommandBar_ShowsExpectedBindingsByScreen(t *testing.T) {
	path := t.TempDir() + "/applications/app.desktop"
	writeDesktopEntryFile(t, path, "[Desktop Entry]\nName=App\nExec=app\n")

	home := t.TempDir()
	t.Setenv("HOME", home)

	m := newModelForUXTests()
	m.screen = screenList
	assertViewContains(t, m, "ctrl+k commands", "enter edit")

	if err := m.openEditor(desktop.Entry{Path: path, ID: "app.desktop", Source: desktop.SourceUser}); err != nil {
		t.Fatalf("openEditor: %v", err)
	}
	assertViewContains(t, m, "ctrl+k commands", "ctrl+s save")

	m.openIconPicker()
	assertViewContains(t, m, "ctrl+k commands", "enter accept")

	m.openInstallPath()
	assertViewContains(t, m, "ctrl+k commands", "enter", "install/browse")

	m.openInstallBrowse(home)
	assertViewContains(t, m, "ctrl+k commands", "enter", "open/select")
}

func TestLayoutChrome_FooterFrameAndChipsVisibleAcrossScreens(t *testing.T) {
	path := t.TempDir() + "/applications/app.desktop"
	writeDesktopEntryFile(t, path, "[Desktop Entry]\nName=App\nExec=app\n")

	home := t.TempDir()
	t.Setenv("HOME", home)

	m := newModelForUXTests()
	m.screen = screenList
	assertViewContains(t, m, "┌", "[ctrl+k commands]")

	if err := m.openEditor(desktop.Entry{Path: path, ID: "app.desktop", Source: desktop.SourceUser}); err != nil {
		t.Fatalf("openEditor: %v", err)
	}
	assertViewContains(t, m, "┌", "[ctrl+s save]", "[ctrl+i icon picker]")

	m.openIconPicker()
	assertViewContains(t, m, "┌", "[enter accept]", "[esc cancel]")

	m.openInstallPath()
	assertViewContains(t, m, "┌", "[enter", "install/browse", "[ctrl+b browse files]")

	m.openInstallBrowse(home)
	assertViewContains(t, m, "┌", "[enter", "open/select", "[esc back]")

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	assertViewContains(t, m, "┌", "[enter run]", "[esc cancel]")
}

func TestRegionFocus_TraversalCyclesHeaderBodyFooter(t *testing.T) {
	m := newModelForUXTests()
	if m.regionFocus != regionBody {
		t.Fatalf("initial region = %v, want body", m.regionFocus)
	}

	m.focusNextRegion()
	if m.regionFocus != regionFooter {
		t.Fatalf("after next = %v, want footer", m.regionFocus)
	}
	m.focusNextRegion()
	if m.regionFocus != regionHeader {
		t.Fatalf("after next = %v, want header", m.regionFocus)
	}
	m.focusNextRegion()
	if m.regionFocus != regionBody {
		t.Fatalf("after next = %v, want body", m.regionFocus)
	}

	m.focusPrevRegion()
	if m.regionFocus != regionHeader {
		t.Fatalf("after prev = %v, want header", m.regionFocus)
	}
	m.focusPrevRegion()
	if m.regionFocus != regionFooter {
		t.Fatalf("after prev = %v, want footer", m.regionFocus)
	}
}

func TestRegionFocus_NonBodyBlocksBodyTabHandling(t *testing.T) {
	path := t.TempDir() + "/applications/app.desktop"
	writeDesktopEntryFile(t, path, "[Desktop Entry]\nName=App\nExec=app\n")

	m := newModelForUXTests()
	if err := m.openEditor(desktop.Entry{Path: path, ID: "app.desktop", Source: desktop.SourceUser}); err != nil {
		t.Fatalf("openEditor: %v", err)
	}

	if m.focused != fieldName {
		t.Fatalf("focused = %v, want name", m.focused)
	}

	m.regionFocus = regionHeader
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.focused != fieldName {
		t.Fatalf("focused changed in non-body region: got %v, want %v", m.focused, fieldName)
	}

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.regionFocus != regionBody {
		t.Fatalf("esc should return focus to body; got %v", m.regionFocus)
	}

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.focused == fieldName {
		t.Fatalf("tab in body region should advance editor field focus")
	}
}

func TestFooterRegion_TabCyclesFooterActions(t *testing.T) {
	m := newModelForUXTests()
	m.screen = screenList
	m.regionFocus = regionFooter

	start := m.footerAction
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.footerAction == start {
		t.Fatalf("footer action did not advance on tab")
	}

	afterTab := m.footerAction
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.footerAction == afterTab {
		t.Fatalf("footer action did not move on shift+tab")
	}
}

func TestFooterRegion_EnterExecutesSelectedAction(t *testing.T) {
	path := t.TempDir() + "/applications/app.desktop"
	writeDesktopEntryFile(t, path, "[Desktop Entry]\nName=List App\nExec=app\n")

	m := newModelForUXTests()
	entry := desktop.Entry{Path: path, ID: "app.desktop", Name: "List App", Source: desktop.SourceUser}
	m.setEntries([]desktop.Entry{entry})
	m.screen = screenList
	m.list.Select(0)
	m.regionFocus = regionFooter

	actions := m.footerActionsFor(screenList)
	idx := -1
	for i, a := range actions {
		if a.id == "list_edit" {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatalf("list_edit footer action not found")
	}
	m.footerAction = idx

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != screenEditor {
		t.Fatalf("screen = %v, want editor", m.screen)
	}
	if m.regionFocus != regionBody {
		t.Fatalf("region focus = %v, want body", m.regionFocus)
	}
}

func TestNarrowTerminal_InstallNavigationStillWorks(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	sizes := []struct{ width, height int }{{40, 12}, {60, 16}}
	for _, sz := range sizes {
		m := newModelForUXTests()
		_, _ = m.Update(tea.WindowSizeMsg{Width: sz.width, Height: sz.height})

		m.openInstallPath()
		_, _ = m.updateInstallPath(tea.KeyMsg{Type: tea.KeyEnter})
		if m.screen != screenInstallBrowse {
			t.Fatalf("screen = %v, want install browse", m.screen)
		}

		_, _ = m.updateInstallBrowse(tea.KeyMsg{Type: tea.KeyEsc})
		if m.screen != screenInstallPath {
			t.Fatalf("screen = %v, want install path", m.screen)
		}
	}
}

func TestFooterRegion_SaveActionParityWithKeyAndPalette(t *testing.T) {
	pathViaKey := t.TempDir() + "/applications/key.desktop"
	pathViaPalette := t.TempDir() + "/applications/palette.desktop"
	pathViaFooter := t.TempDir() + "/applications/footer.desktop"
	writeDesktopEntryFile(t, pathViaKey, "[Desktop Entry]\nName=App\nExec=app\n")
	writeDesktopEntryFile(t, pathViaPalette, "[Desktop Entry]\nName=App\nExec=app\n")
	writeDesktopEntryFile(t, pathViaFooter, "[Desktop Entry]\nName=App\nExec=app\n")

	byKey := newModelForUXTests()
	if err := byKey.openEditor(desktop.Entry{Path: pathViaKey, ID: "key.desktop", Source: desktop.SourceUser}); err != nil {
		t.Fatalf("openEditor key path: %v", err)
	}
	byKey.inputs[fieldName].SetValue("Parity Name")
	_, _ = byKey.updateEditor(tea.KeyMsg{Type: tea.KeyCtrlS})

	byPalette := newModelForUXTests()
	if err := byPalette.openEditor(desktop.Entry{Path: pathViaPalette, ID: "palette.desktop", Source: desktop.SourceUser}); err != nil {
		t.Fatalf("openEditor palette path: %v", err)
	}
	byPalette.inputs[fieldName].SetValue("Parity Name")
	_, _ = byPalette.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	selectPaletteCommand(t, byPalette, "editor_save")
	_, _ = byPalette.Update(tea.KeyMsg{Type: tea.KeyEnter})

	byFooter := newModelForUXTests()
	if err := byFooter.openEditor(desktop.Entry{Path: pathViaFooter, ID: "footer.desktop", Source: desktop.SourceUser}); err != nil {
		t.Fatalf("openEditor footer path: %v", err)
	}
	byFooter.inputs[fieldName].SetValue("Parity Name")
	byFooter.regionFocus = regionFooter
	actions := byFooter.footerActionsFor(screenEditor)
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
	byFooter.footerAction = idx
	_, _ = byFooter.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if byKey.screen != screenList || byPalette.screen != screenList || byFooter.screen != screenList {
		t.Fatalf("screens after save key=%v palette=%v footer=%v; want all list", byKey.screen, byPalette.screen, byFooter.screen)
	}

	fKey, err := desktop.Load(pathViaKey)
	if err != nil {
		t.Fatalf("load key file: %v", err)
	}
	fPalette, err := desktop.Load(pathViaPalette)
	if err != nil {
		t.Fatalf("load palette file: %v", err)
	}
	fFooter, err := desktop.Load(pathViaFooter)
	if err != nil {
		t.Fatalf("load footer file: %v", err)
	}

	nameKey, _ := fKey.Get("Name")
	namePalette, _ := fPalette.Get("Name")
	nameFooter, _ := fFooter.Get("Name")

	if nameKey != namePalette || nameKey != nameFooter {
		t.Fatalf("saved name mismatch key=%q palette=%q footer=%q", nameKey, namePalette, nameFooter)
	}
}

func assertScreenViewAtSize(t *testing.T, m *Model, width, height int, wantTokens ...string) {
	t.Helper()
	_, _ = m.Update(tea.WindowSizeMsg{Width: width, Height: height})

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("View panicked at %dx%d: %v", width, height, r)
		}
	}()

	v := normalizeHelp(m.View())
	if strings.TrimSpace(v) == "" {
		t.Fatalf("View returned empty output at %dx%d", width, height)
	}
	for _, wantToken := range wantTokens {
		if !strings.Contains(v, wantToken) {
			t.Fatalf("View at %dx%d missing token %q:\n%s", width, height, wantToken, v)
		}
	}
}

func assertViewContains(t *testing.T, m *Model, wantTokens ...string) {
	t.Helper()
	v := normalizeHelp(m.View())
	for _, token := range wantTokens {
		if !strings.Contains(v, token) {
			t.Fatalf("view missing token %q:\n%s", token, v)
		}
	}
}

package ui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var ansiSeq = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func TestHelpGolden(t *testing.T) {
	m := &Model{}
	m.ensureDefaults()

	cases := []struct {
		name    string
		showAll bool
		view    func(*Model) string
	}{
		{name: "list_short", showAll: false, view: func(m *Model) string { return m.help.View(m.keys.List) }},
		{name: "editor_short", showAll: false, view: func(m *Model) string { return m.help.View(m.keys.Editor) }},
		{name: "icon_picker_short", showAll: false, view: func(m *Model) string { return m.help.View(m.keys.IconPicker) }},
		{name: "install_path_short", showAll: false, view: func(m *Model) string { return m.help.View(m.keys.InstallPath) }},
		{name: "install_browse_short", showAll: false, view: func(m *Model) string { return m.help.View(m.keys.InstallBrowse) }},
		{name: "editor_full", showAll: true, view: func(m *Model) string { return m.help.View(m.keys.Editor) }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m.help.ShowAll = tc.showAll
			got := normalizeHelp(tc.view(m))
			assertGolden(t, tc.name+".golden", got)
		})
	}
}

func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	goldenPath := filepath.Join("testdata", "help", name)
	update := os.Getenv("UPDATE_GOLDEN") == "1"

	if update {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("mkdir golden dir: %v", err)
		}
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden %s: %v", goldenPath, err)
		}
	}

	wantBytes, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden %s: %v (set UPDATE_GOLDEN=1 to create/update)", goldenPath, err)
	}
	want := string(wantBytes)
	if got != want {
		t.Fatalf("golden mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", goldenPath, got, want)
	}
}

func normalizeHelp(s string) string {
	s = ansiSeq.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimRight(s, "\n") + "\n"
	return s
}

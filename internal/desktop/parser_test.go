package desktop

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleFile = `[Desktop Entry]
Version=1.0
Type=Application
# Firefox is a browser
Name=Firefox
Name[de]=Feuerfuchs
Exec=firefox %u
Icon=firefox
Categories=Network;WebBrowser;

[Desktop Action NewWindow]
Exec=firefox --new-window
Name=New Window
`

func TestParseRoundTrip(t *testing.T) {
	f, err := Parse(strings.NewReader(sampleFile))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var buf bytes.Buffer
	if err := f.Serialise(&buf); err != nil {
		t.Fatalf("serialise: %v", err)
	}
	got := buf.String()
	if got != sampleFile {
		t.Errorf("round-trip mismatch:\n--- want ---\n%s--- got ---\n%s", sampleFile, got)
	}
}

func TestGetSet(t *testing.T) {
	f, _ := Parse(strings.NewReader(sampleFile))

	if v, ok := f.Get("Exec"); !ok || v != "firefox %u" {
		t.Errorf("Get(Exec) = %q, %v; want \"firefox %%u\", true", v, ok)
	}

	// Localised key preserved as a distinct entry
	if v, ok := f.Get("Name[de]"); !ok || v != "Feuerfuchs" {
		t.Errorf("Get(Name[de]) = %q, %v", v, ok)
	}

	// Set updates existing
	f.Set("Exec", "firefox --safe-mode %u")
	if v, _ := f.Get("Exec"); v != "firefox --safe-mode %u" {
		t.Errorf("after Set, Get(Exec) = %q", v)
	}

	// Set appends new key within [Desktop Entry], not [Desktop Action NewWindow]
	f.Set("StartupWMClass", "firefox")
	var buf bytes.Buffer
	_ = f.Serialise(&buf)
	out := buf.String()
	idxEntry := strings.Index(out, "[Desktop Entry]")
	idxAction := strings.Index(out, "[Desktop Action NewWindow]")
	idxKey := strings.Index(out, "StartupWMClass=firefox")
	if idxKey < idxEntry || idxKey > idxAction {
		t.Errorf("new key landed outside [Desktop Entry]: entry@%d action@%d key@%d",
			idxEntry, idxAction, idxKey)
	}
}

func TestSetAddsDesktopEntry(t *testing.T) {
	// No [Desktop Entry] section — Set should prepend one.
	f, _ := Parse(strings.NewReader(""))
	f.Set("Name", "Test")
	var buf bytes.Buffer
	_ = f.Serialise(&buf)
	out := buf.String()
	if !strings.HasPrefix(out, "[Desktop Entry]\nName=Test\n") {
		t.Errorf("expected prepended section, got:\n%s", out)
	}
}

func TestDelete(t *testing.T) {
	f, _ := Parse(strings.NewReader(sampleFile))
	f.Delete("Icon")
	if _, ok := f.Get("Icon"); ok {
		t.Error("Icon should have been deleted")
	}
	// Unrelated keys untouched
	if v, ok := f.Get("Name"); !ok || v != "Firefox" {
		t.Errorf("Name changed unexpectedly: %q", v)
	}
}

func TestSave_LeavesNoTempFilesAndOverwritesAtomically(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "app.desktop")

	f, err := Parse(strings.NewReader(sampleFile))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	f.Set("Name", "First")
	if err := f.Save(target); err != nil {
		t.Fatalf("first save: %v", err)
	}

	f.Set("Name", "Second")
	if err := f.Save(target); err != nil {
		t.Fatalf("second save: %v", err)
	}

	saved, err := Load(target)
	if err != nil {
		t.Fatalf("load saved file: %v", err)
	}
	if got, ok := saved.Get("Name"); !ok || got != "Second" {
		t.Fatalf("saved Name = %q, %v; want %q, true", got, ok, "Second")
	}

	tmpFiles, err := filepath.Glob(filepath.Join(tmp, ".deskedit-*.tmp"))
	if err != nil {
		t.Fatalf("glob temp files: %v", err)
	}
	if len(tmpFiles) != 0 {
		t.Fatalf("temporary files leaked: %v", tmpFiles)
	}

	if _, err := os.Stat(target); err != nil {
		t.Fatalf("target missing after save: %v", err)
	}
}

package desktop

import (
	"os"
	"path/filepath"
	"testing"
)

func writeDesktopFile(t *testing.T, baseDir, id, name string) string {
	t.Helper()

	appsDir := filepath.Join(baseDir, "applications")
	if err := os.MkdirAll(appsDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", appsDir, err)
	}

	path := filepath.Join(appsDir, id)
	content := "[Desktop Entry]\nName=" + name + "\nExec=/bin/true\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func byID(entries []Entry) map[string]Entry {
	out := make(map[string]Entry, len(entries))
	for _, e := range entries {
		out[e.ID] = e
	}
	return out
}

func TestDiscover_UserOverridesSystemByDesktopID(t *testing.T) {
	userBase := t.TempDir()
	sysBase := t.TempDir()

	userPath := writeDesktopFile(t, userBase, "app.desktop", "User App")
	_ = writeDesktopFile(t, sysBase, "app.desktop", "System App")
	sysOnlyPath := writeDesktopFile(t, sysBase, "sys-only.desktop", "System Only")

	t.Setenv("XDG_DATA_HOME", userBase)
	t.Setenv("XDG_DATA_DIRS", sysBase)

	entries, err := Discover()
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}

	got := byID(entries)
	app := got["app.desktop"]
	if app.Path != userPath {
		t.Fatalf("app.desktop Path = %q, want %q", app.Path, userPath)
	}
	if app.Source != SourceUser {
		t.Fatalf("app.desktop Source = %v, want %v", app.Source, SourceUser)
	}
	if app.Name != "User App" {
		t.Fatalf("app.desktop Name = %q, want %q", app.Name, "User App")
	}

	sysOnly := got["sys-only.desktop"]
	if sysOnly.Path != sysOnlyPath {
		t.Fatalf("sys-only.desktop Path = %q, want %q", sysOnly.Path, sysOnlyPath)
	}
	if sysOnly.Source != SourceSystem {
		t.Fatalf("sys-only.desktop Source = %v, want %v", sysOnly.Source, SourceSystem)
	}
}

func TestDiscover_MultipleSystemDirsFirstSeenWins(t *testing.T) {
	userBase := t.TempDir()
	sys1Base := t.TempDir()
	sys2Base := t.TempDir()

	_ = writeDesktopFile(t, sys1Base, "dup.desktop", "From System One")
	dupFromSys2 := writeDesktopFile(t, sys2Base, "dup.desktop", "From System Two")

	t.Setenv("XDG_DATA_HOME", userBase)
	t.Setenv("XDG_DATA_DIRS", sys1Base+":"+sys2Base)

	entries, err := Discover()
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}

	e := entries[0]
	if e.ID != "dup.desktop" {
		t.Fatalf("ID = %q, want %q", e.ID, "dup.desktop")
	}
	if e.Path == dupFromSys2 {
		t.Fatalf("Path picked second system dir %q; expected first-seen winner", dupFromSys2)
	}
	if e.Name != "From System One" {
		t.Fatalf("Name = %q, want %q", e.Name, "From System One")
	}
}

func TestDiscover_NonExistentLocationsAreNonFatal(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", filepath.Join(t.TempDir(), "does-not-exist"))
	t.Setenv("XDG_DATA_DIRS", filepath.Join(t.TempDir(), "missing-a")+":"+filepath.Join(t.TempDir(), "missing-b"))

	entries, err := Discover()
	if err != nil {
		t.Fatalf("Discover returned error for missing locations: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("len(entries) = %d, want 0", len(entries))
	}
}

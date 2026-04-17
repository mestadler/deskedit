package desktop

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Entry is a discovered .desktop file with metadata useful for listing.
type Entry struct {
	Path      string // absolute path to the file actually being shown
	ID        string // e.g. "firefox.desktop" — the basename used for override matching
	Name      string // localised Name= from [Desktop Entry], falls back to ID
	Source    Source
	NoDisplay bool // true if NoDisplay=true (hidden apps)
	Hidden    bool // true if Hidden=true
}

type Source int

const (
	SourceUser   Source = iota // ~/.local/share/applications
	SourceSystem               // /usr/share/applications or XDG_DATA_DIRS
	SourceFlatpak
)

func (s Source) String() string {
	switch s {
	case SourceUser:
		return "user"
	case SourceSystem:
		return "system"
	case SourceFlatpak:
		return "flatpak"
	}
	return "unknown"
}

// Locations returns the search paths in precedence order (user first).
// Per XDG Base Directory Specification.
func Locations() []struct {
	Path   string
	Source Source
} {
	var locs []struct {
		Path   string
		Source Source
	}

	// XDG_DATA_HOME (user), default ~/.local/share
	home := os.Getenv("XDG_DATA_HOME")
	if home == "" {
		if h, err := os.UserHomeDir(); err == nil {
			home = filepath.Join(h, ".local", "share")
		}
	}
	if home != "" {
		locs = append(locs, struct {
			Path   string
			Source Source
		}{filepath.Join(home, "applications"), SourceUser})
	}

	// XDG_DATA_DIRS (system), default /usr/local/share:/usr/share
	dirs := os.Getenv("XDG_DATA_DIRS")
	if dirs == "" {
		dirs = "/usr/local/share:/usr/share"
	}
	for _, d := range strings.Split(dirs, ":") {
		if d == "" {
			continue
		}
		locs = append(locs, struct {
			Path   string
			Source Source
		}{filepath.Join(d, "applications"), SourceSystem})
	}

	return locs
}

// Discover walks all XDG locations and returns one Entry per unique desktop ID.
// User overrides shadow system entries (first-seen-wins, user comes first).
func Discover() ([]Entry, error) {
	seen := make(map[string]Entry) // keyed by desktop ID (basename)

	for _, loc := range Locations() {
		ents, err := os.ReadDir(loc.Path)
		if err != nil {
			// Non-existent location is not an error — just skip.
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, de := range ents {
			if de.IsDir() {
				continue
			}
			name := de.Name()
			if !strings.HasSuffix(name, ".desktop") {
				continue
			}
			if _, dup := seen[name]; dup {
				continue // user version already won
			}
			full := filepath.Join(loc.Path, name)
			e := Entry{Path: full, ID: name, Source: loc.Source}
			// Best-effort parse for Name/NoDisplay/Hidden. Failures are tolerated;
			// we still show the file with its ID as fallback.
			if f, err := Load(full); err == nil {
				if v, ok := f.Get("Name"); ok {
					e.Name = v
				}
				if v, ok := f.Get("NoDisplay"); ok && strings.EqualFold(strings.TrimSpace(v), "true") {
					e.NoDisplay = true
				}
				if v, ok := f.Get("Hidden"); ok && strings.EqualFold(strings.TrimSpace(v), "true") {
					e.Hidden = true
				}
			}
			if e.Name == "" {
				e.Name = strings.TrimSuffix(name, ".desktop")
			}
			seen[name] = e
		}
	}

	out := make([]Entry, 0, len(seen))
	for _, e := range seen {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

// UserPathFor returns the user-override path for a given desktop ID.
// This is where edits are written when the source is System.
func UserPathFor(id string) (string, error) {
	home := os.Getenv("XDG_DATA_HOME")
	if home == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		home = filepath.Join(h, ".local", "share")
	}
	return filepath.Join(home, "applications", id), nil
}

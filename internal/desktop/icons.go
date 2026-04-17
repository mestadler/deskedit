package desktop

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// IconDirs returns candidate directories for icon lookup, in XDG order.
func IconDirs() []string {
	var dirs []string

	// ~/.icons (legacy) and XDG_DATA_HOME/icons
	if h, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(h, ".icons"))
	}
	if xdh := os.Getenv("XDG_DATA_HOME"); xdh != "" {
		dirs = append(dirs, filepath.Join(xdh, "icons"))
	} else if h, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(h, ".local", "share", "icons"))
	}

	// XDG_DATA_DIRS/icons
	dd := os.Getenv("XDG_DATA_DIRS")
	if dd == "" {
		dd = "/usr/local/share:/usr/share"
	}
	for _, d := range strings.Split(dd, ":") {
		if d == "" {
			continue
		}
		dirs = append(dirs, filepath.Join(d, "icons"))
	}

	// Legacy pixmap locations
	dirs = append(dirs, "/usr/share/pixmaps", "/usr/local/share/pixmaps")
	return dirs
}

// ListAvailableIcons scans icon directories and returns unique icon names.
// This is intentionally shallow — a full XDG icon theme implementation would
// be huge; we return every distinct basename (without extension) we find.
// Suitable for a picker that filters by substring.
func ListAvailableIcons() ([]string, error) {
	set := make(map[string]struct{})
	exts := map[string]bool{
		".png": true, ".svg": true, ".xpm": true, ".svgz": true,
	}

	for _, root := range IconDirs() {
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil // tolerate unreadable dirs
			}
			if d.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if !exts[ext] {
				return nil
			}
			base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			set[base] = struct{}{}
			return nil
		})
	}

	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}

// ResolveIcon attempts to locate an icon file by name. Returns the first
// matching absolute path, or "" if not found. Absolute paths in `name` are
// returned unchanged if they exist.
func ResolveIcon(name string) string {
	if name == "" {
		return ""
	}
	if filepath.IsAbs(name) {
		if _, err := os.Stat(name); err == nil {
			return name
		}
		return ""
	}
	exts := []string{".svg", ".png", ".xpm", ".svgz"}
	for _, root := range IconDirs() {
		// Shallow check: pixmaps are flat
		for _, e := range exts {
			p := filepath.Join(root, name+e)
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
		// Deep check: icon themes are nested
		var found string
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || found != "" {
				return nil
			}
			base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			if base == name {
				found = path
			}
			return nil
		})
		if found != "" {
			return found
		}
	}
	return ""
}

package ui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
)

// fsItem is a list entry in the file browser.
type fsItem struct {
	name  string
	path  string
	isDir bool
}

func (i fsItem) Title() string {
	if i.isDir {
		return "📁 " + i.name
	}
	return "  " + i.name
}
func (i fsItem) Description() string { return "" }
func (i fsItem) FilterValue() string { return i.name }

// listDir returns dir entries suitable for the file browser.
// Hidden files are skipped; "../" is prepended unless at /.
func listDir(dir string) ([]list.Item, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var dirs, files []fsItem
	for _, e := range ents {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		full := filepath.Join(dir, name)
		if e.IsDir() {
			dirs = append(dirs, fsItem{name: name, path: full, isDir: true})
			continue
		}
		// Only show images — png, jpg, jpeg, gif, svg.
		ext := strings.ToLower(filepath.Ext(name))
		switch ext {
		case ".png", ".jpg", ".jpeg", ".gif", ".svg", ".svgz":
			files = append(files, fsItem{name: name, path: full})
		}
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].name < dirs[j].name })
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })

	items := make([]list.Item, 0, len(dirs)+len(files)+1)
	if dir != "/" {
		items = append(items, fsItem{
			name: "..", path: filepath.Dir(dir), isDir: true,
		})
	}
	for _, d := range dirs {
		items = append(items, d)
	}
	for _, f := range files {
		items = append(items, f)
	}
	return items, nil
}

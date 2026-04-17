// Package desktop parses and serialises freedesktop.org .desktop entry files.
//
// The parser preserves key order, comments, and blank lines within sections so
// that round-tripping an unmodified file produces byte-identical output (modulo
// trailing whitespace). Keys are case-sensitive per the XDG spec.
//
// This is deliberately more careful than a naive key=value map: a desktop-file
// editor must not reorder or drop entries it does not understand.
package desktop

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Line represents a single line in a .desktop file. Exactly one of the
// discriminator fields (IsGroup, IsKeyValue, IsComment, IsBlank) will be set.
type Line struct {
	Raw string // original text, used for comments/blanks/unknown

	IsGroup   bool
	GroupName string // e.g. "Desktop Entry"

	IsKeyValue bool
	Key        string
	Value      string

	IsComment bool
	IsBlank   bool
}

// File is a parsed .desktop file. Lines preserves original ordering.
type File struct {
	Path  string
	Lines []Line
}

// Parse reads a .desktop file from r.
func Parse(r io.Reader) (*File, error) {
	f := &File{}
	scanner := bufio.NewScanner(r)
	// .desktop files are usually small, but the default 64k buffer is plenty
	// for any sane case; bump it anyway for safety against very long Exec lines.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		raw := scanner.Text()
		trimmed := strings.TrimSpace(raw)

		switch {
		case trimmed == "":
			f.Lines = append(f.Lines, Line{Raw: raw, IsBlank: true})
		case strings.HasPrefix(trimmed, "#"):
			f.Lines = append(f.Lines, Line{Raw: raw, IsComment: true})
		case strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]"):
			name := strings.TrimSpace(trimmed[1 : len(trimmed)-1])
			f.Lines = append(f.Lines, Line{Raw: raw, IsGroup: true, GroupName: name})
		default:
			// key=value; locale suffix keys like Name[de] are preserved in Key as-is.
			eq := strings.IndexByte(raw, '=')
			if eq <= 0 {
				// Malformed line — keep as raw so we don't lose it.
				f.Lines = append(f.Lines, Line{Raw: raw, IsComment: true})
				continue
			}
			key := strings.TrimSpace(raw[:eq])
			val := raw[eq+1:] // do NOT trim value; trailing spaces can be meaningful
			f.Lines = append(f.Lines, Line{
				Raw: raw, IsKeyValue: true, Key: key, Value: val,
			})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning desktop file: %w", err)
	}
	return f, nil
}

// Load reads and parses a .desktop file from disk.
func Load(path string) (*File, error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	f, err := Parse(fh)
	if err != nil {
		return nil, err
	}
	f.Path = path
	return f, nil
}

// Get returns the value of the first matching key in [Desktop Entry].
// Returns ("", false) if the key is absent.
func (f *File) Get(key string) (string, bool) {
	inMain := false
	for _, ln := range f.Lines {
		if ln.IsGroup {
			inMain = ln.GroupName == "Desktop Entry"
			continue
		}
		if inMain && ln.IsKeyValue && ln.Key == key {
			return ln.Value, true
		}
	}
	return "", false
}

// Set updates an existing key in [Desktop Entry], or appends it if absent.
// If the file has no [Desktop Entry] section, one is prepended.
func (f *File) Set(key, value string) {
	// Try to update in place.
	inMain := false
	for i, ln := range f.Lines {
		if ln.IsGroup {
			inMain = ln.GroupName == "Desktop Entry"
			continue
		}
		if inMain && ln.IsKeyValue && ln.Key == key {
			f.Lines[i].Value = value
			f.Lines[i].Raw = key + "=" + value
			return
		}
	}

	// Key absent — find end of [Desktop Entry] section and append there.
	entryStart := -1
	entryEnd := len(f.Lines)
	for i, ln := range f.Lines {
		if ln.IsGroup {
			if ln.GroupName == "Desktop Entry" {
				entryStart = i
			} else if entryStart >= 0 {
				entryEnd = i
				break
			}
		}
	}

	newLine := Line{
		Raw: key + "=" + value, IsKeyValue: true, Key: key, Value: value,
	}

	if entryStart < 0 {
		// No [Desktop Entry] at all — prepend section + key.
		header := Line{Raw: "[Desktop Entry]", IsGroup: true, GroupName: "Desktop Entry"}
		f.Lines = append([]Line{header, newLine}, f.Lines...)
		return
	}

	// Insert just before entryEnd, skipping trailing blank lines so the new
	// key lands next to other keys in the section rather than after gaps.
	insertAt := entryEnd
	for insertAt > entryStart+1 && f.Lines[insertAt-1].IsBlank {
		insertAt--
	}
	f.Lines = append(f.Lines[:insertAt],
		append([]Line{newLine}, f.Lines[insertAt:]...)...)
}

// Delete removes the first matching key from [Desktop Entry]. No-op if absent.
func (f *File) Delete(key string) {
	inMain := false
	for i, ln := range f.Lines {
		if ln.IsGroup {
			inMain = ln.GroupName == "Desktop Entry"
			continue
		}
		if inMain && ln.IsKeyValue && ln.Key == key {
			f.Lines = append(f.Lines[:i], f.Lines[i+1:]...)
			return
		}
	}
}

// Serialise writes the file to w, preserving original formatting where
// possible. Modified key/value lines use the form "key=value" with no spaces
// around the '=' (per XDG convention).
func (f *File) Serialise(w io.Writer) error {
	bw := bufio.NewWriter(w)
	for _, ln := range f.Lines {
		var out string
		switch {
		case ln.IsKeyValue:
			// Reconstruct from Key/Value so edits are reflected.
			out = ln.Key + "=" + ln.Value
		default:
			out = ln.Raw
		}
		if _, err := bw.WriteString(out); err != nil {
			return err
		}
		if _, err := bw.WriteString("\n"); err != nil {
			return err
		}
	}
	return bw.Flush()
}

// Save writes the file atomically to path (tmp + rename).
func (f *File) Save(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".deskedit-*.tmp")
	if err != nil {
		return fmt.Errorf("creating tmp file: %w", err)
	}
	tmpPath := tmp.Name()
	// Ensure we don't leak the tmp file on error paths.
	defer func() { _ = os.Remove(tmpPath) }()

	if err := f.Serialise(tmp); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("renaming to %s: %w", path, err)
	}
	return nil
}

package desktop

import (
	"bufio"
	"bytes"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	_ "image/gif"
	_ "image/jpeg" // DecodeConfig should understand JPEG sources too

	"golang.org/x/image/draw"
)

// IconInstallRequest describes an icon installation.
type IconInstallRequest struct {
	SourcePath string // absolute path to user-supplied file (.png or .svg)
	Name       string // installed icon name, without extension (e.g. "firefox")
}

// IconInstallResult reports what actually happened.
type IconInstallResult struct {
	InstalledPath  string // final path on disk
	SizeDir        string // hicolor size dir used (e.g. "256x256" or "scalable")
	Resized        bool   // true if the image was downscaled
	OriginalW      int
	OriginalH      int
	CacheUpdateErr error // non-nil if gtk-update-icon-cache failed (non-fatal)
}

// maxIconSize is the upper bound for PNG icons — anything larger is downscaled.
const maxIconSize = 256

// userHicolorBase returns $XDG_DATA_HOME/icons/hicolor (or the default).
func userHicolorBase() (string, error) {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(h, ".local", "share")
	}
	return filepath.Join(base, "icons", "hicolor"), nil
}

// ValidateIconName rejects names that would break the XDG icon lookup.
// Per spec, names are matched verbatim — no path separators, no extensions,
// and no leading dots (hidden files are ignored by the cache).
func ValidateIconName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("icon name is empty")
	}
	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("icon name must not contain path separators")
	}
	if strings.HasPrefix(name, ".") {
		return fmt.Errorf("icon name must not start with a dot")
	}
	// .desktop spec doesn't forbid extensions in Icon=, but if the user types
	// "firefox.png" they almost certainly mean "firefox". Tell them.
	if ext := filepath.Ext(name); ext != "" {
		return fmt.Errorf("icon name must not include an extension (got %q)", ext)
	}
	return nil
}

// sniffImage returns (format, width, height) for a file without full decoding.
// Format is one of "png", "jpeg", "gif", "svg", "" (unknown).
func sniffImage(path string) (format string, w, h int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, 0, err
	}
	defer f.Close()

	br := bufio.NewReader(f)
	peek, _ := br.Peek(512)

	if bytes.HasPrefix(peek, []byte("\x89PNG\r\n\x1a\n")) ||
		bytes.HasPrefix(peek, []byte{0xff, 0xd8, 0xff}) ||
		bytes.HasPrefix(peek, []byte("GIF87a")) ||
		bytes.HasPrefix(peek, []byte("GIF89a")) {
		cfg, fmtName, cfgErr := image.DecodeConfig(br)
		if cfgErr != nil {
			return "", 0, 0, fmt.Errorf("decoding header: %w", cfgErr)
		}
		return fmtName, cfg.Width, cfg.Height, nil
	}

	// Very loose SVG sniff — look for "<svg" in the first 512 bytes, possibly
	// after XML prolog and whitespace.
	if bytes.Contains(peek, []byte("<svg")) {
		return "svg", 0, 0, nil
	}

	return "", 0, 0, fmt.Errorf("unrecognised image format")
}

// InstallIcon copies (and possibly resizes) an icon file into the user's
// hicolor theme and refreshes the icon cache.
//
// Behaviour:
//   - .svg sources go into scalable/apps/<name>.svg, copied verbatim.
//   - .png sources >256px in either dimension are resized to fit 256x256
//     (aspect-preserving) and installed as 256x256/apps/<name>.png.
//   - .png sources <=256px are copied verbatim to the matching hicolor NxN
//     size dir (rounded up to standard size).
//   - .jpeg/.gif sources are transcoded to PNG and treated as PNG above.
//
// gtk-update-icon-cache is invoked if present. A cache failure is reported
// in the result but does not fail the install — the icon is on disk either way.
func InstallIcon(req IconInstallRequest) (*IconInstallResult, error) {
	if err := ValidateIconName(req.Name); err != nil {
		return nil, err
	}
	if _, err := os.Stat(req.SourcePath); err != nil {
		return nil, fmt.Errorf("source file: %w", err)
	}

	format, w, h, err := sniffImage(req.SourcePath)
	if err != nil {
		return nil, err
	}

	base, err := userHicolorBase()
	if err != nil {
		return nil, err
	}

	res := &IconInstallResult{OriginalW: w, OriginalH: h}

	switch format {
	case "svg":
		target := filepath.Join(base, "scalable", "apps", req.Name+".svg")
		if err := copyFile(req.SourcePath, target); err != nil {
			return nil, err
		}
		res.InstalledPath = target
		res.SizeDir = "scalable"

	case "png", "jpeg", "gif":
		// Normalise to PNG regardless of source; .desktop convention is PNG
		// in size-specific dirs.
		needResize := w > maxIconSize || h > maxIconSize
		var sizeDir string
		var targetW, targetH int

		if needResize {
			sizeDir = fmt.Sprintf("%dx%d", maxIconSize, maxIconSize)
			// Aspect-preserving fit inside 256x256.
			if w >= h {
				targetW = maxIconSize
				targetH = h * maxIconSize / w
			} else {
				targetH = maxIconSize
				targetW = w * maxIconSize / h
			}
			res.Resized = true
		} else {
			sizeDir = roundUpHicolorSize(w, h)
			targetW, targetH = w, h
		}

		target := filepath.Join(base, sizeDir, "apps", req.Name+".png")
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return nil, err
		}

		if format == "png" && !needResize {
			if err := copyFile(req.SourcePath, target); err != nil {
				return nil, err
			}
		} else {
			if err := resizeAndWritePNG(req.SourcePath, target, targetW, targetH); err != nil {
				return nil, err
			}
		}
		res.InstalledPath = target
		res.SizeDir = sizeDir

	default:
		return nil, fmt.Errorf("unsupported image format: %s", format)
	}

	// Best-effort cache refresh. Failure is non-fatal.
	res.CacheUpdateErr = updateIconCache(base)
	return res, nil
}

// roundUpHicolorSize picks the smallest standard hicolor size dir that can
// contain a width x height icon. Falls back to "256x256" if larger than all.
func roundUpHicolorSize(w, h int) string {
	// Max of the two dimensions drives the choice.
	m := w
	if h > m {
		m = h
	}
	standard := []int{16, 22, 24, 32, 48, 64, 96, 128, 256}
	for _, s := range standard {
		if m <= s {
			return fmt.Sprintf("%dx%d", s, s)
		}
	}
	return "256x256"
}

// copyFile writes src to dst atomically (tmp + rename).
func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp, err := os.CreateTemp(filepath.Dir(dst), ".deskedit-icon-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op if rename succeeded

	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, dst)
}

// resizeAndWritePNG decodes src, scales to (w,h) with Catmull-Rom, and
// writes a PNG to dst atomically.
func resizeAndWritePNG(src, dst string, w, h int) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	img, _, err := image.Decode(in)
	if err != nil {
		return fmt.Errorf("decode %s: %w", src, err)
	}

	dstImg := image.NewRGBA(image.Rect(0, 0, w, h))
	// CatmullRom gives sharper results than Bilinear for downscaling icons.
	draw.CatmullRom.Scale(dstImg, dstImg.Bounds(), img, img.Bounds(), draw.Over, nil)

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".deskedit-icon-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if err := png.Encode(tmp, dstImg); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, dst)
}

// updateIconCache calls gtk-update-icon-cache on the given theme root.
// Returns nil if the tool isn't installed (non-fatal — GNOME/KDE will
// pick up new icons on next login anyway).
func updateIconCache(themeRoot string) error {
	bin, err := exec.LookPath("gtk-update-icon-cache")
	if err != nil {
		// Tool not installed — not an error.
		return nil
	}
	// -f: force, -q: quiet, -t: skip theme index check (we haven't written one).
	// If there's no index.theme, the tool will refuse — that's fine on first
	// run; we'll create a minimal one if needed.
	if err := ensureHicolorIndex(themeRoot); err != nil {
		return err
	}
	cmd := exec.Command(bin, "-f", "-q", themeRoot)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gtk-update-icon-cache: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// ensureHicolorIndex writes a minimal index.theme if the user's hicolor
// directory doesn't already have one. Without this, gtk-update-icon-cache
// refuses to build the cache.
func ensureHicolorIndex(themeRoot string) error {
	idx := filepath.Join(themeRoot, "index.theme")
	if _, err := os.Stat(idx); err == nil {
		return nil
	}
	const minimal = `[Icon Theme]
Name=Hicolor
Comment=User icon theme (generated by deskedit)
Directories=scalable/apps,256x256/apps,128x128/apps,96x96/apps,64x64/apps,48x48/apps,32x32/apps,24x24/apps,22x22/apps,16x16/apps

[scalable/apps]
Size=48
Scale=1
Context=Applications
Type=Scalable
MinSize=8
MaxSize=512

[256x256/apps]
Size=256
Context=Applications
Type=Fixed

[128x128/apps]
Size=128
Context=Applications
Type=Fixed

[96x96/apps]
Size=96
Context=Applications
Type=Fixed

[64x64/apps]
Size=64
Context=Applications
Type=Fixed

[48x48/apps]
Size=48
Context=Applications
Type=Fixed

[32x32/apps]
Size=32
Context=Applications
Type=Fixed

[24x24/apps]
Size=24
Context=Applications
Type=Fixed

[22x22/apps]
Size=22
Context=Applications
Type=Fixed

[16x16/apps]
Size=16
Context=Applications
Type=Fixed
`
	if err := os.MkdirAll(themeRoot, 0o755); err != nil {
		return err
	}
	return os.WriteFile(idx, []byte(minimal), 0o644)
}

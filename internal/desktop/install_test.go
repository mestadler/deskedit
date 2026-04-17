package desktop

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateIconName(t *testing.T) {
	cases := []struct {
		in      string
		wantErr bool
	}{
		{"firefox", false},
		{"my-app_1", false},
		{"", true},
		{"foo/bar", true},
		{"foo\\bar", true},
		{".hidden", true},
		{"foo.png", true},
	}
	for _, tc := range cases {
		err := ValidateIconName(tc.in)
		if (err != nil) != tc.wantErr {
			t.Errorf("ValidateIconName(%q) err=%v, wantErr=%v", tc.in, err, tc.wantErr)
		}
	}
}

func TestRoundUpHicolorSize(t *testing.T) {
	cases := []struct {
		w, h int
		want string
	}{
		{16, 16, "16x16"},
		{17, 17, "22x22"},
		{48, 48, "48x48"},
		{49, 33, "64x64"},
		{65, 65, "96x96"},
		{100, 50, "128x128"},
		{200, 200, "256x256"},
		{300, 300, "256x256"}, // above all standard sizes — capped
		{48, 64, "64x64"},     // mixed dims — take the larger
	}
	for _, tc := range cases {
		got := roundUpHicolorSize(tc.w, tc.h)
		if got != tc.want {
			t.Errorf("roundUpHicolorSize(%d,%d) = %q; want %q",
				tc.w, tc.h, got, tc.want)
		}
	}
}

// makeTestPNG writes a solid-colour w x h PNG to path.
func makeTestPNG(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	red := color.RGBA{R: 0xff, A: 0xff}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, red)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestInstallIcon_SmallPNG(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)

	src := filepath.Join(tmp, "in.png")
	makeTestPNG(t, src, 48, 48)

	res, err := InstallIcon(IconInstallRequest{SourcePath: src, Name: "testapp"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Resized {
		t.Error("48x48 should not be resized")
	}
	if res.SizeDir != "48x48" {
		t.Errorf("SizeDir = %q, want 48x48", res.SizeDir)
	}
	if _, err := os.Stat(res.InstalledPath); err != nil {
		t.Errorf("installed icon missing: %v", err)
	}
}

func TestInstallIcon_LargePNG_IsResized(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)

	src := filepath.Join(tmp, "huge.png")
	makeTestPNG(t, src, 1024, 768)

	res, err := InstallIcon(IconInstallRequest{SourcePath: src, Name: "bigapp"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Resized {
		t.Error("1024x768 should be resized")
	}
	if res.SizeDir != "256x256" {
		t.Errorf("SizeDir = %q, want 256x256", res.SizeDir)
	}
	// Verify the installed PNG really is at most 256 in the larger dim.
	f, err := os.Open(res.InstalledPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	cfg, err := png.DecodeConfig(f)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width > 256 || cfg.Height > 256 {
		t.Errorf("resized icon exceeds 256: got %dx%d", cfg.Width, cfg.Height)
	}
	// And the aspect ratio should be roughly preserved (1024/768 ≈ 1.333)
	ratio := float64(cfg.Width) / float64(cfg.Height)
	if ratio < 1.30 || ratio > 1.37 {
		t.Errorf("aspect ratio not preserved: %dx%d (ratio %.3f)",
			cfg.Width, cfg.Height, ratio)
	}
}

func TestInstallIcon_RejectsBadName(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)
	src := filepath.Join(tmp, "in.png")
	makeTestPNG(t, src, 48, 48)

	_, err := InstallIcon(IconInstallRequest{SourcePath: src, Name: "foo/bar"})
	if err == nil {
		t.Error("expected error for name with slash")
	}
}

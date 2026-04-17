package gpu

import "testing"

func TestWrapUnwrapIdempotent(t *testing.T) {
	cases := []struct {
		name string
		in   string
		mode Mode
		want string
	}{
		{
			name: "plain to DRI_PRIME",
			in:   "firefox %u",
			mode: ModeDRIPrime,
			want: "env DRI_PRIME=1 firefox %u",
		},
		{
			name: "plain to switcheroo",
			in:   "blender",
			mode: ModeSwitcheroo,
			want: "switcherooctl launch blender",
		},
		{
			name: "switch NVIDIA -> DRI",
			in:   "env __NV_PRIME_RENDER_OFFLOAD=1 __GLX_VENDOR_LIBRARY_NAME=nvidia blender",
			mode: ModeDRIPrime,
			want: "env DRI_PRIME=1 blender",
		},
		{
			name: "remove offload",
			in:   "switcherooctl launch firefox %u",
			mode: ModeNone,
			want: "firefox %u",
		},
		{
			name: "prime-run unwrapped",
			in:   "prime-run steam",
			mode: ModeNone,
			want: "steam",
		},
		{
			name: "double wrap prevented",
			in:   "env DRI_PRIME=1 firefox",
			mode: ModeDRIPrime,
			want: "env DRI_PRIME=1 firefox",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Wrap(tc.in, tc.mode)
			if got != tc.want {
				t.Errorf("Wrap(%q, %v) = %q; want %q", tc.in, tc.mode, got, tc.want)
			}
		})
	}
}

func TestDetectMode(t *testing.T) {
	cases := map[string]Mode{
		"firefox":                            ModeNone,
		"switcherooctl launch firefox":       ModeSwitcheroo,
		"prime-run firefox":                  ModeNVIDIA,
		"env DRI_PRIME=1 firefox":            ModeDRIPrime,
		"env __NV_PRIME_RENDER_OFFLOAD=1 fx": ModeNVIDIA,
	}
	for in, want := range cases {
		if got := DetectMode(in); got != want {
			t.Errorf("DetectMode(%q) = %v; want %v", in, got, want)
		}
	}
}

func TestUnwrapMixedPrefixLeavesRealCommand(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "switcheroo then env",
			in:   "switcherooctl launch env FOO=bar app --x",
			want: "env FOO=bar app --x",
		},
		{
			name: "plain env vars without env keyword",
			in:   "DRI_PRIME=1 __GLX_VENDOR_LIBRARY_NAME=nvidia app %u",
			want: "app %u",
		},
		{
			name: "leading spaces preserved by trim then unwrap",
			in:   "   env DRI_PRIME=1 app",
			want: "app",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Unwrap(tc.in); got != tc.want {
				t.Fatalf("Unwrap(%q) = %q; want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestWrapModeSwitchChainDoesNotStackPrefixes(t *testing.T) {
	cmd := "firefox --profile test"

	cmd = Wrap(cmd, ModeSwitcheroo)
	if cmd != "switcherooctl launch firefox --profile test" {
		t.Fatalf("after switcheroo = %q", cmd)
	}

	cmd = Wrap(cmd, ModeNVIDIA)
	wantNVIDIA := "env __NV_PRIME_RENDER_OFFLOAD=1 __GLX_VENDOR_LIBRARY_NAME=nvidia __VK_LAYER_NV_optimus=NVIDIA_only firefox --profile test"
	if cmd != wantNVIDIA {
		t.Fatalf("after nvidia = %q; want %q", cmd, wantNVIDIA)
	}

	cmd = Wrap(cmd, ModeDRIPrime)
	if cmd != "env DRI_PRIME=1 firefox --profile test" {
		t.Fatalf("after dri_prime = %q", cmd)
	}

	cmd = Wrap(cmd, ModeNone)
	if cmd != "firefox --profile test" {
		t.Fatalf("after none = %q", cmd)
	}
}

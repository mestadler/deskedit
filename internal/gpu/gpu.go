// Package gpu handles detection of available GPU offload mechanisms and
// transforms .desktop Exec lines to launch applications on the dGPU.
//
// Three mechanisms are supported, tried in this order of preference:
//
//  1. switcherooctl (logind-integrated, works on GNOME and modern setups):
//     prefix: "switcherooctl launch "
//
//  2. NVIDIA PRIME render offload:
//     env prefix: "env __NV_PRIME_RENDER_OFFLOAD=1 __GLX_VENDOR_LIBRARY_NAME=nvidia "
//     (also VK_LAYER_NV_optimus=NVIDIA_only for Vulkan)
//
//  3. Mesa DRI_PRIME (AMD/Intel hybrid):
//     env prefix: "env DRI_PRIME=1 "
//
// The user selects which they want; the tool wraps/unwraps the Exec value
// idempotently so repeated toggles don't stack prefixes.
package gpu

import (
	"os/exec"
	"regexp"
	"strings"
)

type Mode string

const (
	ModeNone       Mode = "none"
	ModeSwitcheroo Mode = "switcheroo"
	ModeNVIDIA     Mode = "nvidia"
	ModeDRIPrime   Mode = "dri_prime"
)

// Capabilities describes which mechanisms are available on this host.
type Capabilities struct {
	Switcheroo bool
	NVIDIA     bool
	DRIPrime   bool
}

// Detect inspects the running system for available offload mechanisms.
// This is heuristic: presence of a binary or kernel module is enough.
func Detect() Capabilities {
	var c Capabilities

	// switcherooctl — part of switcheroo-control, common on GNOME.
	if _, err := exec.LookPath("switcherooctl"); err == nil {
		c.Switcheroo = true
	}

	// NVIDIA offload: prime-run wrapper exists, or nvidia module loaded.
	if _, err := exec.LookPath("prime-run"); err == nil {
		c.NVIDIA = true
	}
	if fileContains("/proc/modules", "nvidia ") {
		c.NVIDIA = true
	}

	// DRI_PRIME: works whenever there are multiple DRI devices.
	// Cheapest heuristic: /dev/dri/card1 exists (card0 is always primary).
	if statExists("/dev/dri/card1") {
		c.DRIPrime = true
	}
	// Also true on AMD-only hybrid systems; always safe to expose on Mesa.

	return c
}

// Best returns the highest-priority available mode, or ModeNone.
func (c Capabilities) Best() Mode {
	switch {
	case c.Switcheroo:
		return ModeSwitcheroo
	case c.NVIDIA:
		return ModeNVIDIA
	case c.DRIPrime:
		return ModeDRIPrime
	}
	return ModeNone
}

// Prefixes recognised by the Exec transformer. The order matters for
// Unwrap: longer/more specific patterns must come first.
var knownPrefixes = []*regexp.Regexp{
	regexp.MustCompile(`^switcherooctl\s+launch\s+`),
	regexp.MustCompile(`^prime-run\s+`),
	regexp.MustCompile(`^env\s+(?:[A-Za-z_][A-Za-z0-9_]*=\S+\s+)+`),
	regexp.MustCompile(`^(?:[A-Za-z_][A-Za-z0-9_]*=\S+\s+)+`), // env vars without explicit "env "
}

// Unwrap removes any known GPU-offload prefix from an Exec line, returning
// the bare command. Idempotent on already-clean input.
func Unwrap(exec string) string {
	s := strings.TrimLeft(exec, " \t")
	for _, re := range knownPrefixes {
		if loc := re.FindStringIndex(s); loc != nil && loc[0] == 0 {
			s = s[loc[1]:]
			// Only strip one layer — don't chew through the real command.
			break
		}
	}
	return s
}

// Wrap prepends the offload prefix for the given mode to a (possibly already
// wrapped) Exec line. Always unwraps first for idempotency.
func Wrap(exec string, mode Mode) string {
	base := Unwrap(exec)
	switch mode {
	case ModeSwitcheroo:
		return "switcherooctl launch " + base
	case ModeNVIDIA:
		// Use env-based form rather than prime-run: no external dep, and the
		// vars are visible to the user in the .desktop file.
		return "env __NV_PRIME_RENDER_OFFLOAD=1 __GLX_VENDOR_LIBRARY_NAME=nvidia " +
			"__VK_LAYER_NV_optimus=NVIDIA_only " + base
	case ModeDRIPrime:
		return "env DRI_PRIME=1 " + base
	case ModeNone:
		return base
	}
	return base
}

// DetectMode inspects an Exec line and returns which offload mode (if any)
// it currently uses. Useful for showing the current state in the UI.
func DetectMode(exec string) Mode {
	s := strings.TrimLeft(exec, " \t")
	switch {
	case strings.HasPrefix(s, "switcherooctl "):
		return ModeSwitcheroo
	case strings.HasPrefix(s, "prime-run "):
		return ModeNVIDIA
	case strings.Contains(s, "__NV_PRIME_RENDER_OFFLOAD=1"):
		return ModeNVIDIA
	case strings.Contains(s, "DRI_PRIME=1"):
		return ModeDRIPrime
	}
	return ModeNone
}

// ---- helpers ---------------------------------------------------------------

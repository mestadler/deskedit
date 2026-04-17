// deskedit is a TUI for editing freedesktop.org .desktop entries.
//
// Features:
//   - Browse user and system .desktop files (XDG spec compliant).
//   - Edit Name, Exec, and Icon fields with preservation of comments/order.
//   - Toggle GPU offload via switcherooctl, NVIDIA PRIME, or DRI_PRIME.
//   - System files are edited via user-level overrides.
//
// Keybindings are shown on-screen; in brief:
//
//	list:    enter=edit  /=filter  q=quit
//	editor:  tab/shift-tab=navigate  ctrl+s=save  ctrl+i=icon picker  esc=cancel
//	picker:  enter=accept  esc=cancel  /=filter
package main

import (
	"fmt"
	"os"

	"github.com/mestadler/deskedit/internal/ui"
)

func main() {
	if err := ui.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "deskedit:", err)
		os.Exit(1)
	}
}

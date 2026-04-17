package ui

import "github.com/charmbracelet/bubbles/list"

func newPlainDelegate() list.DefaultDelegate {
	delegate := list.NewDefaultDelegate()
	delegate.SetSpacing(0)
	delegate.ShowDescription = false
	return delegate
}

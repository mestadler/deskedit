package ui

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type installModel struct {
	pathInput     textinput.Model
	nameInput     textinput.Model
	focus         int
	browser       list.Model
	browserCWD    string
	lastBrowseDir string
}

func (im *installModel) resetForm() {
	pathTI := textinput.New()
	pathTI.Placeholder = "/path/to/icon.png"
	pathTI.CharLimit = 4096
	pathTI.Width = 60
	pathTI.Focus()

	nameTI := textinput.New()
	nameTI.Placeholder = "icon name (no extension)"
	nameTI.CharLimit = 128
	nameTI.Width = 40

	im.pathInput = pathTI
	im.nameInput = nameTI
	im.focus = 0
}

func (im *installModel) updateFocusedInput(msg tea.KeyMsg) tea.Cmd {
	var cmd tea.Cmd
	if im.focus == 0 {
		im.pathInput, cmd = im.pathInput.Update(msg)
	} else {
		im.nameInput, cmd = im.nameInput.Update(msg)
	}
	return cmd
}

func (im *installModel) cycleFocus(delta int) {
	if im.focus == 0 {
		im.pathInput.Blur()
	} else {
		im.nameInput.Blur()
	}
	im.focus = (im.focus + delta + 2) % 2
	if im.focus == 0 {
		im.pathInput.Focus()
	} else {
		im.nameInput.Focus()
	}
}

func (im *installModel) browseStartDir() string {
	if strings.TrimSpace(im.lastBrowseDir) != "" {
		return im.lastBrowseDir
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "."
	}
	return home
}

func (im *installModel) openBrowse(start string, width, height int) error {
	items, err := listDir(start)
	if err != nil {
		return err
	}
	delegate := newPlainDelegate()
	l := list.New(items, delegate, width-2, height-6)
	l.Title = start
	l.SetFilteringEnabled(true)
	im.browser = l
	im.browserCWD = start
	im.lastBrowseDir = start
	return nil
}

func (im *installModel) enterDirectory(path string) error {
	items, err := listDir(path)
	if err != nil {
		return err
	}
	im.browser.SetItems(items)
	im.browser.Title = path
	im.browserCWD = path
	im.lastBrowseDir = path
	im.browser.ResetSelected()
	return nil
}

func (im *installModel) selectFile(path string) {
	im.pathInput.SetValue(path)
	im.lastBrowseDir = filepath.Dir(path)
	if strings.TrimSpace(im.nameInput.Value()) == "" {
		base := filepath.Base(path)
		base = strings.TrimSuffix(base, filepath.Ext(base))
		im.nameInput.SetValue(base)
	}
	im.focus = 0
	im.cycleFocus(+1)
}

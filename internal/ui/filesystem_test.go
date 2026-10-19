package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestFilesystemChoiceOnDiskScreen(t *testing.T) {
	m := testModel()
	m.Update(safeEnvironment())
	m.screen = disks
	press(m, tea.KeyRight)
	if m.config.FilesystemName() != "ext4" || !strings.Contains(ansi.Strip(m.diskPanel(60, 20)), "(*) ext4") {
		t.Fatal("ext4 option not selected")
	}
	press(m, tea.KeyEnter)
	if m.screen != account || m.config.FilesystemName() != "ext4" {
		t.Fatal("disk confirmation lost filesystem")
	}
	press(m, tea.KeyEsc)
	press(m, tea.KeySpace)
	if m.config.FilesystemName() != "btrfs" {
		t.Fatal("filesystem toggle did not restore Btrfs")
	}
}

package ui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"void-installer/internal/installer"
)

func TestPackageSearchAndSelectionPersistence(t *testing.T) {
	m := testModel()
	enterAccount(m)
	m.screen = utilities
	m.packages = []installer.Package{
		{Name: "firefox", Version: "1_1", Description: "Web browser"},
		{Name: "ripgrep", Version: "1_1", Description: "Fast text search"},
	}
	m.setFocus(14)
	textKey(m, "browser")
	if packages := m.filteredPackages(); len(packages) != 1 || packages[0].Name != "firefox" {
		t.Fatal("description search failed", packages)
	}
	press(m, tea.KeySpace)
	m.packageFilter.SetValue("")
	m.selectedUtility = 0
	textKey(m, "RIPGREP")
	press(m, tea.KeySpace)
	if len(m.config.Utilities) != 2 {
		t.Fatal("filter changes lost selected packages", m.config.Utilities)
	}
	press(m, tea.KeyTab)
	press(m, tea.KeyTab)
	press(m, tea.KeyTab)
	press(m, tea.KeyTab)
	if m.focus != 14 || !m.packageFilter.Focused() {
		t.Fatal("return from Continue did not focus the filter")
	}
	m.packageFilter.SetValue("nothing-matches")
	m.selectedUtility = 0
	press(m, tea.KeyDown)
	press(m, tea.KeySpace)
	if !strings.Contains(ansi.Strip(m.body(60, 20)), "No matching packages") || len(m.config.Utilities) != 2 {
		t.Fatal("empty result changed the selection or crashed")
	}
	press(m, tea.KeyEnter)
	if m.screen != review {
		t.Fatal("search filter must not discard selections on Continue")
	}
}

func TestPackageLoadingErrorsAndMirrorChanges(t *testing.T) {
	m := testModel()
	m.screen = utilities
	m.packagesRoot = ""
	if cmd := m.openPackages(); cmd == nil || !m.packagesLoading {
		t.Fatal("package list must load asynchronously")
	}
	press(m, tea.KeyEnter)
	if m.screen != utilities {
		t.Fatal("must wait for package list validation")
	}
	m.Update(packagesMsg{root: m.config.Mirror, err: errors.New("mirror unavailable")})
	if m.packagesLoading || !strings.Contains(m.body(60, 20), "Ctrl+R") {
		t.Fatal("failed package fetch must allow retry")
	}
	press(m, tea.KeyEnter)
	if m.screen != utilities {
		t.Fatal("failed package fetch must not silently use an old list")
	}
	m.Update(packagesMsg{root: m.config.Mirror, packages: []installer.Package{{Name: "vim"}}})
	m.config.Utilities = []string{"vim", "removed-package"}
	m.Update(packagesMsg{root: m.config.Mirror, packages: []installer.Package{{Name: "vim"}}})
	if len(m.config.Utilities) != 1 || m.config.Utilities[0] != "vim" || m.err == "" {
		t.Fatal("refresh must remove unavailable selections with an explanation")
	}
	m.config.Utilities = []string{"vim"}
	m.Update(repositoryMsg{root: "https://mirror.example/void"})
	if m.packagesRoot != "" || len(m.packages) != 0 || len(m.config.Utilities) != 0 {
		t.Fatal("mirror change must invalidate the old list and selections")
	}
	m.Update(packagesMsg{root: installer.DefaultMirror, packages: []installer.Package{{Name: "stale"}}})
	if len(m.packages) != 0 {
		t.Fatal("stale package response changed the new mirror")
	}
}

func TestSelectedOnlyCheckboxAndFocusOrder(t *testing.T) {
	m := testModel()
	m.screen = utilities
	m.packages = []installer.Package{{Name: "firefox", Description: "browser"}, {Name: "vim", Description: "editor"}, {Name: "zathura", Description: "PDF reader"}}
	m.config.Utilities = []string{"vim", "zathura"}
	m.focusPackages(0)
	press(m, tea.KeyTab)
	if m.packageFocus != 1 {
		t.Fatal("Tab must focus the selected-only checkbox")
	}
	press(m, tea.KeySpace)
	if !m.selectedOnly || len(m.filteredPackages()) != 2 {
		t.Fatal("selected-only filter failed")
	}
	press(m, tea.KeyTab)
	if m.packageFocus != 2 {
		t.Fatal("Tab must focus the package list")
	}
	m.selectedUtility = 1
	press(m, tea.KeySpace)
	if len(m.filteredPackages()) != 1 || m.selectedUtility != 0 {
		t.Fatal("removed selection left a stale index")
	}
	press(m, tea.KeySpace)
	if len(m.filteredPackages()) != 0 {
		t.Fatal("last selected package was not removed")
	}
	m.View()
	press(m, tea.KeyTab)
	if m.packageFocus != 3 || !m.continueFocused {
		t.Fatal("Tab must focus Continue")
	}
	press(m, tea.KeyTab)
	if m.packageFocus != 0 || !m.packageFilter.Focused() {
		t.Fatal("Tab must return to search")
	}
	m.config.Utilities = []string{"vim"}
	textKey(m, "reader")
	if len(m.filteredPackages()) != 0 {
		t.Fatal("selected-only search included an unselected description")
	}
	press(m, tea.KeyTab)
	press(m, tea.KeySpace)
	if m.selectedOnly || len(m.filteredPackages()) != 1 {
		t.Fatal("disabling selected-only lost the search filter")
	}
}

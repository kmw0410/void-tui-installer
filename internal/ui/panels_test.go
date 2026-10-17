package ui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"void-installer/internal/installer"
)

func TestPreparationStagesAndFailure(t *testing.T) {
	m := testModel()
	updates := make(chan tea.Msg, 1)
	for _, state := range []preparationMsg{{"environment", "completed", updates}, {"tools", "completed", updates}, {"xbps", "skipped", updates}, {"install", "running", updates}} {
		m.Update(state)
	}
	view := ansi.Strip(m.View().Content)
	for _, text := range []string{"[✓] Verify Live environment", "[-] Update XBPS (not needed)", "Install missing tools", "Installing: jq"} {
		if !strings.Contains(view, text) {
			t.Fatalf("preparation state missing: %s", text)
		}
	}
	m.Update(environmentMsg{err: errors.New("Tool installation failed")})
	if m.preparation["install"] != "failed" || !strings.Contains(ansi.Strip(m.View().Content), "[!] Install missing tools") {
		t.Fatal("failed stage must remain visible")
	}
	textKey(m, "r")
	if len(m.preparation) != 0 || !m.checking {
		t.Fatal("retry must clear stale preparation state")
	}
}

func TestInlineAccountErrorsAndContinue(t *testing.T) {
	m := testModel()
	m.screen = account
	m.fields[0].SetValue("root")
	m.fields[3].SetValue("void")
	m.fields[1].SetValue("valid-password")
	m.fields[2].SetValue("valid-password")
	m.setFocus(3)
	press(m, tea.KeyEnter)
	if m.screen != account || m.focus != 13 {
		t.Fatal("last input must focus Continue without submitting")
	}
	press(m, tea.KeyEnter)
	if m.focus != 0 || m.fieldErrors[0] == "" {
		t.Fatal("Continue must focus the invalid input")
	}
	view := ansi.Strip(m.body(60, 20))
	if strings.Index(view, "! Username") < strings.Index(view, "Username") || strings.Index(view, "! Username") > strings.Index(view, "Password ") {
		t.Fatal("error must appear directly below username")
	}
	m.fields[0].SetValue("alic")
	textKey(m, "e")
	if m.fieldErrors[0] != "" {
		t.Fatal("editing must clear that field error")
	}
}

func TestInlineNetworkErrors(t *testing.T) {
	m := testModel()
	enterAccount(m)
	press(m, tea.KeySpace)
	m.fields[4].SetValue("192.0.2.10/24")
	m.fields[5].SetValue("invalid")
	m.fields[6].SetValue("1.1.1.1")
	m.setFocus(7)
	press(m, tea.KeyEnter)
	if m.screen != network || m.focus != 5 || m.fieldErrors[5] == "" {
		t.Fatal("gateway error must focus gateway")
	}
	view := ansi.Strip(m.body(60, 20))
	if !strings.Contains(view, "! Gateway") || strings.Index(view, "! Gateway") > strings.Index(view, "DNS server") {
		t.Fatal("gateway error must precede next input")
	}
}

func TestContinuePositionAndBlockedDiskDetails(t *testing.T) {
	m := testModel()
	enterAccount(m)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	row := -1
	for _, screen := range []screen{mirror, disks, account, timezone, network, desktop, utilities} {
		m.screen = screen
		view := ansi.Strip(m.View().Content)
		found := -1
		for i, line := range strings.Split(view, "\n") {
			if strings.Contains(line, "[ Continue ]") {
				found = i
			}
		}
		if found < 0 || (row >= 0 && row != found) {
			t.Fatalf("Continue must occupy the same row: screen %d row %d expected %d", screen, found, row)
		}
		row = found
	}
	m.disks = []installer.Disk{{Path: "/dev/vda", Model: "VirtIO Disk", Size: "64 GiB", Reason: "Stable serial or WWN is required"}}
	m.selectedDisk = 0
	m.screen = disks
	view := ansi.Strip(m.View().Content)
	for _, text := range []string{"[Blocked]", "Serial", "WWN", "Not available", "Stable serial or WWN", "unique disk serial", "[ Continue ] (blocked)"} {
		if !strings.Contains(view, text) {
			t.Fatalf("disk detail missing: %s", text)
		}
	}
	press(m, tea.KeyEnter)
	if m.screen != disks {
		t.Fatal("blocked disk must never advance")
	}
	m.disks[0].Serial = "VOIDTEST001"
	m.disks[0].WWN = "test-wwn"
	if !strings.Contains(m.diskPanel(60, 20), "VOIDTEST001") {
		t.Fatal("disk serial must be visible")
	}
}

func TestFixedDescriptionsAndCompactReview(t *testing.T) {
	m := testModel()
	enterAccount(m)
	m.screen = utilities
	m.selectedUtility = 0
	first := ansi.Strip(m.body(60, 20))
	m.selectedUtility = len(installer.UtilityPackages()) - 1
	last := ansi.Strip(m.body(60, 20))
	lineOf := func(s string) int {
		for i, line := range strings.Split(s, "\n") {
			if strings.Contains(line, "selected.") {
				return i
			}
		}
		return -1
	}
	if lineOf(first) < 0 || lineOf(first) != lineOf(last) {
		t.Fatal("description must not move with list position")
	}
	m.screen = review
	m.config.Desktop = "kde"
	m.config.Graphics = "nvidia"
	m.config.Utilities = []string{"firefox", "kitty"}
	review := ansi.Strip(m.reviewBody(75, 50))
	details := strings.Index(review, "Details")
	for _, text := range []string{"Target", "Account", "System", "KDE Plasma", "NVIDIA", "2 selected"} {
		if at := strings.Index(review, text); at < 0 || at > details {
			t.Fatalf("important selection must precede details: %s", text)
		}
	}
	if !strings.Contains(review, "bluetoothd, tlp") {
		t.Fatal("mandatory services must remain in details")
	}
}

func TestRecentLogsVisibleAtMinimumSize(t *testing.T) {
	m := testModel()
	m.screen = installing
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m.appendLog("Recent diagnostic from engine")
	view := ansi.Strip(m.View().Content)
	if !strings.Contains(view, "Recent logs") || !strings.Contains(view, "Recent diagnostic from engine") {
		t.Fatal("recent log must be visible without F2 at 80x24")
	}
	if lipgloss.Width(view) > 80 || lipgloss.Height(view) > 24 {
		t.Fatal("log preview must fit minimum terminal")
	}
	press(m, tea.KeyF2)
	if !m.showLogs {
		t.Fatal("full log access must remain available")
	}
}

func TestDiskInspectionCannotBypassFailedEnvironment(t *testing.T) {
	m := testModel()
	env := safeEnvironment()
	env.interfaces = nil
	m.Update(env)
	textKey(m, "d")
	if m.screen != disks || !m.diskInspection {
		t.Fatal("failed environment may only inspect disks")
	}
	press(m, tea.KeyEnter)
	press(m, tea.KeyTab)
	press(m, tea.KeyEnter)
	if m.screen != disks {
		t.Fatal("inspection must never advance even for a safe disk")
	}
	press(m, tea.KeyEsc)
	if m.screen != welcome || m.ready {
		t.Fatal("inspection must return to failed Welcome")
	}
}

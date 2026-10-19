package ui

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"void-installer/internal/installer"
)

func reviewModel() *model {
	m := testModel()
	enterAccount(m)
	press(m, tea.KeyEnter)
	press(m, tea.KeyEnter)
	press(m, tea.KeyEnter)
	return m
}

func TestReviewCheckGateAndStaleResults(t *testing.T) {
	m := reviewModel()
	m.fields[8].SetValue("ERASE /dev/vda")
	if m.erasureConfirmed() {
		t.Fatal("pending checks enabled installation")
	}
	finishReviewChecks(m)
	if !m.erasureConfirmed() {
		t.Fatal("passed checks failed to enable exact consent")
	}
	m.config.Hostname = "changed"
	if m.erasureConfirmed() {
		t.Fatal("changed configuration reused passed checks")
	}
	_ = m.checkReview()
	id := m.reviewCheckID
	finishReviewChecks(m)
	m.Update(reviewCheckedMsg{id: id - 1, key: configKey(m.config), err: errors.New("stale")})
	if m.reviewCheckError != "" {
		t.Fatal("stale check overwrote the current report")
	}
	m.Update(reviewCheckedMsg{id: id, key: configKey(m.config), err: errors.New("capacity failure")})
	if m.reviewChecksPassed() || !strings.Contains(m.checksSummary(), "capacity failure") {
		t.Fatal("failed checks enabled installation")
	}
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	v := m.View()
	if lipgloss.Width(v.Content) > 80 || lipgloss.Height(v.Content) > 24 {
		t.Fatal("check result dialog exceeds terminal")
	}
}

func TestSettingsSaveDialogAndLoadReset(t *testing.T) {
	m := reviewModel()
	press(m, tea.KeyF5)
	if m.settingsMode != "save" {
		t.Fatal("save dialog missing")
	}
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	v := m.View()
	if lipgloss.Width(v.Content) > 80 || lipgloss.Height(v.Content) > 24 {
		t.Fatal("settings dialog exceeds minimum terminal")
	}
	m.settingsPath.SetValue(filepath.Join(t.TempDir(), "saved.json"))
	cmd := press(m, tea.KeyEnter)
	if cmd == nil || !m.settingsBusy {
		t.Fatal("save was not scheduled")
	}
	m.Update(cmd())
	if m.settingsMode != "" || m.settingsNotice == "" {
		t.Fatal("save completion missing", m.err)
	}
	c, err := installer.LoadSettings(m.settingsPath.Value())
	if err != nil {
		t.Fatal(err)
	}
	c.Utilities = []string{"vim"}
	m.fields[8].SetValue("ERASE /dev/vda")
	msg := settingsMsg{mode: "load", config: c, disks: safeEnvironment().disks, interfaces: []string{"enp1s0"}}
	m.Update(msg)
	if m.screen != mirror || m.selectedDisk != 0 || m.fields[1].Value() != "" || m.fields[2].Value() != "" || m.fields[8].Value() != "" || m.config.Confirmation != "" || m.reviewChecksPassed() {
		t.Fatal("load retained secrets, consent or stale checks")
	}
	if m.fields[0].Value() != "alice" || len(m.config.Utilities) != 1 {
		t.Fatal("public settings lost")
	}
	msg.disks[0].Identity = "changed"
	m.Update(msg)
	if m.selectedDisk != -1 || m.config.Disk != "" {
		t.Fatal("changed disk identity was trusted")
	}
}

package ui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"void-installer/internal/installer"
)

func testModel() *model {
	m := New().(*model)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return m
}
func press(m *model, code rune) tea.Cmd { _, cmd := m.Update(tea.KeyPressMsg{Code: code}); return cmd }
func textKey(m *model, s string) tea.Cmd {
	_, cmd := m.Update(tea.KeyPressMsg{Code: []rune(s)[0], Text: s})
	return cmd
}
func safeEnvironment() environmentMsg {
	return environmentMsg{disks: []installer.Disk{{Path: "/dev/vda", Model: "Virtual Disk", Size: "32 GiB", Type: "disk", Identity: "254:0 34359738368 512 TEST", Safe: true}}, interfaces: []string{"enp1s0", "enp2s0"}}
}
func enterAccount(m *model) {
	m.Update(safeEnvironment())
	press(m, tea.KeyEnter)
	m.Update(repositoryMsg{root: installer.DefaultMirror})
	press(m, tea.KeyEnter)
	m.fields[0].SetValue("alice")
	m.fields[1].SetValue("Secret-valid-123")
	m.fields[2].SetValue("Secret-valid-123")
	m.setFocus(3)
	press(m, tea.KeyEnter)
	press(m, tea.KeyEnter)
	press(m, tea.KeyEnter)
}
func TestTimezoneFilteringSelectionAndBack(t *testing.T) {
	m := testModel()
	enterAccount(m)
	press(m, tea.KeyEsc)
	if m.screen != timezone || m.focus != 10 || m.config.Timezone != "UTC" {
		t.Fatal("network must return to timezone selection with UTC default")
	}
	textKey(m, "Seoul")
	if zones := m.filteredTimezones(); len(zones) != 1 || zones[0] != "Asia/Seoul" {
		t.Fatal("city filter failed", zones)
	}
	press(m, tea.KeyEnter)
	if m.screen != network || m.config.Timezone != "Asia/Seoul" {
		t.Fatal("timezone selection was lost")
	}
	press(m, tea.KeyEsc)
	press(m, tea.KeyEnter)
	if m.config.Timezone != "Asia/Seoul" {
		t.Fatal("back navigation changed the selected timezone")
	}
	press(m, tea.KeyEsc)
	m.fields[10].SetValue("no-such-timezone")
	m.selectedTimezone = 0
	press(m, tea.KeyEnter)
	if m.screen != timezone || m.err == "" {
		t.Fatal("empty timezone results must block navigation")
	}
	press(m, tea.KeyDown)
	m.View()
}
func TestEnvironmentFailureBlocksNavigation(t *testing.T) {
	m := testModel()
	m.Update(environmentMsg{err: errors.New("UEFI boot is required")})
	press(m, tea.KeyEnter)
	if m.screen != welcome || m.ready {
		t.Fatal("failed preflight must block navigation")
	}
	m.Update(environmentMsg{disks: []installer.Disk{{Path: "/dev/vda", Reason: "Mounted"}}, interfaces: []string{"eth0"}})
	press(m, tea.KeyEnter)
	if m.screen != welcome || m.ready {
		t.Fatal("unsafe disk must block navigation")
	}
}

func TestMirrorSelectionPrecedesRepositoryAccess(t *testing.T) {
	m := testModel()
	m.Update(safeEnvironment())
	press(m, tea.KeyEnter)
	if m.screen != mirror {
		t.Fatal("mirror selection must precede repository checking")
	}
	m.selectedMirror = len(installer.Mirrors)
	m.fields[9].SetValue("https://mirror.example/voidlinux/current")
	if cmd := press(m, tea.KeyEnter); cmd != nil || m.checking {
		t.Fatal("repository suffix must be rejected without network access")
	}
	m.fields[9].SetValue("https://mirror.example/voidlinux/")
	if cmd := press(m, tea.KeyEnter); cmd == nil || !m.checking {
		t.Fatal("mirror must be checked asynchronously")
	}
	press(m, tea.KeyEsc)
	if m.screen != mirror {
		t.Fatal("must wait for the active repository check")
	}
	m.Update(repositoryMsg{root: "https://mirror.example/voidlinux", err: errors.New("Mirror unreachable")})
	if m.screen != mirror || m.repositoryReady {
		t.Fatal("failed mirror must not advance")
	}
	m.Update(repositoryMsg{root: "https://mirror.example/voidlinux"})
	if m.screen != disks || m.config.Mirror != "https://mirror.example/voidlinux" {
		t.Fatal("selected mirror was lost")
	}
}

func TestPresetDoesNotFillCustomMirrorInput(t *testing.T) {
	m := testModel()
	if err := m.SetMirror(installer.Mirrors[1].URL); err != nil {
		t.Fatal(err)
	}
	if m.selectedMirror != 1 || m.fields[9].Value() != "" {
		t.Fatal("preset must not leave a hidden custom input value")
	}
	if err := m.SetMirror("https://mirror.example/voidlinux/"); err != nil {
		t.Fatal(err)
	}
	if m.selectedMirror != len(installer.Mirrors) || m.fields[9].Value() != "https://mirror.example/voidlinux" {
		t.Fatal("custom CLI mirror was lost")
	}
}

func TestUnavailableDiskExplainsMissingSerial(t *testing.T) {
	m := testModel()
	m.Update(environmentMsg{disks: []installer.Disk{{Path: "/dev/vda", Reason: "Stable serial or WWN is required"}}, interfaces: []string{"eth0"}})
	if m.ready || !strings.Contains(m.err, "/dev/vda: Stable serial or WWN is required") {
		t.Fatal("blocked virtual disk must display its specific exclusion reason")
	}
	if !strings.Contains(m.View().Content, "Stable serial or WWN") {
		t.Fatal("disk exclusion reason was not rendered")
	}
}
func TestReviewRequiresExactConsentAndStartsOnce(t *testing.T) {
	m := testModel()
	enterAccount(m)
	if m.screen != network {
		t.Fatal("account should advance to network")
	}
	press(m, tea.KeyEnter)
	if m.screen != desktop {
		t.Fatal("DHCP should advance to desktop selection")
	}
	press(m, tea.KeyEnter)
	if m.screen != utilities {
		t.Fatal("desktop should advance to utilities")
	}
	press(m, tea.KeyEnter)
	if m.screen != review {
		t.Fatal("utilities should advance to review")
	}
	if cmd := press(m, tea.KeyEnter); cmd != nil || m.screen != review {
		t.Fatal("empty consent must not start engine")
	}
	m.fields[8].SetValue("ERASE /dev/vdb")
	if cmd := press(m, tea.KeyEnter); cmd != nil || m.screen != review {
		t.Fatal("wrong disk consent must not start engine")
	}
	m.fields[8].SetValue("ERASE /dev/vda")
	if m.reviewFocus != 2 {
		t.Fatal("Enter on the consent field must focus Install")
	}
	if cmd := press(m, tea.KeyEnter); cmd == nil || m.screen != installing || !m.starting {
		t.Fatal("valid consent should schedule engine asynchronously")
	}
	if cmd := press(m, tea.KeyEnter); cmd != nil {
		t.Fatal("install screen must never start a second engine")
	}
	// Never run the returned start command: this test must not touch a disk.
}
func TestStaticNetworkAndDHCPReset(t *testing.T) {
	m := testModel()
	enterAccount(m)
	press(m, tea.KeyRight)
	m.fields[4].SetValue("192.0.2.10/24")
	m.fields[5].SetValue("192.0.2.1")
	m.fields[6].SetValue("1.1.1.1")
	m.setFocus(7)
	press(m, tea.KeyEnter)
	if m.screen != desktop || m.config.Network != "static" {
		t.Fatalf("static config failed: %s", m.err)
	}
	press(m, tea.KeyEsc)
	if m.focus != 4 {
		t.Fatal("back to static network must focus address")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	press(m, tea.KeySpace)
	press(m, tea.KeyEnter)
	if m.screen != desktop || m.config.Network != "dhcp" || m.config.Address != "" || m.config.Interface != "" {
		t.Fatal("DHCP must clear static fields")
	}
}
func TestPasswordsNeverRenderedAndAreCleared(t *testing.T) {
	m := testModel()
	enterAccount(m)
	for _, screen := range []screen{mirror, account, timezone, network, desktop, utilities, review, installing, result} {
		m.screen = screen
		if strings.Contains(m.View().Content, "Secret-valid-123") {
			t.Fatalf("password rendered on screen %d", screen)
		}
	}
	m.Update(startedMsg{err: errors.New("Cannot start engine")})
	if m.fields[1].Value() != "" || m.fields[2].Value() != "" {
		t.Fatal("password retained after start failure")
	}
}
func TestAccountValidation(t *testing.T) {
	m := testModel()
	m.fields[0].SetValue("root")
	m.fields[3].SetValue("void")
	m.fields[1].SetValue("valid-password")
	m.fields[2].SetValue("valid-password")
	if m.validateAccount() == nil {
		t.Fatal("root account accepted")
	}
	if validatePassword("short", "short") == nil {
		t.Fatal("weak password length accepted")
	}
	if validatePassword("valid-password", "different-password") == nil {
		t.Fatal("password mismatch accepted")
	}
}
func TestAbortWaitsForEngineAndLogsKeepUpdating(t *testing.T) {
	m := testModel()
	m.screen = installing
	m.starting = true
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd != nil || !m.abortPrompt || m.screen != installing {
		t.Fatal("Ctrl+C must show confirmation without quitting")
	}
	textKey(m, "y")
	if !m.cancelling || m.screen != installing {
		t.Fatal("abort must wait for engine exit")
	}
	press(m, tea.KeyF2)
	if !m.showLogs {
		t.Fatal("logs must open during cancellation")
	}
	m.Update(doneMsg{err: errors.New("Installation cancelled")})
	if m.screen != result || m.success {
		t.Fatal("cancelled engine must report failure after reaping")
	}
}
func TestProgressFailureAndScrollableLogs(t *testing.T) {
	m := testModel()
	m.screen = installing
	for i := 0; i < 30; i++ {
		m.appendLog("A diagnostic line")
	}
	m.scrollLogs("pgup")
	if m.logOffset == 0 {
		t.Fatal("logs did not scroll")
	}
	m.scrollLogs("end")
	if m.logOffset != 0 {
		t.Fatal("log follow did not resume")
	}
	m.current = "grub"
	m.stages["grub"] = "failed"
	m.Update(doneMsg{err: errors.New("Engine exited with failure")})
	view := m.View().Content
	if !strings.Contains(view, "GRUB") || !strings.Contains(view, "failure") || m.success {
		t.Fatal("failed stage and error must remain visible")
	}
}
func TestResizeAndAlternateScreen(t *testing.T) {
	m := testModel()
	enterAccount(m)
	for _, screen := range []screen{welcome, mirror, disks, account, timezone, network, desktop, utilities, review, installing, result} {
		m.screen = screen
		m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
		v := m.View()
		if !v.AltScreen {
			t.Fatal("alternate screen missing")
		}
		if lipgloss.Width(v.Content) > 80 || lipgloss.Height(v.Content) > 24 {
			t.Fatalf("screen %d exceeds terminal: %dx%d", screen, lipgloss.Width(v.Content), lipgloss.Height(v.Content))
		}
	}
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	if !strings.Contains(m.View().Content, "Terminal too small") {
		t.Fatal("missing minimum size guidance")
	}
}

func TestFinalEventsAreDrainedAfterProcessCompletion(t *testing.T) {
	m := testModel()
	events := make(chan installer.Event)
	close(events)
	m.session = &installer.Session{Events: events}
	m.screen = installing
	m.Update(doneMsg{})
	_, cmd := m.Update(eventMsg{event: installer.Event{Type: "log", Message: "Cleanup finished"}, open: true})
	if cmd == nil {
		t.Fatal("remaining events must be drained")
	}
	msg, ok := cmd().(eventMsg)
	if !ok || msg.open {
		t.Fatal("closed event stream must stop polling")
	}
	if !strings.Contains(m.logView(60, 20), "Cleanup finished") {
		t.Fatal("final diagnostics were lost")
	}
}

func TestShutdownCancelsAndWaitsForOutstandingJobs(t *testing.T) {
	m := testModel()
	if !m.beginJob() {
		t.Fatal("active lifecycle must accept jobs")
	}
	stopped := make(chan struct{})
	go func() { m.Shutdown(); close(stopped) }()
	<-m.ctx.Done()
	select {
	case <-stopped:
		t.Fatal("shutdown returned before the process job finished")
	default:
	}
	m.jobs.Done()
	<-stopped
}

func TestUnscheduledCommandsDoNotBlockShutdown(t *testing.T) {
	m := testModel()
	check := m.checkCommand()
	enterAccount(m)
	press(m, tea.KeyEnter)
	press(m, tea.KeyEnter)
	press(m, tea.KeyEnter)
	m.fields[8].SetValue("ERASE /dev/vda")
	press(m, tea.KeyEnter)
	start := press(m, tea.KeyEnter)
	if start == nil {
		t.Fatal("installation command was not scheduled")
	}
	stopped := make(chan struct{})
	go func() { m.Shutdown(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("commands that never ran blocked shutdown")
	}
	checkResult, ok := check().(environmentMsg)
	if !ok || !errors.Is(checkResult.err, context.Canceled) {
		t.Fatal("late check must be rejected before checking the live environment")
	}
	startResult, ok := start().(startedMsg)
	if !ok || startResult.session != nil || !errors.Is(startResult.err, context.Canceled) {
		t.Fatal("late installation must be rejected before starting the engine")
	}
	if m.beginJob() {
		t.Fatal("stopped lifecycle accepted a late job")
	}
}

func TestSoftwareSelectionAndReviewScrolling(t *testing.T) {
	m := testModel()
	enterAccount(m)
	m.gpus = []installer.GPU{{Address: "0000:01:00.0", Device: "2684"}}
	press(m, tea.KeyEnter)
	press(m, tea.KeyDown)
	press(m, tea.KeySpace)
	press(m, tea.KeyDown)
	press(m, tea.KeyDown)
	press(m, tea.KeySpace)
	press(m, tea.KeyEnter)
	if m.config.Desktop != "kde" || m.config.Graphics != "nvidia" || m.screen != utilities {
		t.Fatal("software choices were lost")
	}
	packages := installer.UtilityPackages()
	for i, p := range packages {
		if p == "firefox" || p == "kitty" || strings.HasPrefix(p, "fcitx5") {
			m.selectedUtility = i
			press(m, tea.KeySpace)
		}
	}
	if len(m.config.Utilities) < 3 {
		t.Fatal("utility checkboxes did not toggle")
	}
	press(m, tea.KeyEnter)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	view := m.View().Content
	if !strings.Contains(view, "ERASE /dev/vda") {
		t.Fatal("confirmation must remain visible on a small terminal")
	}
	press(m, tea.KeyPgDown)
	if !strings.Contains(m.View().Content, "ALL DATA ON") || m.reviewOffset == 0 {
		t.Fatal("review must scroll without hiding consent")
	}
	press(m, tea.KeyEsc)
	if m.screen != utilities || len(m.config.Utilities) < 3 {
		t.Fatal("back navigation lost selections")
	}
}

func TestHeaderBuildVersion(t *testing.T) {
	m := NewWithVersion(context.Background(), "0.4.1-dev")
	defer m.Shutdown()
	for _, width := range []int{80, 100, 160} {
		m.width, m.height = width, 24
		lines := strings.Split(ansi.Strip(m.View().Content), "\n")
		if !strings.Contains(lines[0], "Void Installer") || !strings.Contains(lines[0], "v0.4.1-dev") {
			t.Fatal("title and version must share the first line")
		}
		if lipgloss.Width(lines[0]) != width || !strings.HasSuffix(strings.TrimSpace(lines[0]), "v0.4.1-dev") {
			t.Fatalf("version must align with the right header margin: width=%d want=%d line=%q", lipgloss.Width(lines[0]), width, lines[0])
		}
		if !strings.Contains(lines[1], "─") {
			t.Fatal("header must not wrap above the separator")
		}
	}
}

func TestNVIDIACheckboxIndependentOfDesktop(t *testing.T) {
	m := testModel()
	m.screen = desktop
	m.gpus = []installer.GPU{{Device: "2684"}}
	press(m, tea.KeySpace)
	if m.config.Graphics != "default" {
		t.Fatal("space on desktop must not enable driver")
	}
	for range 2 {
		press(m, tea.KeyDown)
	}
	press(m, tea.KeySpace)
	press(m, tea.KeyDown)
	press(m, tea.KeySpace)
	if m.selectedDesktop != 2 || !strings.Contains(m.View().Content, "[V] Install NVIDIA driver") {
		t.Fatal("driver checkbox must preserve chosen desktop")
	}
	press(m, tea.KeySpace)
	if m.config.Graphics != "default" || !strings.Contains(m.View().Content, "[ ] Install NVIDIA driver") {
		t.Fatal("driver checkbox must toggle off")
	}
	press(m, tea.KeyEnter)
	if m.config.Desktop != "niri" {
		t.Fatal("checkbox focus must preserve desktop on continue")
	}
	m = testModel()
	m.screen = desktop
	for range 4 {
		press(m, tea.KeyDown)
	}
	press(m, tea.KeySpace)
	if m.desktopFocus != 2 || m.config.Graphics != "default" {
		t.Fatal("driver must remain unavailable without NVIDIA hardware")
	}
}

func TestReviewDialogCenteredAndIsolated(t *testing.T) {
	m := testModel()
	enterAccount(m)
	press(m, tea.KeyEnter)
	press(m, tea.KeyEnter)
	press(m, tea.KeyEnter)
	for _, size := range [][2]int{{80, 24}, {100, 30}, {160, 50}} {
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		view := m.View().Content
		if strings.Contains(view, "TUI INSTALLER") || strings.Contains(view, "01 Welcome") || strings.Contains(view, "09 Review") {
			t.Fatal("review must hide application chrome")
		}
		if !strings.Contains(view, "[ Back ]") || !strings.Contains(view, "[ Install ] (disabled)") {
			t.Fatal("review controls missing")
		}
		if lipgloss.Width(view) != size[0] || lipgloss.Height(view) != size[1] {
			t.Fatalf("dialog must fill terminal: %dx%d", lipgloss.Width(view), lipgloss.Height(view))
		}
		// Inspect the plain dialog separately to avoid depending on terminal color support.
		dialog := m.reviewDialog()
		lines := strings.Split(dialog, "\n")
		top, bottom, left, right := -1, -1, -1, -1
		for i, line := range lines {
			if at := strings.Index(line, "┌"); at >= 0 {
				top = i
				left = lipgloss.Width(line[:at])
				right = size[0] - lipgloss.Width(strings.TrimRight(line, " "))
			}
			if strings.Contains(line, "└") {
				bottom = i
			}
		}
		if top < 1 || bottom >= size[1]-1 || abs(top-(size[1]-1-bottom)) > 1 || abs(left-right) > 1 {
			t.Fatalf("dialog must be centered with black margins: %d %d %d %d", top, bottom, left, right)
		}
	}
}

func TestReviewButtonsAndFieldRequireExplicitActivation(t *testing.T) {
	m := testModel()
	enterAccount(m)
	press(m, tea.KeyEnter)
	press(m, tea.KeyEnter)
	press(m, tea.KeyEnter)
	m.fields[8].SetValue("ERASE /dev/vda")
	if cmd := press(m, tea.KeyEnter); cmd != nil || m.screen != review || m.reviewFocus != 2 {
		t.Fatal("field Enter must focus Install without starting")
	}
	m.fields[8].SetValue("ERASE /dev/vdb")
	if cmd := press(m, tea.KeyEnter); cmd != nil || m.screen != review {
		t.Fatal("Install must recheck the exact disk phrase")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	if m.reviewFocus != 1 {
		t.Fatal("Shift+Tab must focus Back")
	}
	press(m, tea.KeyEnter)
	if m.screen != utilities {
		t.Fatal("Back must return without starting")
	}
	press(m, tea.KeyEnter)
	if m.reviewFocus != 0 || m.fields[8].Value() != "" {
		t.Fatal("reopening review must reset consent and focus")
	}
}

func TestAccountTabStaysOnForm(t *testing.T) {
	m := testModel()
	m.screen = account
	m.setFocus(3)
	press(m, tea.KeyTab)
	if m.screen != account || m.focus != 13 {
		t.Fatal("Tab must move focus without submitting")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	if m.focus != 3 {
		t.Fatal("Shift+Tab must move to the previous field")
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

package ui

import (
	"fmt"
	"strings"
	"void-installer/internal/installer"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

var (
	green  = lipgloss.Color("#73C48F")
	white  = lipgloss.Color("#E6E6E6")
	gray   = lipgloss.Color("#999999")
	red    = lipgloss.Color("#FF817D")
	accent = lipgloss.NewStyle().Foreground(green).Bold(true)
	muted  = lipgloss.NewStyle().Foreground(gray)
	danger = lipgloss.NewStyle().Foreground(red)
)

func (m *model) View() tea.View {
	var content string
	if m.width < 80 || m.height < 24 {
		content = fmt.Sprintf("VOID LINUX INSTALLER\n\nTerminal too small: %d x %d.\nResize to at least 80 x 24.\n\nCtrl+C: quit", m.width, m.height)
		if m.screen == installing {
			content = "VOID LINUX INSTALLER\n\nInstallation is still running.\nResize to at least 80 x 24.\nCtrl+C: request cancellation"
		}
	} else if m.settingsMode != "" {
		content = m.settingsDialog()
	} else if m.screen == review {
		content = m.reviewDialog()
	} else {
		header := accent.Render("Void Installer")
		version := muted.Render("v" + m.version)
		header += strings.Repeat(" ", max(1, m.width-4-lipgloss.Width(header)-lipgloss.Width(version))) + version
		header = lipgloss.NewStyle().Padding(0, 2).Render(header)
		sideWidth := 19
		if m.width < 100 {
			sideWidth = 13
		}
		bodyHeight := m.height - 5
		contentWidth := m.width - sideWidth - 5
		side := lipgloss.NewStyle().Width(sideWidth).Height(bodyHeight).PaddingLeft(2).Render(m.navigation())
		panelHeight := bodyHeight
		if m.screen <= utilities {
			panelHeight -= 3
		}
		body := m.body(contentWidth-2, panelHeight)
		if m.err != "" && m.screen != result && len(m.fieldErrors) == 0 {
			lines := strings.Split(lipgloss.NewStyle().Width(contentWidth-4).Render(m.err), "\n")
			limit := min(8, max(2, panelHeight-4))
			if len(lines) > limit {
				lines = append(append(lines[:limit-2], "…"), lines[len(lines)-1])
			}
			body = lipgloss.NewStyle().MaxHeight(max(1, panelHeight-len(lines)-1)).Render(body) + "\n" + danger.Render(strings.Join(lines, "\n"))
		}
		if m.screen <= utilities {
			body = lipgloss.NewStyle().Height(panelHeight).MaxHeight(panelHeight).Render(body) + "\n\n" + m.continueButton()
		}
		body = lipgloss.NewStyle().Width(contentWidth).Height(bodyHeight).MaxHeight(bodyHeight).PaddingLeft(2).Render(body)
		line := muted.Render(strings.Repeat("─", m.width))
		help := m.help()
		if m.settingsNotice != "" {
			help = truncate(m.settingsNotice, m.width-4)
		}
		footer := lipgloss.NewStyle().Width(m.width-4).Padding(0, 2).Render(help)
		content = header + "\n" + line + "\n" + lipgloss.JoinHorizontal(lipgloss.Top, side, body) + "\n" + footer
	}
	background := lipgloss.Color("#0A0A0A")
	if m.screen == review {
		background = lipgloss.Color("#000000")
	}
	view := tea.NewView(lipgloss.NewStyle().Foreground(white).Background(background).Width(max(1, m.width)).Height(max(1, m.height)).Render(content))
	view.AltScreen = true
	return view
}
func (m *model) navigation() string {
	names := []string{"Welcome", "Mirror", "Disk", "Account", "Timezone", "Network", "Desktop", "Packages", "Review", "Install", "Result"}
	var lines []string
	for i, name := range names {
		line := fmt.Sprintf("%02d %s", i+1, name)
		if int(m.screen) == i {
			line = accent.Render(line)
		} else {
			line = muted.Render(line)
		}
		lines = append(lines, line)
	}
	if m.height < 29 {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines, "\n\n")
}
func (m *model) body(width, height int) string {
	title := func(s string) string { return accent.Render(s) + "\n\n" }
	switch m.screen {
	case welcome:
		if m.checking || len(m.preparation) > 0 {
			return m.preparationBody(width)
		}
		if !m.checking && !m.ready {
			return title("Environment check failed") + "Installation is blocked.\nResolve the issue below, then press R to retry.\nD: inspect detected disks."
		}
		s := title("Install Void Linux") + "x86_64 glibc · UEFI · GPT · Btrfs\n\n" +
			"A minimal system with GRUB, sudo and Snapper.\nUse an already connected wired network.\n\n" +
			"The selected disk will be erased completely.\nNo disk changes occur before final confirmation.\n\n"
		if m.ready {
			return s + accent.Render("Local environment checks passed.") + "\nPress Enter to select a repository mirror."
		}
		return s + danger.Render("Environment check failed. Installation is blocked.")
	case mirror:
		s := title("Repository mirror")
		if m.checking {
			return s + "Checking the selected mirror " + m.spinner() + "\n\nSynchronizing signed indexes and resolving packages.\nNo disk changes are being made."
		}
		for i, item := range installer.Mirrors {
			line := "  " + item.Name
			if i == m.selectedMirror {
				line = accent.Render("> " + item.Name)
			}
			s += line + "\n"
		}
		line := "  Custom HTTPS mirror"
		if m.selectedMirror == len(installer.Mirrors) {
			line = accent.Render("> Custom HTTPS mirror")
		}
		s += line + "\n\n"
		if m.selectedMirror == len(installer.Mirrors) {
			s += m.inputRow("Mirror root", 9, width) + "Example: https://mirror.example/voidlinux\n"
		} else {
			s += truncate(installer.Mirrors[m.selectedMirror].URL, width-2) + "\n"
		}
		return s + "/current and optional /current/nonfree are automatic.\nThe mirror is saved in the installed system."
	case disks:
		return m.diskPanel(width, height)
	case account:
		s := title("Account and hostname")
		var rows []string
		focusedLine := 0
		labels := []string{"Username", "Password", "Confirm password", "Hostname"}
		for i, label := range labels {
			if i == m.focus {
				focusedLine = len(rows)
			}
			rows = append(rows, strings.Split(strings.TrimSuffix(m.inputRow(label, i, width), "\n"), "\n")...)
		}
		count := max(1, height-5)
		start := max(0, min(focusedLine, max(0, len(rows)-count)))
		if m.focus == 13 {
			start = max(0, len(rows)-count)
		}
		s += strings.Join(rows[start:min(len(rows), start+count)], "\n") + "\n\n"
		return s + muted.Render("Password: 8–128 ASCII characters; no colon.\nRoot receives the same password.")
	case timezone:
		s := title("Select timezone") + "Filter: " + m.fields[10].View() + "\n\n"
		if message := m.fieldErrors[10]; message != "" {
			s += danger.Render(message) + "\n"
		}
		zones := m.filteredTimezones()
		if len(zones) == 0 {
			return s + "No matching timezone. Change the filter."
		}
		count := max(1, height-8)
		start := max(0, m.selectedTimezone-count+1)
		for i := start; i < min(len(zones), start+count); i++ {
			line := "  " + zones[i]
			if i == m.selectedTimezone {
				line = accent.Render("> " + zones[i])
			}
			s += line + "\n"
		}
		return s + fmt.Sprintf("\n%d matching zones. Type a region or city, e.g. Seoul.\nTimezone only; no NTP service is added.", len(zones))
	case network:
		s := title("Network configuration")
		if !m.static {
			return s + accent.Render("[ DHCP ]") + "    Static IPv4\n\nDHCP needs no additional settings.\n\nSpace: select Static IPv4\nEnter: focus Continue"
		}
		mode := "DHCP    [ Static IPv4 ]"
		iface := "Interface         " + m.interfaces[m.selectedInterface]
		if m.focus == 11 {
			mode = accent.Render("> " + mode + " (Space: toggle)")
		}
		if m.focus == 12 {
			iface = accent.Render("> " + iface + " (Space: next)")
		}
		s += mode + "\n\n" + iface + "\n"
		if message := m.fieldErrors[12]; message != "" {
			s += danger.Render(message) + "\n"
		}
		s += "\n"
		for i, label := range []string{"Address / CIDR", "Gateway", "DNS server"} {
			s += m.inputRow(label, i+4, width)
		}
		return s
	case desktop:
		s := title("Desktop and NVIDIA driver")
		for i, name := range []string{"None (console only)", "plasma-desktop / sddm", "niri / sddm"} {
			prefix := "  "
			if i == m.desktopFocus {
				prefix = "> "
			}
			mark := "( ) "
			if i == m.selectedDesktop {
				mark = "(*) "
			}
			s += prefix + mark + name + "\n"
		}
		description := "Firefox and Kitty are optional packages. Both desktops include SDDM, PipeWire and CJK fonts."
		if len(m.gpus) == 0 {
			return selectionPanel(s+"\nNo NVIDIA display controller detected.", description, width, height)
		}
		s += fmt.Sprintf("\n%d NVIDIA display controller(s) detected.\n", len(m.gpus))
		mark := "[ ]"
		if m.config.Graphics == "nvidia" {
			mark = "[V]"
		}
		prefix := "  "
		if m.desktopFocus == 3 {
			prefix = "> "
		}
		s += "\n" + prefix + mark + " Install NVIDIA driver\n"
		if m.desktopFocus == 3 {
			description = "NVIDIA adds nonfree and linux-headers. GPU support is checked before erasure; legacy-only GPUs are blocked."
		}
		return selectionPanel(s, description, width, height)
	case utilities:
		mark := "[ ]"
		if m.selectedOnly {
			mark = "[V]"
		}
		selectedToggle := "  " + mark + " Show selected only"
		if m.packageFocus == 1 {
			selectedToggle = accent.Render(">" + selectedToggle[1:])
		}
		s := title("Additional packages") + "Filter: " + m.packageFilter.View() + "\n" + selectedToggle + "\n\n"
		if m.packagesLoading {
			return s + "Fetching the selected mirror's main package list " + m.spinner()
		}
		if m.packagesError != "" {
			return s + "Package list could not be loaded.\nCtrl+R: retry\n\n" + truncate(m.packagesError, max(1, width-2))
		}
		packages := m.filteredPackages()
		count := max(1, height-11)
		start := max(0, m.selectedUtility-count+1)
		for i := start; i < min(len(packages), start+count); i++ {
			mark := "[ ]"
			for _, p := range m.config.Utilities {
				if p == packages[i].Name {
					mark = "[V]"
					break
				}
			}
			line := truncate("  "+mark+" "+packages[i].Name+"  "+packages[i].Version, width-1)
			if i == m.selectedUtility && m.packageFocus != 1 && m.packageFocus != 3 {
				line = accent.Render(">" + line[1:])
			}
			s += line + "\n"
		}
		hint := fmt.Sprintf("%d selected. %d matching / %d available.", len(m.config.Utilities), len(packages), len(m.packages))
		if len(packages) == 0 {
			s += "No matching packages. Change the filter.\n"
		} else {
			p := packages[m.selectedUtility]
			hint += " " + p.Description
			if p.Name == "fcitx5" || p.Name == "fcitx5-hangul" || p.Name == "openssh" || p.Name == "firefox" || p.Name == "kitty" {
				hint += " " + utilityDescription(p.Name)
			}
		}
		return selectionPanel(s, hint, width, height)
	case review:
		return m.reviewBody(width, height)
	case installing:
		if m.abortPrompt {
			return title("Cancel installation?") + danger.Render("The target disk may become unbootable.") + "\n\nPress Y to stop and clean up mounts.\nPress N or Esc to continue installation.\n\nThe installer waits for the engine to exit."
		}
		if m.showLogs {
			return m.logView(width, height)
		}
		s := title("Installing Void Linux")
		for _, name := range stageNames {
			symbol := "[ ]"
			switch m.stages[name] {
			case "running":
				symbol = "[" + m.spinner() + "]"
			case "completed":
				symbol = "[✓]"
			case "failed":
				symbol = "[!]"
			}
			s += fmt.Sprintf("%s %s\n", symbol, stageLabel(name))
		}
		if m.cancelling {
			s += "\n" + danger.Render("Cancelling; waiting for engine cleanup...")
		} else if m.starting {
			s += "\nStarting installation engine..."
		} else {
			s += "\nCurrent operation: " + stageLabel(m.current)
		}
		available := height - lipgloss.Height(s) - 2
		if available > 0 {
			s += "\n" + muted.Render("Recent logs (F2: full log)") + "\n"
			count := min(3, available)
			for _, entry := range m.logs[max(0, len(m.logs)-count):] {
				s += truncate(entry, width-2) + "\n"
			}
		}
		return s
	case result:
		if m.showLogs {
			return m.logView(width, height)
		}
		logPath := ""
		if m.session != nil && m.session.LogPath != "" {
			logPath = "\n\nLog: " + m.session.LogPath
		}
		if m.success {
			return title("Installation complete") + "Void Linux was installed successfully.\n\nThe engine verified the installation and unmounted\nthe target filesystems.\n\nExit the installer, remove the installation medium,\nand reboot into your new system.\n\nUse Snapper to inspect snapshots.\nAutomatic rollback is not configured." + logPath
		}
		return title("Installation did not complete") + danger.Render(m.err) + "\n\nLast operation: " + stageLabel(m.current) + "\n\nThe target disk may contain an incomplete system.\nPress F2 to inspect the installation logs.\nReview cleanup errors before rebooting." + logPath
	}
	return ""
}
func (m *model) logView(width, height int) string {
	count := max(1, height-4)
	end := max(0, len(m.logs)-m.logOffset)
	start := max(0, end-count)
	var lines []string
	for _, line := range m.logs[start:end] {
		lines = append(lines, truncate(line, width))
	}
	return accent.Render("Installation logs") + fmt.Sprintf("  %d–%d / %d\n\n", start+1, end, len(m.logs)) + strings.Join(lines, "\n")
}
func (m *model) reviewBody(width, height int) string {
	d := m.disks[m.selectedDisk]
	manager := "dhcpcd"
	if m.config.Desktop == "kde" || m.config.Desktop == "niri" {
		manager = "NetworkManager"
	}
	driver := "No"
	if m.config.Graphics == "nvidia" {
		driver = "Yes"
	}
	summary := fmt.Sprintf("Target     %s · %s\nAccount    %s@%s\nSystem     %s · %s\nDesktop    %s\nNVIDIA     %s\nPackages   %d selected\n\n", d.Path, d.Size, m.config.Username, m.config.Hostname, m.config.TimezoneName(), strings.ToUpper(m.config.Network), desktopName(m.config.Desktop), driver, len(m.config.Utilities))
	summary += m.checksSummary() + "\nDetails\nRoot login: locked; user has password-protected sudo.\n"
	utilities := strings.Join(m.config.Utilities, ", ")
	if utilities == "" {
		utilities = "None"
	}
	summary += "Additional packages: " + utilities + "\nPackages: " + strings.Join(m.config.SoftwarePackages(), ", ") + "\n"
	summary += "Disk model: " + d.Model + "\nMirror: " + m.config.Mirror + "/current\n"
	if m.config.Network == "static" {
		summary += m.networkSummary() + "\n"
	}
	summary += "Services: dbus, cronie, bluetoothd, tlp, " + manager
	if m.config.Desktop != "none" && m.config.Desktop != "" {
		summary += ", sddm (elogind via D-Bus)"
	}
	if m.config.Graphics == "nvidia" {
		summary += "\nAdditional repository: current/nonfree"
	}
	summary += "\nEFI: 1 GiB FAT32 at /boot/efi\nRoot: remaining space, Btrfs\n@  @home  @snapshots  @log  @cache\nNo swap. Snapper snapshots; no automatic rollback."
	lines := strings.Split(lipgloss.NewStyle().Width(max(1, width-2)).Render(summary), "\n")
	count := max(1, height-13)
	gap := "\n\n"
	if height < 20 {
		gap = "\n"
		count = max(1, height-9)
	}
	if m.err != "" {
		count = max(1, count-2)
	}
	count = max(1, count-1)
	m.reviewOffset = min(m.reviewOffset, max(0, len(lines)-count))
	visible := strings.Join(lines[m.reviewOffset:min(len(lines), m.reviewOffset+count)], "\n")
	visible = lipgloss.NewStyle().Height(count).Render(visible)
	back, install := "[ Back ]", "[ Install ]"
	if m.reviewFocus == 1 {
		back = accent.Render("> " + back)
	}
	if !m.erasureConfirmed() {
		install = "[ Install ] (disabled)"
		if m.reviewFocus == 2 {
			install = "> " + install
		}
		install = muted.Render(install)
	} else if m.reviewFocus == 2 {
		install = accent.Render("> " + install)
	}
	buttons := lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(back + "    " + install)
	message := ""
	if m.err != "" {
		message = "\n" + danger.Render(truncate(m.err, width))
	}
	state := "Checks pending"
	if m.reviewChecking {
		state = "Checking installation plan " + m.spinner()
	}
	if m.reviewChecksPassed() {
		state = "Checks passed"
	}
	if m.reviewCheckError != "" {
		state = "Checks failed: Ctrl+R retries"
	}
	if m.settingsNotice != "" {
		state = truncate(m.settingsNotice, width)
	}
	return accent.Render("Review installation") + "\n" + muted.Render(state) + gap + visible +
		fmt.Sprintf("\n[%d-%d/%d] PgUp/PgDn", m.reviewOffset+1, min(len(lines), m.reviewOffset+count), len(lines)) + gap +
		danger.Render("ALL DATA ON "+d.Path+" WILL BE ERASED.") + "\nType " + accent.Render("ERASE "+d.Path) + " to confirm:\n" + m.fields[8].View() + message + gap + buttons + gap + muted.Render("Tab Move  Ctrl+R Check  F5 Save  F9 Load  Esc Back")
}

func (m *model) reviewDialog() string {
	// Use a wide dialog with at least four columns of outer margin.
	width := min(m.width-8, max(68, m.width*4/5))
	innerWidth := width - 6
	height := m.height - 4
	m.fields[8].SetWidth(max(1, innerWidth-2))
	body := m.reviewBody(innerWidth, height-4)
	dialog := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(gray).
		Background(lipgloss.Color("#000000")).Padding(1, 2).Width(width - 2).
		Render(body)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, dialog)
}
func (m *model) help() string {
	if m.screen == installing {
		if m.showLogs {
			return "↑↓ / PgUp/PgDn Scroll   End Follow   F2 Stages   Ctrl+C Cancel"
		}
		return "F2 View logs   Ctrl+C Request cancellation"
	}
	if m.screen == result {
		return "F2 Toggle logs   ↑↓ Scroll logs   Enter / Q Exit"
	}
	if m.screen == welcome {
		return "Enter Continue   R Retry checks   F9 Load settings   Q Quit"
	}
	if m.screen == mirror {
		return "↑↓ / Tab Move   Enter Check and continue   Esc Back"
	}
	if m.screen == disks {
		return "↑↓ / Tab Move   Enter Confirm   Esc Back"
	}
	if m.screen == timezone {
		return "Type Filter   ↑↓ / PgUp/PgDn Select   Enter Continue   Esc Back"
	}
	if m.screen == network && m.static {
		return "↑↓ / Tab Move   Space Toggle/cycle   Enter Confirm   Esc Back"
	}
	if m.screen == desktop {
		return "↑↓ / Tab Move   Space Select/toggle   Enter Confirm   Esc Back"
	}
	if m.screen == utilities {
		return "Type Search  ↑↓ Move  Space Toggle  Enter Next  Ctrl+R Reload  Esc Back"
	}
	if m.screen == review {
		return "Tab Move   Enter Activate   PgUp/PgDn Scroll   Esc Back"
	}
	return "Tab / ↑↓ Move   Enter Confirm   Shift+Tab Previous   Esc Back"
}
func (m *model) spinner() string { return []string{"|", "/", "-", "\\"}[m.pulse%4] }
func stageLabel(s string) string {
	switch s {
	case "preflight":
		return "Checking environment"
	case "partitioning":
		return "Creating partitions"
	case "filesystems":
		return "Creating and mounting Btrfs"
	case "bootstrap":
		return "Installing system and selected packages"
	case "system":
		return "Configuring accounts and system"
	case "network":
		return "Configuring network"
	case "software":
		return "Configuring desktop and NVIDIA driver"
	case "grub":
		return "Installing GRUB and initramfs"
	case "snapper":
		return "Configuring Snapper"
	case "verification":
		return "Verifying installation"
	case "cleanup":
		return "Unmounting target filesystems"
	case "":
		return "Starting engine"
	}
	return s
}
func truncate(s string, width int) string {
	return ansi.Truncate(s, max(0, width), "…")
}

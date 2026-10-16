package ui

import (
	"fmt"
	"strings"
	"void-installer/internal/installer"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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
	} else if m.screen == review {
		content = m.reviewDialog()
	} else {
		header := accent.Render("VOID LINUX") + "  " + muted.Render("TUI INSTALLER")
		version := muted.Render("v" + m.version)
		header += strings.Repeat(" ", max(1, m.width-4-lipgloss.Width(header)-lipgloss.Width(version))) + version
		header = lipgloss.NewStyle().Width(m.width-4).Padding(0, 2).Render(header)
		sideWidth := 19
		if m.width < 100 {
			sideWidth = 13
		}
		bodyHeight := m.height - 5
		contentWidth := m.width - sideWidth - 5
		side := lipgloss.NewStyle().Width(sideWidth).Height(bodyHeight).PaddingLeft(2).Render(m.navigation())
		body := m.body(contentWidth, bodyHeight)
		if m.err != "" && m.screen != result {
			body += "\n\n" + danger.Render(m.err)
		}
		body = lipgloss.NewStyle().Width(contentWidth).Height(bodyHeight).MaxHeight(bodyHeight).PaddingLeft(2).Render(body)
		line := muted.Render(strings.Repeat("─", m.width))
		footer := lipgloss.NewStyle().Width(m.width-4).Padding(0, 2).Render(m.help())
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
	names := []string{"Welcome", "Mirror", "Disk", "Account", "Timezone", "Network", "Desktop", "Utilities", "Review", "Install", "Result"}
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
		if !m.checking && !m.ready {
			return title("Environment check failed") + "Installation is blocked.\nResolve the issue below, then press R to retry."
		}
		s := title("Install Void Linux") + "x86_64 glibc · UEFI · GPT · Btrfs\n\n" +
			"A minimal system with GRUB, sudo and Snapper.\nUse an already connected wired network.\n\n" +
			"The selected disk will be erased completely.\nNo disk changes occur before final confirmation.\n\n"
		if m.checking {
			return s + "Preparing the live environment " + m.spinner() + "\n\nMissing installation tools are installed automatically.\nXBPS is updated first using the Live repositories.\nKeep the Live network connected; this can take a few minutes."
		}
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
			s += m.fields[9].View() + "\nExample: https://mirror.example/voidlinux\n"
		} else {
			s += truncate(installer.Mirrors[m.selectedMirror].URL, width-2) + "\n"
		}
		return s + "/current and optional /current/nonfree are automatic.\nThe mirror is saved in the installed system."
	case disks:
		s := title("Select installation disk")
		if len(m.disks) == 0 {
			return s + "No disks found."
		}
		count := max(1, (height-7)/3)
		start := max(0, m.selectedDisk-count+1)
		for i := start; i < min(len(m.disks), start+count); i++ {
			d := m.disks[i]
			marker := "  "
			if i == m.selectedDisk {
				marker = "> "
			}
			line := marker + d.Path + "  " + d.Size + "  " + d.Type
			if i == m.selectedDisk {
				line = accent.Render(line)
			}
			detail := d.Model
			if !d.Safe {
				detail = "BLOCKED: " + d.Reason
			}
			s += line + "\n  " + truncate(detail, width-4) + "\n\n"
		}
		return s + danger.Render("All data on the selected disk will be lost.")
	case account:
		s := title("Account and hostname")
		labels := []string{"Username", "Password", "Confirm password", "Hostname"}
		for i, label := range labels {
			s += fmt.Sprintf("%-17s %s\n\n", label, m.fields[i].View())
		}
		return s + muted.Render("Password: 8–128 printable ASCII characters; no colon.\nThe root account receives the same password.")
	case timezone:
		s := title("Select timezone") + "Filter: " + m.fields[10].View() + "\n\n"
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
			return s + accent.Render("[ DHCP ]") + "    Static IPv4\n\nDHCP needs no additional settings.\n\nSpace: select Static IPv4\nEnter: continue with DHCP"
		}
		mode := "DHCP    [ Static IPv4 ]"
		iface := "Interface         " + m.interfaces[m.selectedInterface]
		if m.focus == 11 {
			mode = accent.Render("> " + mode + " (Space: toggle)")
		}
		if m.focus == 12 {
			iface = accent.Render("> " + iface + " (Space: next)")
		}
		s += mode + "\n\n" + iface + "\n\n"
		for i, label := range []string{"Address / CIDR", "Gateway", "DNS server"} {
			s += fmt.Sprintf("%-17s %s\n\n", label, m.fields[i+4].View())
		}
		if m.focus == 7 {
			s += accent.Render("> Continue")
		} else {
			s += muted.Render("Enter after DNS to continue")
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
			s += prefix + mark + name + "\n\n"
		}
		s += "Firefox and Kitty are optional utilities.\n\n"
		if len(m.gpus) == 0 {
			return s + "No NVIDIA display controller detected."
		}
		for _, g := range m.gpus {
			s += fmt.Sprintf("NVIDIA 10de:%s at %s\n", g.Device, g.Address)
		}
		mark := "[ ]"
		if m.config.Graphics == "nvidia" {
			mark = "[V]"
		}
		prefix := "  "
		if m.desktopFocus == 3 {
			prefix = "> "
		}
		s += "\n" + prefix + mark + " Install NVIDIA driver\n"
		return s + muted.Render("nvidia requires nonfree; GPU support is checked\nbefore erasure. Legacy-only GPUs are blocked.")
	case utilities:
		s := title("Optional utility packages")
		packages := installer.UtilityPackages()
		count := max(1, height-7)
		start := max(0, m.selectedUtility-count+1)
		for i := start; i < min(len(packages), start+count); i++ {
			mark := "[ ]"
			for _, p := range m.config.Utilities {
				if p == packages[i] {
					mark = "[V]"
					break
				}
			}
			line := "  " + mark + " " + packages[i]
			if i == m.selectedUtility {
				line = accent.Render("> " + mark + " " + packages[i])
			}
			s += line + "\n"
		}
		hint := "SSH stays disabled after installation."
		switch packages[m.selectedUtility] {
		case "fcitx5":
			hint = "Includes Qt5/Qt6, GTK3/GTK4 modules and configtool."
		case "fcitx5-hangul":
			hint = ""
		}
		return s + fmt.Sprintf("\n%d selected. Space: toggle. Enter: review.\n", len(m.config.Utilities)) + muted.Render(hint)
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
	summary := fmt.Sprintf("Disk: %s  %s\n%s\nUser: %s   Hostname: %s\n%s\nNetwork service: %s\nDesktop: %s\n", d.Path, d.Size, d.Model, m.config.Username, m.config.Hostname, m.networkSummary(), manager, m.config.Desktop)
	driver := "[ ]"
	if m.config.Graphics == "nvidia" {
		driver = "[V]"
	}
	summary += driver + " Install NVIDIA driver\n"
	summary += "Mirror: " + m.config.Mirror + "/current\n"
	summary += "Timezone: " + m.config.TimezoneName() + "\n"
	packages := m.config.SoftwarePackages()
	if len(packages) > 0 {
		summary += "Packages: " + strings.Join(packages, ", ") + "\n"
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
	if m.err != "" {
		count = max(1, count-2)
	}
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
	return accent.Render("Review installation") + "\n\n" + visible +
		fmt.Sprintf("\n[%d-%d/%d] PgUp/PgDn\n\n", m.reviewOffset+1, min(len(lines), m.reviewOffset+count), len(lines)) +
		danger.Render("ALL DATA ON "+d.Path+" WILL BE ERASED.") + "\nType " + accent.Render("ERASE "+d.Path) + " to confirm:\n" + m.fields[8].View() + message + "\n\n" + buttons + "\n\n" + muted.Render("Tab Move   Enter Activate   Esc Back")
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
		return "Enter Continue   R Retry checks   Q Quit"
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
		return "↑↓ / Tab Move   Space Toggle   Enter Review   Esc Back"
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
	r := []rune(s)
	if len(r) > width && width > 1 {
		return string(r[:width-1]) + "…"
	}
	return s
}

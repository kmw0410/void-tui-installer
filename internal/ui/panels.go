package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

var preparationSteps = []struct{ key, label string }{
	{"environment", "Verify Live environment"},
	{"tools", "Check required tools"},
	{"xbps", "Update XBPS"},
	{"install", "Install missing tools"},
	{"discovery", "Discover installation disks"},
}

func (m *model) preparationBody(width int) string {
	s := accent.Render("Prepare Live environment") + "\n\n"
	for _, step := range preparationSteps {
		mark := "[ ]"
		note := ""
		switch m.preparation[step.key] {
		case "running":
			mark = "[" + m.spinner() + "]"
		case "completed":
			mark = "[✓]"
		case "failed":
			mark = "[!]"
		case "skipped":
			mark = "[-]"
			note = " (not needed)"
		}
		s += mark + " " + step.label + note + "\n"
	}
	s += "\n"
	if m.preparation["install"] == "running" {
		s += lipgloss.NewStyle().Width(width-2).Render("Installing: jq, gptfdisk, parted, dosfstools, btrfs-progs, e2fsprogs, psmisc") + "\n"
	}
	if m.checking {
		s += muted.Render("Keep the Live network connected. Please wait.")
	} else if m.ready {
		s += accent.Render("Ready. Continue to select a repository mirror.")
	} else {
		s += danger.Render("Preparation failed. R: retry")
		if len(m.disks) > 0 {
			s += "\nD: inspect detected disks (installation stays blocked)."
		}
	}
	return s
}

func (m *model) inputRow(label string, field, width int) string {
	s := fmt.Sprintf("%-17s %s\n", label, m.fields[field].View())
	if message := m.fieldErrors[field]; message != "" {
		s += danger.Render(lipgloss.NewStyle().Width(width-2).Render("! "+message)) + "\n"
	}
	return s + "\n"
}

func (m *model) continueButton() string {
	label := "[ Continue ]"
	focused := m.continueFocused || (m.screen == account && m.focus == 13) || (m.screen == network && m.static && m.focus == 7)
	if m.screen == welcome && !m.ready {
		return muted.Render("[ Continue ] (unavailable)")
	}
	if m.screen == disks && (m.diskInspection || m.selectedDisk < 0 || m.selectedDisk >= len(m.disks) || !m.disks[m.selectedDisk].Safe) {
		return muted.Render("[ Continue ] (blocked)")
	}
	if m.checking {
		return muted.Render("[ Continue ] (please wait)")
	}
	if focused {
		return accent.Render("> " + label)
	}
	return label
}

// Reserve the same bottom area for selection help regardless of list length.
func selectionPanel(content, description string, width, height int) string {
	top := lipgloss.NewStyle().Height(max(1, height-4)).MaxHeight(max(1, height-4)).Render(content)
	help := lipgloss.NewStyle().Width(width - 2).Height(3).MaxHeight(3).Render(description)
	return top + "\n" + muted.Render(help)
}

func (m *model) diskPanel(width, height int) string {
	s := accent.Render("Select installation disk") + "\n\n"
	if len(m.disks) == 0 {
		return s + "No disks found."
	}
	selected := max(0, min(m.selectedDisk, len(m.disks)-1))
	count := max(1, height-17)
	start := max(0, selected-count+1)
	for i := start; i < min(len(m.disks), start+count); i++ {
		d := m.disks[i]
		prefix := "  "
		if i == selected {
			prefix = "> "
		}
		status := ""
		if !d.Safe {
			status = " [Blocked]"
		}
		// Keep the status visible even when a model name needs truncation.
		base := prefix + d.Path + "  " + d.Size
		modelWidth := max(1, width-lipgloss.Width(base+status)-5)
		row := base + "  " + truncate(d.Model, modelWidth) + status
		if i == selected {
			row = accent.Render(row)
		}
		s += row + "\n"
	}
	d := m.disks[selected]
	serial, wwn := d.Serial, d.WWN
	if serial == "" {
		serial = "Not available"
	}
	if wwn == "" {
		wwn = "Not available"
	}
	status := "Available"
	if !d.Safe {
		status = "Blocked"
	}
	s += "\n" + strings.Repeat("─", max(1, width-2)) + "\n"
	for _, row := range [][2]string{{"Model", d.Model}, {"Device", d.Path}, {"Capacity", d.Size}, {"Serial", serial}, {"WWN", wwn}, {"Status", status}} {
		s += fmt.Sprintf("%-10s %s\n", row[0], truncate(row[1], max(1, width-13)))
	}
	if !d.Safe {
		s += danger.Render("Reason: "+truncate(d.Reason, width-10)) + "\n"
		action := "Use an idle supported disk; resolve the blocker."
		if strings.Contains(d.Reason, "serial or WWN") {
			action = "VM: set a unique disk serial, then restart."
		}
		s += muted.Render(truncate(action, width-2)) + "\n"
	} else {
		s += danger.Render("All data on this disk will be erased.") + "\n"
	}
	if !m.diskInspection {
		btrfs, ext4 := "(*) Btrfs", "( ) ext4"
		note := "Compression and Snapper snapshots."
		if m.config.FilesystemName() == "ext4" {
			btrfs, ext4 = "( ) Btrfs", "(*) ext4"
			note = "Single root filesystem; no snapshots."
		}
		s += "\nRoot filesystem: " + btrfs + "   " + ext4 + "\n" + muted.Render(truncate(note, width-2))
	}
	return s
}

func utilityDescription(name string) string {
	switch name {
	case "fcitx5":
		return "Fcitx5 input method. Includes Qt5/Qt6, GTK3/GTK4 modules and configtool."
	case "fcitx5-hangul":
		return "Korean input. Includes the full Fcitx5 bundle and an English/Hangul profile."
	case "openssh":
		return "SSH client and server tools. The sshd service stays disabled."
	case "firefox":
		return "Firefox browser. Adds Mod+B in Niri."
	case "kitty":
		return "Kitty terminal. Adds Mod+Enter in Niri."
	default:
		return "Optional package: " + name + ". Space selects or removes this package."
	}
}

func desktopName(value string) string {
	switch value {
	case "kde":
		return "KDE Plasma"
	case "niri":
		return "Niri"
	default:
		return "Console only"
	}
}

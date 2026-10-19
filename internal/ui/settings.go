package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"void-installer/internal/installer"
)

type settingsMsg struct {
	mode, path string
	config     installer.Config
	disks      []installer.Disk
	interfaces []string
	err        error
}

func (m *model) openSettingsDialog(save bool) tea.Cmd {
	if !m.ready || m.checking {
		m.err = "Wait for Live preparation to finish before loading settings."
		return nil
	}
	if save && m.screen != review {
		m.err = "Complete setup and save settings from Review with F5."
		return nil
	}
	if save {
		m.settingsMode = "save"
	} else {
		m.settingsMode = "load"
	}
	if m.settingsPath.Value() == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "/root"
		}
		m.settingsPath.SetValue(filepath.Join(home, "void-installer-settings.json"))
	}
	m.blurAll()
	m.err = ""
	return m.settingsPath.Focus()
}

func (m *model) resumeSettingsFocus() tea.Cmd {
	m.settingsPath.Blur()
	if m.screen == review {
		return m.setFocus(8)
	}
	if m.screen == utilities {
		return m.focusPackages(m.packageFocus)
	}
	return m.setFocus(m.focus)
}

func (m *model) updateSettingsDialog(msg tea.KeyPressMsg) tea.Cmd {
	key := msg.String()
	if key == "ctrl+c" {
		m.cancel()
		m.clearSecrets()
		return tea.Quit
	}
	if m.settingsBusy {
		return nil
	}
	if key == "esc" {
		m.settingsMode = ""
		m.err = ""
		return m.resumeSettingsFocus()
	}
	if key != "enter" {
		var cmd tea.Cmd
		m.settingsPath, cmd = m.settingsPath.Update(msg)
		return cmd
	}
	path := strings.TrimSpace(m.settingsPath.Value())
	if path == "" {
		m.err = "Enter a settings file path."
		return nil
	}
	mode := m.settingsMode
	c := m.config
	c.Utilities = slices.Clone(c.Utilities)
	m.settingsBusy = true
	m.err = ""
	return func() tea.Msg {
		result := settingsMsg{mode: mode, path: path}
		if !m.beginJob() {
			result.err = context.Canceled
			return result
		}
		defer m.jobs.Done()
		if mode == "save" {
			result.err = installer.SaveSettings(path, c)
			return result
		}
		result.config, result.err = installer.LoadSettings(path)
		if result.err == nil {
			result.disks, result.err = installer.DiscoverContext(m.ctx)
		}
		if result.err == nil {
			result.interfaces, result.err = installer.Interfaces()
		}
		if result.err == nil && len(result.interfaces) == 0 {
			result.err = fmt.Errorf("No wired interface is available; settings were not applied")
		}
		return result
	}
}

func (m *model) applyLoadedSettings(msg settingsMsg) tea.Cmd {
	if m.reviewCancel != nil {
		m.reviewCancel()
	}
	m.reviewCheckID++
	m.reviewChecking = false
	m.reviewCheckedConfig = ""
	m.reviewReport = installer.CheckReport{}
	m.clearSecrets()
	m.fields[8].SetValue("")
	m.config = msg.config
	m.config.Mirror, _ = installer.NormalizeMirror(m.config.Mirror)
	m.config.Confirmation = ""
	m.disks, m.interfaces = msg.disks, msg.interfaces
	m.selectedDisk = -1
	for i, disk := range m.disks {
		if disk.Path == m.config.Disk && disk.Identity == m.config.Identity && disk.Safe {
			m.selectedDisk = i
		}
	}
	if m.selectedDisk < 0 {
		m.config.Disk, m.config.Identity = "", ""
		m.settingsNotice = "Settings loaded. Disk changed or unavailable; select it again."
	} else {
		m.settingsNotice = "Settings loaded. Recheck mirror/disk and enter your password again."
	}
	m.fields[0].SetValue(m.config.Username)
	m.fields[3].SetValue(m.config.Hostname)
	m.fields[4].SetValue(m.config.Address)
	m.fields[5].SetValue(m.config.Gateway)
	m.fields[6].SetValue(m.config.DNS)
	m.static = m.config.Network == "static"
	m.selectedInterface = max(0, slices.Index(m.interfaces, m.config.Interface))
	m.selectedDesktop = max(0, slices.Index([]string{"none", "kde", "niri"}, m.config.Desktop))
	m.desktopFocus = m.selectedDesktop
	m.selectedTimezone = max(0, slices.Index(installer.Timezones(), m.config.TimezoneName()))
	m.fields[10].SetValue("")
	m.packages, m.packagesRoot = nil, ""
	m.packagesLoading = false
	m.packagesError = ""
	m.selectedOnly = false
	m.packageFilter.SetValue("")
	m.selectedUtility = 0
	m.continueFocused = false
	m.fieldErrors = nil
	m.screen = mirror
	_ = m.SetMirror(m.config.Mirror)
	if m.selectedMirror == len(installer.Mirrors) {
		return m.setFocus(9)
	}
	m.blurAll()
	return nil
}

func (m *model) settingsDialog() string {
	title := "Load settings"
	if m.settingsMode == "save" {
		title = "Save settings"
	}
	width := min(m.width-8, 78)
	body := accent.Render(title) + "\n\nFile: " + m.settingsPath.View() + "\n\nPasswords and erase confirmation are never saved.\nLoading rechecks disks and requires password entry."
	if m.settingsBusy {
		body += "\n\nWorking " + m.spinner()
	} else {
		body += "\n\nEnter Confirm   Esc Back"
	}
	if m.err != "" {
		body += "\n" + danger.Render(truncate(m.err, width-6))
	}
	dialog := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Padding(1, 2).Width(width - 2).Render(body)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, dialog)
}

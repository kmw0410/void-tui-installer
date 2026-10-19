package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"void-installer/internal/installer"
)

type reviewCheckedMsg struct {
	id     int
	key    string
	report installer.CheckReport
	err    error
}

func configKey(c installer.Config) string {
	c.Confirmation = ""
	data, _ := json.Marshal(c)
	return string(data)
}

func (m *model) checkReview() tea.Cmd {
	if m.reviewCancel != nil {
		m.reviewCancel()
	}
	m.reviewCheckID++
	id := m.reviewCheckID
	m.reviewChecking = true
	m.reviewReport = installer.CheckReport{}
	m.reviewCheckError = ""
	m.reviewCheckedConfig = ""
	m.fields[8].SetValue("")
	c := m.config
	c.Utilities = slices.Clone(c.Utilities)
	key := configKey(c)
	ctx, cancel := context.WithCancel(m.ctx)
	m.reviewCancel = cancel
	return func() tea.Msg {
		if !m.beginJob() {
			return reviewCheckedMsg{id: id, key: key, err: context.Canceled}
		}
		defer m.jobs.Done()
		report, err := installer.CheckInstallation(ctx, c)
		return reviewCheckedMsg{id: id, key: key, report: report, err: err}
	}
}

func (m *model) reviewChecksPassed() bool {
	return !m.reviewChecking && m.reviewCheckError == "" && m.reviewReport.Passed() && m.reviewCheckedConfig == configKey(m.config)
}

func (m *model) checksSummary() string {
	state := "Not checked"
	if m.reviewChecking {
		state = "Checking " + m.spinner()
	}
	if m.reviewChecksPassed() {
		state = "Passed"
	}
	if m.reviewCheckError != "" {
		state = "Failed (Ctrl+R retries)"
	}
	var b strings.Builder
	b.WriteString("Pre-install checks: " + state + "\n")
	labels := map[string]string{"environment": "Live environment", "network": "Network adapter", "repository": "Mirror / dependencies", "graphics": "NVIDIA support", "disk": "Disk safety", "capacity": "Target capacity"}
	for _, name := range installer.ReviewChecks {
		status := m.reviewReport.Status[name]
		if status == "" {
			status = "pending"
		}
		if status == "running" && m.reviewCheckError != "" {
			status = "failed"
		}
		b.WriteString(fmt.Sprintf("  %s: %s\n", labels[name], status))
	}
	if m.reviewReport.RequiredBytes > 0 {
		b.WriteString(fmt.Sprintf("  Required %.1f GiB + 1 GiB EFI; disk %.1f GiB\n", float64(m.reviewReport.RequiredBytes)/(1<<30), float64(m.reviewReport.DiskBytes)/(1<<30)))
	}
	if m.reviewCheckError != "" {
		b.WriteString(m.reviewCheckError + "\n")
	}
	return b.String()
}

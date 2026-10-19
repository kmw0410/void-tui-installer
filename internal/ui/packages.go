package ui

import (
	"context"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"void-installer/internal/installer"
)

func (m *model) openPackages() tea.Cmd {
	focus := m.focusPackages(0)
	if m.packagesRoot == m.config.Mirror || m.packagesLoading {
		return focus
	}
	m.packagesLoading = true
	m.packagesError = ""
	root := m.config.Mirror
	return tea.Batch(focus, func() tea.Msg {
		if !m.beginJob() {
			return packagesMsg{root: root, err: context.Canceled}
		}
		defer m.jobs.Done()
		packages, err := installer.RepositoryPackages(m.ctx, root)
		return packagesMsg{root: root, packages: packages, err: err}
	})
}

func (m *model) filteredPackages() []installer.Package {
	filter := strings.ToLower(strings.TrimSpace(m.packageFilter.Value()))
	filtered := make([]installer.Package, 0)
	for _, p := range m.packages {
		if m.selectedOnly && !slices.Contains(m.config.Utilities, p.Name) {
			continue
		}
		if strings.Contains(strings.ToLower(p.Name), filter) || strings.Contains(strings.ToLower(p.Description), filter) {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

func (m *model) focusPackages(focus int) tea.Cmd {
	m.packageFocus = focus
	m.continueFocused = focus == 3
	m.blurAll()
	if focus == 0 {
		return m.setFocus(14)
	}
	return nil
}

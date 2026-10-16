package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
)

// ConsoleKeyFilter handles legacy Linux VT F1-F5 sequences until upstream does.
// Ultraviolet decodes ESC [[ as an unknown CSI, followed by a letter key.
func ConsoleKeyFilter(term string) func(tea.Model, tea.Msg) tea.Msg {
	var pending time.Time
	return func(_ tea.Model, msg tea.Msg) tea.Msg {
		if term != "linux" {
			return msg
		}
		if e, ok := msg.(uv.UnknownCsiEvent); ok && string(e) == "\x1b[[" {
			pending = time.Now()
			return nil
		}
		if e, ok := msg.(tea.KeyPressMsg); ok && !pending.IsZero() {
			age := time.Since(pending)
			pending = time.Time{}
			if age < 250*time.Millisecond && len(e.Text) == 1 && e.Text[0] >= 'A' && e.Text[0] <= 'E' && (e.Mod == 0 || e.Mod == tea.ModShift) {
				return tea.KeyPressMsg{Code: tea.KeyF1 + rune(e.Text[0]) - 'A'}
			}
		}
		return msg
	}
}

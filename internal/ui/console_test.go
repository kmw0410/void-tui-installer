package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
)

func TestLinuxConsoleF2DecoderCompatibility(t *testing.T) {
	filter := ConsoleKeyFilter("linux")
	var decoder uv.EventDecoder
	data := []byte("\x1b[[B")
	var got tea.Msg
	for len(data) > 0 {
		n, e := decoder.Decode(data)
		if n <= 0 {
			t.Fatal("decoder made no progress")
		}
		data = data[n:]
		var msg tea.Msg = e
		if key, ok := e.(uv.KeyPressEvent); ok {
			msg = tea.KeyPressMsg(key)
		}
		if out := filter(nil, msg); out != nil {
			got = out
		}
	}
	key, ok := got.(tea.KeyPressMsg)
	if !ok || key.Code != tea.KeyF2 {
		t.Fatalf("Linux console F2 decoded as %#v", got)
	}
	m := testModel()
	m.screen = installing
	m.Update(key)
	if !m.showLogs {
		t.Fatal("Linux console F2 did not open logs")
	}
}

func TestConsoleFilterPreservesOrdinaryInput(t *testing.T) {
	for _, term := range []string{"linux", "xterm-256color"} {
		filter := ConsoleKeyFilter(term)
		key := tea.KeyPressMsg{Code: 'B', Text: "B"}
		if filter(nil, key) != key {
			t.Fatal("ordinary letter changed")
		}
		f2 := tea.KeyPressMsg{Code: tea.KeyF2}
		if filter(nil, f2) != f2 {
			t.Fatal("standard F2 changed")
		}
	}
}

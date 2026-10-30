package main

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// hint is one key/description pair shown in a footer or the help sheet.
type hint struct {
	keys string
	desc string
}

// match reports whether a key press is any of the given keystrokes, so screens
// can group key handling without a separate binding registry.
func match(msg tea.KeyPressMsg, keys ...string) bool {
	got := msg.String()
	for _, want := range keys {
		if got == want {
			return true
		}
	}
	return false
}

// hints renders a footer line of key chords.
func hints(s Styles, items ...hint) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, s.FootKey.Render(item.keys)+" "+s.FootDesc.Render(item.desc))
	}
	return strings.Join(parts, s.Faint.Render(" · "))
}

// helpSection groups hints in the help sheet.
type helpSection struct {
	title string
	items []hint
}

// helpSections is the full key reference.
func helpSections() []helpSection {
	return []helpSection{
		{
			title: "Library",
			items: []hint{
				{"↑/k", "move up"},
				{"↓/j", "move down"},
				{"enter", "open the cloud"},
				{"n", "new cloud"},
				{"d", "delete cloud"},
				{"q", "quit"},
			},
		},
		{
			title: "Cloud",
			items: []hint{
				{"↑/k ↓/j", "move"},
				{"enter", "edit"},
				{"a", "add assumption"},
				{"v", "cycle verdict"},
				{"e", "evidence"},
				{"x", "delete assumption"},
				{"tab", "switch pane"},
				{"t", "rename cloud"},
				{"esc/q", "back"},
			},
		},
	}
}

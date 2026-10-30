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

// helpSections is the key reference. It stays short enough to fit a normal
// terminal; each pane's footer carries the keys specific to that pane.
func helpSections() []helpSection {
	return []helpSection{
		{
			title: "Library",
			items: []hint{
				{"↑/k ↓/j", "move"},
				{"enter", "open"},
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
				{"a", "add"},
				{"i", "inject"},
				{"v", "verdict"},
				{"V", "verify"},
				{"tab", "switch pane"},
				{"esc/q", "back"},
			},
		},
	}
}

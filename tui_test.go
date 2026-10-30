package main

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func TestVisibleRange(t *testing.T) {
	heights := []int{5, 5, 5, 5, 5}

	start, end := visibleRange(heights, 0, 25)
	if start != 0 || end != 5 {
		t.Errorf("all fit: [%d,%d), want [0,5)", start, end)
	}

	start, end = visibleRange(heights, 4, 12)
	if start > 4 || end <= 4 {
		t.Fatalf("cursor 4 not inside [%d,%d)", start, end)
	}
	if sumInts(heights[start:end]) > 12 {
		t.Errorf("window [%d,%d) overflows the height", start, end)
	}

	// A cursor past the end is clamped rather than panicking.
	start, end = visibleRange(heights, 99, 12)
	if end > len(heights) || start >= end {
		t.Errorf("out-of-range cursor gave [%d,%d)", start, end)
	}

	if start, end := visibleRange(nil, 0, 10); start != 0 || end != 0 {
		t.Errorf("empty items: [%d,%d)", start, end)
	}

	// Items taller than the space still return the cursor's item.
	tall := []int{30, 30}
	start, end = visibleRange(tall, 1, 10)
	if start != 1 || end != 2 {
		t.Errorf("oversized items: [%d,%d), want [1,2)", start, end)
	}
}

func TestFitLinesExactHeight(t *testing.T) {
	got := fitLines("one\ntwo", 4, 20)
	if len(got) != 4 {
		t.Fatalf("got %d lines, want 4", len(got))
	}
	if got[0] != "one" || got[1] != "two" || got[2] != "" || got[3] != "" {
		t.Errorf("lines = %q", got)
	}

	long := fitLines("abcdef", 1, 3)
	if w := lipgloss.Width(long[0]); w > 3 {
		t.Errorf("line width = %d, want <= 3", w)
	}
}

func TestTruncateAndSpread(t *testing.T) {
	if got := truncate("short", 10); got != "short" {
		t.Errorf("truncate kept nothing: %q", got)
	}
	if got := lipgloss.Width(truncate("a very long title here", 10)); got != 10 {
		t.Errorf("truncated width = %d, want 10", got)
	}

	row := spread("left", "right", 20)
	if w := lipgloss.Width(row); w != 20 {
		t.Errorf("spread width = %d, want 20 (%q)", w, row)
	}
	if !strings.HasPrefix(row, "left") || !strings.HasSuffix(row, "right") {
		t.Errorf("spread = %q", row)
	}

	// Styled input must measure by display width, not byte length.
	styled := lipgloss.NewStyle().Bold(true).Render("bold")
	row = spread(styled, "r", 12)
	if w := lipgloss.Width(row); w != 12 {
		t.Errorf("styled spread width = %d, want 12", w)
	}

	if got := lipgloss.Width(spread("left", "a very long right side", 10)); got != 10 {
		t.Errorf("overflowing right width = %d, want 10", got)
	}
}

func TestHumanAge(t *testing.T) {
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		when time.Time
		want string
	}{
		{time.Time{}, "never"},
		{now.Add(-30 * time.Second), "just now"},
		{now.Add(-5 * time.Minute), "5m ago"},
		{now.Add(-3 * time.Hour), "3h ago"},
		{now.Add(-2 * 24 * time.Hour), "2d ago"},
		{now.Add(-20 * 24 * time.Hour), "Oct 10"},
	}
	for _, tc := range cases {
		if got := humanAge(tc.when, now); got != tc.want {
			t.Errorf("humanAge(%v) = %q, want %q", tc.when, got, tc.want)
		}
	}
}

func TestLibraryCursorAndSorting(t *testing.T) {
	now := time.Now().UTC()
	older := *New("older")
	older.ID = "older"
	older.Updated = now.Add(-time.Hour)
	newer := *New("newer")
	newer.ID = "newer"
	newer.Updated = now

	var l library
	l.set([]Cloud{older, newer})

	l.move(-5)
	if l.cursor != 0 {
		t.Errorf("cursor = %d, want clamped to 0", l.cursor)
	}
	l.move(99)
	if l.cursor != 1 {
		t.Errorf("cursor = %d, want clamped to 1", l.cursor)
	}

	// Editing an older cloud moves it to the top; the cursor must follow it.
	l.cursor = 0 // "older"
	older.Updated = now.Add(time.Minute)
	l.upsert(older)
	if l.cursor != 0 {
		t.Fatalf("cursor = %d, want to follow the edited cloud to index 0", l.cursor)
	}
	if got := l.clouds[0].ID; got != "older" {
		t.Errorf("first row = %q, want the edited cloud", got)
	}

	l.remove("older")
	if len(l.clouds) != 1 || l.clouds[0].ID != "newer" {
		t.Errorf("after remove: %+v", l.clouds)
	}

	l.remove("newer")
	if _, ok := l.selected(); ok {
		t.Error("selected returned a cloud from an empty library")
	}
	if l.cursor != 0 {
		t.Errorf("cursor = %d, want 0 on an empty library", l.cursor)
	}
}

func TestCloudScreenCursorClamps(t *testing.T) {
	cs := newCloudScreen(*New("t"))
	cs.move(-1, 5)
	if cs.cursor != 0 {
		t.Errorf("cursor = %d, want 0", cs.cursor)
	}
	cs.move(99, 5)
	if cs.cursor != 4 {
		t.Errorf("cursor = %d, want 4", cs.cursor)
	}
}

func keyMsg(key string) tea.KeyPressMsg {
	switch key {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEsc}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "ctrl+s":
		return tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	default:
		r := []rune(key)[0]
		return tea.KeyPressMsg{Code: r, Text: key}
	}
}

func TestEditorSavesSingleLineOnEnter(t *testing.T) {
	e := newEditor(true)
	e.Open(editTarget{kind: editCloudTitle}, "Title", "hint", "", 80, false)

	for _, r := range "ship it" {
		if result, _ := e.Update(keyMsg(string(r))); result != editPending {
			t.Fatalf("typing %q ended the editor early", r)
		}
	}
	if got := e.Value(); got != "ship it" {
		t.Fatalf("value = %q", got)
	}

	result, _ := e.Update(keyMsg("enter"))
	if result != editSaved {
		t.Errorf("enter gave %v, want editSaved", result)
	}
	if e.open {
		t.Error("editor still open after saving")
	}
}

func TestEditorMultiLineKeepsEnterForNewLines(t *testing.T) {
	e := newEditor(true)
	e.Open(editTarget{kind: editParties}, "Sides", "one per line", "", 80, true)

	e.Update(keyMsg("a"))
	result, _ := e.Update(keyMsg("enter"))
	if result != editPending {
		t.Fatalf("enter closed a multi-line editor (got %v)", result)
	}
	e.Update(keyMsg("b"))

	if got := e.Value(); !strings.Contains(got, "\n") {
		t.Fatalf("value = %q, want a newline", got)
	}

	result, _ = e.Update(keyMsg("ctrl+s"))
	if result != editSaved {
		t.Errorf("ctrl+s gave %v, want editSaved", result)
	}
}

func TestEditorCancelDiscards(t *testing.T) {
	e := newEditor(true)
	e.Open(editTarget{kind: editStatement, slot: SlotA}, "A", "hint", "original", 80, false)
	e.Update(keyMsg("x"))

	result, _ := e.Update(keyMsg("esc"))
	if result != editCancelled {
		t.Errorf("esc gave %v, want editCancelled", result)
	}
	if e.open {
		t.Error("editor still open after cancelling")
	}
}

func TestEditorWidthStaysInsideTheWindow(t *testing.T) {
	for _, width := range []int{40, 60, 100, 200} {
		e := newEditor(true)
		e.Open(editTarget{kind: editStatement, slot: SlotA}, "A", "hint", "", width, false)
		if w := e.contentWidth() + 4; w > width {
			t.Errorf("width %d: modal is %d wide", width, w)
		}
	}
}

package main

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestNextStep(t *testing.T) {
	draft := New("draft")
	draft.SetText(SlotA, "ship on time")
	if got := nextStep(*draft); !strings.Contains(got, "4 statements are") || !strings.Contains(got, "B, C, D, D'") {
		t.Errorf("draft nextStep = %q", got)
	}

	single := New("one missing")
	single.SetText(SlotA, "a")
	single.SetText(SlotB, "b")
	single.SetText(SlotC, "c")
	single.SetText(SlotD, "d")
	if got := nextStep(*single); !strings.Contains(got, "statement is") {
		t.Errorf("singular nextStep = %q", got)
	}

	complete := resolved()
	if got := nextStep(*complete); !strings.Contains(got, "All five statements are set") {
		t.Errorf("complete nextStep = %q", got)
	}

	complete.AddAssumption(ArrowBD, "because")
	if got := nextStep(*complete); !strings.Contains(got, "1 assumption surfaced") {
		t.Errorf("assumption nextStep = %q", got)
	}
}

func TestPlural(t *testing.T) {
	cases := []struct {
		n         int
		one, many string
		want      string
	}{
		{0, "cloud", "clouds", "0 clouds"},
		{1, "cloud", "clouds", "1 cloud"},
		{2, "cloud", "clouds", "2 clouds"},
	}
	for _, tc := range cases {
		if got := plural(tc.n, tc.one, tc.many); got != tc.want {
			t.Errorf("plural(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

// newSizedApp returns an app with one loaded cloud, sized like a terminal.
func newSizedApp(t *testing.T, width, height int) *App {
	t.Helper()
	a := NewApp(testStore(t))
	a.Update(tea.WindowSizeMsg{Width: width, Height: height})
	c := resolved()
	c.ID = "abc123"
	a.Update(cloudsLoadedMsg{clouds: []Cloud{*c}})
	return a
}

// TestViewFitsTheWindow is the layout invariant: every screen fills exactly the
// terminal height and never emits a line wider than the terminal, which would
// wrap and destroy the layout.
func TestViewFitsTheWindow(t *testing.T) {
	sizes := [][2]int{{40, 12}, {80, 24}, {100, 28}, {140, 40}}

	for _, size := range sizes {
		width, height := size[0], size[1]

		states := map[string]func(a *App){
			"library":     func(a *App) {},
			"cloud":       func(a *App) { a.Update(keyMsg("enter")) },
			"assumptions": func(a *App) { a.Update(keyMsg("enter")); a.Update(keyMsg("3")) },
			"process":     func(a *App) { a.Update(keyMsg("enter")); a.Update(keyMsg("5")) },
			"editor":      func(a *App) { a.Update(keyMsg("enter")); a.Update(keyMsg("enter")) },
			"help":        func(a *App) { a.Update(keyMsg("?")) },
		}

		for name, prepare := range states {
			a := newSizedApp(t, width, height)
			prepare(a)

			lines := strings.Split(a.View().Content, "\n")
			if len(lines) != height {
				t.Errorf("%dx%d %s: %d lines, want %d", width, height, name, len(lines), height)
			}
			for i, line := range lines {
				if w := lipgloss.Width(line); w > width {
					t.Errorf("%dx%d %s: line %d is %d wide: %q", width, height, name, i, w, line)
				}
			}
		}
	}
}

func TestViewShowsWireframeContent(t *testing.T) {
	a := newSizedApp(t, 100, 30)
	view := a.View().Content

	for _, want := range []string{"cloud-break", "Ship on Friday", "5/5", "open", "new", "quit"} {
		if !strings.Contains(view, want) {
			t.Errorf("library view missing %q", want)
		}
	}

	a.Update(keyMsg("enter"))
	view = a.View().Content
	for _, want := range []string{"A · Objective", "B · Need (Us)", "C · Need (Them)", "D · Want (Us)", "D' · Want (Them)"} {
		if !strings.Contains(view, want) {
			t.Errorf("cloud view missing %q", want)
		}
	}

	// The footer explains the highlighted slot.
	if !strings.Contains(view, "shared aim") {
		t.Errorf("cloud footer missing the objective hint:\n%s", view)
	}
}

func TestTinyTerminalAsksForRoom(t *testing.T) {
	a := NewApp(testStore(t))
	a.Update(tea.WindowSizeMsg{Width: 20, Height: 6})
	if got := a.View().Content; !strings.Contains(got, "at least") {
		t.Errorf("tiny view = %q, want a minimum-size message", got)
	}
}

func TestEmptyLibraryExplainsItself(t *testing.T) {
	a := NewApp(testStore(t))
	a.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	a.Update(cloudsLoadedMsg{clouds: nil})
	view := a.View().Content
	if !strings.Contains(view, "No clouds yet") || !strings.Contains(view, "invalidating an assumption") {
		t.Errorf("empty view = %q", view)
	}
}

func TestHelpSheetIsNotClipped(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {100, 28}, {120, 40}, {90, 20}} {
		a := newSizedApp(t, size[0], size[1])
		a.Update(keyMsg("?"))
		view := a.View().Content
		// A clipped sheet loses its bottom border, which is the failure that
		// looks broken. The closing hint is allowed to drop when space is tight.
		if got := strings.Count(view, "╯"); got != 1 {
			t.Errorf("%dx%d: help box bottom border rendered %d times, want 1", size[0], size[1], got)
		}
		for _, section := range []string{"Library", "Cloud"} {
			if !strings.Contains(view, section) {
				t.Errorf("%dx%d: help lost the %s section", size[0], size[1], section)
			}
		}
	}
}

func TestCursorScrollsTheLastSlotIntoView(t *testing.T) {
	a := newSizedApp(t, 100, 24)
	a.Update(keyMsg("enter")) // open the cloud
	for range 4 {
		a.Update(keyMsg("down"))
	}
	if !strings.Contains(a.View().Content, "D' · Want") {
		t.Errorf("the last slot is not on screen after moving to it:\n%s", a.View().Content)
	}
}

func TestTabsShowEveryPaneAndTheActiveOne(t *testing.T) {
	a := newSizedApp(t, 120, 30)
	a.Update(keyMsg("enter"))

	view := a.View().Content
	for _, label := range []string{"1 statements", "2 cloud", "3 assumptions", "4 injections", "5 process"} {
		if !strings.Contains(view, label) {
			t.Errorf("tab bar missing %q:\n%s", label, ansi.Strip(view))
		}
	}

	if !strings.Contains(view, a.styles.TabActive.Render("1 statements")) {
		t.Error("the pane in front is not marked as active")
	}

	a.Update(keyMsg("3"))
	if !strings.Contains(a.View().Content, a.styles.TabActive.Render("3 assumptions")) {
		t.Error("the active chip did not follow the pane")
	}
}

func TestTabBarIsSeparatedFromThePane(t *testing.T) {
	a := newSizedApp(t, 120, 30)
	a.Update(keyMsg("enter"))

	lines := strings.Split(a.View().Content, "\n")
	if !strings.Contains(ansi.Strip(lines[2]), "1 statements") {
		t.Fatalf("row 2 is not the tab bar: %q", ansi.Strip(lines[2]))
	}
	if strings.TrimSpace(ansi.Strip(lines[3])) != "" {
		t.Errorf("no spacer between the tabs and the pane: %q", ansi.Strip(lines[3]))
	}
}

func TestLeftAndRightSwitchPanes(t *testing.T) {
	a := newSizedApp(t, 120, 30)
	a.Update(keyMsg("enter"))
	if a.cv.pane != paneStatements {
		t.Fatalf("opened on %v", a.cv.pane)
	}

	a.Update(keyMsg("right"))
	if a.cv.pane != paneCloud {
		t.Errorf("right moved to %v, want cloud", a.cv.pane)
	}
	a.Update(keyMsg("left"))
	if a.cv.pane != paneStatements {
		t.Errorf("left moved to %v, want statements", a.cv.pane)
	}
	a.Update(keyMsg("left"))
	if a.cv.pane != paneProcess {
		t.Errorf("left from the first pane = %v, want wrap to process", a.cv.pane)
	}
}

func TestContextIsWrappedNotTruncated(t *testing.T) {
	a := newSizedApp(t, 80, 24)
	a.Update(keyMsg("enter"))
	a.Update(keyMsg("3"))
	// The footer describes the arrow under the cursor; walk to B-D.
	a.Update(keyMsg("down"))
	a.Update(keyMsg("down"))

	// The reading for B-D is longer than one narrow footer row.
	reading := strings.Join(strings.Fields(ArrowBD.Reading(&a.cv.cloud)), " ")
	plain := strings.Join(strings.Fields(ansi.Strip(a.View().Content)), " ")
	if !strings.Contains(plain, reading) {
		t.Errorf("the necessity reading was cut off:\n%s", plain)
	}
}

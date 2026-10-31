package main

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

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
	for _, label := range []string{"1 statements", "2 cloud", "3 challenge", "4 process"} {
		if !strings.Contains(view, label) {
			t.Errorf("tab bar missing %q:\n%s", label, ansi.Strip(view))
		}
	}

	if !strings.Contains(view, a.styles.TabActive.Render("1 statements")) {
		t.Error("the pane in front is not marked as active")
	}

	a.Update(keyMsg("3"))
	if !strings.Contains(a.View().Content, a.styles.TabActive.Render("3 challenge")) {
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

func TestTabChangesPanesAndArrowsChangeArrows(t *testing.T) {
	a := newSizedApp(t, 120, 30)
	a.Update(keyMsg("enter"))
	if a.cv.pane != paneStatements {
		t.Fatalf("opened on %v", a.cv.pane)
	}

	// Tab is the only thing that changes pane.
	a.Update(keyMsg("tab"))
	if a.cv.pane != paneCloud {
		t.Errorf("tab moved to %v, want cloud", a.cv.pane)
	}
	a.Update(keyMsg("tab"))
	if a.cv.pane != paneChallenge {
		t.Errorf("tab moved to %v, want challenge", a.cv.pane)
	}

	// Left and right move between arrows inside the pane, never between panes.
	a.Update(keyMsg("right"))
	if a.cv.pane != paneChallenge {
		t.Errorf("right changed pane to %v", a.cv.pane)
	}
	if a.cv.challengeArrow != 1 {
		t.Errorf("right moved arrow to %d, want 1", a.cv.challengeArrow)
	}
	a.Update(keyMsg("left"))
	if a.cv.challengeArrow != 0 {
		t.Errorf("left moved arrow to %d, want 0", a.cv.challengeArrow)
	}

	a.Update(keyMsg("shift+tab"))
	if a.cv.pane != paneCloud {
		t.Errorf("shift+tab moved to %v, want cloud", a.cv.pane)
	}
}

func TestEveryArrowShowsItsSentenceInFull(t *testing.T) {
	// Wide enough that no sentence needs wrapping.
	a := newSizedApp(t, 200, 40)
	a.Update(keyMsg("enter"))
	a.Update(keyMsg("3"))

	for i, arrow := range Arrows() {
		a.cv.challengeArrow = i
		a.cv.challengeMode = challengeForm
		plain := strings.Join(strings.Fields(ansi.Strip(a.renderChallenge(200, 30))), " ")
		reading := strings.Join(strings.Fields(arrow.Reading(&a.cv.cloud)), " ")
		if !strings.Contains(plain, reading) {
			t.Errorf("%s: sentence is not shown in full", arrow)
		}
	}
}

func TestSelectedClaimCarriesItsEvidence(t *testing.T) {
	a := newSizedApp(t, 80, 24)
	a.Update(keyMsg("enter"))
	a.Update(keyMsg("3"))

	id := a.cv.cloud.AddAssumption(ArrowBD, "review catches every regression")
	a.cv.cloud.AssumptionByID(id).Evidence = "three regressions shipped last quarter"
	a.focusClaim(ArrowBD, id)

	plain := strings.Join(strings.Fields(ansi.Strip(a.renderChallenge(80, 20))), " ")
	if !strings.Contains(plain, "three regressions shipped last quarter") {
		t.Errorf("evidence is not shown with its claim:\n%s", plain)
	}
}

func TestFooterHoldsOnlyKeys(t *testing.T) {
	a := newSizedApp(t, 100, 30)
	a.Update(keyMsg("enter"))
	a.Update(keyMsg("3"))
	a.Update(keyMsg("enter")) // inside an arrow, where the row keys are

	lines := strings.Split(ansi.Strip(a.View().Content), "\n")
	footer := lines[len(lines)-1]
	if !strings.Contains(footer, "move") || !strings.Contains(footer, "verdict") {
		t.Errorf("last row is not the key hints: %q", footer)
	}
	if strings.Contains(footer, "In order to") {
		t.Errorf("the footer carrying prose again: %q", footer)
	}
	if above := lines[len(lines)-2]; !strings.HasPrefix(above, "───") {
		t.Errorf("the footer is not separated from the pane: %q", above)
	}
}

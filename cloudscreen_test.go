package main

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestWrapText(t *testing.T) {
	cases := []struct {
		name     string
		text     string
		width    int
		maxLines int
		want     []string
	}{
		{"fits", "ship on time", 20, 3, []string{"ship on time"}},
		{
			"wraps on word boundaries",
			"keep the code free of regressions",
			12,
			5,
			[]string{"keep the", "code free of", "regressions"},
		},
		{
			"caps the lines and marks the cut",
			"one two three four five six seven",
			9,
			2,
			[]string{"one two", "three…"},
		},
		{"collapses whitespace", "  ship\n\tit  now ", 20, 3, []string{"ship it now"}},
		{"empty", "   ", 10, 3, []string{""}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := wrapText(tc.text, tc.width, tc.maxLines)
			if len(got) != len(tc.want) {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("line %d = %q, want %q", i, got[i], tc.want[i])
				}
			}
			for _, line := range got {
				if w := lipgloss.Width(line); w > tc.width {
					t.Errorf("line %q is %d wide, limit %d", line, w, tc.width)
				}
			}
		})
	}
}

func TestWrapTextHardBreaksLongWords(t *testing.T) {
	got := wrapText("abcdefghijklmnop", 5, 5)
	for _, line := range got {
		if w := lipgloss.Width(line); w > 5 {
			t.Errorf("line %q is %d wide, want at most 5", line, w)
		}
	}
	if joined := strings.Join(got, ""); !strings.HasPrefix(joined, "abcde") {
		t.Errorf("long word was dropped: %q", got)
	}
}

func TestPadRight(t *testing.T) {
	if got := padRight("ab", 5); got != "ab   " {
		t.Errorf("padRight = %q", got)
	}
	if got := lipgloss.Width(padRight("abcdef", 4)); got != 4 {
		t.Errorf("padRight overflow width = %d", got)
	}
	if got := lipgloss.Width(padRight("héllo", 6)); got != 6 {
		t.Errorf("padRight wide-rune width = %d", got)
	}
}

func TestWorstVerdict(t *testing.T) {
	cases := []struct {
		name        string
		assumptions []Assumption
		want        Verdict
	}{
		{"none", nil, Unassessed},
		{"unassessed", []Assumption{{Verdict: Unassessed}}, Unassessed},
		{"holds", []Assumption{{Verdict: Holds}}, Holds},
		{"questionable beats holds", []Assumption{{Verdict: Holds}, {Verdict: Questionable}}, Questionable},
		{"invalid wins", []Assumption{{Verdict: Questionable}, {Verdict: Invalid}, {Verdict: Holds}}, Invalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := worstVerdict(tc.assumptions); got != tc.want {
				t.Errorf("worstVerdict = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPaneCycle(t *testing.T) {
	if got := paneStatements.next(1); got != paneCloud {
		t.Errorf("statements.next(1) = %v", got)
	}
	if got := paneCloud.next(1); got != paneStatements {
		t.Errorf("cloud.next(1) = %v, want wrap", got)
	}
	if got := paneStatements.next(-1); got != paneCloud {
		t.Errorf("statements.next(-1) = %v, want wrap", got)
	}
	if got := paneCloud.String(); got != "cloud" {
		t.Errorf("String = %q", got)
	}
}

// figure renders the diagram pane for a cloud and returns the plain text.
func figure(t *testing.T, a *App, width, height int) (string, string) {
	t.Helper()
	styled := a.renderDiagram(width, height)
	return styled, ansi.Strip(styled)
}

func TestDiagramDrawsEveryNode(t *testing.T) {
	a := newSizedApp(t, 112, 30)
	a.Update(keyMsg("enter"))

	styled, plain := figure(t, a, a.diagramWidth(), a.bodyHeight())
	if plain == "" {
		t.Fatal("no diagram at 112 columns")
	}
	for _, want := range []string{
		"A · Objective",
		"B · Need (Us)",
		"C · Need (Them)",
		"D · Want (Us)",
		"D' · Want (Them)",
		"ship the release", // wrapped to the node width
		"×",
		"assumptions",
		"A-B 0",
		"D-D' 0",
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("diagram missing %q\n%s", want, plain)
		}
	}
	if !strings.Contains(styled, "\x1b[") {
		t.Error("diagram has no styling at all")
	}
	for _, line := range strings.Split(plain, "\n") {
		if w := lipgloss.Width(line); w > a.diagramWidth() {
			t.Errorf("diagram line is %d wide, column is %d: %q", w, a.diagramWidth(), line)
		}
	}
}

func TestDiagramNeverDrawsThroughText(t *testing.T) {
	a := newSizedApp(t, 112, 30)
	a.Update(keyMsg("enter"))

	_, plain := figure(t, a, a.diagramWidth(), a.bodyHeight())
	for _, line := range strings.Split(plain, "\n") {
		runes := []rune(line)
		for i := 1; i < len(runes)-1; i++ {
			switch runes[i] {
			case '│', '┬', '┴':
			default:
				continue
			}
			if isWordRune(runes[i-1]) && isWordRune(runes[i+1]) {
				t.Errorf("a connector cuts through text: %q", line)
				break
			}
		}
	}
}

func isWordRune(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
}

func TestDiagramRefusesTinySpace(t *testing.T) {
	a := newSizedApp(t, 112, 30)
	a.Update(keyMsg("enter"))

	for _, width := range []int{0, 20, 43} {
		if got := a.renderDiagram(width, 40); got != "" {
			t.Errorf("width %d produced a diagram and should not", width)
		}
	}
	if got := a.renderDiagram(80, 6); got != "" {
		t.Error("a six-row window produced a diagram and should not")
	}
}

func TestDiagramShowsAssumptionCounts(t *testing.T) {
	a := newSizedApp(t, 112, 30)
	a.Update(keyMsg("enter"))
	id := a.cv.cloud.AddAssumption(ArrowBD, "review catches every regression")
	a.cv.cloud.AssumptionByID(id).Verdict = Invalid

	_, plain := figure(t, a, a.diagramWidth(), a.bodyHeight())
	if !strings.Contains(plain, "B-D 1") {
		t.Errorf("ledger does not count the B-D assumption:\n%s", plain)
	}
	if !strings.Contains(plain, "◄") || !strings.Contains(plain, "►") {
		t.Errorf("conflict arrow missing:\n%s", plain)
	}
}

func TestCloudScreenSplitsWhenWide(t *testing.T) {
	a := newSizedApp(t, 120, 30)
	a.Update(keyMsg("enter"))

	if !a.splitView() {
		t.Fatal("120 columns should allow the split view")
	}
	view := ansi.Strip(a.View().Content)
	if !strings.Contains(view, "A · Objective") {
		t.Error("wide cloud screen does not show the diagram")
	}
	if !strings.Contains(view, "B · Need (Us)") || !strings.Contains(view, "5/5") {
		t.Error("wide cloud screen lost the header or diagram")
	}

	narrow := newSizedApp(t, 80, 24)
	narrow.Update(keyMsg("enter"))
	if narrow.splitView() {
		t.Error("80 columns should not split")
	}
	// The cards use the same label text, so look for the figure's ledger.
	if strings.Contains(ansi.Strip(narrow.View().Content), "assumptions A-B") {
		t.Error("narrow cloud screen should not draw the diagram beside the cards")
	}
}

func TestCloudPaneShowsTheDiagram(t *testing.T) {
	a := newSizedApp(t, 80, 24)
	a.Update(keyMsg("enter"))
	a.Update(keyMsg("2"))

	if a.cv.pane != paneCloud {
		t.Fatalf("pane = %v, want cloud", a.cv.pane)
	}
	view := ansi.Strip(a.View().Content)
	if !strings.Contains(view, "A · Objective") {
		t.Errorf("diagram pane is empty:\n%s", view)
	}

	a.Update(keyMsg("tab"))
	if a.cv.pane != paneStatements {
		t.Errorf("tab did not return to statements, got %v", a.cv.pane)
	}
	a.Update(keyMsg("tab"))
	a.Update(keyMsg("1"))
	if a.cv.pane != paneStatements {
		t.Errorf("1 did not select statements, got %v", a.cv.pane)
	}
}

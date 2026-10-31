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
	if got := paneCloud.next(1); got != paneAssumptions {
		t.Errorf("cloud.next(1) = %v", got)
	}
	if got := paneAssumptions.next(1); got != paneProcess {
		t.Errorf("assumptions.next(1) = %v", got)
	}
	if got := paneProcess.next(1); got != paneStatements {
		t.Errorf("process.next(1) = %v, want wrap", got)
	}
	if got := paneStatements.next(-1); got != paneProcess {
		t.Errorf("statements.next(-1) = %v, want wrap", got)
	}
	for want, p := range map[string]pane{
		"statements":  paneStatements,
		"cloud":       paneCloud,
		"assumptions": paneAssumptions,
		"process":     paneProcess,
	} {
		if got := p.String(); got != want {
			t.Errorf("String = %q, want %q", got, want)
		}
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
	if a.cv.pane != paneAssumptions {
		t.Errorf("tab from the cloud pane should reach assumptions, got %v", a.cv.pane)
	}
	a.Update(keyMsg("1"))
	if a.cv.pane != paneStatements {
		t.Errorf("1 did not select statements, got %v", a.cv.pane)
	}
}

// openAssumptions opens the test cloud on the assumptions pane.
func openAssumptions(t *testing.T) *App {
	t.Helper()
	a := newSizedApp(t, 110, 30)
	a.Update(keyMsg("enter"))
	a.Update(keyMsg("3"))
	return a
}

func TestArrowsAppearOnceTheyCanBeRead(t *testing.T) {
	a := openAssumptions(t) // a complete cloud: every arrow can be read
	rows := a.assumptionRows()
	if len(rows) != len(Arrows()) {
		t.Fatalf("rows = %d, want one per arrow (%d)", len(rows), len(Arrows()))
	}
	for i, arrow := range Arrows() {
		if rows[i].arrow != arrow || rows[i].id != "" {
			t.Errorf("row %d = %+v, want heading %s", i, rows[i], arrow)
		}
	}

	// With nothing written, no arrow can be read, and the pane says so instead
	// of showing five bare badges.
	empty := New("empty")
	a.cv.cloud = *empty
	if rows := a.assumptionRows(); len(rows) != 0 {
		t.Errorf("empty cloud has %d arrow rows, want none", len(rows))
	}
	pane := ansi.Strip(a.renderAssumptions(80, 16))
	if !strings.Contains(pane, "No arrows yet") {
		t.Errorf("empty pane says nothing:\n%s", pane)
	}
}

func TestArrowSubjectsAreDistinct(t *testing.T) {
	a := openAssumptions(t)
	seen := map[string]Arrow{}
	for _, arrow := range Arrows() {
		subject := a.arrowSubject(arrow)
		if subject == "" {
			t.Errorf("%s has no subject", arrow)
		}
		if previous, ok := seen[subject]; ok {
			t.Errorf("%s and %s share the subject %q", previous, arrow, subject)
		}
		seen[subject] = arrow
	}

	// With nothing written, an arrow names nothing: the heading is just the
	// arrow, and no sentence is invented from placeholders.
	empty := New("empty")
	a.cv.cloud = *empty
	if got := a.arrowSubject(ArrowBD); got != "" {
		t.Errorf("empty subject = %q, want none", got)
	}
	if got := a.arrowReading(ArrowBD); got != "" {
		t.Errorf("empty reading = %q, want none", got)
	}
	if view := a.renderAssumptions(80, 20); strings.Contains(view, "unstated") {
		t.Errorf("placeholder leaked into the pane:\n%s", view)
	}
}

func TestAddAssumptionFromHeading(t *testing.T) {
	a := openAssumptions(t)
	a.cv.assumptionCursor = 2 // the B-D heading

	a.Update(keyMsg("a"))
	if !a.ed.open {
		t.Fatal("a did not open the editor")
	}
	for _, r := range "review catches every regression" {
		a.Update(keyMsg(string(r)))
	}
	a.Update(keyMsg("enter"))

	if len(a.cv.cloud.Assumptions) != 1 {
		t.Fatalf("assumptions = %d, want 1", len(a.cv.cloud.Assumptions))
	}
	added := a.cv.cloud.Assumptions[0]
	if added.Arrow != ArrowBD || added.Text != "review catches every regression" {
		t.Errorf("assumption = %+v", added)
	}

	row, ok := a.selectedAssumptionRow()
	if !ok || row.id != added.ID {
		t.Errorf("cursor did not follow the new assumption: %+v", row)
	}

	saved, err := a.store.Load(a.cv.cloud.ID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(saved.Assumptions) != 1 || saved.Assumptions[0].Text != added.Text {
		t.Errorf("not persisted: %+v", saved.Assumptions)
	}
}

func TestAddingAnEmptyAssumptionIsIgnored(t *testing.T) {
	a := openAssumptions(t)
	a.Update(keyMsg("a"))
	a.Update(keyMsg("enter")) // save with no text

	if len(a.cv.cloud.Assumptions) != 0 {
		t.Errorf("an empty assumption was stored: %+v", a.cv.cloud.Assumptions)
	}
}

func TestVerdictCyclePersists(t *testing.T) {
	a := openAssumptions(t)
	id := a.cv.cloud.AddAssumption(ArrowBD, "review catches everything")
	a.selectAssumption(id)

	want := []Verdict{Holds, Questionable, Invalid, Unassessed}
	for i, expected := range want {
		a.Update(keyMsg("v"))
		if got := a.cv.cloud.AssumptionByID(id).Verdict; got != expected {
			t.Fatalf("press %d: verdict = %v, want %v", i+1, got, expected)
		}
	}

	a.Update(keyMsg("v"))
	a.Update(keyMsg("v"))
	saved, err := a.store.Load(a.cv.cloud.ID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := saved.Assumptions[0].Verdict; got != Questionable {
		t.Errorf("persisted verdict = %v, want questionable", got)
	}
}

func TestEvidenceIsSavedAndTrims(t *testing.T) {
	a := openAssumptions(t)
	id := a.cv.cloud.AddAssumption(ArrowBD, "review catches everything")
	a.selectAssumption(id)

	a.Update(keyMsg("e"))
	if !a.ed.open {
		t.Fatal("e did not open the evidence editor")
	}
	for _, r := range "three regressions" {
		a.Update(keyMsg(string(r)))
	}
	a.Update(keyMsg("enter")) // multi-line editor: enter is a newline
	for _, r := range "last quarter" {
		a.Update(keyMsg(string(r)))
	}
	a.Update(keyMsg("ctrl+s"))

	got := a.cv.cloud.AssumptionByID(id).Evidence
	if !strings.Contains(got, "three regressions") || !strings.Contains(got, "last quarter") {
		t.Errorf("evidence = %q", got)
	}
	if !strings.Contains(got, "\n") {
		t.Errorf("multi-line evidence lost its line break: %q", got)
	}
}

func TestActionsOnAHeadingExplainThemselves(t *testing.T) {
	a := openAssumptions(t)
	a.cv.assumptionCursor = 0 // A-B heading

	for _, key := range []string{"v", "e", "x"} {
		a.Update(keyMsg(key))
		if a.flash.text == "" {
			t.Errorf("%q on a heading gave no message", key)
		}
		if a.ed.open {
			t.Errorf("%q opened an editor on a heading", key)
		}
	}
}

func TestDeleteAssumptionNeedsConfirmation(t *testing.T) {
	a := openAssumptions(t)
	id := a.cv.cloud.AddAssumption(ArrowBD, "review catches everything")
	a.selectAssumption(id)

	a.Update(keyMsg("x"))
	if a.confirm == nil {
		t.Fatal("x did not ask for confirmation")
	}
	a.Update(keyMsg("n"))
	if a.cv.cloud.AssumptionByID(id) == nil {
		t.Fatal("n deleted the assumption anyway")
	}

	a.Update(keyMsg("x"))
	a.Update(keyMsg("y"))
	if a.cv.cloud.AssumptionByID(id) != nil {
		t.Error("y did not delete the assumption")
	}

	saved, err := a.store.Load(a.cv.cloud.ID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(saved.Assumptions) != 0 {
		t.Errorf("deletion not persisted: %+v", saved.Assumptions)
	}
}

func TestAssumptionCursorStaysInRange(t *testing.T) {
	a := openAssumptions(t)
	for range 20 {
		a.Update(keyMsg("down"))
	}
	if got, want := a.cv.assumptionCursor, len(a.assumptionRows())-1; got != want {
		t.Errorf("cursor = %d, want %d", got, want)
	}
	for range 20 {
		a.Update(keyMsg("up"))
	}
	if a.cv.assumptionCursor != 0 {
		t.Errorf("cursor = %d, want 0", a.cv.assumptionCursor)
	}
}

func typeString(a *App, text string) {
	for _, r := range text {
		a.Update(keyMsg(string(r)))
	}
}

func TestInjectAgainstAnAssumption(t *testing.T) {
	a := openAssumptions(t)
	id := a.cv.cloud.AddAssumption(ArrowBD, "review catches every regression")
	a.cv.cloud.AssumptionByID(id).Verdict = Invalid
	a.selectAssumption(id)

	a.Update(keyMsg("i"))
	if !a.ed.open {
		t.Fatal("i did not open the injection editor")
	}
	typeString(a, "pair every change with a regression test")
	a.Update(keyMsg("enter"))

	if len(a.cv.cloud.Injections) != 1 {
		t.Fatalf("injections = %d, want 1", len(a.cv.cloud.Injections))
	}
	injection := a.cv.cloud.Injections[0]
	if len(injection.Breaks) != 1 || injection.Breaks[0] != id {
		t.Errorf("injection breaks %v, want the selected assumption", injection.Breaks)
	}
	if got := a.cv.cloud.Status(); got != StatusDissolving {
		t.Errorf("status = %v, want dissolving until verified", got)
	}

	saved, err := a.store.Load(a.cv.cloud.ID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(saved.Injections) != 1 || saved.Injections[0].Text != injection.Text {
		t.Errorf("not persisted: %+v", saved.Injections)
	}
}

func TestInjectAgainstTheSelectedAssumption(t *testing.T) {
	a := openAssumptions(t)
	id := a.cv.cloud.AddAssumption(ArrowBD, "review catches every regression")
	a.cv.cloud.AssumptionByID(id).Verdict = Invalid
	a.selectAssumption(id)

	a.Update(keyMsg("i"))
	if !a.ed.open {
		t.Fatal("i did not open the injection editor")
	}
	typeString(a, "pair every change with a test")
	a.Update(keyMsg("enter"))

	if len(a.cv.cloud.Injections) != 1 {
		t.Fatalf("injections = %d, want 1", len(a.cv.cloud.Injections))
	}
	injection := a.cv.cloud.Injections[0]
	if len(injection.Breaks) != 1 || injection.Breaks[0] != id {
		t.Errorf("injection breaks %v, want the selected assumption", injection.Breaks)
	}

	// The new injection is listed under the assumption it answers.
	row, ok := a.selectedAssumptionRow()
	if !ok || row.injectionID != injection.ID {
		t.Errorf("cursor did not follow the new injection: %+v", row)
	}
}

func TestInjectOpensTheInjectionThatAlreadyAnswers(t *testing.T) {
	a := openAssumptions(t)
	id := a.cv.cloud.AddAssumption(ArrowBD, "review catches everything")
	injectionID := a.cv.cloud.AddInjection([]string{id}, "automate the review")
	a.selectAssumption(id)

	a.Update(keyMsg("i"))
	if !a.ed.open {
		t.Fatal("i did not open the existing injection")
	}
	if got := a.ed.Value(); got != "automate the review" {
		t.Errorf("editor value = %q, want the existing idea", got)
	}
	a.Update(keyMsg("esc"))

	if len(a.cv.cloud.Injections) != 1 {
		t.Errorf("injections = %d, want no duplicate", len(a.cv.cloud.Injections))
	}
	if row, _ := a.selectedAssumptionRow(); row.injectionID != injectionID {
		t.Error("the cursor did not move to the existing injection")
	}
}

func TestInjectionRationaleIsSaved(t *testing.T) {
	a := openAssumptions(t)
	id := a.cv.cloud.AddAssumption(ArrowBD, "review catches everything")
	injectionID := a.cv.cloud.AddInjection([]string{id}, "automate the review")

	a.selectInjectionRow(injectionID)
	a.Update(keyMsg("r"))
	if !a.ed.open {
		t.Fatal("r did not open the rationale editor")
	}
	typeString(a, "the test outlives the review")
	a.Update(keyMsg("ctrl+s"))

	if got := a.cv.cloud.InjectionByID(injectionID).Rationale; got != "the test outlives the review" {
		t.Errorf("rationale = %q", got)
	}
}

func TestDeleteInjectionNeedsConfirmation(t *testing.T) {
	a := openAssumptions(t)
	id := a.cv.cloud.AddAssumption(ArrowBD, "review catches everything")
	injectionID := a.cv.cloud.AddInjection([]string{id}, "automate the review")

	a.selectInjectionRow(injectionID)

	a.Update(keyMsg("x"))
	if a.confirm == nil {
		t.Fatal("x did not ask for confirmation")
	}
	a.Update(keyMsg("n"))
	if a.cv.cloud.InjectionByID(injectionID) == nil {
		t.Fatal("n deleted the injection anyway")
	}

	a.Update(keyMsg("x"))
	a.Update(keyMsg("y"))
	if a.cv.cloud.InjectionByID(injectionID) != nil {
		t.Error("y did not delete the injection")
	}
}

func TestVerificationRequiresABrokenAssumption(t *testing.T) {
	a := openAssumptions(t)
	a.Update(keyMsg("V"))
	if a.cv.cloud.Verified {
		t.Error("verified a cloud where nothing is broken")
	}
	if a.flash.text == "" {
		t.Error("no explanation for refusing to verify")
	}
}

func TestVerifyWorksFromAnyPane(t *testing.T) {
	a := openAssumptions(t)
	id := a.cv.cloud.AddAssumption(ArrowBD, "review catches everything")
	a.cv.cloud.AssumptionByID(id).Verdict = Invalid
	a.cv.cloud.AddInjection([]string{id}, "automate the review")

	a.cv.pane = paneStatements
	a.Update(keyMsg("V"))
	if !a.cv.cloud.Verified {
		t.Error("V did not verify from the statements pane")
	}
}

func TestInjectionSitsUnderItsAssumption(t *testing.T) {
	a := openAssumptions(t)
	id := a.cv.cloud.AddAssumption(ArrowBD, "review catches every regression")
	injectionID := a.cv.cloud.AddInjection([]string{id}, "automate the review")

	rows := a.assumptionRows()
	for i, row := range rows {
		if row.injectionID != injectionID {
			continue
		}
		if i == 0 || rows[i-1].id != id {
			t.Error("the injection is not directly under its assumption")
		}
		return
	}
	t.Errorf("injection row not found in %d rows", len(rows))
}

func TestVerificationDissolvesTheCloud(t *testing.T) {
	a := openAssumptions(t)
	id := a.cv.cloud.AddAssumption(ArrowBD, "review catches everything")
	a.cv.cloud.AssumptionByID(id).Verdict = Invalid
	a.cv.cloud.AddInjection([]string{id}, "automate the review")
	a.cv.pane = paneProcess

	a.Update(keyMsg("V"))
	if !a.cv.cloud.Verified {
		t.Fatal("V did not verify the cloud")
	}
	if got := a.cv.cloud.Status(); got != StatusDissolved {
		t.Errorf("status = %v, want dissolved", got)
	}

	saved, err := a.store.Load(a.cv.cloud.ID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !saved.Verified {
		t.Error("verification not persisted")
	}

	a.Update(keyMsg("V"))
	if a.cv.cloud.Verified {
		t.Error("V did not withdraw the verification")
	}
}

func TestProcessPaneShowsStepsAndIssues(t *testing.T) {
	a := openAssumptions(t)
	a.cv.pane = paneProcess
	view := ansi.Strip(a.View().Content)

	for _, want := range []string{"Process", "1. State the conflict", "[x]", "Issues", "no assumptions yet"} {
		if !strings.Contains(view, want) {
			t.Errorf("process pane missing %q:\n%s", want, view)
		}
	}
}

func TestProcessShowsConcreteEvidence(t *testing.T) {
	a := newSizedApp(t, 120, 30)
	a.Update(keyMsg("enter"))

	id := a.cv.cloud.AddAssumption(ArrowBD, "review catches every regression")
	a.cv.cloud.AssumptionByID(id).Verdict = Invalid
	a.cv.cloud.AddInjection([]string{id}, "automate the review")
	a.Update(keyMsg("5"))

	// Render the pane on its own: in the split view the diagram's column would
	// sit between the wrapped lines.
	plain := strings.Join(strings.Fields(ansi.Strip(a.renderProcess(80, 20))), " ")
	for _, want := range []string{
		"5 of 5 statements stated",
		"1 of 5 arrows carry an assumption",
		"B-D invalid: review catches every regression",
		"automate the review",
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("process pane missing %q:\n%s", want, plain)
		}
	}
}

func TestInlineEditCommitsInPlace(t *testing.T) {
	a := newSizedApp(t, 100, 30)
	a.Update(keyMsg("enter"))
	a.Update(keyMsg("enter")) // edit slot A where it sits

	if !a.inline.open || a.ed.open {
		t.Fatalf("enter opened the wrong editor: inline=%v modal=%v", a.inline.open, a.ed.open)
	}
	if !strings.Contains(ansi.Strip(a.View().Content), "╭─ ▸ A · Objective") {
		t.Error("the card lost its label while editing")
	}

	typeString(a, " and never freeze the scope")
	a.Update(keyMsg("enter"))

	if a.inline.open {
		t.Error("the field stayed open after commit")
	}
	if got := a.cv.cloud.Text(SlotA); !strings.HasSuffix(got, "never freeze the scope") {
		t.Errorf("statement = %q", got)
	}
	saved, err := a.store.Load(a.cv.cloud.ID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !strings.HasSuffix(saved.Text(SlotA), "never freeze the scope") {
		t.Error("the edit was not persisted")
	}
}

func TestInlineEditEscapeLeavesTheStatementAlone(t *testing.T) {
	a := newSizedApp(t, 100, 30)
	a.Update(keyMsg("enter"))
	before := a.cv.cloud.Text(SlotA)

	a.Update(keyMsg("enter"))
	typeString(a, " scribbled")
	a.Update(keyMsg("esc"))

	if got := a.cv.cloud.Text(SlotA); got != before {
		t.Errorf("statement changed to %q, want %q", got, before)
	}
}

func TestSidesAreTwoInlineFields(t *testing.T) {
	a := newSizedApp(t, 100, 30)
	a.Update(keyMsg("enter"))
	a.Update(keyMsg("s"))

	if a.cv.pane != paneStatements || !a.inline.open {
		t.Fatalf("s went to pane %v, inline=%v", a.cv.pane, a.inline.open)
	}
	typeString(a, "Engineering")
	a.Update(keyMsg("enter")) // straight on to the second name
	typeString(a, "Sales")
	a.Update(keyMsg("enter"))

	if got := a.cv.cloud.Parties; got != [2]string{"Engineering", "Sales"} {
		t.Errorf("parties = %v", got)
	}
	saved, err := a.store.Load(a.cv.cloud.ID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if saved.Parties != [2]string{"Engineering", "Sales"} {
		t.Errorf("parties not persisted: %v", saved.Parties)
	}
}

func TestSidesEditCanBeAbandoned(t *testing.T) {
	a := newSizedApp(t, 100, 30)
	a.Update(keyMsg("enter"))
	before := a.cv.cloud.Parties

	a.Update(keyMsg("s"))
	typeString(a, "Half")
	a.Update(keyMsg("esc"))

	if got := a.cv.cloud.Parties; got != before {
		t.Errorf("parties = %v, want %v", got, before)
	}
}

func TestEmptySlotAsksItsQuestion(t *testing.T) {
	a := newSizedApp(t, 100, 30)
	a.Update(keyMsg("enter"))
	a.cv.cloud.SetText(SlotB, "")

	card := ansi.Strip(a.slotCard(SlotOf(SlotB), false, 90))
	if !strings.Contains(card, "What does the first side need to reach the objective?") {
		t.Errorf("empty card does not ask its question:\n%s", card)
	}
	// One question, nothing trailing after it.
	if strings.Count(card, "?") != 1 {
		t.Errorf("empty card carries more than the question:\n%s", card)
	}
}

func TestFilledSlotShowsOnlyWhatWasWritten(t *testing.T) {
	a := newSizedApp(t, 100, 30)
	a.Update(keyMsg("enter"))

	card := ansi.Strip(a.slotCard(SlotOf(SlotA), true, 90))
	if !strings.Contains(card, "ship the release on time") {
		t.Errorf("filled card lost its statement:\n%s", card)
	}
	if strings.Contains(card, "?") {
		t.Errorf("filled card still asks a question:\n%s", card)
	}
}

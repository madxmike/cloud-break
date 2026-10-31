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
	if got := paneCloud.next(1); got != paneChallenge {
		t.Errorf("cloud.next(1) = %v", got)
	}
	if got := paneChallenge.next(1); got != paneProcess {
		t.Errorf("assumptions.next(1) = %v", got)
	}
	if got := paneProcess.next(1); got != paneStatements {
		t.Errorf("process.next(1) = %v, want wrap", got)
	}
	if got := paneStatements.next(-1); got != paneProcess {
		t.Errorf("statements.next(-1) = %v, want wrap", got)
	}
	for want, p := range map[string]pane{
		"statements": paneStatements,
		"cloud":      paneCloud,
		"challenge":  paneChallenge,
		"process":    paneProcess,
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
	if a.cv.pane != paneChallenge {
		t.Errorf("tab from the cloud pane should reach assumptions, got %v", a.cv.pane)
	}
	a.Update(keyMsg("1"))
	if a.cv.pane != paneStatements {
		t.Errorf("1 did not select statements, got %v", a.cv.pane)
	}
}

// openAssumptions opens the test cloud on the assumptions pane.
func openChallenge(t *testing.T) *App {
	t.Helper()
	a := newSizedApp(t, 110, 30)
	a.Update(keyMsg("enter"))
	a.Update(keyMsg("3"))
	return a
}

func TestArrowsAppearOnceTheyCanBeRead(t *testing.T) {
	a := openChallenge(t) // a complete cloud: every arrow can be read
	if got := len(a.challengeArrows()); got != len(Arrows()) {
		t.Fatalf("arrows = %d, want %d", got, len(Arrows()))
	}

	// With nothing written, no arrow can be read, and the pane says so.
	empty := New("empty")
	a.cv.cloud = *empty
	if got := len(a.challengeArrows()); got != 0 {
		t.Errorf("empty cloud has %d arrows, want none", got)
	}
	if pane := ansi.Strip(a.renderChallenge(80, 16)); !strings.Contains(pane, "No arrows yet") {
		t.Errorf("empty pane says nothing:\n%s", pane)
	}
}

func TestAddClaimToTheArrowOnScreen(t *testing.T) {
	a := openChallenge(t)
	a.moveArrow(2) // A-B, A-C, B-D
	if arrow, _ := a.challengeArrow(); arrow != ArrowBD {
		t.Fatalf("arrow = %s, want B-D", arrow)
	}

	a.Update(keyMsg("a"))
	if !a.inline.open {
		t.Fatal("a did not open the claim field")
	}
	typeString(a, "review catches every regression")
	a.Update(keyMsg("enter"))

	claims := a.challengeAssumptions()
	if len(claims) != 1 {
		t.Fatalf("claims = %d, want 1", len(claims))
	}
	if claims[0].Arrow != ArrowBD || claims[0].Text != "review catches every regression" {
		t.Errorf("claim = %+v", claims[0])
	}
	if selected, ok := a.selectedAssumption(); !ok || selected.ID != claims[0].ID {
		t.Error("the cursor did not move onto the new claim")
	}

	saved, err := a.store.Load(a.cv.cloud.ID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(saved.Assumptions) != 1 {
		t.Errorf("not persisted: %+v", saved.Assumptions)
	}
}

func TestAddingAnEmptyClaimIsIgnored(t *testing.T) {
	a := openChallenge(t)
	a.Update(keyMsg("a"))
	a.Update(keyMsg("enter")) // save with no text

	if len(a.cv.cloud.Assumptions) != 0 {
		t.Errorf("an empty claim was stored: %+v", a.cv.cloud.Assumptions)
	}
}

func TestVerdictCyclePersists(t *testing.T) {
	a := openChallenge(t)
	id := a.cv.cloud.AddAssumption(ArrowBD, "review catches everything")
	a.focusClaim(ArrowBD, id)

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
	a := openChallenge(t)
	id := a.cv.cloud.AddAssumption(ArrowBD, "review catches everything")
	a.focusClaim(ArrowBD, id)

	a.Update(keyMsg("e"))
	if !a.inline.open {
		t.Fatal("e did not open the evidence field")
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

func TestClaimActionsWithoutAClaimExplainThemselves(t *testing.T) {
	a := openChallenge(t)
	a.cv.cloud.Assumptions = nil

	for _, k := range []string{"v", "e", "i", "x"} {
		a.Update(keyMsg(k))
		if a.flash.text == "" {
			t.Errorf("%q with no claim gave no message", k)
		}
		if a.ed.open || a.inline.open {
			t.Errorf("%q opened an editor with no claim", k)
		}
	}
}

func TestDeleteAssumptionNeedsConfirmation(t *testing.T) {
	a := openChallenge(t)
	id := a.cv.cloud.AddAssumption(ArrowBD, "review catches everything")
	a.focusClaim(ArrowBD, id)

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

func TestClaimCursorStaysInRange(t *testing.T) {
	a := openChallenge(t)
	for _, text := range []string{"one", "two", "three"} {
		a.cv.cloud.AddAssumption(ArrowBD, text)
	}
	a.moveArrow(2) // onto B-D

	for range 10 {
		a.Update(keyMsg("down"))
	}
	if got, want := a.cv.challengeAssumption, 2; got != want {
		t.Errorf("cursor = %d, want %d", got, want)
	}
	for range 10 {
		a.Update(keyMsg("up"))
	}
	if a.cv.challengeAssumption != 0 {
		t.Errorf("cursor = %d, want 0", a.cv.challengeAssumption)
	}
}

func typeString(a *App, text string) {
	for _, r := range text {
		a.Update(keyMsg(string(r)))
	}
}

func TestInjectAnswersTheSelectedClaim(t *testing.T) {
	a := openChallenge(t)
	id := a.cv.cloud.AddAssumption(ArrowBD, "review catches every regression")
	a.cv.cloud.AssumptionByID(id).Verdict = Invalid
	a.focusClaim(ArrowBD, id)

	a.Update(keyMsg("i"))
	if !a.inline.open {
		t.Fatal("i did not open the answer field")
	}
	typeString(a, "pair every change with a test")
	a.Update(keyMsg("enter"))

	if len(a.cv.cloud.Injections) != 1 {
		t.Fatalf("answers = %d, want 1", len(a.cv.cloud.Injections))
	}
	if got := a.cv.cloud.Injections[0].Breaks; len(got) != 1 || got[0] != id {
		t.Errorf("answer breaks %v, want the selected claim", got)
	}

	pane := ansi.Strip(a.renderChallenge(100, 30))
	if !strings.Contains(pane, "pair every change with a test") {
		t.Errorf("the answer is not visible under its claim:\n%s", pane)
	}
}

func TestInjectOpensTheAnswerThatAlreadyExists(t *testing.T) {
	a := openChallenge(t)
	id := a.cv.cloud.AddAssumption(ArrowBD, "review catches everything")
	answerID := a.cv.cloud.AddInjection([]string{id}, "automate the review")
	a.focusClaim(ArrowBD, id)

	a.Update(keyMsg("i"))
	if !a.inline.open {
		t.Fatal("i did not open the existing answer")
	}
	if got := a.inline.Value(); got != "automate the review" {
		t.Errorf("editor value = %q", got)
	}
	a.Update(keyMsg("esc"))

	if len(a.cv.cloud.Injections) != 1 || a.cv.cloud.InjectionByID(answerID) == nil {
		t.Error("a duplicate answer was created")
	}
}

func TestAnswerRationaleIsSaved(t *testing.T) {
	a := openChallenge(t)
	id := a.cv.cloud.AddAssumption(ArrowBD, "review catches everything")
	answerID := a.cv.cloud.AddInjection([]string{id}, "automate the review")
	a.focusClaim(ArrowBD, id)

	a.Update(keyMsg("r"))
	if !a.inline.open {
		t.Fatal("r did not open the rationale field")
	}
	typeString(a, "the test outlives the review")
	a.Update(keyMsg("ctrl+s"))

	if got := a.cv.cloud.InjectionByID(answerID).Rationale; got != "the test outlives the review" {
		t.Errorf("rationale = %q", got)
	}
}

func TestClearingAnAnswerRemovesIt(t *testing.T) {
	a := openChallenge(t)
	id := a.cv.cloud.AddAssumption(ArrowBD, "review catches everything")
	answerID := a.cv.cloud.AddInjection([]string{id}, "automate the review")
	a.focusClaim(ArrowBD, id)

	a.Update(keyMsg("i")) // opens the existing answer
	if !a.inline.open {
		t.Fatal("i did not open the answer")
	}
	for range len("automate the review") {
		a.Update(keyMsg("backspace"))
	}
	a.Update(keyMsg("enter"))

	if a.cv.cloud.InjectionByID(answerID) != nil {
		t.Error("emptying the answer did not remove it")
	}
}

func TestVerificationRequiresABrokenAssumption(t *testing.T) {
	a := openChallenge(t)
	a.Update(keyMsg("V"))
	if a.cv.cloud.Verified {
		t.Error("verified a cloud where nothing is broken")
	}
	if a.flash.text == "" {
		t.Error("no explanation for refusing to verify")
	}
}

func TestVerifyWorksFromAnyPane(t *testing.T) {
	a := openChallenge(t)
	id := a.cv.cloud.AddAssumption(ArrowBD, "review catches everything")
	a.cv.cloud.AssumptionByID(id).Verdict = Invalid
	a.cv.cloud.AddInjection([]string{id}, "automate the review")

	a.cv.pane = paneStatements
	a.Update(keyMsg("V"))
	if !a.cv.cloud.Verified {
		t.Error("V did not verify from the statements pane")
	}
}

func TestVerificationDissolvesTheCloud(t *testing.T) {
	a := openChallenge(t)
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
	a := openChallenge(t)
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

package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// resolved returns a complete cloud with the canonical "ship on time" conflict.
func resolved() *Cloud {
	c := New("Ship on Friday")
	c.SetText(SlotA, "ship the release on time")
	c.SetText(SlotB, "keep the code free of regressions")
	c.SetText(SlotC, "deliver the promised features")
	c.SetText(SlotD, "freeze the feature set until release")
	c.SetText(SlotD2, "add every requested feature before release")
	return c
}

func TestNewDefaults(t *testing.T) {
	c := New("  Ship on Friday  ")
	if c.Title != "Ship on Friday" {
		t.Errorf("title = %q, want trimmed", c.Title)
	}
	if c.Parties != DefaultParties {
		t.Errorf("parties = %v, want defaults", c.Parties)
	}
	if c.ID == "" {
		t.Error("id is empty")
	}
	if c.Created.IsZero() || c.Updated.IsZero() {
		t.Error("timestamps not set")
	}
	if c.Statements == nil {
		t.Error("statements map not initialised")
	}

	if got := New("   ").Title; got != "Untitled cloud" {
		t.Errorf("blank title = %q, want default", got)
	}
}

func TestNewIDIsUniqueAndShort(t *testing.T) {
	seen := make(map[ID]bool, 200)
	for range 200 {
		id := NewID()
		if len(id) != 12 {
			t.Fatalf("id %q has length %d, want 12", id, len(id))
		}
		if seen[id] {
			t.Fatalf("duplicate id %q", id)
		}
		seen[id] = true
	}
}

func TestSetTextCollapsesToOneLine(t *testing.T) {
	c := New("t")
	c.SetText(SlotA, "  ship\nthe   release\t on time  ")
	if got, want := c.Text(SlotA), "ship the release on time"; got != want {
		t.Errorf("Text = %q, want %q", got, want)
	}
	if got := c.Text(SlotD); got != "" {
		t.Errorf("unset slot = %q, want empty", got)
	}
}

func TestMissingAndProgress(t *testing.T) {
	empty := New("t")
	if got := empty.Missing(); len(got) != 5 {
		t.Fatalf("Missing = %v, want all five slots", got)
	}
	if got := empty.Missing()[0]; got != SlotA {
		t.Errorf("first missing = %q, want A", got)
	}
	if filled, total := empty.Progress(); filled != 0 || total != 5 {
		t.Errorf("Progress = %d/%d, want 0/5", filled, total)
	}
	if empty.Complete() {
		t.Error("empty cloud reports complete")
	}

	full := resolved()
	if got := full.Missing(); len(got) != 0 {
		t.Errorf("Missing = %v, want none", got)
	}
	if filled, total := full.Progress(); filled != 5 || total != 5 {
		t.Errorf("Progress = %d/%d, want 5/5", filled, total)
	}
	if !full.Complete() {
		t.Error("filled cloud reports incomplete")
	}
}

func TestStatusTransitions(t *testing.T) {
	c := New("t")
	if got := c.Status(); got != StatusDraft {
		t.Fatalf("empty status = %v, want draft", got)
	}

	c = resolved()
	if got := c.Status(); got != StatusOpen {
		t.Fatalf("complete status = %v, want open", got)
	}

	id := c.AddAssumption(ArrowBD, "review catches every regression")
	if got := c.Status(); got != StatusOpen {
		t.Fatalf("unassessed assumption changed status to %v", got)
	}

	c.AssumptionByID(id).Verdict = Invalid
	if got := c.Status(); got != StatusDissolving {
		t.Fatalf("invalid assumption status = %v, want dissolving", got)
	}

	c.AddInjection([]string{id}, "pair review with a regression test each time")
	if got := c.Status(); got != StatusDissolving {
		t.Fatalf("unverified injection status = %v, want dissolving", got)
	}

	c.Verified = true
	if got := c.Status(); got != StatusDissolved {
		t.Fatalf("verified status = %v, want dissolved", got)
	}
}

func TestVerifiedAloneDoesNotDissolve(t *testing.T) {
	c := resolved()
	c.Verified = true
	if got := c.Status(); got != StatusOpen {
		t.Errorf("status = %v, want open: nothing was broken", got)
	}
}

func TestAssumptionsAreGroupedByArrow(t *testing.T) {
	c := resolved()
	first := c.AddAssumption(ArrowBD, "a")
	c.AddAssumption(ArrowCD2, "b")
	c.AddAssumption(ArrowBD, "c")

	if got := c.CountFor(ArrowBD); got != 2 {
		t.Errorf("CountFor(B-D) = %d, want 2", got)
	}
	if got := c.CountFor(ArrowAB); got != 0 {
		t.Errorf("CountFor(A-B) = %d, want 0", got)
	}
	got := c.AssumptionsFor(ArrowBD)
	if len(got) != 2 || got[0].ID != first {
		t.Errorf("AssumptionsFor(B-D) = %v, want the two B-D entries in order", got)
	}
	if c.AssumptionByID("missing") != nil {
		t.Error("AssumptionByID returned a value for an unknown id")
	}
}

func TestRemoveAssumptionUnlinksInjections(t *testing.T) {
	c := resolved()
	id := c.AddAssumption(ArrowBD, "a")
	c.AddInjection([]string{id}, "fix")
	if c.InjectionBreaking(id) == nil {
		t.Fatal("injection does not break the assumption")
	}

	c.RemoveAssumption(id)
	if c.AssumptionByID(id) != nil {
		t.Error("assumption still present")
	}
	if c.InjectionBreaking(id) != nil {
		t.Error("injection still links to the removed assumption")
	}
	if len(c.Injections) != 1 {
		t.Errorf("injection count = %d, want 1: deleting an assumption must not delete the idea", len(c.Injections))
	}
}

func TestBrokenBy(t *testing.T) {
	c := resolved()
	a := c.AddAssumption(ArrowBD, "first")
	b := c.AddAssumption(ArrowAC, "second")
	injID := c.AddInjection([]string{a, b}, "idea")

	inj := c.InjectionByID(injID)
	broken := c.BrokenBy(inj)
	if len(broken) != 2 {
		t.Fatalf("BrokenBy = %v, want both assumptions", broken)
	}
	if broken[0].Text != "first" || broken[1].Text != "second" {
		t.Errorf("BrokenBy order = %v, want link order", broken)
	}
	if len(c.BrokenBy(nil)) != 0 {
		t.Error("BrokenBy(nil) should be empty")
	}
}

func TestValidate(t *testing.T) {
	t.Run("missing statements are errors", func(t *testing.T) {
		c := New("t")
		issues := c.Validate()
		if got := c.Errors(); got != 5 {
			t.Errorf("Errors = %d, want 5 (one per empty slot)", got)
		}
		for _, issue := range issues {
			if issue.Severity == SeverityError && !strings.Contains(issue.Message, "is empty") {
				t.Errorf("unexpected error message %q", issue.Message)
			}
		}
	})

	t.Run("complete cloud with no assumptions warns per arrow", func(t *testing.T) {
		c := resolved()
		warnings := 0
		for _, issue := range c.Validate() {
			if issue.Severity == SeverityWarning && strings.Contains(issue.Message, "no assumptions") {
				warnings++
			}
		}
		if warnings != 5 {
			t.Errorf("arrow warnings = %d, want 5", warnings)
		}
		if c.Errors() != 0 {
			t.Errorf("Errors = %d, want 0", c.Errors())
		}
	})

	t.Run("identical wants are not a conflict", func(t *testing.T) {
		c := resolved()
		c.SetText(SlotD2, c.Text(SlotD))
		if !hasIssue(c, SeverityError, "no conflict") {
			t.Errorf("Validate = %v, want an error about the absent conflict", c.Validate())
		}
	})

	t.Run("straight negation is flagged", func(t *testing.T) {
		c := resolved()
		c.SetText(SlotD, "ship now")
		c.SetText(SlotD2, "don't ship now")
		if !hasIssue(c, SeverityInfo, "straight negation") {
			t.Errorf("Validate = %v, want an info note about the negation", c.Validate())
		}
	})

	t.Run("injection against a valid assumption warns", func(t *testing.T) {
		c := resolved()
		id := c.AddAssumption(ArrowBD, "holds")
		c.AssumptionByID(id).Verdict = Holds
		c.AddInjection([]string{id}, "idea")
		if !hasIssue(c, SeverityWarning, "judged to hold") {
			t.Errorf("Validate = %v, want a warning about breaking a held assumption", c.Validate())
		}
	})

	t.Run("injection breaking nothing warns", func(t *testing.T) {
		c := resolved()
		c.AddInjection(nil, "orphan idea")
		if !hasIssue(c, SeverityWarning, "breaks no assumption") {
			t.Errorf("Validate = %v, want a warning about the orphan injection", c.Validate())
		}
	})

	t.Run("dangling link warns", func(t *testing.T) {
		c := resolved()
		c.AddInjection([]string{"ghost"}, "idea")
		if !hasIssue(c, SeverityWarning, "no longer exists") {
			t.Errorf("Validate = %v, want a warning about the dangling link", c.Validate())
		}
	})

	t.Run("invalid assumption without injection is noted", func(t *testing.T) {
		c := resolved()
		id := c.AddAssumption(ArrowBD, "weak")
		c.AssumptionByID(id).Verdict = Invalid
		if !hasIssue(c, SeverityInfo, "nothing injects against it") {
			t.Errorf("Validate = %v, want an info note", c.Validate())
		}
	})
}

func hasIssue(c *Cloud, sev Severity, substr string) bool {
	for _, issue := range c.Validate() {
		if issue.Severity == sev && strings.Contains(issue.Message, substr) {
			return true
		}
	}
	return false
}

func TestStepsTrackTheProcess(t *testing.T) {
	c := New("t")
	if done, _ := stepStates(c); done != 0 {
		t.Fatalf("draft cloud completed %d steps, want 0", done)
	}

	c = resolved()
	if done, _ := stepStates(c); done != 1 {
		t.Fatalf("complete cloud completed %d steps, want 1", done)
	}

	for _, a := range Arrows() {
		c.AddAssumption(a, "because "+string(a))
	}
	if done, _ := stepStates(c); done != 2 {
		t.Fatalf("all arrows covered: %d steps, want 2", done)
	}

	weak := c.AssumptionsFor(ArrowBD)[0]
	c.AssumptionByID(weak.ID).Verdict = Invalid
	if done, _ := stepStates(c); done != 3 {
		t.Fatalf("weak link found: %d steps, want 3", done)
	}

	c.AddInjection([]string{weak.ID}, "break it")
	if done, _ := stepStates(c); done != 4 {
		t.Fatalf("injection added: %d steps, want 4", done)
	}

	c.Verified = true
	steps := c.Steps()
	for i, step := range steps {
		if !step.Done {
			t.Errorf("step %d (%s) not done after verification", i+1, step.Title)
		}
		if step.Number != i+1 {
			t.Errorf("step number = %d, want %d", step.Number, i+1)
		}
	}
}

func stepStates(c *Cloud) (done, total int) {
	for _, step := range c.Steps() {
		if step.Done {
			done++
		}
	}
	return done, len(c.Steps())
}

func TestArrowReadings(t *testing.T) {
	c := resolved()
	cases := map[Arrow]string{
		ArrowAB:  "In order to achieve ship the release on time, we must satisfy keep the code free of regressions.",
		ArrowAC:  "In order to achieve ship the release on time, we must satisfy deliver the promised features.",
		ArrowBD:  "In order to satisfy keep the code free of regressions, we must freeze the feature set until release.",
		ArrowCD2: "In order to satisfy deliver the promised features, we must add every requested feature before release.",
		ArrowDD2: "We cannot freeze the feature set until release while we also add every requested feature before release.",
	}
	for arrow, want := range cases {
		if got := arrow.Reading(c); got != want {
			t.Errorf("%s reading =\n  %q\nwant\n  %q", arrow, got, want)
		}
	}

	// An empty cloud must still produce a sentence rather than a blank.
	if got := ArrowAB.Reading(New("t")); !strings.Contains(got, "something unstated") {
		t.Errorf("empty reading = %q, want a placeholder", got)
	}
}

func TestArrowEnds(t *testing.T) {
	cases := []struct {
		arrow    Arrow
		from, to Slot
	}{
		{ArrowAB, SlotB, SlotA},
		{ArrowAC, SlotC, SlotA},
		{ArrowBD, SlotD, SlotB},
		{ArrowCD2, SlotD2, SlotC},
		{ArrowDD2, SlotD, SlotD2},
	}
	for _, tc := range cases {
		from, to := tc.arrow.Ends()
		if from != tc.from || to != tc.to {
			t.Errorf("%s.Ends() = %q,%q want %q,%q", tc.arrow, from, to, tc.from, tc.to)
		}
	}
}

func TestSlotInfosCoverEverySlot(t *testing.T) {
	infos := SlotInfos()
	if len(infos) != 5 {
		t.Fatalf("SlotInfos length = %d, want 5", len(infos))
	}
	if infos[0].Slot != SlotA || infos[4].Slot != SlotD2 {
		t.Errorf("SlotInfos order = %v", infos)
	}
	for _, info := range infos {
		if info.Name == "" || info.Prompt == "" || info.Hint == "" {
			t.Errorf("slot %s has an empty field: %+v", info.Slot, info)
		}
		if got := SlotOf(info.Slot); got.Name != info.Name {
			t.Errorf("SlotOf(%s).Name = %q, want %q", info.Slot, got.Name, info.Name)
		}
	}
}

func TestVerdictsCycle(t *testing.T) {
	v := Unassessed
	for i, want := range []Verdict{Holds, Questionable, Invalid, Unassessed} {
		v = v.Next()
		if v != want {
			t.Fatalf("cycle step %d = %v, want %v", i, v, want)
		}
	}
	for v, glyph := range map[Verdict]string{Holds: "✓", Questionable: "?", Invalid: "✗"} {
		if v.Glyph() != glyph {
			t.Errorf("%v glyph = %q, want %q", v, v.Glyph(), glyph)
		}
	}
	if got := Questionable.Label(); got != "Questionable" {
		t.Errorf("Label = %q", got)
	}
}

func TestWeakestFindsInvalidFirst(t *testing.T) {
	c := resolved()
	q := c.AddAssumption(ArrowBD, "questionable")
	c.AssumptionByID(q).Verdict = Questionable
	if got := c.Weakest(); got == nil || got.ID != q {
		t.Fatalf("Weakest = %v, want the questionable assumption", got)
	}

	invalid := c.AddAssumption(ArrowAC, "invalid")
	c.AssumptionByID(invalid).Verdict = Invalid
	if got := c.Weakest(); got == nil || got.ID != invalid {
		t.Fatalf("Weakest = %v, want the invalid assumption", got)
	}

	if got := resolved().Weakest(); got != nil {
		t.Errorf("Weakest on an unassessed cloud = %v, want nil", got)
	}
}

func TestSetParties(t *testing.T) {
	c := New("t")
	c.SetParties("Sales", " Engineering ")
	if c.Party(1) != "Sales" || c.Party(2) != "Engineering" {
		t.Errorf("parties = %q,%q", c.Party(1), c.Party(2))
	}
	if got := c.Party(0); got != "Both sides" {
		t.Errorf("Party(0) = %q", got)
	}
	c.SetParties("", "")
	if c.Party(1) != DefaultParties[0] || c.Party(2) != DefaultParties[1] {
		t.Errorf("blank parties were not reset: %v", c.Parties)
	}
}

func TestJSONRoundTrip(t *testing.T) {
	c := resolved()
	id := c.AddAssumption(ArrowBD, "review catches every regression")
	c.AssumptionByID(id).Verdict = Invalid
	c.AssumptionByID(id).Evidence = "last quarter shipped three regressions"
	c.AddInjection([]string{id}, "regression test per review")
	c.Verified = true

	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back Cloud
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(*c, back) {
		t.Errorf("round trip mismatch:\n got %+v\nwant %+v", back, *c)
	}
	if got := back.Status(); got != StatusDissolved {
		t.Errorf("restored status = %v, want dissolved", got)
	}
}

func TestJSONToleratesMissingFields(t *testing.T) {
	var c Cloud
	if err := json.Unmarshal([]byte(`{"id":"abc"}`), &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	c.SetText(SlotA, "still works")
	if got := c.Text(SlotA); got != "still works" {
		t.Errorf("Text = %q", got)
	}
	if got := c.Party(1); got != DefaultParties[0] {
		t.Errorf("default party = %q", got)
	}
}

func TestVerdictJSONIsReadable(t *testing.T) {
	data, err := json.Marshal(Invalid)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(data) != `"invalid"` {
		t.Errorf("marshalled verdict = %s, want \"invalid\"", data)
	}

	for _, tc := range []struct {
		raw  string
		want Verdict
	}{
		{`"holds"`, Holds},
		{`"questionable"`, Questionable},
		{`"invalid"`, Invalid},
		{`"unassessed"`, Unassessed},
		{`"something else"`, Unassessed},
		{`2`, Questionable}, // files written by earlier versions
		{`99`, Unassessed},
	} {
		var got Verdict
		if err := json.Unmarshal([]byte(tc.raw), &got); err != nil {
			t.Errorf("unmarshal %s: %v", tc.raw, err)
			continue
		}
		if got != tc.want {
			t.Errorf("unmarshal %s = %v, want %v", tc.raw, got, tc.want)
		}
	}

	var bad Verdict
	if err := json.Unmarshal([]byte(`true`), &bad); err == nil {
		t.Error("a boolean was accepted as a verdict")
	}
}

func TestCloudFileKeepsReadableVerdicts(t *testing.T) {
	c := resolved()
	id := c.AddAssumption(ArrowBD, "review catches everything")
	c.AssumptionByID(id).Verdict = Invalid

	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), `"verdict":"invalid"`) {
		t.Errorf("cloud file hides the verdict:\n%s", data)
	}
}

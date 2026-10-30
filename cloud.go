// An evaporating cloud: the conflict resolution diagram from Goldratt's Theory
// of Constraints thinking processes.
//
// An evaporating cloud states a disagreement as five necessary conditions:
//
//	A   the objective both sides share
//	B   a need of the first side        C   a need of the second side
//	D   what the first side wants       D'  what the second side wants
//
// A-B, A-C, B-D and C-D' are necessity arrows ("in order to <upper>, we must
// <lower>"). D and D' collide. Every arrow hides assumptions, and the cloud
// evaporates when an injection invalidates one of them, breaking an arrow
// without breaking the objective.
package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ID identifies a cloud in the library.
type ID string

// NewID returns a short, random, filename-safe identifier.
func NewID() ID {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ID(fmt.Sprintf("c%d", time.Now().UnixNano()))
	}
	return ID(hex.EncodeToString(b[:]))
}

// Slot identifies one of the five statements in a cloud.
type Slot string

const (
	SlotA  Slot = "A"  // shared objective
	SlotB  Slot = "B"  // need of the first side
	SlotC  Slot = "C"  // need of the second side
	SlotD  Slot = "D"  // want of the first side
	SlotD2 Slot = "D'" // want of the second side
)

// SlotInfo describes a slot for the interface: what to ask, and why.
type SlotInfo struct {
	Slot   Slot
	Name   string
	Prompt string
	Hint   string
	Side   int // 0 shared, 1 first side, 2 second side
}

// SlotInfos returns the five slots in reading order.
func SlotInfos() []SlotInfo {
	return []SlotInfo{
		{
			Slot:   SlotA,
			Name:   "Objective",
			Prompt: "What do both sides actually want?",
			Hint:   "The shared aim that both needs serve. Not a compromise, not a position.",
			Side:   0,
		},
		{
			Slot:   SlotB,
			Name:   "Need",
			Prompt: "What does the first side need in order to reach the objective?",
			Hint:   "A necessary condition, not a solution. What must be true for them?",
			Side:   1,
		},
		{
			Slot:   SlotC,
			Name:   "Need",
			Prompt: "What does the second side need in order to reach the objective?",
			Hint:   "Write it as they would say it, not as a rebuttal of B.",
			Side:   2,
		},
		{
			Slot:   SlotD,
			Name:   "Want",
			Prompt: "What does the first side want to do to satisfy their need?",
			Hint:   "The specific action being pushed for.",
			Side:   1,
		},
		{
			Slot:   SlotD2,
			Name:   "Want",
			Prompt: "What does the second side want to do to satisfy their need?",
			Hint:   "It must genuinely collide with the other want, or there is no cloud.",
			Side:   2,
		},
	}
}

// SlotOf looks up a slot's description.
func SlotOf(s Slot) SlotInfo {
	for _, info := range SlotInfos() {
		if info.Slot == s {
			return info
		}
	}
	return SlotInfo{Slot: s, Name: string(s)}
}

// Arrow identifies one of the five necessity arrows in a cloud.
type Arrow string

const (
	ArrowAB  Arrow = "A-B"  // objective -> need of first side
	ArrowAC  Arrow = "A-C"  // objective -> need of second side
	ArrowBD  Arrow = "B-D"  // need -> want, first side
	ArrowCD2 Arrow = "C-D'" // need -> want, second side
	ArrowDD2 Arrow = "D-D'" // the conflict itself
)

// Arrows returns the arrows in reading order.
func Arrows() []Arrow {
	return []Arrow{ArrowAB, ArrowAC, ArrowBD, ArrowCD2, ArrowDD2}
}

// Reading renders the "in order to ..." sentence for an arrow, using the
// cloud's current statements.
func (a Arrow) Reading(c *Cloud) string {
	switch a {
	case ArrowAB:
		return fmt.Sprintf("In order to achieve %s, we must satisfy %s.", phrase(c.Text(SlotA)), phrase(c.Text(SlotB)))
	case ArrowAC:
		return fmt.Sprintf("In order to achieve %s, we must satisfy %s.", phrase(c.Text(SlotA)), phrase(c.Text(SlotC)))
	case ArrowBD:
		return fmt.Sprintf("In order to satisfy %s, we must %s.", phrase(c.Text(SlotB)), phrase(c.Text(SlotD)))
	case ArrowCD2:
		return fmt.Sprintf("In order to satisfy %s, we must %s.", phrase(c.Text(SlotC)), phrase(c.Text(SlotD2)))
	case ArrowDD2:
		return fmt.Sprintf("We cannot %s while we also %s.", phrase(c.Text(SlotD)), phrase(c.Text(SlotD2)))
	}
	return string(a)
}

// Ends returns the two slots an arrow connects.
func (a Arrow) Ends() (from, to Slot) {
	switch a {
	case ArrowAB:
		return SlotB, SlotA
	case ArrowAC:
		return SlotC, SlotA
	case ArrowBD:
		return SlotD, SlotB
	case ArrowCD2:
		return SlotD2, SlotC
	case ArrowDD2:
		return SlotD, SlotD2
	}
	return "", ""
}

// phrase presents a statement inside a sentence, or a placeholder when empty.
func phrase(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "something unstated"
	}
	return s
}

// Verdict is a judgement about whether an assumption holds.
type Verdict int

const (
	Unassessed Verdict = iota
	Holds
	Questionable
	Invalid
)

// String implements fmt.Stringer.
func (v Verdict) String() string {
	switch v {
	case Holds:
		return "holds"
	case Questionable:
		return "questionable"
	case Invalid:
		return "invalid"
	}
	return "unassessed"
}

// Label is the capitalised name, for headings.
func (v Verdict) Label() string {
	s := v.String()
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// Glyph is the single-character marker used in lists.
func (v Verdict) Glyph() string {
	switch v {
	case Holds:
		return "✓"
	case Questionable:
		return "?"
	case Invalid:
		return "✗"
	}
	return "·"
}

// Next cycles to the following verdict, so a single key can move through all
// four states.
func (v Verdict) Next() Verdict { return (v + 1) % 4 }

// MarshalJSON writes the verdict as a word. The files are meant to be opened,
// read and hand-edited, and "invalid" says more than 3.
func (v Verdict) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.String())
}

// UnmarshalJSON accepts the words this version writes and the numbers earlier
// files used, so an existing library keeps loading.
func (v *Verdict) UnmarshalJSON(data []byte) error {
	var word string
	if err := json.Unmarshal(data, &word); err == nil {
		*v = verdictNamed(word)
		return nil
	}

	var number int
	if err := json.Unmarshal(data, &number); err != nil {
		return fmt.Errorf("verdict %s: %w", data, err)
	}
	if number < 0 || int(Verdict(number)) > int(Invalid) {
		*v = Unassessed
		return nil
	}
	*v = Verdict(number)
	return nil
}

// verdictNamed maps a stored word back to its verdict.
func verdictNamed(word string) Verdict {
	switch strings.ToLower(strings.TrimSpace(word)) {
	case "holds":
		return Holds
	case "questionable":
		return Questionable
	case "invalid":
		return Invalid
	}
	return Unassessed
}

// Assumption is one of the hidden "because ..." clauses behind an arrow.
type Assumption struct {
	ID       string  `json:"id"`
	Arrow    Arrow   `json:"arrow"`
	Text     string  `json:"text"`
	Verdict  Verdict `json:"verdict"`
	Evidence string  `json:"evidence,omitempty"`
}

// Injection is a change that invalidates an assumption and so breaks an arrow.
type Injection struct {
	ID        string   `json:"id"`
	Breaks    []string `json:"breaks,omitempty"` // assumption IDs
	Text      string   `json:"text"`
	Rationale string   `json:"rationale,omitempty"`
}

// Cloud is a whole evaporating cloud, saved as one JSON document.
type Cloud struct {
	ID          ID              `json:"id"`
	Title       string          `json:"title"`
	Parties     [2]string       `json:"parties"`
	Statements  map[Slot]string `json:"statements"`
	Assumptions []Assumption    `json:"assumptions,omitempty"`
	Injections  []Injection     `json:"injections,omitempty"`
	Verified    bool            `json:"verified,omitempty"`
	Created     time.Time       `json:"created"`
	Updated     time.Time       `json:"updated"`
}

// DefaultParties is the neutral naming used until the user renames the sides.
var DefaultParties = [2]string{"Us", "Them"}

// New returns an empty, saved-ready cloud.
func New(title string) *Cloud {
	now := time.Now().UTC()
	title = strings.TrimSpace(title)
	if title == "" {
		title = "Untitled cloud"
	}
	return &Cloud{
		ID:         NewID(),
		Title:      title,
		Parties:    DefaultParties,
		Statements: map[Slot]string{},
		Created:    now,
		Updated:    now,
	}
}

func (c *Cloud) ensure() {
	if c.Statements == nil {
		c.Statements = map[Slot]string{}
	}
	if strings.TrimSpace(c.Parties[0]) == "" {
		c.Parties[0] = DefaultParties[0]
	}
	if strings.TrimSpace(c.Parties[1]) == "" {
		c.Parties[1] = DefaultParties[1]
	}
}

// Touch marks the cloud as modified.
func (c *Cloud) Touch() { c.Updated = time.Now().UTC() }

// Text returns a statement, or "" when unset.
func (c *Cloud) Text(s Slot) string {
	if c.Statements == nil {
		return ""
	}
	return strings.TrimSpace(c.Statements[s])
}

// SetText stores a statement, collapsing it to a single line.
func (c *Cloud) SetText(s Slot, text string) {
	c.ensure()
	c.Statements[s] = OneLine(text)
	c.Touch()
}

// Party returns the name of a side, 1 or 2.
func (c *Cloud) Party(side int) string {
	if side != 1 && side != 2 {
		return "Both sides"
	}
	c.ensure()
	return strings.TrimSpace(c.Parties[side-1])
}

// SetParties renames both sides.
func (c *Cloud) SetParties(first, second string) {
	first, second = OneLine(first), OneLine(second)
	if first == "" {
		first = DefaultParties[0]
	}
	if second == "" {
		second = DefaultParties[1]
	}
	c.Parties = [2]string{first, second}
	c.Touch()
}

// Complete reports whether all five statements are filled in.
func (c *Cloud) Complete() bool { return len(c.Missing()) == 0 }

// Missing lists the slots still awaiting a statement.
func (c *Cloud) Missing() []Slot {
	var missing []Slot
	for _, info := range SlotInfos() {
		if c.Text(info.Slot) == "" {
			missing = append(missing, info.Slot)
		}
	}
	return missing
}

// Progress reports how many of the five statements are filled in.
func (c *Cloud) Progress() (filled, total int) {
	return 5 - len(c.Missing()), 5
}

// AssumptionByID returns a pointer to an assumption, or nil.
func (c *Cloud) AssumptionByID(id string) *Assumption {
	for i := range c.Assumptions {
		if c.Assumptions[i].ID == id {
			return &c.Assumptions[i]
		}
	}
	return nil
}

// InjectionByID returns a pointer to an injection, or nil.
func (c *Cloud) InjectionByID(id string) *Injection {
	for i := range c.Injections {
		if c.Injections[i].ID == id {
			return &c.Injections[i]
		}
	}
	return nil
}

// AssumptionsFor returns the assumptions behind one arrow.
func (c *Cloud) AssumptionsFor(a Arrow) []Assumption {
	var out []Assumption
	for _, as := range c.Assumptions {
		if as.Arrow == a {
			out = append(out, as)
		}
	}
	return out
}

// CountFor returns the number of assumptions behind an arrow.
func (c *Cloud) CountFor(a Arrow) int { return len(c.AssumptionsFor(a)) }

// AddAssumption records a new assumption on an arrow and returns its ID.
func (c *Cloud) AddAssumption(a Arrow, text string) string {
	id := string(NewID())
	c.Assumptions = append(c.Assumptions, Assumption{
		ID:    id,
		Arrow: a,
		Text:  strings.TrimSpace(text),
	})
	c.Touch()
	return id
}

// RemoveAssumption deletes an assumption and any injection links to it.
func (c *Cloud) RemoveAssumption(id string) {
	out := c.Assumptions[:0]
	for _, as := range c.Assumptions {
		if as.ID == id {
			continue
		}
		out = append(out, as)
	}
	c.Assumptions = out
	for i := range c.Injections {
		c.Injections[i].Breaks = without(c.Injections[i].Breaks, id)
	}
	c.Touch()
}

// AddInjection records a breakthrough idea that breaks the given assumptions.
func (c *Cloud) AddInjection(breaks []string, text string) string {
	id := string(NewID())
	c.Injections = append(c.Injections, Injection{
		ID:     id,
		Breaks: append([]string(nil), breaks...),
		Text:   strings.TrimSpace(text),
	})
	c.Touch()
	return id
}

// RemoveInjection deletes an injection.
func (c *Cloud) RemoveInjection(id string) {
	out := c.Injections[:0]
	for _, inj := range c.Injections {
		if inj.ID == id {
			continue
		}
		out = append(out, inj)
	}
	c.Injections = out
	c.Touch()
}

// InjectionBreaking returns the first injection that invalidates an assumption.
func (c *Cloud) InjectionBreaking(assumptionID string) *Injection {
	for i := range c.Injections {
		if contains(c.Injections[i].Breaks, assumptionID) {
			return &c.Injections[i]
		}
	}
	return nil
}

// BrokenBy returns the assumptions an injection invalidates.
func (c *Cloud) BrokenBy(inj *Injection) []Assumption {
	if inj == nil {
		return nil
	}
	var out []Assumption
	for _, id := range inj.Breaks {
		if as := c.AssumptionByID(id); as != nil {
			out = append(out, *as)
		}
	}
	return out
}

// Status is the coarse state of a cloud, used for sorting and colouring.
type Status int

const (
	StatusDraft      Status = iota // statements still missing
	StatusOpen                     // complete, no assumption has fallen
	StatusDissolving               // an assumption is invalid, nothing breaks it yet
	StatusDissolved                // an injection breaks it, and the result is verified
)

// String implements fmt.Stringer.
func (s Status) String() string {
	switch s {
	case StatusDraft:
		return "draft"
	case StatusOpen:
		return "open"
	case StatusDissolving:
		return "dissolving"
	case StatusDissolved:
		return "dissolved"
	}
	return "unknown"
}

// Glyph is the compact marker used in the library list.
func (s Status) Glyph() string {
	switch s {
	case StatusDraft:
		return "○"
	case StatusOpen:
		return "◔"
	case StatusDissolving:
		return "◑"
	case StatusDissolved:
		return "●"
	}
	return "·"
}

// HasInvalid reports whether any assumption has been judged invalid.
func (c *Cloud) HasInvalid() bool {
	for _, as := range c.Assumptions {
		if as.Verdict == Invalid {
			return true
		}
	}
	return false
}

// Status derives the state of the cloud from its contents.
func (c *Cloud) Status() Status {
	if !c.Complete() {
		return StatusDraft
	}
	if len(c.BrokenAssumptions()) > 0 {
		if c.Verified {
			return StatusDissolved
		}
		return StatusDissolving
	}
	if c.HasInvalid() {
		return StatusDissolving
	}
	return StatusOpen
}

// BrokenAssumptions returns assumptions judged invalid that an injection breaks.
func (c *Cloud) BrokenAssumptions() []Assumption {
	var out []Assumption
	for _, as := range c.Assumptions {
		if as.Verdict == Invalid && c.InjectionBreaking(as.ID) != nil {
			out = append(out, as)
		}
	}
	return out
}

// Weakest returns the assumption most likely to be the weak link: the first
// invalid one, else the first questionable one.
func (c *Cloud) Weakest() *Assumption {
	var questionable *Assumption
	for i := range c.Assumptions {
		switch c.Assumptions[i].Verdict {
		case Invalid:
			return &c.Assumptions[i]
		case Questionable:
			if questionable == nil {
				questionable = &c.Assumptions[i]
			}
		}
	}
	return questionable
}

// Step is one stage of the evaporating-cloud process.
type Step struct {
	Number int
	Title  string
	Hint   string
	Done   bool
}

// Steps derives the five-stage TOC process from the cloud's current contents.
func (c *Cloud) Steps() []Step {
	covered := 0
	for _, a := range Arrows() {
		if c.CountFor(a) > 0 {
			covered++
		}
	}
	hasInvalid := c.HasInvalid()
	broken := len(c.BrokenAssumptions()) > 0

	return []Step{
		{
			Number: 1,
			Title:  "State the conflict",
			Hint:   "Fill in A, both needs, and the two colliding wants.",
			Done:   c.Complete(),
		},
		{
			Number: 2,
			Title:  "Surface the assumptions",
			Hint:   fmt.Sprintf("Assumptions surfaced on %d of 5 arrows.", covered),
			Done:   covered == len(Arrows()),
		},
		{
			Number: 3,
			Title:  "Find the weak link",
			Hint:   "Challenge each assumption until one cannot stand.",
			Done:   hasInvalid,
		},
		{
			Number: 4,
			Title:  "Inject a breakthrough",
			Hint:   "A change that makes the broken assumption irrelevant.",
			Done:   broken,
		},
		{
			Number: 5,
			Title:  "Verify the cloud evaporates",
			Hint:   "Confirm the injection dissolves the conflict without harming A.",
			Done:   c.Verified && broken,
		},
	}
}

// Severity ranks a validation finding.
type Severity int

const (
	SeverityInfo Severity = iota
	SeverityWarning
	SeverityError
)

// Issue is one validation finding about a cloud.
type Issue struct {
	Severity Severity
	Message  string
}

// Validate checks the cloud for the mistakes that make it unreadable or
// inconclusive: missing statements, an absent conflict, unassessed arrows.
func (c *Cloud) Validate() []Issue {
	var issues []Issue

	missing := c.Missing()
	for _, s := range missing {
		info := SlotOf(s)
		issues = append(issues, Issue{SeverityError, fmt.Sprintf("%s (%s) is empty: %s", s, info.Name, info.Prompt)})
	}

	if c.Complete() {
		d, d2 := c.Text(SlotD), c.Text(SlotD2)
		if strings.EqualFold(d, d2) {
			issues = append(issues, Issue{SeverityError,
				"D and D' say the same thing, so there is no conflict to evaporate."})
		}
		if isNegationOf(d, d2) {
			issues = append(issues, Issue{SeverityInfo,
				"D' is written as a straight negation of D. Name what the other side actually wants instead."})
		}
	}

	for _, a := range Arrows() {
		if c.CountFor(a) == 0 {
			issues = append(issues, Issue{SeverityWarning,
				fmt.Sprintf("Arrow %s has no assumptions yet: %s", a, a.Reading(c))})
			continue
		}
		from, to := a.Ends()
		if c.Text(from) == "" || c.Text(to) == "" {
			issues = append(issues, Issue{SeverityError,
				fmt.Sprintf("Arrow %s joins a statement that is still empty.", a)})
		}
	}

	for _, inj := range c.Injections {
		if strings.TrimSpace(inj.Text) == "" {
			issues = append(issues, Issue{SeverityWarning, "An injection has no text."})
		}
		if len(inj.Breaks) == 0 {
			issues = append(issues, Issue{SeverityWarning,
				fmt.Sprintf("Injection %q breaks no assumption, so it cannot evaporate anything.", shorten(inj.Text, 40))})
		}
		for _, id := range inj.Breaks {
			as := c.AssumptionByID(id)
			if as == nil {
				issues = append(issues, Issue{SeverityWarning,
					fmt.Sprintf("Injection %q points at an assumption that no longer exists.", shorten(inj.Text, 40))})
				continue
			}
			if as.Verdict == Holds {
				issues = append(issues, Issue{SeverityWarning,
					fmt.Sprintf("Injection %q breaks an assumption you judged to hold.", shorten(inj.Text, 40))})
			}
		}
	}

	for _, as := range c.Assumptions {
		if as.Verdict == Invalid && c.InjectionBreaking(as.ID) == nil {
			issues = append(issues, Issue{SeverityInfo,
				fmt.Sprintf("Assumption %q is invalid but nothing injects against it yet.", shorten(as.Text, 40))})
		}
		if strings.TrimSpace(as.Text) == "" {
			issues = append(issues, Issue{SeverityWarning, "An assumption has no text."})
		}
	}

	if c.Verified && len(c.BrokenAssumptions()) == 0 {
		issues = append(issues, Issue{SeverityWarning,
			"The cloud is marked verified but no injection breaks an invalid assumption."})
	}

	return issues
}

// Errors reports whether validation found blocking problems.
func (c *Cloud) Errors() int {
	n := 0
	for _, issue := range c.Validate() {
		if issue.Severity == SeverityError {
			n++
		}
	}
	return n
}

// OneLine collapses whitespace so a statement fits on one rendered line.
func OneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

// Normalize tidies free text before it is written to disk: statements, titles
// and injections become single lines, empty statements are dropped, and blank
// names fall back to their defaults so no reader has to guess.
func (c *Cloud) Normalize() {
	c.ensure()
	c.Title = OneLine(c.Title)
	if c.Title == "" {
		c.Title = "Untitled cloud"
	}
	c.Parties[0] = OneLine(c.Parties[0])
	c.Parties[1] = OneLine(c.Parties[1])
	if c.Parties[0] == "" {
		c.Parties[0] = DefaultParties[0]
	}
	if c.Parties[1] == "" {
		c.Parties[1] = DefaultParties[1]
	}
	for _, info := range SlotInfos() {
		text := OneLine(c.Statements[info.Slot])
		if text == "" {
			delete(c.Statements, info.Slot)
			continue
		}
		c.Statements[info.Slot] = text
	}
	for i := range c.Assumptions {
		c.Assumptions[i].Text = OneLine(c.Assumptions[i].Text)
		c.Assumptions[i].Evidence = strings.TrimSpace(c.Assumptions[i].Evidence)
	}
	for i := range c.Injections {
		c.Injections[i].Text = OneLine(c.Injections[i].Text)
		c.Injections[i].Rationale = strings.TrimSpace(c.Injections[i].Rationale)
	}
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

func without(list []string, drop string) []string {
	out := list[:0]
	for _, v := range list {
		if v != drop {
			out = append(out, v)
		}
	}
	return out
}

// isNegationOf reports the common "do X" / "don't do X" shape of a fake cloud:
// a want that only negates the other side rather than naming what they want.
func isNegationOf(d, d2 string) bool {
	clean := func(s string) string {
		s = strings.ToLower(OneLine(s))
		return strings.TrimSuffix(s, ".")
	}
	a, b := clean(d), clean(d2)
	negations := []string{"don't ", "do not ", "never ", "stop ", "not "}
	for _, neg := range negations {
		if strings.HasPrefix(b, neg) && strings.TrimSpace(strings.TrimPrefix(b, neg)) == a {
			return true
		}
		if strings.HasPrefix(a, neg) && strings.TrimSpace(strings.TrimPrefix(a, neg)) == b {
			return true
		}
	}
	return false
}

// shorten trims a string for a message.
func shorten(s string, max int) string {
	s = OneLine(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

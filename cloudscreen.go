package main

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// pane is one column of the cloud screen.
type pane int

const (
	paneStatements pane = iota
	paneCloud
	paneChallenge
	paneProcess
)

// cloudPanes is the switch order for the pane keys.
var cloudPanes = []pane{paneStatements, paneCloud, paneChallenge, paneProcess}

func (p pane) String() string {
	switch p {
	case paneCloud:
		return "cloud"
	case paneChallenge:
		return "challenge"
	case paneProcess:
		return "process"
	}
	return "statements"
}

// next cycles through the panes in either direction.
func (p pane) next(delta int) pane {
	index := 0
	for i, candidate := range cloudPanes {
		if candidate == p {
			index = i
		}
	}
	index = (index + delta + len(cloudPanes)) % len(cloudPanes)
	return cloudPanes[index]
}

// cloudScreen is the editor for one cloud: the statement slots, the active pane
// and the cursor that says which slot is being worked on.
type cloudScreen struct {
	cloud          Cloud
	pane           pane
	cursor         int         // statement slot
	paneWidth      int         // column the pane was last drawn in
	challengeArrow int         // which readable arrow the form shows
	challengeSlot  claimCursor // which row of the form has the cursor
}

func newCloudScreen(c Cloud) cloudScreen { return cloudScreen{cloud: c} }

func (cs *cloudScreen) move(delta, count int) {
	cs.cursor += delta
	cs.cursor = max(0, min(cs.cursor, count-1))
}

// renderCloud lays out the cloud screen: the figure beside the active pane when
// the window is wide enough (see diagram.go), the pane alone otherwise.
func (a *App) renderCloud() string {
	height := a.bodyHeight()

	if a.splitView() && a.cv.pane != paneCloud {
		figureWidth := a.diagramWidth()
		if figure := a.renderDiagram(figureWidth, height); figure != "" {
			const gutter = 2
			paneWidth := a.width - figureWidth - gutter
			a.cv.paneWidth = paneWidth
			left := fitLines(figure, height, figureWidth)
			right := fitLines(a.renderPane(paneWidth, height), height, paneWidth)
			gap := strings.Repeat(" ", gutter)
			joined := make([]string, height)
			for i := range joined {
				// Canvas rows are trimmed, so each column is padded back to its
				// full width: otherwise the right column slides left wherever
				// the figure ends early.
				joined[i] = padRight(left[i], figureWidth) + gap + padRight(right[i], paneWidth)
			}
			return strings.Join(joined, "\n")
		}
	}
	a.cv.paneWidth = a.width
	return a.renderPane(a.width, height)
}

// renderPane draws the active pane into the given box.
func (a *App) renderPane(width, height int) string {
	switch a.cv.pane {
	case paneCloud:
		return a.renderDiagramPane(width, height)
	case paneChallenge:
		return a.renderChallenge(width, height)
	case paneProcess:
		return a.renderProcess(width, height)
	default:
		return a.renderStatements(width, height)
	}
}

// renderDiagramPane shows the figure alone, centred, for narrow windows.
func (a *App) renderDiagramPane(width, height int) string {
	figureWidth := min(width, 92)
	figure := a.renderDiagram(figureWidth, height)
	if figure == "" {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center,
			a.styles.Dim.Render("The diagram needs a wider window (at least 44 columns)."))
	}

	indent := max(0, (width-figureWidth)/2)
	lines := strings.Split(figure, "\n")
	for i, line := range lines {
		lines[i] = strings.Repeat(" ", indent) + line
	}
	return strings.Join(lines, "\n")
}

// assumptionRow is one line of the assumptions pane: an arrow heading, or one
// assumption behind that arrow.
// challengeArrows are the arrows with something to say: both statements
// written, or a claim already recorded against them.
func (a *App) challengeArrows() []Arrow {
	var arrows []Arrow
	for _, arrow := range Arrows() {
		if a.arrowReading(arrow) == "" && a.cv.cloud.CountFor(arrow) == 0 {
			continue
		}
		arrows = append(arrows, arrow)
	}
	return arrows
}

// challengeArrow is the arrow the form is showing.
func (a *App) challengeArrow() (Arrow, bool) {
	arrows := a.challengeArrows()
	if len(arrows) == 0 {
		return "", false
	}
	return arrows[max(0, min(a.cv.challengeArrow, len(arrows)-1))], true
}

// challengeAssumptions are the claims behind the arrow on screen.
func (a *App) challengeAssumptions() []Assumption {
	arrow, ok := a.challengeArrow()
	if !ok {
		return nil
	}
	return a.cv.cloud.AssumptionsFor(arrow)
}

// claimSlot is one labelled row of a claim.
type claimSlot int

const (
	slotBecause claimSlot = iota
	slotEvidence
	slotInstead
	slotWhy
)

// claimCursor points at one row: the claim it belongs to and which slot of it.
type claimCursor struct {
	claim int
	slot  claimSlot
}

// claimSlots lists the rows on screen, in the order they are drawn. The why row
// only exists once there is something to explain.
func (a *App) claimSlots() []claimCursor {
	claims := a.challengeAssumptions()
	slots := make([]claimCursor, 0, len(claims)*4)
	for i, as := range claims {
		slots = append(slots,
			claimCursor{claim: i, slot: slotBecause},
			claimCursor{claim: i, slot: slotEvidence},
			claimCursor{claim: i, slot: slotInstead})
		if a.cv.cloud.InjectionBreaking(as.ID) != nil {
			slots = append(slots, claimCursor{claim: i, slot: slotWhy})
		}
	}
	return slots
}

// focusCursor puts the cursor on a row that exists.
func (a *App) focusCursor(c claimCursor) {
	slots := a.claimSlots()
	if len(slots) == 0 {
		a.cv.challengeSlot = claimCursor{}
		return
	}
	for _, slot := range slots {
		if slot == c {
			a.cv.challengeSlot = c
			return
		}
	}
	a.cv.challengeSlot = slots[0]
}

// moveSlot walks the rows of the form.
func (a *App) moveSlot(delta int) {
	slots := a.claimSlots()
	if len(slots) == 0 {
		return
	}
	position := 0
	for i, slot := range slots {
		if slot == a.cv.challengeSlot {
			position = i
			break
		}
	}
	position = max(0, min(position+delta, len(slots)-1))
	a.cv.challengeSlot = slots[position]
}

// currentClaim is the claim the cursor is inside.
func (a *App) currentClaim() (Assumption, bool) {
	claims := a.challengeAssumptions()
	if len(claims) == 0 {
		return Assumption{}, false
	}
	return claims[max(0, min(a.cv.challengeSlot.claim, len(claims)-1))], true
}

// renderChallenge is the workbench: one arrow, its sentence, and the claims
// behind it with the answer to each.
func (a *App) renderChallenge(width, height int) string {
	arrows := a.challengeArrows()
	if len(arrows) == 0 {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center,
			a.styles.Dim.Width(min(width, 46)).Align(lipgloss.Center).
				Render("No arrows yet.\n\nAn arrow appears once both of its statements are written."))
	}

	index := max(0, min(a.cv.challengeArrow, len(arrows)-1))
	arrow := arrows[index]

	head := []string{a.challengeStrip(arrows, index, width), ""}
	for _, line := range strings.Split(a.styles.Dim.Width(max(8, width-3)).Render(arrow.Reading(&a.cv.cloud)), "\n") {
		head = append(head, "  "+line)
	}
	head = append(head, "")

	claims := a.challengeAssumptions()
	if len(claims) == 0 {
		return strings.Join(append(head, a.emptyClaim(width)), "\n")
	}

	focused := a.cv.challengeSlot.claim
	if focused < 0 || focused >= len(claims) {
		focused = 0
	}
	blocks := make([]string, len(claims))
	heights := make([]int, len(claims))
	for i, as := range claims {
		hasFocus := i == focused && !a.addingClaim()
		blocks[i] = a.claimBlock(as, a.cv.challengeSlot.slot, hasFocus, width)
		heights[i] = lipgloss.Height(blocks[i])
	}

	if a.addingClaim() {
		blocks = append(blocks, a.claimField("  ", width))
		heights = append(heights, lipgloss.Height(blocks[len(blocks)-1]))
	}

	start, end := visibleRange(heights, focused, max(3, height-len(head)))
	return strings.Join(append(head, strings.Join(blocks[start:end], "\n")), "\n")
}

// challengeStrip is the arrow selector. All five arrows are always shown, so
// the row never changes shape as statements land: arrows that can be read are
// normal, the ones still waiting are dim, and the arrow on screen carries the
// marker and its claim count. The arrows on either end say which keys move.
func (a *App) challengeStrip(arrows []Arrow, current, width int) string {
	active, hasActive := a.challengeArrow()
	ready := make(map[Arrow]bool, len(arrows))
	for _, arrow := range arrows {
		ready[arrow] = true
	}

	position := 0
	parts := make([]string, 0, len(Arrows()))
	for i, arrow := range Arrows() {
		label := string(arrow)
		if count := a.cv.cloud.CountFor(arrow); count > 0 {
			label = fmt.Sprintf("%s %d", arrow, count)
		}

		switch {
		case hasActive && arrow == active:
			position = i + 1
			parts = append(parts, a.styles.Accent.Bold(true).Render("▸"+label))
		case ready[arrow]:
			parts = append(parts, a.styles.Dim.Render(label))
		default:
			parts = append(parts, a.styles.Faint.Render(label))
		}
	}

	left := "  " + a.styles.Faint.Render("←") + "  " + strings.Join(parts, a.styles.Faint.Render("   "))
	right := a.styles.Faint.Render(fmt.Sprintf("%d of %d", position, len(Arrows()))) +
		"  " + a.styles.Faint.Render("→")
	return fitLine(spread(left, right, width), width)
}

// claimTextColumn is where the words start on every labelled row, so the
// labels line up down the block.
const claimTextColumn = 12

// claimBlock renders one claim: what is asserted, how it was judged, and the
// answer to it.
func (a *App) claimBlock(as Assumption, focused claimSlot, hasFocus bool, width int) string {
	textWidth := max(8, width-claimTextColumn-1)

	mark := func(slot claimSlot) string {
		if hasFocus && slot == focused {
			return "▸ "
		}
		return "  "
	}
	label := func(slot claimSlot, name string) string {
		style := a.styles.CardLabel
		if hasFocus && slot == focused {
			style = a.styles.CardLabelFocused
		}
		return style.Render(name) + strings.Repeat(" ", max(0, claimTextColumn-2-lipgloss.Width(name)))
	}
	tone := func(slot claimSlot, base lipgloss.Style) lipgloss.Style {
		if hasFocus && slot == focused {
			return a.styles.Strong
		}
		return base
	}

	var rows []string

	// because: the claim itself, with its verdict on the right.
	if a.claimEditing(as.ID) {
		a.inline.Resize(textWidth)
		rows = append(rows, a.fieldRows(mark(slotBecause), "because", width)...)
	} else {
		lines := wrapText(as.Text, textWidth, 4)
		first := mark(slotBecause) + label(slotBecause, "because") +
			tone(slotBecause, a.styles.CardValue).Render(lines[0])
		if as.Verdict != Unassessed {
			verdict := a.styles.VerdictStyle(as.Verdict).Render(as.Verdict.Glyph() + " " + as.Verdict.String())
			if room := width - lipgloss.Width(first) - 1; room >= lipgloss.Width(verdict) {
				first = spread(first, verdict, width)
			} else {
				lines = append(lines, verdict)
			}
		}
		rows = append(rows, fitLine(first, width))
		for _, line := range lines[1:] {
			rows = append(rows, fitLine(strings.Repeat(" ", claimTextColumn)+
				tone(slotBecause, a.styles.CardValue).Render(line), width))
		}
	}

	// evidence
	switch {
	case a.editingField(editAssumptionEvidence, as.ID):
		a.inline.Resize(textWidth)
		rows = append(rows, a.fieldRows(mark(slotEvidence), "evidence", width)...)
	case strings.TrimSpace(as.Evidence) != "":
		for i, line := range wrapText(as.Evidence, textWidth, 2) {
			prefix := strings.Repeat(" ", claimTextColumn)
			if i == 0 {
				prefix = mark(slotEvidence) + label(slotEvidence, "evidence")
			}
			rows = append(rows, fitLine(prefix+tone(slotEvidence, a.styles.CardHint).Render(line), width))
		}
	default:
		rows = append(rows, fitLine(mark(slotEvidence)+label(slotEvidence, "evidence")+
			tone(slotEvidence, a.styles.CardPrompt).Render("none yet"), width))
	}

	answer := a.cv.cloud.InjectionBreaking(as.ID)

	// instead: the change that makes the claim irrelevant.
	switch {
	case a.editingField(editInjectionText, as.ID):
		a.inline.Resize(textWidth)
		rows = append(rows, a.fieldRows(mark(slotInstead), "instead", width)...)
	case answer == nil:
		rows = append(rows, fitLine(mark(slotInstead)+label(slotInstead, "instead")+
			tone(slotInstead, a.styles.CardPrompt).Render("none yet"), width))
	default:
		for i, line := range wrapText(answer.Text, textWidth, 2) {
			prefix := strings.Repeat(" ", claimTextColumn)
			if i == 0 {
				prefix = mark(slotInstead) + label(slotInstead, "instead")
			}
			rows = append(rows, fitLine(prefix+tone(slotInstead, a.styles.CardValue).Render(line), width))
		}
	}

	// why: only once there is an answer to explain.
	switch {
	case a.editingField(editInjectionRationale, as.ID):
		a.inline.Resize(textWidth)
		rows = append(rows, a.fieldRows(mark(slotWhy), "why", width)...)
	case answer != nil && strings.TrimSpace(answer.Rationale) != "":
		for i, line := range wrapText(answer.Rationale, textWidth, 2) {
			prefix := strings.Repeat(" ", claimTextColumn)
			if i == 0 {
				prefix = mark(slotWhy) + label(slotWhy, "why")
			}
			rows = append(rows, fitLine(prefix+tone(slotWhy, a.styles.CardHint).Render(line), width))
		}
	}

	return strings.Join(rows, "\n")
}

// emptyClaim is the block shown before the first claim is written: the shape of
// the work, without any prose around it.
func (a *App) emptyClaim(width int) string {
	if a.addingClaim() {
		return a.claimField("  ", width)
	}
	return fitLine("  "+a.styles.CardLabel.Render("because")+"   "+
		a.styles.CardPrompt.Render("what has to be true for this arrow?"), width)
}

// fieldRows renders the inline field under its label, so every piece of a
// claim is typed in the slot it belongs to.
func (a *App) fieldRows(marker, label string, width int) []string {
	lines := strings.Split(a.inline.View(), "\n")
	pad := max(0, claimTextColumn-lipgloss.Width(marker)-lipgloss.Width(label))
	rows := []string{fitLine(marker+a.styles.CardLabel.Render(label)+
		strings.Repeat(" ", pad)+lines[0], width)}
	for _, line := range lines[1:] {
		rows = append(rows, fitLine(strings.Repeat(" ", claimTextColumn)+line, width))
	}
	return rows
}

// claimFieldLines is the because slot.
func (a *App) claimFieldLines(marker string, width int) []string {
	return a.fieldRows(marker, "because", width)
}

// editingField reports whether a particular slot is being typed in.
func (a *App) editingField(kind editKind, claimID string) bool {
	return a.inline.open && a.inline.target.kind == kind && a.inline.target.claimID == claimID
}

// claimField is the same thing as a block of its own.
func (a *App) claimField(marker string, width int) string {
	return strings.Join(a.claimFieldLines(marker, width), "\n")
}

// claimEditing reports whether this claim's words are being typed.
func (a *App) claimEditing(id string) bool {
	return a.inline.open && a.inline.target.kind == editAssumptionText && a.inline.target.id == id
}

// addingClaim reports whether a new claim is being typed.
func (a *App) addingClaim() bool {
	return a.inline.open && a.inline.target.kind == editAssumptionText && a.inline.target.id == ""
}

// arrowReading is the necessity sentence, or "" while it could only be made of
// placeholders.
func (a *App) arrowReading(arrow Arrow) string {
	from, to := arrow.Ends()
	if a.cv.cloud.Text(from) == "" || a.cv.cloud.Text(to) == "" {
		return ""
	}
	return arrow.Reading(&a.cv.cloud)
}

func (a *App) renderProcess(width, height int) string {
	c := &a.cv.cloud
	var lines []string

	lines = append(lines, " "+a.styles.HelpSection.Render("Process"))
	for _, step := range c.Steps() {
		mark := a.styles.Faint.Render("[ ]")
		titleStyle := a.styles.Strong
		if step.Done {
			mark = a.styles.Success.Render("[x]")
			titleStyle = a.styles.Dim
		}
		title := titleStyle.Render(fmt.Sprintf("%d. %s", step.Number, step.Title))

		// The concrete state when a stage is done, the way forward when it is
		// not. Wrapped rather than cut, so the evidence stays readable.
		head := " " + mark + " " + title + "  "
		room := max(8, width-lipgloss.Width(head))
		detail := wrapText(a.processDetail(step), room, 2)
		lines = append(lines, fitLine(head+a.styles.Faint.Render(detail[0]), width))
		for _, extra := range detail[1:] {
			lines = append(lines, fitLine(strings.Repeat(" ", lipgloss.Width(head))+
				a.styles.Faint.Render(extra), width))
		}
	}

	if issues := c.Validate(); len(issues) > 0 {
		lines = append(lines, "",
			" "+a.styles.HelpSection.Render(fmt.Sprintf("Issues (%d)", len(issues))))
		for _, issue := range issues {
			lines = append(lines, " "+a.severityMark(issue.Severity)+" "+
				a.styles.Dim.Render(truncate(issue.Message, max(8, width-4))))
		}
	}

	return strings.Join(lines, "\n")
}

// processDetail says specifically where a stage stands: the evidence when it is
// done, the missing piece when it is not.
func (a *App) processDetail(step Step) string {
	c := &a.cv.cloud

	switch step.Number {
	case 1:
		if missing := c.Missing(); len(missing) > 0 {
			names := make([]string, len(missing))
			for i, slot := range missing {
				names[i] = string(slot)
			}
			return "missing " + strings.Join(names, ", ")
		}
		filled, total := c.Progress()
		return fmt.Sprintf("%d of %d statements stated", filled, total)

	case 2:
		covered := 0
		for _, arrow := range Arrows() {
			if c.CountFor(arrow) > 0 {
				covered++
			}
		}
		return fmt.Sprintf("%d of %d arrows carry an assumption", covered, len(Arrows()))

	case 3:
		weak := c.Weakest()
		if weak == nil {
			return "no assumption challenged yet"
		}
		return fmt.Sprintf("%s %s: %s", weak.Arrow, weak.Verdict, weak.Text)

	case 4:
		for _, as := range c.BrokenAssumptions() {
			if injection := c.InjectionBreaking(as.ID); injection != nil {
				return injection.Text
			}
		}
		return "nothing injected against a broken assumption"

	case 5:
		if c.Verified {
			return "the cloud is confirmed dissolved"
		}
		return "not confirmed yet"
	}
	return step.Hint
}

// severityMark is the glyph for a validation finding.
func (a *App) severityMark(severity Severity) string {
	switch severity {
	case SeverityError:
		return a.styles.Danger.Render("✗")
	case SeverityWarning:
		return a.styles.Warning.Render("⚠")
	}
	return a.styles.Info.Render("ℹ")
}

// renderStatements draws the five statement cards, scrolled to the cursor.
// renderStatements lays the sides row over the five cards.
func (a *App) renderStatements(width, height int) string {
	return a.sidesRow(width) + "\n\n" + a.cardColumn(width, max(3, height-2))
}

// partyFieldWidth is the room a side name gets while it is being typed.
const partyFieldWidth = 20

// sidesRow names the two parties. s edits each name in place, one field at a
// time, so the pane and the cards stay where they are.
func (a *App) sidesRow(width int) string {
	label := a.styles.CardLabel.Render("sides")
	arrow := a.styles.Faint.Render("  ⇄  ")

	name := func(index int, party string) string {
		if a.partyEditing(index) {
			a.inline.Resize(partyFieldWidth)
			return a.inline.View()
		}
		return a.styles.CardValue.Render(truncate(party, partyFieldWidth))
	}

	first := a.cv.cloud.Party(1)
	if a.party.index == 1 {
		first = a.party.first
	}
	row := "  " + label + "  " + name(0, first) + arrow + name(1, a.cv.cloud.Party(2))
	return fitLine(row, width)
}

// partyEditing reports whether one of the two name fields is being typed in.
func (a *App) partyEditing(index int) bool {
	return a.inline.open && a.inline.target.kind == editParty && a.party.index == index
}

// cardColumn draws the five statement cards, scrolled to the cursor.
func (a *App) cardColumn(width, height int) string {
	infos := SlotInfos()
	cards := make([]string, len(infos))
	heights := make([]int, len(infos))
	for i, info := range infos {
		cards[i] = a.slotCard(info, i == a.cv.cursor, width)
		heights[i] = lipgloss.Height(cards[i])
	}

	start, end := visibleRange(heights, a.cv.cursor, height)
	// Cards carry their own borders, so they stack without a gap and all five
	// slots stay visible on a normal terminal.
	return strings.Join(cards[start:end], "\n")
}

// slotCard renders one statement. An empty slot shows the question instead of
// nothing, so the shape of the cloud teaches while it is being filled in.
// cardWidth is the outer width of a statement card.
func cardWidth(paneWidth int) int { return max(28, min(paneWidth, 104)) }

// slotCard draws one statement as a card whose label sits in the top border,
// the same way the nodes of the figure are drawn. An empty slot shows the whole
// prompt, guidance included; a filled one shows only what was written.
func (a *App) slotCard(info SlotInfo, selected bool, width int) string {
	width = cardWidth(width)
	inner := width - 4 // a border and a padding column on each side

	borderStyle := a.styles.Faint
	labelStyle := a.styles.CardLabel
	marker := "  "
	if selected {
		borderStyle = a.styles.Accent
		labelStyle = a.styles.CardLabelFocused
		marker = "▸ "
	}

	label := fmt.Sprintf("%s · %s", info.Slot, info.Name)
	if info.Side != 0 {
		label = fmt.Sprintf("%s · %s (%s)", info.Slot, info.Name, a.cv.cloud.Party(info.Side))
	}
	label = truncate(marker+label, max(8, inner-6))

	pad := max(0, width-5-lipgloss.Width(label))
	lines := []string{
		borderStyle.Render("╭─ ") + labelStyle.Render(label) +
			borderStyle.Render(" "+strings.Repeat("─", pad)+"╮"),
	}

	var body string
	switch {
	case a.inline.open && a.inline.target.kind == editStatement && a.inline.target.slot == info.Slot:
		// The field is always the width of the card it sits in, whatever sized
		// it when the edit began.
		a.inline.Resize(inner)
		body = a.inline.View()
	case a.cv.cloud.Text(info.Slot) != "":
		body = a.styles.CardValue.Width(inner).Render(a.cv.cloud.Text(info.Slot))
	default:
		body = a.styles.CardPrompt.Width(inner).Render(info.Prompt)
	}

	for row := range strings.SplitSeq(body, "\n") {
		lines = append(lines, borderStyle.Render("│")+" "+row+" "+borderStyle.Render("│"))
	}
	lines = append(lines, borderStyle.Render("╰"+strings.Repeat("─", width-2)+"╯"))
	return strings.Join(lines, "\n")
}

// The figure is drawn on a rune grid in fixed rows: the objective, a stub and
// a fan-out, the two needs, connectors, then the two wants with the conflict
// arrow between them. Connectors land on a ┬ or ┴ in the border they meet, so
// the drawing stays connected even when the node texts are different lengths.

// Style ids used by the diagram canvas. They index diagramPalette.
const (
	diagPlain = iota
	diagBorder
	diagObjective
	diagSideOne
	diagSideTwo
	diagPlaceholder
	diagDanger
)

// diagramMinWidth is the narrowest column that still holds two readable nodes
// with the conflict arrow between them.
const diagramMinWidth = 44

// maxNodeLines caps a node's statement so one long sentence cannot push the
// figure off the screen.
const maxNodeLines = 3

// cell is one styled rune on the canvas.
type cell struct {
	r     rune
	style int
}

// canvas is a grid of styled runes. Style is tracked per cell so runs of equal
// style can be wrapped in escape sequences once, at render time.
type canvas struct {
	width, height int
	cells         []cell
}

func newCanvas(width, height int) *canvas {
	c := &canvas{width: width, height: height, cells: make([]cell, width*height)}
	for i := range c.cells {
		c.cells[i] = cell{r: ' '}
	}
	return c
}

func (c *canvas) set(x, y int, r rune, style int) {
	if x < 0 || y < 0 || x >= c.width || y >= c.height {
		return
	}
	c.cells[y*c.width+x] = cell{r: r, style: style}
}

func (c *canvas) text(x, y int, s string, style int) {
	for _, r := range s {
		c.set(x, y, r, style)
		x++
	}
}

func (c *canvas) hline(x0, x1, y int, r rune, style int) {
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	for x := x0; x <= x1; x++ {
		c.set(x, y, r, style)
	}
}

// render turns the grid into lines, grouping equal styles and trimming the
// trailing blanks that would otherwise paint the whole terminal.
func (c *canvas) render(palette []lipgloss.Style) []string {
	out := make([]string, 0, c.height)
	for y := range c.height {
		end := c.width
		for end > 0 && c.cells[y*c.width+end-1].r == ' ' {
			end--
		}

		var line strings.Builder
		for x := 0; x < end; x++ {
			style := c.cells[y*c.width+x].style
			runEnd := x
			for runEnd < end && c.cells[y*c.width+runEnd].style == style {
				runEnd++
			}

			var run strings.Builder
			for k := x; k < runEnd; k++ {
				run.WriteRune(c.cells[y*c.width+k].r)
			}
			if style > diagPlain && style < len(palette) {
				line.WriteString(palette[style].Render(run.String()))
			} else {
				line.WriteString(run.String())
			}
			x = runEnd - 1
		}
		out = append(out, line.String())
	}
	return out
}

// drawNode draws a labelled node box and returns its height. When junction is
// set the top and bottom borders carry a join character at the horizontal
// centre, so a connector lands on the border instead of floating above it.
func (c *canvas) drawNode(x, y, width int, title, text string, titleStyle, bodyStyle, borderStyle int, joinTop, joinBottom bool) int {
	inner := width - 4 // one border and one padding column on each side
	if inner < 4 {
		return 0
	}

	lines := wrapText(text, inner, maxNodeLines)
	height := len(lines) + 2
	if y+height > c.height {
		return 0
	}

	top := []rune("╭" + strings.Repeat("─", width-2) + "╮")
	if joinTop {
		if centre := width / 2; top[centre] == '─' {
			top[centre] = '┴'
		}
	}
	for i, r := range top {
		c.set(x+i, y, r, borderStyle)
	}
	// The label sits in the border, so it is written after the border runes.
	if name := truncate(title, inner-4); name != "" {
		c.text(x+2, y, " "+name+" ", titleStyle)
	}

	for i, line := range lines {
		row := y + 1 + i
		c.set(x, row, '│', borderStyle)
		c.text(x+2, row, padRight(line, inner), bodyStyle)
		c.set(x+3+inner, row, '│', borderStyle)
	}

	bottom := []rune("╰" + strings.Repeat("─", width-2) + "╯")
	if joinBottom {
		if centre := width / 2; bottom[centre] == '─' {
			bottom[centre] = '┬'
		}
	}
	for i, r := range bottom {
		c.set(x+i, y+height-1, r, borderStyle)
	}

	return height
}

// diagramPalette maps the canvas style ids onto the app's palette.
func (a *App) diagramPalette() []lipgloss.Style {
	return []lipgloss.Style{
		diagPlain:       lipgloss.NewStyle(),
		diagBorder:      a.styles.Faint,
		diagObjective:   a.styles.Accent,
		diagSideOne:     a.styles.SideOne,
		diagSideTwo:     a.styles.SideTwo,
		diagPlaceholder: a.styles.CardPrompt,
		diagDanger:      a.styles.Danger,
	}
}

// diagramWidth is the column the figure gets in the side-by-side layout.
func (a *App) diagramWidth() int {
	return min(max(a.width*52/100, diagramMinWidth), 74)
}

// splitView reports whether the diagram and a pane can sit side by side.
func (a *App) splitView() bool { return a.width >= 100 }

// renderDiagram draws the cloud: the objective with its two needs above, the
// two colliding wants below, and a ledger of assumptions per arrow. It returns
// "" when the space is too small to draw honestly.
func (a *App) renderDiagram(width, height int) string {
	if width < diagramMinWidth {
		return ""
	}

	c := a.cv.cloud
	boxWidth := min(30, (width-5)/2-4)
	if boxWidth < 14 {
		return ""
	}
	total := boxWidth + 4
	gap := width - 2*total
	if gap < 6 {
		return ""
	}

	hA := nodeHeight(c.Text(SlotA), boxWidth)
	hBC := max(nodeHeight(c.Text(SlotB), boxWidth), nodeHeight(c.Text(SlotC), boxWidth))
	hD := max(nodeHeight(c.Text(SlotD), boxWidth), nodeHeight(c.Text(SlotD2), boxWidth))
	canvasHeight := hA + 3 + hBC + 1 + hD
	if canvasHeight+2 > height {
		return ""
	}

	cv := newCanvas(width, canvasHeight)
	leftX, rightX := 0, width-total
	leftCX := leftX + total/2
	rightCX := rightX + total/2
	midCX := (leftCX + rightCX) / 2

	palette := a.diagramPalette()

	// The objective is the top of the figure, so only its bottom joins.
	cv.drawNode((width-total)/2, 0, total,
		a.nodeTitle(SlotA), a.nodeText(SlotA),
		diagObjective, diagPlain, diagBorder, false, true)

	y := hA
	cv.set(midCX, y, '│', diagBorder)
	y++
	cv.hline(leftCX+1, rightCX-1, y, '─', diagBorder)
	cv.set(leftCX, y, '┌', diagBorder)
	cv.set(rightCX, y, '┐', diagBorder)
	if leftCX < midCX && midCX < rightCX {
		cv.set(midCX, y, '┴', diagBorder)
	}
	y++
	cv.set(leftCX, y, '│', diagBorder)
	cv.set(rightCX, y, '│', diagBorder)
	y++

	needsTop := y
	hB := cv.drawNode(leftX, needsTop, total,
		a.nodeTitle(SlotB), a.nodeText(SlotB),
		diagSideOne, diagPlain, diagBorder, true, true)
	hC := cv.drawNode(rightX, needsTop, total,
		a.nodeTitle(SlotC), a.nodeText(SlotC),
		diagSideTwo, diagPlain, diagBorder, true, true)

	// The two needs are different lengths, so each connector starts just below
	// its own node's bottom border and runs down to the row above the wants.
	for row := needsTop + hB; row < needsTop+hBC; row++ {
		cv.set(leftCX, row, '│', diagBorder)
	}
	for row := needsTop + hC; row < needsTop+hBC; row++ {
		cv.set(rightCX, row, '│', diagBorder)
	}
	y = needsTop + hBC

	// The wants are the bottom of the figure, so nothing joins below them.
	cv.drawNode(leftX, y, total,
		a.nodeTitle(SlotD), a.nodeText(SlotD),
		diagSideOne, diagPlain, diagBorder, true, false)
	cv.drawNode(rightX, y, total,
		a.nodeTitle(SlotD2), a.nodeText(SlotD2),
		diagSideTwo, diagPlain, diagBorder, true, false)

	// The conflict: arrows pushing against each other across the gap.
	conflictRow := y + hD/2
	from, to := leftX+total, rightX-1
	if to-from >= 3 {
		cv.set(from, conflictRow, '◄', diagDanger)
		cv.set(to, conflictRow, '►', diagDanger)
		cv.hline(from+1, to-1, conflictRow, '─', diagDanger)
		if mid := (from + to) / 2; mid > from && mid < to {
			cv.set(mid, conflictRow, '×', diagDanger)
		}
	}

	lines := cv.render(palette)
	return strings.Join(append(lines, a.diagramLedger(width)...), "\n")
}

// nodeTitle labels a node with its slot, role and side.
func (a *App) nodeTitle(s Slot) string {
	info := SlotOf(s)
	if info.Side == 0 {
		return fmt.Sprintf("%s · %s", info.Slot, info.Name)
	}
	return fmt.Sprintf("%s · %s (%s)", info.Slot, info.Name, a.cv.cloud.Party(info.Side))
}

func (a *App) nodeText(s Slot) string {
	if text := a.cv.cloud.Text(s); text != "" {
		return text
	}
	return "not yet stated"
}

// diagramLedger reports how many assumptions sit behind each arrow, coloured by
// the worst verdict on it: the diagram doubles as a progress map.
func (a *App) diagramLedger(width int) []string {
	var line strings.Builder
	line.WriteString(a.styles.Dim.Render("assumptions "))
	for i, arrow := range Arrows() {
		if i > 0 {
			line.WriteString(a.styles.Faint.Render(" · "))
		}
		count := a.cv.cloud.CountFor(arrow)
		style := a.styles.Faint
		switch worstVerdict(a.cv.cloud.AssumptionsFor(arrow)) {
		case Invalid:
			style = a.styles.Danger
		case Questionable:
			style = a.styles.Warning
		case Holds:
			style = a.styles.Success
		default:
			if count > 0 {
				style = a.styles.Dim
			}
		}
		line.WriteString(style.Render(fmt.Sprintf("%s %d", arrow, count)))
	}
	return []string{"", fitLine(line.String(), width)}
}

// worstVerdict ranks the assumptions on one arrow, invalid first.
func worstVerdict(assumptions []Assumption) Verdict {
	worst := Unassessed
	for _, as := range assumptions {
		switch {
		case as.Verdict == Invalid:
			return Invalid
		case as.Verdict == Questionable:
			worst = Questionable
		case as.Verdict == Holds && worst == Unassessed:
			worst = Holds
		}
	}
	return worst
}

// nodeHeight is the height a node will need for its text at this width.
func nodeHeight(text string, inner int) int {
	if strings.TrimSpace(text) == "" {
		text = "not yet stated"
	}
	return len(wrapText(text, inner, maxNodeLines)) + 2
}

// wrapText greedily wraps words to width, capping the result at maxLines and
// marking the cut with an ellipsis.
func wrapText(text string, width, maxLines int) []string {
	words := strings.Fields(OneLine(text))
	if len(words) == 0 || width < 1 {
		return []string{""}
	}

	var lines []string
	current := ""
	for _, word := range words {
		for lipgloss.Width(word) > width {
			head := ansi.Truncate(word, width, "")
			lines = append(lines, head)
			word = word[len(head):]
		}

		candidate := word
		if current != "" {
			candidate = current + " " + word
		}
		if lipgloss.Width(candidate) <= width {
			current = candidate
			continue
		}
		if current != "" {
			lines = append(lines, current)
		}
		current = word
	}
	if current != "" {
		lines = append(lines, current)
	}

	if len(lines) > maxLines {
		lines = lines[:maxLines]
		last := lines[maxLines-1]
		lines[maxLines-1] = ansi.Truncate(last, max(width-1, 1), "") + "…"
	}
	return lines
}

// padRight pads to a display width, truncating anything that would overflow.
func padRight(s string, width int) string {
	w := lipgloss.Width(s)
	if w > width {
		return ansi.Truncate(s, width, "…")
	}
	return s + strings.Repeat(" ", width-w)
}

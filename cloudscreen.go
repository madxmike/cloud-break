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
	paneAssumptions
)

// cloudPanes is the switch order for the pane keys.
var cloudPanes = []pane{paneStatements, paneCloud, paneAssumptions}

func (p pane) String() string {
	switch p {
	case paneCloud:
		return "cloud"
	case paneAssumptions:
		return "assumptions"
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
	cloud            Cloud
	pane             pane
	cursor           int // statement slot
	assumptionCursor int // row in the assumptions pane
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
	return a.renderPane(a.width, height)
}

// renderPane draws the active pane into the given box.
func (a *App) renderPane(width, height int) string {
	switch a.cv.pane {
	case paneCloud:
		return a.renderDiagramPane(width, height)
	case paneAssumptions:
		return a.renderAssumptions(width, height)
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
type assumptionRow struct {
	arrow Arrow
	id    string // empty on a heading
}

// assumptionRows flattens the cloud into headings and assumptions, so a single
// cursor can walk the whole argument.
func (a *App) assumptionRows() []assumptionRow {
	var rows []assumptionRow
	for _, arrow := range Arrows() {
		rows = append(rows, assumptionRow{arrow: arrow})
		for _, as := range a.cv.cloud.AssumptionsFor(arrow) {
			rows = append(rows, assumptionRow{arrow: arrow, id: as.ID})
		}
	}
	return rows
}

// renderAssumptions draws every arrow with the assumptions behind it.
func (a *App) renderAssumptions(width, height int) string {
	rows := a.assumptionRows()
	if len(rows) == 0 {
		return ""
	}

	selected := max(0, min(a.cv.assumptionCursor, len(rows)-1))
	lines := make([]string, len(rows))
	heights := make([]int, len(rows))
	for i, row := range rows {
		lines[i] = a.assumptionLine(row, i == selected, width)
		heights[i] = 1
	}

	start, end := visibleRange(heights, selected, height)
	return strings.Join(lines[start:end], "\n")
}

func (a *App) assumptionLine(row assumptionRow, selected bool, width int) string {
	if row.id == "" {
		return a.arrowHeading(row.arrow, selected, width)
	}
	as := a.cv.cloud.AssumptionByID(row.id)
	if as == nil {
		return ""
	}
	return a.assumptionText(*as, selected, width)
}

// arrowHeading shows the sentence an arrow stands for, so the "because" being
// added has a meaning in the argument.
func (a *App) arrowHeading(arrow Arrow, selected bool, width int) string {
	badge := a.styles.Accent.Bold(true).Render(string(arrow))
	if count := a.cv.cloud.CountFor(arrow); count > 0 {
		badge += a.styles.Faint.Render(fmt.Sprintf(" %d", count))
	}

	// The full "in order to ..." sentence is in the footer. Here the heading
	// carries the statement the arrow rests on, which keeps every row distinct
	// even when the readings share a long prefix.
	room := max(8, width-lipgloss.Width(badge)-4)
	line := " " + badge + "  " + a.styles.Dim.Render(truncate(a.arrowSubject(arrow), room))
	line = fitLine(line, width)
	if selected {
		return a.styles.RowSelected.Width(width).Render(line)
	}
	return line
}

// arrowSubject names the condition an arrow rests on.
func (a *App) arrowSubject(arrow Arrow) string {
	if arrow == ArrowDD2 {
		return "the two wants cannot both happen"
	}
	from, _ := arrow.Ends()
	if text := a.cv.cloud.Text(from); text != "" {
		return text
	}
	return "not yet stated"
}

// assumptionText is one assumption: verdict glyph, the "because", and a mark
// when evidence has been recorded for it.
func (a *App) assumptionText(as Assumption, selected bool, width int) string {
	marker := "   "
	if selected {
		marker = " ▸ "
	}
	glyph := a.styles.VerdictStyle(as.Verdict).Render(as.Verdict.Glyph())

	evidence := ""
	if strings.TrimSpace(as.Evidence) != "" {
		evidence = " " + a.styles.Faint.Render("≡")
	}

	room := max(8, width-lipgloss.Width(marker)-lipgloss.Width(glyph)-lipgloss.Width(evidence)-3)
	text := a.styles.CardValue
	if selected {
		text = a.styles.Strong
	}
	return fitLine(marker+glyph+" "+text.Render(truncate(as.Text, room))+evidence, width)
}

// renderStatements draws the five statement cards, scrolled to the cursor.
func (a *App) renderStatements(width, height int) string {
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
func (a *App) slotCard(info SlotInfo, selected bool, width int) string {
	width = max(24, min(width, 104))
	inner := width - 4 // two border columns, two padding columns

	label := fmt.Sprintf("%s · %s", info.Slot, info.Name)
	if info.Side != 0 {
		label = fmt.Sprintf("%s · %s (%s)", info.Slot, info.Name, a.cv.cloud.Party(info.Side))
	}
	marker := "  "
	if selected {
		marker = "▸ "
	}

	labelStyle := a.styles.CardLabel
	if selected {
		labelStyle = a.styles.CardLabelFocused
	}
	rows := []string{labelStyle.Render(marker + truncate(label, inner-2))}

	if value := a.cv.cloud.Text(info.Slot); value != "" {
		rows = append(rows, a.styles.CardValue.Width(inner).Render(value))
	} else {
		rows = append(rows, a.styles.CardPrompt.Width(inner).Render(info.Prompt))
	}

	style := a.styles.Card
	if selected {
		style = a.styles.CardFocused
	}
	// Width() is the total block width, so the padding and border sit inside it.
	return style.Width(width).Render(strings.Join(rows, "\n"))
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

package main

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// cloudScreen is the editor for one cloud: the five statement slots and the
// cursor that says which one is being worked on.
type cloudScreen struct {
	cloud  Cloud
	cursor int
}

func newCloudScreen(c Cloud) cloudScreen { return cloudScreen{cloud: c} }

func (cs *cloudScreen) move(delta, count int) {
	cs.cursor += delta
	cs.cursor = max(0, min(cs.cursor, count-1))
}

// cardWidth is the outer width of a statement card.
func (a *App) cardWidth() int {
	w := min(a.width-2, 104)
	if w < 24 {
		w = a.width - 2
	}
	return w
}

// slotCard renders one statement. An empty slot shows the question instead of
// nothing, so the shape of the cloud teaches while it is being filled in.
func (a *App) slotCard(info SlotInfo, selected bool) string {
	width := a.cardWidth()
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
	rows := []string{labelStyle.Render(marker + label)}

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

// renderCloud draws the statement cards, scrolled so the cursor stays visible.
func (a *App) renderCloud() string {
	infos := SlotInfos()
	cards := make([]string, len(infos))
	for i, info := range infos {
		cards[i] = a.slotCard(info, i == a.cv.cursor)
	}

	heights := make([]int, len(cards))
	for i, card := range cards {
		heights[i] = lipgloss.Height(card)
	}

	start, end := visibleRange(heights, a.cv.cursor, a.bodyHeight())
	// Cards carry their own borders, so they stack without a gap and all five
	// slots stay visible on a normal terminal.
	return strings.Join(cards[start:end], "\n")
}

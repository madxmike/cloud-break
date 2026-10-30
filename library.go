package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

// library is the list of saved clouds and the cursor into it.
type library struct {
	clouds  []Cloud
	cursor  int
	loading bool
}

// set replaces the list after a read from disk.
func (l *library) set(clouds []Cloud) {
	l.clouds = clouds
	l.loading = false
	l.clamp()
}

func (l *library) clamp() {
	if len(l.clouds) == 0 {
		l.cursor = 0
		return
	}
	l.cursor = max(0, min(l.cursor, len(l.clouds)-1))
}

func (l *library) move(delta int) {
	l.cursor += delta
	l.clamp()
}

func (l *library) selected() (Cloud, bool) {
	if l.cursor < 0 || l.cursor >= len(l.clouds) {
		return Cloud{}, false
	}
	return l.clouds[l.cursor], true
}

func (l *library) remove(id ID) {
	out := l.clouds[:0]
	for _, c := range l.clouds {
		if c.ID != id {
			out = append(out, c)
		}
	}
	l.clouds = out
	l.clamp()
}

// upsert keeps the list in step with an edit without a disk round trip. The
// cursor follows the cloud it was on, even when the resort moves the row.
func (l *library) upsert(c Cloud) {
	previous, hadPrevious := l.selected()
	var follow ID
	if hadPrevious {
		follow = previous.ID
	}

	replaced := false
	for i := range l.clouds {
		if l.clouds[i].ID == c.ID {
			l.clouds[i] = c
			replaced = true
			break
		}
	}
	if !replaced {
		l.clouds = append(l.clouds, c)
	}
	l.sort()

	if follow != "" {
		for i, cloud := range l.clouds {
			if cloud.ID == follow {
				l.cursor = i
				break
			}
		}
	}
	l.clamp()
}

// sort orders clouds the way the store does: most recently updated first.
func (l *library) sort() {
	sort.Slice(l.clouds, func(i, j int) bool {
		a, b := l.clouds[i], l.clouds[j]
		if a.Updated.Equal(b.Updated) {
			return a.Title < b.Title
		}
		return a.Updated.After(b.Updated)
	})
}

// renderLibrary draws the list of clouds.
func (a *App) renderLibrary() string {
	if a.lib.loading {
		return lipgloss.Place(a.width, a.bodyHeight(), lipgloss.Center, lipgloss.Center,
			a.styles.Dim.Render("reading the library…"))
	}
	if len(a.lib.clouds) == 0 {
		return a.renderEmptyLibrary()
	}

	now := time.Now().UTC()
	rows := make([]string, 0, len(a.lib.clouds))
	for i, c := range a.lib.clouds {
		rows = append(rows, a.libraryRow(c, i == a.lib.cursor, now))
	}
	return strings.Join(rows, "\n")
}

// libraryRow is one cloud: status, title, then the facts used to choose it.
func (a *App) libraryRow(c Cloud, selected bool, now time.Time) string {
	filled, total := c.Progress()
	meta := fmt.Sprintf("%d/%d · %d asm · %s · %s",
		filled, total, len(c.Assumptions), c.Status(), humanAge(c.Updated, now))

	left := " " + a.styles.StatusStyle(c.Status()).Render(c.Status().Glyph()) +
		" " + truncate(c.Title, a.width-6)
	right := a.styles.RowMeta.Render(meta) + " "

	row := spread(left, right, a.width)
	if selected {
		return a.styles.RowSelected.Width(a.width).Render(row)
	}
	return a.styles.Row.Render(row)
}

// renderEmptyLibrary explains the tool instead of showing a blank screen.
func (a *App) renderEmptyLibrary() string {
	w := min(a.width-8, 58)
	blocks := []string{
		a.styles.Title.Render("No clouds yet."),
		a.styles.Dim.Width(w).Render("A cloud states a conflict as five statements, then breaks it by invalidating an assumption."),
		a.styles.Subtle.Width(w).Render(
			"A cloud is five statements: a shared objective, one need per side, and the two wants that collide. " +
				"You then surface the assumptions behind them until one of them breaks."),
	}
	return lipgloss.Place(a.width, a.bodyHeight(), lipgloss.Center, lipgloss.Center,
		strings.Join(blocks, "\n\n"))
}

// humanAge renders a timestamp as a compact age for the library rows.
func humanAge(t, now time.Time) string {
	if t.IsZero() {
		return "never"
	}
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return t.Local().Format("Jan 2")
	}
}

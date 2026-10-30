package main

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// boxFrame is the horizontal space a bordered, padded box spends on its own
// chrome: one border column and two padding columns on each side. Lipgloss v2
// counts these inside Style.Width, so content plus frame gives the total.
const boxFrame = 6

// Styles is the resolved palette for one terminal background. Every screen
// renders through it, so light and dark terminals both stay readable.
type Styles struct {
	Title   lipgloss.Style
	Strong  lipgloss.Style
	Dim     lipgloss.Style
	Faint   lipgloss.Style
	Accent  lipgloss.Style
	Danger  lipgloss.Style
	Warning lipgloss.Style
	Success lipgloss.Style
	Info    lipgloss.Style
	Subtle  lipgloss.Style

	// SideOne and SideTwo colour the two parties, so the diagram and the
	// assumption lists agree on who is who.
	SideOne lipgloss.Style
	SideTwo lipgloss.Style

	HeaderRule lipgloss.Style
	FootKey    lipgloss.Style
	FootDesc   lipgloss.Style

	Card             lipgloss.Style
	CardFocused      lipgloss.Style
	CardLabel        lipgloss.Style
	CardLabelFocused lipgloss.Style
	CardValue        lipgloss.Style
	CardPrompt       lipgloss.Style
	CardHint         lipgloss.Style

	Row         lipgloss.Style
	RowSelected lipgloss.Style
	RowMeta     lipgloss.Style

	// Tab and TabActive are the pane chips across the cloud screen; the active
	// chip is filled so the current pane reads at a glance.
	Tab       lipgloss.Style
	TabActive lipgloss.Style

	EditorBox   lipgloss.Style
	HelpBox     lipgloss.Style
	HelpSection lipgloss.Style
	HelpKey     lipgloss.Style
	HelpDesc    lipgloss.Style

	// Status is indexed by the Status value (draft, open, dissolving, dissolved).
	Status [4]lipgloss.Style
}

// StatusStyle returns the pill style for a cloud state.
func (s Styles) StatusStyle(st Status) lipgloss.Style {
	if int(st) < 0 || int(st) >= len(s.Status) {
		return s.Dim
	}
	return s.Status[st]
}

// VerdictStyle colours an assumption by the judgement recorded for it.
func (s Styles) VerdictStyle(v Verdict) lipgloss.Style {
	switch v {
	case Holds:
		return s.Success
	case Questionable:
		return s.Warning
	case Invalid:
		return s.Danger
	}
	return s.Faint
}

// NewStyles builds a palette. Accent hues are darkened for light terminals so
// text keeps its contrast instead of glowing out.
func NewStyles(isDark bool) Styles {
	ld := lipgloss.LightDark(isDark)
	var (
		accent  = ld(lipgloss.Color("#5B21B6"), lipgloss.Color("#A78BFA"))
		muted   = ld(lipgloss.Color("#52525B"), lipgloss.Color("#A1A1AA"))
		faint   = ld(lipgloss.Color("#71717A"), lipgloss.Color("#71717A"))
		danger  = ld(lipgloss.Color("#B91C1C"), lipgloss.Color("#F87171"))
		warning = ld(lipgloss.Color("#B45309"), lipgloss.Color("#FBBF24"))
		success = ld(lipgloss.Color("#15803D"), lipgloss.Color("#4ADE80"))
		info    = ld(lipgloss.Color("#1D4ED8"), lipgloss.Color("#60A5FA"))
		border  = ld(lipgloss.Color("#D4D4D8"), lipgloss.Color("#3F3F46"))
		sideOne = ld(lipgloss.Color("#1D4ED8"), lipgloss.Color("#60A5FA"))
		sideTwo = ld(lipgloss.Color("#A21CAF"), lipgloss.Color("#E879F9"))
		select_ = ld(lipgloss.Color("#EDE9FE"), lipgloss.Color("#2E1065"))
	)

	fg := func(c color.Color) lipgloss.Style { return lipgloss.NewStyle().Foreground(c) }

	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, 1)
	cardFocused := card.BorderForeground(accent)

	box := func() lipgloss.Style {
		return lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accent).
			Padding(1, 2)
	}

	return Styles{
		Title:   fg(accent).Bold(true),
		Strong:  lipgloss.NewStyle().Bold(true),
		Dim:     fg(muted),
		Faint:   fg(faint),
		Accent:  fg(accent),
		Danger:  fg(danger),
		Warning: fg(warning),
		Success: fg(success),
		Info:    fg(info),
		Subtle:  fg(muted).Italic(true),
		SideOne: fg(sideOne),
		SideTwo: fg(sideTwo),

		HeaderRule: fg(border),
		FootKey:    fg(accent).Bold(true),
		FootDesc:   fg(muted),

		Card:             card,
		CardFocused:      cardFocused,
		CardLabel:        fg(muted).Bold(true),
		CardLabelFocused: fg(accent).Bold(true),
		CardValue:        lipgloss.NewStyle(),
		CardPrompt:       fg(faint).Italic(true),
		CardHint:         fg(muted).Italic(true),

		Row:         lipgloss.NewStyle(),
		RowSelected: lipgloss.NewStyle().Background(select_).Bold(true),
		RowMeta:     fg(muted),

		Tab:       fg(muted).Padding(0, 1),
		TabActive: fg(accent).Bold(true).Background(select_).Padding(0, 1),

		EditorBox: box(),
		// The help sheet is dense, so it spends its rows on keys rather than
		// on vertical padding.
		HelpBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accent).
			Padding(0, 2),
		HelpSection: fg(accent).Bold(true),
		HelpKey:     fg(accent),
		HelpDesc:    fg(muted),

		Status: [4]lipgloss.Style{
			StatusDraft:      fg(muted),
			StatusOpen:       fg(info),
			StatusDissolving: fg(warning),
			StatusDissolved:  fg(success),
		},
	}
}

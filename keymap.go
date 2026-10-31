package main

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
)

// keyMap is every binding in the app. Matching and help text come from the same
// table, so a key cannot be handled without also being documented, and the
// footer, the help sheet and the handler can never drift apart.
type keyMap struct {
	Up   key.Binding
	Down key.Binding
	// Move is display-only: a single footer entry for the four movement keys,
	// which the handlers match separately so they know the direction.
	Move        key.Binding
	Open        key.Binding
	Back        key.Binding
	New         key.Binding
	Delete      key.Binding // the assumption or injection under the cursor
	DeleteCloud key.Binding
	Edit        key.Binding
	Add         key.Binding
	Verdict     key.Binding
	Evidence    key.Binding
	Inject      key.Binding
	Why         key.Binding
	Verify      key.Binding
	Title       key.Binding
	Sides       key.Binding
	Help        key.Binding
	Quit        key.Binding
	CtrlC       key.Binding
	NextPane    key.Binding
	PrevPane    key.Binding
	NextArrow   key.Binding
	PrevArrow   key.Binding
	// ArrowStep is display-only: one footer entry for stepping between arrows.
	ArrowStep key.Binding
	Dismiss   key.Binding
	Yes       key.Binding
	No        key.Binding

	// Pane holds the direct-jump keys, in the order of cloudPanes.
	Pane []key.Binding
}

func newKeyMap() keyMap {
	jump := func(number, name string) key.Binding {
		return key.NewBinding(key.WithKeys(number), key.WithHelp(number, name))
	}

	return keyMap{
		Up:          key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "move up")),
		Down:        key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "move down")),
		Move:        key.NewBinding(key.WithKeys("up", "k", "down", "j"), key.WithHelp("↑/k ↓/j", "move")),
		Open:        key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
		Back:        key.NewBinding(key.WithKeys("esc", "q"), key.WithHelp("esc/q", "back")),
		New:         key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new cloud")),
		Delete:      key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "delete")),
		DeleteCloud: key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete cloud")),

		Edit:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "edit")),
		Add:      key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add")),
		Verdict:  key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "verdict")),
		Evidence: key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "evidence")),
		Inject:   key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "instead")),
		Why:      key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "why")),
		Verify:   key.NewBinding(key.WithKeys("V"), key.WithHelp("V", "verify")),
		Title:    key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "title")),
		Sides:    key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "sides")),

		Help:      key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:      key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
		CtrlC:     key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("^C", "quit")),
		NextPane:  key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next pane")),
		PrevPane:  key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("⇧tab", "previous pane")),
		NextArrow: key.NewBinding(key.WithKeys("right"), key.WithHelp("→", "next arrow")),
		PrevArrow: key.NewBinding(key.WithKeys("left"), key.WithHelp("←", "previous arrow")),
		ArrowStep: key.NewBinding(key.WithKeys("left", "right"), key.WithHelp("←/→", "arrow")),
		Dismiss:   key.NewBinding(key.WithKeys("esc", "q", "?", "enter")),
		Yes:       key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "yes")),
		No:        key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "no")),

		Pane: []key.Binding{
			jump("1", "statements"),
			jump("2", "cloud"),
			jump("3", "assumptions"),
			jump("4", "process"),
		},
	}
}

// helpBarStyles paints the shared help model with this app's palette.
func helpBarStyles(s Styles) help.Styles {
	return help.Styles{
		ShortKey:       s.FootKey,
		ShortDesc:      s.FootDesc,
		ShortSeparator: s.Faint,
		FullKey:        s.HelpKey,
		FullDesc:       s.HelpDesc,
		FullSeparator:  s.Faint,
		Ellipsis:       s.Faint,
	}
}

// initHelpBar rebuilds the help model for the current palette and width.
func (a *App) initHelpBar() {
	a.helpBar = help.New()
	a.helpBar.ShortSeparator = " · "
	a.helpBar.Styles = helpBarStyles(a.styles)
	a.resizeHelpBar()
}

func (a *App) resizeHelpBar() {
	if a.width > 0 {
		a.helpBar.SetWidth(max(1, a.width-1))
	}
}

// footerBindings are the few keys worth showing for whatever is in front. The
// tabs already say which pane is open, so switching is not repeated here.
func (a *App) footerBindings() []key.Binding {
	k := a.keys

	if a.inline.open {
		return a.inline.bindings()
	}
	if a.ed.open {
		return a.ed.bindings()
	}

	if a.screen == screenLibrary {
		return []key.Binding{k.Move, k.Open, k.New, k.DeleteCloud, k.Help, k.Quit}
	}

	switch a.cv.pane {
	case paneChallenge:
		return []key.Binding{k.ArrowStep, k.Move, k.Verdict, k.Evidence, k.Inject, k.Add, k.Edit, k.Delete}
	case paneProcess:
		return []key.Binding{k.Verify, k.Back}
	case paneCloud:
		return []key.Binding{k.Back}
	}

	return []key.Binding{k.Move, k.Edit, k.Title, k.Sides, k.Back}
}

// helpSection is one titled group of bindings in the help sheet.
type helpSection struct {
	title string
	keys  []key.Binding
}

// helpSections is the full reference, grouped by where the keys apply.
func (a *App) helpSections() []helpSection {
	k := a.keys

	panes := make([]key.Binding, 0, len(k.Pane)+3)
	panes = append(panes, k.NextPane, k.PrevPane, k.ArrowStep)
	panes = append(panes, k.Pane...)
	panes = append(panes, k.Help)

	return []helpSection{
		{
			title: "Library",
			keys:  []key.Binding{k.Move, k.Open, k.New, k.DeleteCloud, k.Quit},
		},
		{
			title: "Cloud",
			keys: []key.Binding{
				k.Edit, k.Add, k.Verdict, k.Evidence, k.Inject, k.Why,
				k.Verify, k.Title, k.Sides, k.Delete, k.Back,
			},
		},
		{title: "Panes", keys: panes},
	}
}

// helpBlock renders one titled group of keys.
func (a *App) helpBlock(section helpSection) string {
	rows := []string{a.styles.HelpSection.Render(section.title)}
	for _, binding := range section.keys {
		if !binding.Enabled() {
			continue
		}
		rows = append(rows, "  "+a.styles.HelpKey.Width(9).Render(binding.Help().Key)+
			"  "+a.styles.HelpDesc.Render(binding.Help().Desc))
	}
	return strings.Join(rows, "\n")
}

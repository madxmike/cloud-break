package main

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// editorKeys are the bindings inside the modal. A single-line field saves on
// enter; a multi-line field leaves enter to the textarea and saves on ctrl+s.
type editorKeys struct {
	Save    key.Binding
	Newline key.Binding
	Cancel  key.Binding
}

func newEditorKeys(multiline bool) editorKeys {
	if multiline {
		return editorKeys{
			Save:    key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "save")),
			Newline: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "new line")),
			Cancel:  key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
		}
	}
	return editorKeys{
		Save:   key.NewBinding(key.WithKeys("enter", "ctrl+s"), key.WithHelp("enter", "save")),
		Cancel: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
	}
}

// editKind says which field an open editor writes to.
type editKind int

const (
	editCloudTitle editKind = iota
	editStatement
	editParties
	editAssumptionText
	editAssumptionEvidence
	editInjectionText
	editInjectionRationale
)

// editTarget identifies the field being edited. It is plain data rather than a
// closure so the editor stays inspectable and testable.
type editTarget struct {
	kind   editKind
	slot   Slot
	id     string   // record being edited, empty when adding one
	arrow  Arrow    // arrow a new assumption belongs to
	breaks []string // assumptions a new injection invalidates
}

// editResult is the outcome of one key press inside the editor.
type editResult int

const (
	editPending editResult = iota
	editSaved
	editCancelled
)

// editor is a modal text box shown over the current screen. Single-line fields
// save on enter; multi-line fields save on ctrl+s so enter can break lines.
type editor struct {
	open      bool
	ta        textarea.Model
	target    editTarget
	title     string
	hint      string
	width     int
	multiline bool
	keys      editorKeys
}

// editorTextareaStyles drops the textarea's filled background, so the modal
// inherits the terminal instead of painting a black box inside the border.
func editorTextareaStyles(isDark bool) textarea.Styles {
	palette := NewStyles(isDark)
	flat := lipgloss.NewStyle()

	st := textarea.DefaultStyles(isDark)
	st.Focused.Base = flat
	st.Blurred.Base = flat
	st.Focused.CursorLine = flat
	st.Blurred.CursorLine = flat
	st.Focused.Text = flat
	st.Focused.Prompt = palette.Accent
	st.Focused.Placeholder = palette.CardPrompt
	return st
}

func newEditor(isDark bool) editor {
	ta := textarea.New()
	ta.Prompt = "│ "
	ta.ShowLineNumbers = false
	ta.CharLimit = 1000
	ta.SetStyles(editorTextareaStyles(isDark))
	return editor{ta: ta}
}

// SetDark swaps the textarea palette when the terminal theme is known.
func (e *editor) SetDark(isDark bool) { e.ta.SetStyles(editorTextareaStyles(isDark)) }

// contentWidth is the textarea width inside the modal.
func (e *editor) contentWidth() int {
	w := min(e.width-12, 72)
	if w < 20 {
		w = max(20, e.width-8)
	}
	return w
}

// Resize keeps the textarea in step with the window.
func (e *editor) Resize(width int) {
	e.width = width
	if e.open {
		e.ta.SetWidth(e.contentWidth())
	}
}

// Open starts editing a field.
func (e *editor) Open(target editTarget, title, hint, value string, width int, multiline bool) tea.Cmd {
	e.open = true
	e.target = target
	e.title = title
	e.hint = hint
	e.width = width
	e.multiline = multiline
	e.keys = newEditorKeys(multiline)
	e.ta.SetValue(value)
	e.ta.SetWidth(e.contentWidth())
	e.ta.Placeholder = ""
	if multiline {
		e.ta.SetHeight(6)
		e.ta.KeyMap.InsertNewline.SetKeys("enter")
	} else {
		e.ta.SetHeight(2)
		e.ta.KeyMap.InsertNewline.SetKeys("ctrl+j")
	}
	return e.ta.Focus()
}

// Value is the text currently in the box.
func (e *editor) Value() string { return e.ta.Value() }

// Update handles one key press.
func (e *editor) Update(msg tea.KeyPressMsg) (editResult, tea.Cmd) {
	switch {
	case key.Matches(msg, e.keys.Cancel):
		e.open = false
		return editCancelled, nil
	case key.Matches(msg, e.keys.Save):
		e.open = false
		return editSaved, nil
	}

	// Anything not bound above, including enter in a multi-line field, belongs
	// to the textarea.
	var cmd tea.Cmd
	e.ta, cmd = e.ta.Update(msg)
	return editPending, cmd
}

// bindings are the keys the modal itself understands, for the footer and the
// hint line inside the box.
func (e *editor) bindings() []key.Binding {
	return []key.Binding{e.keys.Save, e.keys.Newline, e.keys.Cancel}
}

// View renders the modal centred in the body area.
func (e *editor) View(s Styles, bar help.Model, width, height int) string {
	w := e.contentWidth()
	rows := []string{s.Title.Render(e.title)}
	if e.hint != "" {
		rows = append(rows, s.Dim.Width(w).Render(e.hint))
	}
	rows = append(rows, "", e.ta.View(), "")

	rows = append(rows, fitLine(bar.ShortHelpView(e.bindings()), w))

	box := s.EditorBox.Width(w + boxFrame).Render(strings.Join(rows, "\n"))
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}

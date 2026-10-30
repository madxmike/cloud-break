package main

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// editKind says which field an open editor writes to.
type editKind int

const (
	editCloudTitle editKind = iota
	editStatement
	editParties
)

// editTarget identifies the field being edited. It is plain data rather than a
// closure so the editor stays inspectable and testable.
type editTarget struct {
	kind editKind
	slot Slot
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
	switch msg.String() {
	case "esc":
		e.open = false
		return editCancelled, nil
	case "ctrl+s":
		e.open = false
		return editSaved, nil
	case "enter":
		if !e.multiline {
			e.open = false
			return editSaved, nil
		}
	}
	var cmd tea.Cmd
	e.ta, cmd = e.ta.Update(msg)
	return editPending, cmd
}

// View renders the modal centred in the body area.
func (e *editor) View(s Styles, width, height int) string {
	w := e.contentWidth()
	rows := []string{s.Title.Render(e.title)}
	if e.hint != "" {
		rows = append(rows, s.Dim.Width(w).Render(e.hint))
	}
	rows = append(rows, "", e.ta.View(), "")

	if e.multiline {
		rows = append(rows, hints(s,
			hint{"ctrl+s", "save"},
			hint{"enter", "new line"},
			hint{"esc", "cancel"},
		))
	} else {
		rows = append(rows, hints(s,
			hint{"enter", "save"},
			hint{"esc", "cancel"},
		))
	}

	box := s.EditorBox.Width(w + boxFrame).Render(strings.Join(rows, "\n"))
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}

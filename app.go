package main

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// screen identifies which top-level view is showing.
type screen int

const (
	screenLibrary screen = iota
	screenCloud
)

const (
	minWidth  = 40
	minHeight = 12
)

// flashKind says whether a status message reports success or failure.
type flashKind int

const (
	flashNone flashKind = iota
	flashInfo
	flashError
)

// flash is a short message shown in the header until it expires.
type flash struct {
	text string
	kind flashKind
}

// cloudsLoadedMsg carries the result of reading the library from disk.
type cloudsLoadedMsg struct {
	clouds []Cloud
	err    error
}

// flashClearMsg retires a status message.
type flashClearMsg struct{}

var (
	quitCmd         tea.Cmd = func() tea.Msg { return tea.Quit() }
	queryBackground tea.Cmd = func() tea.Msg { return tea.RequestBackgroundColor() }
)

// confirmation is a yes/no question pinned to the footer.
type confirmation struct {
	question string
	onYes    func(*App) tea.Cmd
}

// App is the root model. It owns the library, the open cloud and the modal
// editor, and routes keys to whichever is active.
type App struct {
	store  *Store
	styles Styles
	isDark bool

	width  int
	height int

	screen screen
	lib    library
	cv     cloudScreen
	ed     editor

	help    bool
	flash   flash
	confirm *confirmation

	// pendingNew is set while the new-cloud title is being typed, so cancelling
	// the dialog cannot leave an empty cloud in the library.
	pendingNew bool
}

// NewApp builds the root model for a library.
func NewApp(store *Store) *App {
	a := &App{
		store:  store,
		isDark: true,
		styles: NewStyles(true),
		screen: screenLibrary,
	}
	a.lib.loading = true
	a.ed = newEditor(true)
	return a
}

func (a *App) Init() tea.Cmd {
	return tea.Batch(a.reloadLibrary(), queryBackground)
}

func (a *App) reloadLibrary() tea.Cmd {
	s := a.store
	return func() tea.Msg {
		clouds, err := s.List()
		return cloudsLoadedMsg{clouds: clouds, err: err}
	}
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		a.ed.Resize(a.width)
		return a, nil

	case tea.BackgroundColorMsg:
		a.setTheme(msg.IsDark())
		return a, nil

	case cloudsLoadedMsg:
		if msg.err != nil {
			a.lib.loading = false
			return a, a.setFlash("cannot read the library: "+msg.err.Error(), true)
		}
		a.lib.set(msg.clouds)
		return a, nil

	case flashClearMsg:
		a.flash = flash{}
		return a, nil

	case tea.KeyPressMsg:
		return a.handleKey(msg)
	}
	return a, nil
}

func (a *App) setTheme(isDark bool) {
	a.isDark = isDark
	a.styles = NewStyles(isDark)
	a.ed.SetDark(isDark)
}

func (a *App) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if match(msg, "ctrl+c") {
		return a, quitCmd
	}

	if a.help {
		if match(msg, "esc", "q", "?", "enter") {
			a.help = false
		}
		return a, nil
	}

	if a.confirm != nil {
		pending := a.confirm
		a.confirm = nil
		if match(msg, "y") {
			return a, pending.onYes(a)
		}
		return a, nil
	}

	if a.ed.open {
		result, cmd := a.ed.Update(msg)
		target, value := a.ed.target, a.ed.Value()
		switch result {
		case editSaved:
			// The editor closes itself; apply the value it committed.
			return a, tea.Batch(cmd, a.applyEdit(target, value))
		case editCancelled:
			return a, cmd
		}
		return a, cmd
	}

	if a.screen == screenLibrary {
		return a.handleLibraryKey(msg)
	}
	return a.handleCloudKey(msg)
}

func (a *App) handleLibraryKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case match(msg, "q"):
		return a, quitCmd

	case match(msg, "up", "k"):
		a.lib.move(-1)

	case match(msg, "down", "j"):
		a.lib.move(1)

	case match(msg, "enter"):
		if c, ok := a.lib.selected(); ok {
			a.openCloud(c)
		}

	case match(msg, "n"):
		a.pendingNew = true
		return a, a.ed.Open(editTarget{kind: editCloudTitle}, "New cloud",
			"Name the conflict, not the solution you have in mind.", "", a.width, false)

	case match(msg, "d"):
		if c, ok := a.lib.selected(); ok {
			a.askDeleteCloud(c)
		}

	case match(msg, "?"):
		a.help = true
	}
	return a, nil
}

func (a *App) handleCloudKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	count := len(SlotInfos())
	switch {
	case match(msg, "esc", "q"):
		a.screen = screenLibrary

	case match(msg, "up", "k"):
		a.cv.move(-1, count)

	case match(msg, "down", "j"):
		a.cv.move(1, count)

	case match(msg, "enter"):
		if a.cv.pane != paneStatements {
			a.cv.pane = paneStatements
			return a, nil
		}
		return a, a.editSlot()

	case match(msg, "tab"):
		a.cv.pane = a.cv.pane.next(1)

	case match(msg, "shift+tab"):
		a.cv.pane = a.cv.pane.next(-1)

	case match(msg, "1"):
		a.cv.pane = paneStatements

	case match(msg, "2"):
		a.cv.pane = paneCloud

	case match(msg, "t"):
		return a, a.ed.Open(editTarget{kind: editCloudTitle}, "Cloud title",
			"A short name for the conflict.", a.cv.cloud.Title, a.width, false)

	case match(msg, "s"):
		value := a.cv.cloud.Party(1) + "\n" + a.cv.cloud.Party(2)
		return a, a.ed.Open(editTarget{kind: editParties}, "Name the two sides",
			"One name per line: your side, then theirs.", value, a.width, true)

	case match(msg, "d"):
		a.askDeleteCloud(a.cv.cloud)

	case match(msg, "?"):
		a.help = true
	}
	return a, nil
}

// editSlot opens the editor on the highlighted statement.
func (a *App) editSlot() tea.Cmd {
	infos := SlotInfos()
	if a.cv.cursor < 0 || a.cv.cursor >= len(infos) {
		return nil
	}
	info := infos[a.cv.cursor]

	title := fmt.Sprintf("%s · %s", info.Slot, info.Name)
	if info.Side != 0 {
		title = fmt.Sprintf("%s · %s (%s)", info.Slot, info.Name, a.cv.cloud.Party(info.Side))
	}
	return a.ed.Open(editTarget{kind: editStatement, slot: info.Slot},
		title, info.Hint, a.cv.cloud.Text(info.Slot), a.width, false)
}

// applyEdit writes a saved value back into the model and persists it.
func (a *App) applyEdit(target editTarget, value string) tea.Cmd {
	switch target.kind {
	case editCloudTitle:
		if a.pendingNew {
			a.pendingNew = false
			c := New(value)
			if err := a.store.Create(*c); err != nil {
				return a.setFlash("could not create the cloud: "+err.Error(), true)
			}
			a.lib.upsert(*c)
			a.openCloud(*c)
			return a.setFlash("Created "+c.Title+". Start with the objective.", false)
		}
		a.cv.cloud.Title = OneLine(value)
		return a.persist()

	case editStatement:
		a.cv.cloud.SetText(target.slot, value)
		return a.persist()

	case editParties:
		lines := strings.Split(value, "\n")
		first, second := "", ""
		if len(lines) > 0 {
			first = lines[0]
		}
		if len(lines) > 1 {
			second = lines[1]
		}
		a.cv.cloud.SetParties(first, second)
		return a.persist()
	}
	return nil
}

// persist saves the open cloud and refreshes its library row.
func (a *App) persist() tea.Cmd {
	if a.screen != screenCloud {
		return nil
	}
	a.cv.cloud.Normalize()
	a.cv.cloud.Updated = time.Now().UTC()
	if err := a.store.Save(a.cv.cloud); err != nil {
		return a.setFlash("save failed: "+err.Error(), true)
	}
	a.lib.upsert(a.cv.cloud)
	return nil
}

func (a *App) openCloud(c Cloud) {
	a.cv = newCloudScreen(c)
	a.screen = screenCloud
	a.help = false
	a.confirm = nil
}

func (a *App) askDeleteCloud(c Cloud) {
	title, id := c.Title, c.ID
	a.confirm = &confirmation{
		question: fmt.Sprintf("Delete %q? This cannot be undone.", title),
		onYes: func(app *App) tea.Cmd {
			if err := app.store.Delete(id); err != nil {
				return app.setFlash("delete failed: "+err.Error(), true)
			}
			app.lib.remove(id)
			if app.screen == screenCloud && app.cv.cloud.ID == id {
				app.screen = screenLibrary
			}
			return app.setFlash(fmt.Sprintf("Deleted %q.", title), false)
		},
	}
}

// setFlash shows a message that clears itself.
func (a *App) setFlash(text string, isErr bool) tea.Cmd {
	kind := flashInfo
	if isErr {
		kind = flashError
	}
	a.flash = flash{text: text, kind: kind}
	return tea.Tick(5*time.Second, func(time.Time) tea.Msg { return flashClearMsg{} })
}

func (a *App) View() tea.View {
	if a.width < minWidth || a.height < minHeight {
		message := fmt.Sprintf("cloud-break needs at least %d×%d", minWidth, minHeight)
		return tea.NewView(lipgloss.Place(max(a.width, 1), max(a.height, 1),
			lipgloss.Center, lipgloss.Center, a.styles.Dim.Render(message)))
	}

	var body string
	switch {
	case a.help:
		body = a.renderHelp()
	case a.ed.open:
		body = a.ed.View(a.styles, a.width, a.bodyHeight())
	case a.screen == screenLibrary:
		body = a.renderLibrary()
	default:
		body = a.renderCloud()
	}

	view := tea.NewView(a.compose(body))
	view.AltScreen = true
	view.WindowTitle = "cloud-break"
	return view
}

// bodyHeight is the room left between the header and the two-line footer.
func (a *App) bodyHeight() int { return a.height - 5 }

// compose stacks header, body and footer into exactly the terminal height.
func (a *App) compose(body string) string {
	rule := a.styles.HeaderRule.Render(strings.Repeat("─", a.width))
	lines := make([]string, 0, a.height)
	lines = append(lines, a.renderHeader())
	lines = append(lines, rule)
	lines = append(lines, fitLines(body, a.bodyHeight(), a.width)...)
	lines = append(lines, rule)
	lines = append(lines, fitLines(a.renderFooter(), 2, a.width)...)
	return strings.Join(lines, "\n")
}

func (a *App) renderHeader() string {
	left := a.styles.Title.Render("cloud-break")
	if a.screen == screenCloud {
		left += a.styles.Faint.Render("  ·  ") +
			a.styles.Strong.Render(truncate(a.cv.cloud.Title, 40)) +
			a.styles.Faint.Render("  ·  ") +
			a.styles.Dim.Render(a.cv.pane.String())
	} else {
		left += a.styles.Faint.Render("  ·  library")
	}

	var right string
	switch {
	case a.flash.text != "":
		style := a.styles.Info
		if a.flash.kind == flashError {
			style = a.styles.Danger
		}
		right = style.Render(a.flash.text)

	case a.screen == screenCloud:
		c := a.cv.cloud
		filled, total := c.Progress()
		right = a.styles.StatusStyle(c.Status()).Render(c.Status().Glyph()+" "+c.Status().String()) +
			a.styles.Faint.Render(fmt.Sprintf("  %d/%d", filled, total))

	default:
		right = a.styles.Faint.Render(plural(len(a.lib.clouds), "cloud", "clouds"))
	}
	return spread(left, right, a.width)
}

func (a *App) renderFooter() string {
	var keys string
	if a.confirm != nil {
		keys = " " + a.styles.Warning.Render(a.confirm.question) + "  " +
			hints(a.styles, hint{"y", "yes"}, hint{"n", "no"})
	} else {
		var items []hint
		if a.screen == screenLibrary {
			items = []hint{
				{"↑/k ↓/j", "move"},
				{"enter", "open"},
				{"n", "new"},
				{"d", "delete"},
				{"?", "help"},
				{"q", "quit"},
			}
		} else {
			items = []hint{
				{"↑/k ↓/j", "slot"},
				{"enter", "edit"},
				{"tab", "pane"},
				{"t", "title"},
				{"esc/q", "back"},
				{"?", "help"},
			}
		}
		keys = " " + hints(a.styles, items...)
	}

	context := ""
	if line := a.contextLine(); line != "" {
		context = " " + a.styles.Subtle.Render(line)
	}
	return context + "\n" + keys
}

// contextLine explains the current selection: the question a slot is asking,
// or what the highlighted cloud still needs.
func (a *App) contextLine() string {
	if a.screen == screenCloud && a.cv.pane == paneCloud {
		return "Read down: in order to satisfy a node, the nodes beneath it must hold. The × is the conflict."
	}
	if a.screen == screenCloud {
		infos := SlotInfos()
		if a.cv.cursor >= 0 && a.cv.cursor < len(infos) {
			info := infos[a.cv.cursor]
			if a.cv.cloud.Text(info.Slot) == "" {
				return info.Prompt
			}
			return info.Hint
		}
		return ""
	}
	if c, ok := a.lib.selected(); ok {
		return nextStep(c)
	}
	return ""
}

// nextStep describes what a cloud needs next, for the library footer.
func nextStep(c Cloud) string {
	if missing := c.Missing(); len(missing) > 0 {
		names := make([]string, len(missing))
		for i, s := range missing {
			names[i] = string(s)
		}
		return fmt.Sprintf("%s still empty: %s. Open it and press enter to fill one in.",
			plural(len(missing), "statement is", "statements are"), strings.Join(names, ", "))
	}
	if len(c.Assumptions) == 0 {
		return "All five statements are set. The assumptions behind each arrow come next."
	}
	return plural(len(c.Assumptions), "assumption surfaced.", "assumptions surfaced.")
}

func (a *App) renderHelp() string {
	sections := helpSections()
	bodyHeight := a.bodyHeight()

	// Richest layout first; fall back until one fits the body. A help sheet
	// that silently loses its bottom border looks broken, so this is measured
	// rather than guessed from the terminal width.
	var candidates []string
	if len(sections) >= 2 {
		columns := lipgloss.JoinHorizontal(lipgloss.Top,
			a.helpBlock(sections[0]),
			strings.Repeat(" ", 4),
			a.helpBlock(sections[1]),
		)
		if lipgloss.Width(columns) <= a.width-8 {
			candidates = append(candidates,
				a.helpSheet(columns, true),
				a.helpSheet(columns, false),
			)
		}
	}

	single := make([]string, 0, len(sections))
	for _, section := range sections {
		single = append(single, a.helpBlock(section))
	}
	candidates = append(candidates,
		a.helpSheet(strings.Join(single, "\n\n"), true),
		a.helpSheet(strings.Join(single, "\n\n"), false),
	)

	chosen := candidates[len(candidates)-1]
	for _, candidate := range candidates {
		if lipgloss.Height(candidate) <= bodyHeight && lipgloss.Width(candidate) <= a.width {
			chosen = candidate
			break
		}
	}
	return lipgloss.Place(a.width, bodyHeight, lipgloss.Center, lipgloss.Center, chosen)
}

// helpSheet boxes a block of key groups, optionally with its closing hint.
func (a *App) helpSheet(content string, withFooter bool) string {
	blocks := []string{a.styles.Title.Render("Keys"), content}
	if withFooter {
		blocks = append(blocks, a.styles.Faint.Render("any key closes  ·  ^C quits"))
	}
	inner := strings.Join(blocks, "\n\n")

	width := 0
	for line := range strings.SplitSeq(inner, "\n") {
		width = max(width, lipgloss.Width(line))
	}
	width = min(width, max(20, a.width-boxFrame-2))
	return a.styles.HelpBox.Width(width + boxFrame).Render(inner)
}

// helpBlock renders one titled group of keys.
func (a *App) helpBlock(section helpSection) string {
	rows := []string{a.styles.HelpSection.Render(section.title)}
	for _, item := range section.items {
		rows = append(rows, "  "+a.styles.HelpKey.Width(9).Render(item.keys)+
			"  "+a.styles.HelpDesc.Render(item.desc))
	}
	return strings.Join(rows, "\n")
}

// plural picks the singular or plural noun for a count.
func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

// runTUI hands the terminal to Bubble Tea until the user quits.
func runTUI(store *Store) error {
	if _, err := tea.NewProgram(NewApp(store)).Run(); err != nil {
		return fmt.Errorf("run the interface: %w", err)
	}
	return nil
}

// visibleRange picks the slice of items to draw so the cursor stays on screen.
// heights are per item; height is the space available.
func visibleRange(heights []int, cursor, height int) (start, end int) {
	if len(heights) == 0 {
		return 0, 0
	}
	cursor = max(0, min(cursor, len(heights)-1))

	start = 0
	for start < cursor && sumInts(heights[start:cursor+1]) > height {
		start++
	}
	end = cursor + 1
	total := sumInts(heights[start:end])
	for end < len(heights) && total+heights[end] <= height {
		total += heights[end]
		end++
	}
	for start > 0 && total+heights[start-1] <= height {
		start--
		total += heights[start]
	}
	return start, end
}

func sumInts(values []int) int {
	total := 0
	for _, v := range values {
		total += v
	}
	return total
}

// fitLines clips or pads a block to exactly height lines of at most width.
func fitLines(block string, height, width int) []string {
	out := make([]string, height)
	row := 0
	for line := range strings.SplitSeq(block, "\n") {
		if row >= height {
			break
		}
		out[row] = fitLine(line, width)
		row++
	}
	return out
}

// fitLine truncates one rendered line without breaking its escape sequences.
func fitLine(line string, width int) string {
	if lipgloss.Width(line) <= width {
		return line
	}
	return ansi.Truncate(line, width, "…")
}

// truncate shortens plain text to a display width.
func truncate(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
	}
	return ansi.Truncate(s, width, "…")
}

// spread pushes right to the far edge, leaving exactly width columns.
func spread(left, right string, width int) string {
	rightWidth := lipgloss.Width(right)
	if rightWidth >= width {
		return ansi.Truncate(right, width, "…")
	}
	left = truncate(left, width-rightWidth-1)
	gap := width - lipgloss.Width(left) - rightWidth
	return left + strings.Repeat(" ", gap) + right
}

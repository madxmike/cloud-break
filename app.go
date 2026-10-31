package main

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
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

	keys    keyMap
	helpBar help.Model
	inline  inlineEditor
	party   partyEdit

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
	a.keys = newKeyMap()
	a.initHelpBar()
	a.lib.loading = true
	a.ed = newEditor(true)
	a.inline = newInlineEditor(true)
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
		a.inline.Resize(cardWidth(a.cv.paneWidth) - 4)
		a.resizeHelpBar()
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
	a.inline.SetDark(isDark)
	a.initHelpBar()
}

func (a *App) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, a.keys.CtrlC) {
		return a, quitCmd
	}

	if a.help {
		if key.Matches(msg, a.keys.Dismiss) {
			a.help = false
		}
		return a, nil
	}

	if a.confirm != nil {
		pending := a.confirm
		a.confirm = nil
		if key.Matches(msg, a.keys.Yes) {
			return a, pending.onYes(a)
		}
		return a, nil
	}

	if a.inline.open {
		result, cmd := a.inline.Update(msg)
		target, value := a.inline.target, a.inline.Value()
		switch result {
		case editSaved:
			a.inline.open = false
			if target.kind == editParty {
				return a, tea.Batch(cmd, a.nextPartyEdit(value))
			}
			return a, tea.Batch(cmd, a.applyEdit(target, value))
		case editCancelled:
			a.inline.open = false
			a.party = partyEdit{}
		}
		return a, cmd
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
	case key.Matches(msg, a.keys.Quit):
		return a, quitCmd

	case key.Matches(msg, a.keys.Up):
		a.lib.move(-1)

	case key.Matches(msg, a.keys.Down):
		a.lib.move(1)

	case key.Matches(msg, a.keys.Open):
		if c, ok := a.lib.selected(); ok {
			a.openCloud(c)
		}

	case key.Matches(msg, a.keys.New):
		a.pendingNew = true
		return a, a.ed.Open(editTarget{kind: editCloudTitle}, "New cloud",
			"What the two sides disagree about.", "", a.width, false)

	case key.Matches(msg, a.keys.DeleteCloud):
		if c, ok := a.lib.selected(); ok {
			a.askDeleteCloud(c)
		}

	case key.Matches(msg, a.keys.Help):
		a.help = true
	}
	return a, nil
}

func (a *App) handleCloudKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// A digit jumps straight to its pane.
	for i, binding := range a.keys.Pane {
		if key.Matches(msg, binding) {
			a.cv.pane = cloudPanes[i]
			return a, nil
		}
	}

	switch {
	case key.Matches(msg, a.keys.Back):
		a.screen = screenLibrary

	case key.Matches(msg, a.keys.NextPane):
		a.cv.pane = a.cv.pane.next(1)

	case key.Matches(msg, a.keys.PrevPane):
		a.cv.pane = a.cv.pane.next(-1)

	case key.Matches(msg, a.keys.Up):
		a.moveCursor(-1)

	case key.Matches(msg, a.keys.Down):
		a.moveCursor(1)

	case key.Matches(msg, a.keys.Edit):
		return a, a.activate()

	case key.Matches(msg, a.keys.Add):
		return a, a.add()

	case key.Matches(msg, a.keys.Inject):
		return a, a.injectHere()

	case key.Matches(msg, a.keys.Why):
		return a, a.editInjectionRationale()

	case key.Matches(msg, a.keys.Verdict):
		return a, a.cycleVerdict()

	case key.Matches(msg, a.keys.Verify):
		return a, a.toggleVerified()

	case key.Matches(msg, a.keys.Evidence):
		return a, a.editEvidence()

	case key.Matches(msg, a.keys.Delete):
		return a, a.deleteSelected()

	case key.Matches(msg, a.keys.DeleteCloud):
		a.askDeleteCloud(a.cv.cloud)

	case key.Matches(msg, a.keys.Title):
		return a, a.ed.Open(editTarget{kind: editCloudTitle}, "Cloud title",
			"A short name for the conflict.", a.cv.cloud.Title, a.width, false)

	case key.Matches(msg, a.keys.Sides):
		return a, a.startParties()

	case key.Matches(msg, a.keys.Help):
		a.help = true
	}
	return a, nil
}

// moveCursor moves the selection inside the active pane.
func (a *App) moveCursor(delta int) {
	if a.cv.pane == paneAssumptions {
		last := len(a.assumptionRows()) - 1
		a.cv.assumptionCursor = max(0, min(a.cv.assumptionCursor+delta, last))
		return
	}
	a.cv.move(delta, len(SlotInfos()))
}

// activate handles enter: edit the highlighted statement or assumption.
func (a *App) activate() tea.Cmd {
	switch a.cv.pane {
	case paneStatements:
		return a.startInline()
	case paneAssumptions:
		return a.editSelectedRow()
	}
	a.cv.pane = paneStatements
	return nil
}

// editSelectedRow edits whatever the cursor rests on: an assumption's text, an
// injection's idea, or a new assumption under a heading.
func (a *App) editSelectedRow() tea.Cmd {
	row, ok := a.selectedAssumptionRow()
	if !ok {
		return nil
	}
	switch {
	case row.injectionID != "":
		return a.editInjection(row.injectionID)
	case row.id != "":
		return a.editAssumption()
	}
	return a.addAssumption()
}

// partyEdit tracks the two-step edit of the side names.
type partyEdit struct {
	index int    // 0 = first side, 1 = second side
	first string // the first name, held while the second is typed
	orig  [2]string
}

// startParties edits the first side name inline, then the second.
func (a *App) startParties() tea.Cmd {
	a.cv.pane = paneStatements
	a.party = partyEdit{orig: [2]string{a.cv.cloud.Party(1), a.cv.cloud.Party(2)}}
	return a.inline.OpenPlaceholder(editTarget{kind: editParty}, a.party.orig[0], partyFieldWidth)
}

// nextPartyEdit moves from the first name to the second, then saves both.
func (a *App) nextPartyEdit(value string) tea.Cmd {
	value = strings.TrimSpace(value)
	if value == "" {
		value = a.party.orig[a.party.index]
	}

	if a.party.index == 0 {
		a.party.first = value
		a.party.index = 1
		return a.inline.OpenPlaceholder(editTarget{kind: editParty}, a.party.orig[1], partyFieldWidth)
	}

	a.cv.cloud.SetParties(a.party.first, value)
	a.party = partyEdit{}
	return a.persist()
}

// startInline edits the highlighted statement in its own card.
func (a *App) startInline() tea.Cmd {
	infos := SlotInfos()
	if a.cv.cursor < 0 || a.cv.cursor >= len(infos) {
		return nil
	}
	paneWidth := a.cv.paneWidth
	if paneWidth <= 0 {
		paneWidth = a.width
	}
	info := infos[a.cv.cursor]
	return a.inline.Open(editTarget{kind: editStatement, slot: info.Slot},
		a.cv.cloud.Text(info.Slot), cardWidth(paneWidth)-4)
}

// editAssumption opens the selected assumption's text, or starts a new one when
// the cursor is on an arrow heading.
func (a *App) editAssumption() tea.Cmd {
	row, ok := a.selectedAssumptionRow()
	if !ok {
		return nil
	}
	if row.id == "" {
		return a.addAssumption()
	}
	as := a.cv.cloud.AssumptionByID(row.id)
	if as == nil {
		return nil
	}
	return a.ed.Open(editTarget{kind: editAssumptionText, id: as.ID, arrow: row.arrow},
		fmt.Sprintf("%s · assumption", row.arrow), a.arrowHint(row.arrow),
		as.Text, a.width, false)
}

// addAssumption starts a new assumption on the arrow under the cursor.
func (a *App) addAssumption() tea.Cmd {
	if a.cv.pane != paneAssumptions {
		return nil
	}
	row, ok := a.selectedAssumptionRow()
	if !ok {
		return nil
	}
	return a.ed.Open(editTarget{kind: editAssumptionText, id: "", arrow: row.arrow},
		fmt.Sprintf("%s · new assumption", row.arrow), a.arrowHint(row.arrow),
		"", a.width, false)
}

// add dispatches the add key to whichever pane is open.
func (a *App) add() tea.Cmd {
	if a.cv.pane != paneAssumptions {
		return nil
	}
	return a.addAssumption()
}

// deleteSelected dispatches the delete key to whichever pane is open.
func (a *App) deleteSelected() tea.Cmd {
	if a.cv.pane != paneAssumptions {
		return nil
	}
	row, ok := a.selectedAssumptionRow()
	if !ok {
		return nil
	}
	switch {
	case row.injectionID != "":
		return a.askDeleteInjection(row.injectionID)
	case row.id != "":
		return a.askDeleteAssumption(row.id)
	}
	return a.setFlash("That row is an arrow heading; the assumptions sit under it.", false)
}

// injectAgainstSelected starts an injection against the assumption under the
// cursor: the step that dissolves a cloud.
func (a *App) injectHere() tea.Cmd {
	if a.cv.pane != paneAssumptions {
		return nil
	}
	row, ok := a.selectedAssumptionRow()
	if !ok {
		return nil
	}

	// On an injection, i opens it. On an assumption, it opens the injection
	// that answers it, or starts one.
	if row.injectionID != "" {
		return a.editInjection(row.injectionID)
	}
	if row.id == "" {
		return a.setFlash("Injections attach to assumptions.", false)
	}
	if existing := a.cv.cloud.InjectionBreaking(row.id); existing != nil {
		a.selectInjectionRow(existing.ID)
		return a.editInjection(existing.ID)
	}

	as := a.cv.cloud.AssumptionByID(row.id)
	if as == nil {
		return nil
	}
	return a.ed.Open(editTarget{kind: editInjectionText, breaks: []string{as.ID}},
		fmt.Sprintf("%s · new injection", row.arrow),
		"What change makes this assumption irrelevant? "+shorten(as.Text, 60),
		"", a.width, false)
}

// selectInjectionRow puts the cursor on an injection row.
func (a *App) selectInjectionRow(id string) {
	for i, row := range a.assumptionRows() {
		if row.injectionID == id {
			a.cv.assumptionCursor = i
			return
		}
	}
}

// editInjection opens the selected injection's idea.
func (a *App) editInjection(id string) tea.Cmd {
	injection := a.cv.cloud.InjectionByID(id)
	if injection == nil {
		return nil
	}
	return a.ed.Open(editTarget{kind: editInjectionText, id: injection.ID},
		"Injection", "The change that makes the broken assumption irrelevant.",
		injection.Text, a.width, false)
}

// editInjectionRationale records why the injection works.
func (a *App) editInjectionRationale() tea.Cmd {
	if a.cv.pane != paneAssumptions {
		return nil
	}
	row, ok := a.selectedAssumptionRow()
	if !ok || row.injectionID == "" {
		return a.setFlash("The rationale belongs to an injection.", false)
	}
	injection := a.cv.cloud.InjectionByID(row.injectionID)
	if injection == nil {
		return nil
	}
	return a.ed.Open(editTarget{kind: editInjectionRationale, id: injection.ID},
		"Injection · why it works", "Does it reach the objective without breaking anything else?",
		injection.Rationale, a.width, true)
}

// askDeleteInjection confirms before removing an injection.
func (a *App) askDeleteInjection(id string) tea.Cmd {
	injection := a.cv.cloud.InjectionByID(id)
	if injection == nil {
		return nil
	}
	text := injection.Text
	a.confirm = &confirmation{
		question: fmt.Sprintf("Delete the injection %q?", shorten(text, 40)),
		onYes: func(app *App) tea.Cmd {
			app.cv.cloud.RemoveInjection(id)
			app.clampAssumptionCursor()
			return app.persist()
		},
	}
	return nil
}

// toggleVerified records that a person checked the injection really dissolves
// the conflict. Status only becomes dissolved once this is set.
func (a *App) toggleVerified() tea.Cmd {
	c := &a.cv.cloud
	if !c.Verified && len(c.BrokenAssumptions()) == 0 {
		return a.setFlash("Nothing is broken yet: mark an assumption invalid and inject against it.", true)
	}
	c.Verified = !c.Verified
	if c.Verified {
		return tea.Batch(a.persist(), a.setFlash("Cloud marked as evaporated.", false))
	}
	return tea.Batch(a.persist(), a.setFlash("Verification withdrawn.", false))
}

// arrowHint is the question the arrow is asking.
func (a *App) arrowHint(arrow Arrow) string {
	if reading := a.arrowReading(arrow); reading != "" {
		return "What has to be true for this arrow to hold? " + reading
	}
	return "What has to be true for this arrow to hold?"
}

// cycleVerdict moves the selected assumption through the four judgements.
func (a *App) cycleVerdict() tea.Cmd {
	if a.cv.pane != paneAssumptions {
		return nil
	}
	row, ok := a.selectedAssumptionRow()
	if !ok {
		return nil
	}
	if row.injectionID != "" {
		return a.setFlash("Verdicts apply to assumptions, not injections.", false)
	}
	if row.id == "" {
		return a.setFlash("That row is an arrow heading; the assumptions sit under it.", false)
	}
	as := a.cv.cloud.AssumptionByID(row.id)
	if as == nil {
		return nil
	}
	as.Verdict = as.Verdict.Next()
	return tea.Batch(a.persist(), a.setFlash("Marked "+as.Verdict.String()+".", false))
}

// editEvidence records why an assumption is judged the way it is.
func (a *App) editEvidence() tea.Cmd {
	if a.cv.pane != paneAssumptions {
		return nil
	}
	row, ok := a.selectedAssumptionRow()
	if !ok {
		return nil
	}
	if row.injectionID != "" {
		return a.setFlash("That is an injection; r explains why it works.", false)
	}
	if row.id == "" {
		return a.setFlash("That row is an arrow heading; the assumptions sit under it.", false)
	}
	as := a.cv.cloud.AssumptionByID(row.id)
	if as == nil {
		return nil
	}
	return a.ed.Open(editTarget{kind: editAssumptionEvidence, id: as.ID, arrow: row.arrow},
		fmt.Sprintf("%s · evidence", row.arrow),
		"What makes this hold, or fail? Cite what you have seen.",
		as.Evidence, a.width, true)
}

// askDeleteAssumption confirms before removing an assumption.
func (a *App) askDeleteAssumption(id string) tea.Cmd {
	as := a.cv.cloud.AssumptionByID(id)
	if as == nil {
		return nil
	}
	text := as.Text
	a.confirm = &confirmation{
		question: fmt.Sprintf("Delete the assumption %q?", shorten(text, 40)),
		onYes: func(app *App) tea.Cmd {
			app.cv.cloud.RemoveAssumption(id)
			app.clampAssumptionCursor()
			return app.persist()
		},
	}
	return nil
}

// clampAssumptionCursor keeps the cursor inside the row list.
func (a *App) clampAssumptionCursor() {
	last := len(a.assumptionRows()) - 1
	a.cv.assumptionCursor = max(0, min(a.cv.assumptionCursor, last))
}

// selectedAssumptionRow is the row under the cursor in the assumptions pane.
func (a *App) selectedAssumptionRow() (assumptionRow, bool) {
	rows := a.assumptionRows()
	if len(rows) == 0 {
		return assumptionRow{}, false
	}
	index := max(0, min(a.cv.assumptionCursor, len(rows)-1))
	return rows[index], true
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
			return a.setFlash("Created "+c.Title+".", false)
		}
		a.cv.cloud.Title = OneLine(value)
		return a.persist()

	case editStatement:
		a.cv.cloud.SetText(target.slot, value)
		return a.persist()

	case editAssumptionText:
		if strings.TrimSpace(value) == "" && target.id == "" {
			return nil
		}
		if target.id == "" {
			id := a.cv.cloud.AddAssumption(target.arrow, value)
			a.selectAssumption(id)
		} else if as := a.cv.cloud.AssumptionByID(target.id); as != nil {
			as.Text = OneLine(value)
		}
		return a.persist()

	case editAssumptionEvidence:
		if as := a.cv.cloud.AssumptionByID(target.id); as != nil {
			as.Evidence = strings.TrimSpace(value)
		}
		return a.persist()

	case editInjectionText:
		if strings.TrimSpace(value) == "" && target.id == "" {
			return nil
		}
		if target.id == "" {
			id := a.cv.cloud.AddInjection(target.breaks, value)
			a.selectInjectionRow(id)
		} else if injection := a.cv.cloud.InjectionByID(target.id); injection != nil {
			injection.Text = OneLine(value)
		}
		return a.persist()

	case editInjectionRationale:
		if injection := a.cv.cloud.InjectionByID(target.id); injection != nil {
			injection.Rationale = strings.TrimSpace(value)
		}
		return a.persist()
	}
	return nil
}

// selectAssumption puts the cursor on an assumption wherever it sorts.
func (a *App) selectAssumption(id string) {
	for i, row := range a.assumptionRows() {
		if row.id == id {
			a.cv.assumptionCursor = i
			return
		}
	}
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

	view := tea.NewView(a.compose())
	view.AltScreen = true
	view.WindowTitle = "cloud-break"
	return view
}

// chromeHeight is every row that is not body: the header, its rules, the pane
// tabs, the context hint and the key hints. It lives in one place so the body
// always fills exactly the space that is left.
func (a *App) chromeHeight() int {
	switch {
	case a.help:
		return 3 // header, rule, closing hint
	case a.screen == screenLibrary:
		return 4 // header, rule, rule, keys
	default:
		return 6 // header, rule, tabs, spacer, rule, keys
	}
}

// bodyHeight is the room left for the pane.
func (a *App) bodyHeight() int { return a.height - a.chromeHeight() }

// compose stacks the chrome and the body into exactly the terminal height.
func (a *App) compose() string {
	rule := a.styles.HeaderRule.Render(strings.Repeat("─", a.width))
	lines := make([]string, 0, a.height)
	lines = append(lines, a.renderHeader(), rule)

	if a.help {
		lines = append(lines, fitLines(a.renderHelp(), a.bodyHeight(), a.width)...)
		lines = append(lines, fitLine(" "+a.styles.Dim.Render("any key closes  ·  ^C quits"), a.width))
		return strings.Join(lines, "\n")
	}

	if a.screen == screenCloud {
		// The blank row keeps the tabs from crowding the pane beneath them.
		lines = append(lines, a.renderTabs(), "")
	}

	lines = append(lines, fitLines(a.renderBody(), a.bodyHeight(), a.width)...)
	lines = append(lines, rule)
	lines = append(lines, a.keysLine())
	return strings.Join(lines, "\n")
}

// renderBody is whichever pane, list or editor is in front.
func (a *App) renderBody() string {
	switch {
	case a.ed.open:
		return a.ed.View(a.styles, a.helpBar, a.width, a.bodyHeight())
	case a.screen == screenLibrary:
		return a.renderLibrary()
	default:
		return a.renderCloud()
	}
}

// renderTabs draws every pane as a chip and fills the active one, so the pane
// you are in and how many there are is visible rather than remembered.
func (a *App) renderTabs() string {
	helpHint := a.styles.Faint.Render("? help ")

	strip := a.tabStrip(true)
	if lipgloss.Width(strip)+lipgloss.Width(helpHint) > a.width {
		strip = a.tabStrip(false) // too narrow for names: numbers only
	}

	if gap := a.width - lipgloss.Width(strip) - lipgloss.Width(helpHint); gap >= 0 {
		strip += strings.Repeat(" ", gap) + helpHint
	}
	return fitLine(strip, a.width)
}

// tabStrip renders the pane chips, with or without their names.
func (a *App) tabStrip(withNames bool) string {
	chips := make([]string, 0, len(cloudPanes))
	for i, p := range cloudPanes {
		label := fmt.Sprintf("%d", i+1)
		if withNames {
			label = fmt.Sprintf("%d %s", i+1, p)
		}
		if p == a.cv.pane {
			chips = append(chips, a.styles.TabActive.Render(label))
			continue
		}
		chips = append(chips, a.styles.Tab.Render(label))
	}
	return " " + strings.Join(chips, "")
}

func (a *App) keysLine() string {
	if a.confirm != nil {
		return fitLine(" "+a.styles.Warning.Render(a.confirm.question)+"  "+
			a.helpBar.ShortHelpView([]key.Binding{a.keys.Yes, a.keys.No}), a.width)
	}
	return fitLine(" "+a.helpBar.ShortHelpView(a.footerBindings()), a.width)
}

func (a *App) renderHeader() string {
	left := a.styles.Title.Render("cloud-break")
	if a.screen == screenCloud {
		left += a.styles.Faint.Render("  ·  ") +
			a.styles.Strong.Render(truncate(a.cv.cloud.Title, 44))
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
			a.styles.Faint.Render(fmt.Sprintf("  %d/%d · %d asm", filled, total, len(c.Assumptions)))

	default:
		right = a.styles.Faint.Render(plural(len(a.lib.clouds), "cloud", "clouds"))
	}
	return spread(left, right, a.width)
}

func (a *App) renderHelp() string {
	sections := a.helpSections()
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

package main

// The commands. Each one parses its own operands, opens the library, changes or
// reads one cloud, and reports in the shape --json asks for.

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

// unlimitedHeight is the height a diagram may use on the command line, where
// there is no window to fit: the figure only decides whether it fits at all.
const unlimitedHeight = 1 << 16

// list reports the clouds in the library, most recently updated first.
func (c *invocation) list(st settings, args []string) error {
	fs := c.newFlagSet("list", &st)
	status := fs.String("status", "", "only clouds in this status (draft, open, dissolving, dissolved)")
	operands, err := parseCommand(fs, args)
	if err != nil {
		return err
	}
	if len(operands) > 0 {
		return usageError{fmt.Errorf("list takes no operands, got %q", operands)}
	}
	want, err := parseStatusFilter(*status)
	if err != nil {
		return err
	}

	store, cleanup, err := c.store(st)
	if err != nil {
		return err
	}
	defer cleanup()

	clouds, err := store.List()
	if err != nil {
		return err
	}

	reported := make([]Cloud, 0, len(clouds))
	for _, cloud := range clouds {
		if want != nil && cloud.Status() != *want {
			continue
		}
		reported = append(reported, cloud)
	}

	if st.json {
		summaries := make([]summaryJSON, 0, len(reported))
		for _, cloud := range reported {
			summaries = append(summaries, newSummaryJSON(cloud))
		}
		return writeJSON(c.s.stdout, summaries)
	}
	for _, cloud := range reported {
		if err := writeSummaryLine(c.s.stdout, cloud); err != nil {
			return err
		}
	}
	return nil
}

// show reports one cloud in full: statements, assumptions, injections, the
// process and the issues.
func (c *invocation) show(st settings, args []string) error {
	fs := c.newFlagSet("show", &st)
	ref, err := singleOperand(fs, args, "cloud-break show <cloud>")
	if err != nil {
		return err
	}

	store, cleanup, err := c.store(st)
	if err != nil {
		return err
	}
	defer cleanup()

	cloud, err := lookupCloud(store, ref)
	if err != nil {
		return err
	}
	if st.json {
		return writeJSON(c.s.stdout, newCloudJSON(cloud))
	}
	return writeCloudBlock(c.s.stdout, cloud)
}

// render draws one cloud's diagram as text, the figure the README shows.
func (c *invocation) render(st settings, args []string) error {
	fs := c.newFlagSet("render", &st)
	width := fs.Int("width", 74, fmt.Sprintf("columns to draw in (at least %d)", diagramMinWidth))
	ref, err := singleOperand(fs, args, "cloud-break render <cloud>")
	if err != nil {
		return err
	}
	if *width < diagramMinWidth {
		return usageError{fmt.Errorf("--width must be at least %d, got %d", diagramMinWidth, *width)}
	}

	store, cleanup, err := c.store(st)
	if err != nil {
		return err
	}
	defer cleanup()

	cloud, err := lookupCloud(store, ref)
	if err != nil {
		return err
	}

	drawn := figure{cloud: cloud, palette: plainPalette()}.render(*width, unlimitedHeight)
	if drawn == "" {
		return fmt.Errorf("cannot draw %s in %d columns", cloud.ID, *width)
	}
	_, err = fmt.Fprintln(c.s.stdout, drawn)
	return err
}

// check validates clouds and reports every finding. With no operand it checks
// the whole library. It fails when a cloud has an error, so a script can gate on
// the exit status.
func (c *invocation) check(st settings, args []string) error {
	fs := c.newFlagSet("check", &st)
	operands, err := parseCommand(fs, args)
	if err != nil {
		return err
	}

	store, cleanup, err := c.store(st)
	if err != nil {
		return err
	}
	defer cleanup()

	targets := make([]Cloud, 0, len(operands))
	if len(operands) == 0 {
		targets, err = store.List()
		if err != nil {
			return err
		}
		// A file that cannot be parsed is left out of the list, so say so rather
		// than let the run look like it covered the whole library.
		broken, err := store.Broken()
		if err != nil {
			return err
		}
		for _, name := range broken {
			fmt.Fprintf(c.s.stderr, "cloud-break: skipped %s: it cannot be read as a cloud\n", name)
		}
	} else {
		for _, ref := range operands {
			cloud, err := lookupCloud(store, ref)
			if err != nil {
				return err
			}
			targets = append(targets, cloud)
		}
	}

	issues := make([]issueJSON, 0)
	var failures int
	for _, cloud := range targets {
		for _, issue := range newCloudJSON(cloud).Issues {
			if issue.Severity == "error" {
				failures++
			}
			issues = append(issues, issue)
		}
	}

	if st.json {
		if err := writeJSON(c.s.stdout, issues); err != nil {
			return err
		}
	} else {
		for _, issue := range issues {
			if err := writeIssueLine(c.s.stdout, issue); err != nil {
				return err
			}
		}
	}

	if failures > 0 {
		return checksFailed{failures}
	}
	return nil
}

// newCloud creates a cloud, either empty or from a whole JSON document.
func (c *invocation) newCloud(st settings, args []string) error {
	fs := c.newFlagSet("new", &st)
	title := fs.String("title", "", "the title (default \"Untitled cloud\")")
	parties := fs.String("parties", "", `the two sides, as "first,second"`)
	from := fs.String("from", "", "read a whole cloud document from this file (- for standard input)")
	operands, err := parseCommand(fs, args)
	if err != nil {
		return err
	}
	if len(operands) > 0 {
		return usageError{fmt.Errorf("new takes no operands, got %q", operands)}
	}
	if *from != "" && (*title != "" || *parties != "") {
		return usageError{errors.New("--from carries the whole document, so it cannot be combined with --title or --parties")}
	}

	var cloud Cloud
	if *from != "" {
		cloud, err = readCloud(c.s.stdin, *from)
		if err != nil {
			return err
		}
	} else {
		cloud = *New(*title)
	}
	if *parties != "" {
		first, second, err := splitParties(*parties)
		if err != nil {
			return err
		}
		cloud.SetParties(first, second)
	}

	store, cleanup, err := c.store(st)
	if err != nil {
		return err
	}
	defer cleanup()

	if err := store.Create(cloud); err != nil {
		return err
	}
	return c.wrote(st, "cloud", cloud.ID, string(cloud.ID))
}

// set writes one statement. An empty text clears it.
func (c *invocation) set(st settings, args []string) error {
	fs := c.newFlagSet("set", &st)
	operands, err := parseCommand(fs, args)
	if err != nil {
		return err
	}
	if len(operands) < 3 {
		return usageError{errors.New("usage: cloud-break set <cloud> <A|B|C|D|D'> <text>")}
	}
	slot, ok := parseSlot(operands[1])
	if !ok {
		return usageError{fmt.Errorf("unknown statement %q (want A, B, C, D or D')", operands[1])}
	}
	text := strings.Join(operands[2:], " ")

	store, cleanup, err := c.store(st)
	if err != nil {
		return err
	}
	defer cleanup()

	cloud, err := lookupCloud(store, operands[0])
	if err != nil {
		return err
	}
	cloud.SetText(slot, text)
	if cloud, err = saveCloud(store, cloud); err != nil {
		return err
	}
	return c.wrote(st, "cloud", cloud.ID, string(cloud.ID))
}

// title renames a cloud.
func (c *invocation) title(st settings, args []string) error {
	fs := c.newFlagSet("title", &st)
	operands, err := parseCommand(fs, args)
	if err != nil {
		return err
	}
	if len(operands) < 2 {
		return usageError{errors.New("usage: cloud-break title <cloud> <text>")}
	}

	store, cleanup, err := c.store(st)
	if err != nil {
		return err
	}
	defer cleanup()

	cloud, err := lookupCloud(store, operands[0])
	if err != nil {
		return err
	}
	// An empty title is normalized to "Untitled cloud" on the way to disk.
	cloud.Title = strings.Join(operands[1:], " ")
	if cloud, err = saveCloud(store, cloud); err != nil {
		return err
	}
	return c.wrote(st, "cloud", cloud.ID, string(cloud.ID))
}

// parties names the two sides of a cloud.
func (c *invocation) parties(st settings, args []string) error {
	fs := c.newFlagSet("parties", &st)
	operands, err := parseCommand(fs, args)
	if err != nil {
		return err
	}
	if len(operands) != 3 {
		return usageError{errors.New("usage: cloud-break parties <cloud> <first> <second>")}
	}

	store, cleanup, err := c.store(st)
	if err != nil {
		return err
	}
	defer cleanup()

	cloud, err := lookupCloud(store, operands[0])
	if err != nil {
		return err
	}
	cloud.SetParties(operands[1], operands[2])
	if cloud, err = saveCloud(store, cloud); err != nil {
		return err
	}
	return c.wrote(st, "cloud", cloud.ID, string(cloud.ID))
}

// assume groups the commands that work on the assumptions behind the arrows.
func (c *invocation) assume(st settings, args []string) error {
	if len(args) == 0 {
		return usageError{errors.New("assume needs a subcommand: add, set, verdict, evidence or rm")}
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "add":
		return c.assumeAdd(st, rest)
	case "set":
		return c.assumeSet(st, rest)
	case "verdict":
		return c.assumeVerdict(st, rest)
	case "evidence":
		return c.assumeEvidence(st, rest)
	case "rm":
		return c.assumeRemove(st, rest)
	}
	return usageError{fmt.Errorf("unknown assume subcommand %q (want add, set, verdict, evidence or rm)", sub)}
}

// assumeAdd records an assumption behind one arrow.
func (c *invocation) assumeAdd(st settings, args []string) error {
	fs := c.newFlagSet("assume add", &st)
	operands, err := parseCommand(fs, args)
	if err != nil {
		return err
	}
	if len(operands) < 3 {
		return usageError{errors.New("usage: cloud-break assume add <cloud> <arrow> <text>")}
	}
	arrow, ok := parseArrow(operands[1])
	if !ok {
		return usageError{fmt.Errorf("unknown arrow %q (want A-B, A-C, B-D, C-D' or D-D')", operands[1])}
	}
	text := strings.Join(operands[2:], " ")
	if strings.TrimSpace(text) == "" {
		return usageError{errors.New("an assumption needs text; an empty assumption says nothing")}
	}

	store, cleanup, err := c.store(st)
	if err != nil {
		return err
	}
	defer cleanup()

	cloud, err := lookupCloud(store, operands[0])
	if err != nil {
		return err
	}
	id := cloud.AddAssumption(arrow, text)
	if cloud, err = saveCloud(store, cloud); err != nil {
		return err
	}
	return c.wrote(st, "assumption", cloud.ID, id)
}

// assumeSet rewords an assumption.
func (c *invocation) assumeSet(st settings, args []string) error {
	fs := c.newFlagSet("assume set", &st)
	operands, err := parseCommand(fs, args)
	if err != nil {
		return err
	}
	if len(operands) < 3 {
		return usageError{errors.New("usage: cloud-break assume set <cloud> <assumption> <text>")}
	}
	text := strings.Join(operands[2:], " ")

	store, cleanup, err := c.store(st)
	if err != nil {
		return err
	}
	defer cleanup()

	cloud, err := lookupCloud(store, operands[0])
	if err != nil {
		return err
	}
	as, err := findAssumption(cloud, operands[1])
	if err != nil {
		return err
	}
	if target := cloud.AssumptionByID(as.ID); target != nil {
		target.Text = text
	}
	if cloud, err = saveCloud(store, cloud); err != nil {
		return err
	}
	return c.wrote(st, "assumption", cloud.ID, as.ID)
}

// assumeVerdict judges an assumption, optionally recording the evidence for the
// judgement in the same call.
func (c *invocation) assumeVerdict(st settings, args []string) error {
	fs := c.newFlagSet("assume verdict", &st)
	evidence := fs.String("evidence", "", "the evidence for this verdict")
	operands, err := parseCommand(fs, args)
	if err != nil {
		return err
	}
	if len(operands) != 3 {
		return usageError{errors.New("usage: cloud-break assume verdict <cloud> <assumption> <unassessed|holds|questionable|invalid> [--evidence text]")}
	}
	verdict, ok := parseVerdict(operands[2])
	if !ok {
		return usageError{fmt.Errorf("unknown verdict %q (want unassessed, holds, questionable or invalid)", operands[2])}
	}

	store, cleanup, err := c.store(st)
	if err != nil {
		return err
	}
	defer cleanup()

	cloud, err := lookupCloud(store, operands[0])
	if err != nil {
		return err
	}
	as, err := findAssumption(cloud, operands[1])
	if err != nil {
		return err
	}
	if target := cloud.AssumptionByID(as.ID); target != nil {
		target.Verdict = verdict
		if *evidence != "" {
			target.Evidence = *evidence
		}
	}
	if cloud, err = saveCloud(store, cloud); err != nil {
		return err
	}
	return c.wrote(st, "assumption", cloud.ID, as.ID)
}

// assumeEvidence records, or clears, the evidence behind an assumption.
func (c *invocation) assumeEvidence(st settings, args []string) error {
	fs := c.newFlagSet("assume evidence", &st)
	operands, err := parseCommand(fs, args)
	if err != nil {
		return err
	}
	if len(operands) < 3 {
		return usageError{errors.New("usage: cloud-break assume evidence <cloud> <assumption> <text>")}
	}
	text := strings.Join(operands[2:], " ")

	store, cleanup, err := c.store(st)
	if err != nil {
		return err
	}
	defer cleanup()

	cloud, err := lookupCloud(store, operands[0])
	if err != nil {
		return err
	}
	as, err := findAssumption(cloud, operands[1])
	if err != nil {
		return err
	}
	if target := cloud.AssumptionByID(as.ID); target != nil {
		target.Evidence = text
	}
	if cloud, err = saveCloud(store, cloud); err != nil {
		return err
	}
	return c.wrote(st, "assumption", cloud.ID, as.ID)
}

// assumeRemove deletes an assumption, and the links injections held to it.
func (c *invocation) assumeRemove(st settings, args []string) error {
	fs := c.newFlagSet("assume rm", &st)
	operands, err := parseCommand(fs, args)
	if err != nil {
		return err
	}
	if len(operands) != 2 {
		return usageError{errors.New("usage: cloud-break assume rm <cloud> <assumption>")}
	}

	store, cleanup, err := c.store(st)
	if err != nil {
		return err
	}
	defer cleanup()

	cloud, err := lookupCloud(store, operands[0])
	if err != nil {
		return err
	}
	as, err := findAssumption(cloud, operands[1])
	if err != nil {
		return err
	}
	cloud.RemoveAssumption(as.ID)
	if cloud, err = saveCloud(store, cloud); err != nil {
		return err
	}
	return c.wrote(st, "assumption", cloud.ID, as.ID)
}

// inject groups the commands that work on the injections: the changes that
// invalidate an assumption.
func (c *invocation) inject(st settings, args []string) error {
	if len(args) == 0 {
		return usageError{errors.New("inject needs a subcommand: add or rm")}
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "add":
		return c.injectAdd(st, rest)
	case "rm":
		return c.injectRemove(st, rest)
	}
	return usageError{fmt.Errorf("unknown inject subcommand %q (want add or rm)", sub)}
}

// injectAdd records a change that breaks one or more assumptions.
func (c *invocation) injectAdd(st settings, args []string) error {
	fs := c.newFlagSet("inject add", &st)
	breaks := fs.String("breaks", "", "the assumptions this injection breaks, as a comma-separated list")
	rationale := fs.String("rationale", "", "why the injection works")
	operands, err := parseCommand(fs, args)
	if err != nil {
		return err
	}
	if len(operands) < 2 {
		return usageError{errors.New("usage: cloud-break inject add <cloud> [--breaks ids] [--rationale text] <text>")}
	}
	text := strings.Join(operands[1:], " ")

	store, cleanup, err := c.store(st)
	if err != nil {
		return err
	}
	defer cleanup()

	cloud, err := lookupCloud(store, operands[0])
	if err != nil {
		return err
	}

	// Resolve each break to the identifier it names, so a prefix cannot be
	// written to disk as if it were the whole thing.
	var breakIDs []string
	for _, ref := range splitList(*breaks) {
		as, err := findAssumption(cloud, ref)
		if err != nil {
			return err
		}
		if !contains(breakIDs, as.ID) {
			breakIDs = append(breakIDs, as.ID)
		}
	}

	id := cloud.AddInjection(breakIDs, text)
	if target := cloud.InjectionByID(id); target != nil {
		target.Rationale = strings.TrimSpace(*rationale)
	}
	if cloud, err = saveCloud(store, cloud); err != nil {
		return err
	}
	return c.wrote(st, "injection", cloud.ID, id)
}

// injectRemove deletes an injection.
func (c *invocation) injectRemove(st settings, args []string) error {
	fs := c.newFlagSet("inject rm", &st)
	operands, err := parseCommand(fs, args)
	if err != nil {
		return err
	}
	if len(operands) != 2 {
		return usageError{errors.New("usage: cloud-break inject rm <cloud> <injection>")}
	}

	store, cleanup, err := c.store(st)
	if err != nil {
		return err
	}
	defer cleanup()

	cloud, err := lookupCloud(store, operands[0])
	if err != nil {
		return err
	}
	inj, err := findInjection(cloud, operands[1])
	if err != nil {
		return err
	}
	cloud.RemoveInjection(inj.ID)
	if cloud, err = saveCloud(store, cloud); err != nil {
		return err
	}
	return c.wrote(st, "injection", cloud.ID, inj.ID)
}

// verify records that a person checked the injection really dissolves the
// conflict. It refuses while nothing is broken, and --undo withdraws it.
func (c *invocation) verify(st settings, args []string) error {
	fs := c.newFlagSet("verify", &st)
	undo := fs.Bool("undo", false, "withdraw the verification instead of recording it")
	ref, err := singleOperand(fs, args, "cloud-break verify <cloud> [--undo]")
	if err != nil {
		return err
	}

	store, cleanup, err := c.store(st)
	if err != nil {
		return err
	}
	defer cleanup()

	cloud, err := lookupCloud(store, ref)
	if err != nil {
		return err
	}
	if !*undo && len(cloud.BrokenAssumptions()) == 0 {
		return errors.New("nothing is broken yet: mark an assumption invalid and inject against it before verifying")
	}
	cloud.Verified = !*undo
	if cloud, err = saveCloud(store, cloud); err != nil {
		return err
	}
	return c.wrote(st, "cloud", cloud.ID, string(cloud.ID))
}

// delete removes a cloud from the library.
func (c *invocation) delete(st settings, args []string) error {
	fs := c.newFlagSet("delete", &st)
	ref, err := singleOperand(fs, args, "cloud-break delete <cloud>")
	if err != nil {
		return err
	}

	store, cleanup, err := c.store(st)
	if err != nil {
		return err
	}
	defer cleanup()

	cloud, err := lookupCloud(store, ref)
	if err != nil {
		return err
	}
	if err := store.Delete(cloud.ID); err != nil {
		return err
	}
	return c.removed(st, cloud.ID)
}

// export writes the whole cloud document, the same document "new --from" reads
// back, so a cloud can be moved between libraries.
func (c *invocation) export(st settings, args []string) error {
	fs := c.newFlagSet("export", &st)
	ref, err := singleOperand(fs, args, "cloud-break export <cloud>")
	if err != nil {
		return err
	}

	store, cleanup, err := c.store(st)
	if err != nil {
		return err
	}
	defer cleanup()

	cloud, err := lookupCloud(store, ref)
	if err != nil {
		return err
	}
	return writeJSON(c.s.stdout, cloud)
}

// singleOperand parses a command that names exactly one cloud.
func singleOperand(fs *flag.FlagSet, args []string, usage string) (string, error) {
	operands, err := parseCommand(fs, args)
	if err != nil {
		return "", err
	}
	if len(operands) != 1 {
		return "", usageError{fmt.Errorf("want one operand, got %d\nusage: %s", len(operands), usage)}
	}
	return operands[0], nil
}

// readCloud reads a whole cloud document from a file, or from stdin for "-".
func readCloud(stdin io.Reader, from string) (Cloud, error) {
	var (
		data []byte
		err  error
	)
	if from == "-" {
		data, err = io.ReadAll(stdin)
	} else {
		data, err = os.ReadFile(from)
	}
	if err != nil {
		return Cloud{}, fmt.Errorf("read the cloud document: %w", err)
	}

	var cloud Cloud
	if err := json.Unmarshal(data, &cloud); err != nil {
		return Cloud{}, fmt.Errorf("parse the cloud document: %w", err)
	}
	if cloud.ID == "" {
		cloud.ID = NewID()
	}
	cloud.Normalize()
	return cloud, nil
}

// splitParties splits "first,second".
func splitParties(spec string) (string, string, error) {
	parts := strings.Split(spec, ",")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", usageError{fmt.Errorf("--parties wants two names separated by a comma, got %q", spec)}
	}
	return parts[0], parts[1], nil
}

// splitList splits a comma or space separated list, dropping empty entries.
func splitList(spec string) []string {
	fields := strings.FieldsFunc(spec, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' })
	return fields
}

// parseSlot accepts a statement name, case-insensitively. D2 stands in for D'
// because a bare apostrophe is awkward to type in a shell.
func parseSlot(word string) (Slot, bool) {
	switch strings.ToUpper(strings.TrimSpace(word)) {
	case "A":
		return SlotA, true
	case "B":
		return SlotB, true
	case "C":
		return SlotC, true
	case "D":
		return SlotD, true
	case "D'", "D2":
		return SlotD2, true
	}
	return "", false
}

// parseArrow accepts an arrow with or without its single hyphen and with D2
// standing in for D', so "C-D'", "C-D2" and "CD2" all name the same arrow. More
// than one hyphen is not a spelling of anything, so it is rejected rather than
// guessed at.
func parseArrow(word string) (Arrow, bool) {
	normalized := strings.ToUpper(strings.TrimSpace(word))
	normalized = strings.ReplaceAll(normalized, "D2", "D'")
	if strings.Count(normalized, "-") > 1 {
		return "", false
	}
	normalized = strings.ReplaceAll(normalized, "-", "")
	for _, arrow := range Arrows() {
		if strings.ReplaceAll(string(arrow), "-", "") == normalized {
			return arrow, true
		}
	}
	return "", false
}

// parseVerdict accepts the four judgements by name.
func parseVerdict(word string) (Verdict, bool) {
	switch strings.ToLower(strings.TrimSpace(word)) {
	case "unassessed", "unknown", "none":
		return Unassessed, true
	case "holds", "hold", "true":
		return Holds, true
	case "questionable", "question":
		return Questionable, true
	case "invalid", "false":
		return Invalid, true
	}
	return Unassessed, false
}

// parseStatusFilter resolves the --status word.
func parseStatusFilter(word string) (*Status, error) {
	word = strings.ToLower(strings.TrimSpace(word))
	if word == "" {
		return nil, nil
	}
	for _, status := range []Status{StatusDraft, StatusOpen, StatusDissolving, StatusDissolved} {
		if status.String() == word {
			return &status, nil
		}
	}
	return nil, usageError{fmt.Errorf("unknown status %q (want draft, open, dissolving or dissolved)", word)}
}

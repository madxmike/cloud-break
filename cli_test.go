package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cli runs the command line against dir and returns what a caller would see:
// standard output, standard error, and the exit status.
func cli(t *testing.T, dir string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	return cliWithStdin(t, dir, "", args...)
}

// cliWithStdin is cli with standard input, for the commands that read it.
func cliWithStdin(t *testing.T, dir, stdin string, args ...string) (stdout, stderr string, code int) {
	t.Helper()

	var out, errOut strings.Builder
	s := streams{
		stdin:  strings.NewReader(stdin),
		stdout: &out,
		stderr: &errOut,
	}
	code = runCLI(append([]string{"--dir", dir}, args...), s)
	return out.String(), errOut.String(), code
}

// mustCLI runs the command line and fails the test unless it succeeds.
func mustCLI(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, errOut, code := cli(t, dir, args...)
	if code != exitOK {
		t.Fatalf("cloud-break %s: exit %d, stderr %q", strings.Join(args, " "), code, errOut)
	}
	return out
}

// mustID runs a command that prints one identifier and returns it.
func mustID(t *testing.T, dir string, args ...string) string {
	t.Helper()
	id := strings.TrimSpace(mustCLI(t, dir, args...))
	if id == "" {
		t.Fatalf("cloud-break %s printed no id", strings.Join(args, " "))
	}
	return id
}

// populate builds a complete cloud through the command line and returns its id.
func populate(t *testing.T, dir string) string {
	t.Helper()
	id := mustID(t, dir, "new", "--title", "Release scope", "--parties", "Eng,Sales")
	for _, kv := range [][2]string{
		{"A", "ship on time"},
		{"B", "no regressions"},
		{"C", "promised features"},
		{"D", "freeze the scope"},
		{"D2", "add every feature"},
	} {
		mustCLI(t, dir, "set", id, kv[0], kv[1])
	}
	return id
}

// TestCLIBuildsAndDissolvesACloud is the whole process through the command
// line: statements, assumptions, a verdict, an injection, then verification.
func TestCLIBuildsAndDissolvesACloud(t *testing.T) {
	dir := t.TempDir()
	id := populate(t, dir)

	// One assumption on every arrow, so the second process step is reached too.
	for _, arrow := range []string{"A-B", "A-C", "C-D'", "D-D'"} {
		mustID(t, dir, "assume", "add", id, arrow, "because "+arrow)
	}
	assumption := mustID(t, dir, "assume", "add", id, "B-D", "freezing scope prevents regressions")
	mustCLI(t, dir, "assume", "verdict", id, assumption, "invalid",
		"--evidence", "two hotfixes shipped last month")

	// The status follows the evidence, so it is dissolving before the
	// injection and dissolved only after verification.
	if got := statusOf(t, dir, id); got != "dissolving" {
		t.Errorf("status = %q, want dissolving after an invalid verdict", got)
	}

	injection := mustID(t, dir, "inject", "add", id, "--breaks", assumption, "cut scope to a fixed date")
	mustCLI(t, dir, "verify", id)

	if got := statusOf(t, dir, id); got != "dissolved" {
		t.Errorf("status = %q, want dissolved", got)
	}

	show := mustCLI(t, dir, "show", id, "--json")
	var doc cloudJSON
	if err := json.Unmarshal([]byte(show), &doc); err != nil {
		t.Fatalf("show --json is not JSON: %v", err)
	}
	assumptions := doc.Assumptions[:0]
	for _, as := range doc.Assumptions {
		if as.ID == assumption {
			assumptions = append(assumptions, as)
		}
	}
	if len(assumptions) != 1 || assumptions[0].Verdict != "invalid" {
		t.Errorf("assumptions = %+v, want one invalid assumption", doc.Assumptions)
	}
	if got := assumptions[0].BrokenBy; len(got) != 1 || got[0] != injection {
		t.Errorf("broken_by = %v, want [%s]", got, injection)
	}
	if len(doc.Injections) != 1 || len(doc.Injections[0].Breaks) != 1 || doc.Injections[0].Breaks[0] != assumption {
		t.Errorf("injection breaks = %+v, want [%s]", doc.Injections, assumption)
	}
	if !doc.Verified {
		t.Error("verified = false after verify")
	}
	if len(doc.Steps) != 5 {
		t.Errorf("steps = %d, want 5", len(doc.Steps))
	}
	for _, step := range doc.Steps {
		if !step.Done {
			t.Errorf("step %d (%s) not done, want the whole process done", step.Number, step.Title)
		}
	}
}

// statusOf reads a cloud's status from the library list.
func statusOf(t *testing.T, dir, id string) string {
	t.Helper()
	for _, line := range strings.Split(mustCLI(t, dir, "list"), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 1 && fields[0] == id {
			return fields[1]
		}
	}
	t.Fatalf("cloud %s not in list", id)
	return ""
}

// TestCLIListIsLineOriented pins the list format: fixed fields, title last.
func TestCLIListIsLineOriented(t *testing.T) {
	dir := t.TempDir()
	id := populate(t, dir)

	out := mustCLI(t, dir, "list")
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("list printed %d lines, want 1: %q", len(lines), out)
	}

	// <id> <status> <filled>/<total> <assumptions> <updated> <title with spaces>
	fields := strings.SplitN(lines[0], " ", 6)
	if len(fields) != 6 {
		t.Fatalf("list line has %d fields, want 6: %q", len(fields), lines[0])
	}
	if fields[0] != id {
		t.Errorf("id = %q, want %q", fields[0], id)
	}
	if fields[1] != "open" {
		t.Errorf("status = %q, want open", fields[1])
	}
	if fields[2] != "5/5" {
		t.Errorf("progress = %q, want 5/5", fields[2])
	}
	if fields[3] != "0" {
		t.Errorf("assumptions = %q, want 0", fields[3])
	}
	if fields[5] != "Release scope" {
		t.Errorf("title = %q, want the whole title with its space", fields[5])
	}

	// The title is the only field that may hold a space, so a title with several
	// stays one field and the line stays parseable.
	mustCLI(t, dir, "title", id, "Release scope", "for", "Q4")
	if got := strings.SplitN(strings.TrimSpace(mustCLI(t, dir, "list")), " ", 6)[5]; got != "Release scope for Q4" {
		t.Errorf("title field = %q, want the whole retitled text", got)
	}
}

// TestCLIListFiltersByStatus covers the query an agent uses to find work.
func TestCLIListFiltersByStatus(t *testing.T) {
	dir := t.TempDir()
	draft := mustID(t, dir, "new", "--title", "Draft one")
	open := populate(t, dir)

	out := mustCLI(t, dir, "list", "--status", "draft")
	if !strings.Contains(out, draft) || strings.Contains(out, open) {
		t.Errorf("list --status draft = %q, want only %s", out, draft)
	}

	out = mustCLI(t, dir, "list", "--status", "open")
	if !strings.Contains(out, open) || strings.Contains(out, draft) {
		t.Errorf("list --status open = %q, want only %s", out, open)
	}

	if out := mustCLI(t, dir, "list", "--status", "dissolved"); out != "" {
		t.Errorf("list --status dissolved = %q, want nothing", out)
	}

	_, errOut, code := cli(t, dir, "list", "--status", "nonsense")
	if code != exitUsage {
		t.Errorf("exit = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(errOut, "unknown status") {
		t.Errorf("stderr = %q, want an unknown status diagnostic", errOut)
	}
}

// TestCLIEmptyLibraryIsQuiet keeps the unix rule: no records, no output.
func TestCLIEmptyLibraryIsQuiet(t *testing.T) {
	dir := t.TempDir()

	if out, _, code := cli(t, dir, "list"); out != "" || code != exitOK {
		t.Errorf("list on an empty library = %q, exit %d; want no output and %d", out, code, exitOK)
	}
	if out := mustCLI(t, dir, "list", "--json"); strings.TrimSpace(out) != "[]" {
		t.Errorf("list --json on an empty library = %q, want []", strings.TrimSpace(out))
	}
	if out, _, code := cli(t, dir, "check"); out != "" || code != exitOK {
		t.Errorf("check on an empty library = %q, exit %d; want no output and %d", out, code, exitOK)
	}
}

// TestCLIWritePrintsTheID is the property a script composes on.
func TestCLIWritePrintsTheID(t *testing.T) {
	dir := t.TempDir()

	id := mustID(t, dir, "new", "--title", "Release scope")
	if len(id) != 12 {
		t.Errorf("new printed %q, want a 12 character id", id)
	}
	if out := mustCLI(t, dir, "set", id, "A", "ship on time"); strings.TrimSpace(out) != id {
		t.Errorf("set printed %q, want the cloud id %q", strings.TrimSpace(out), id)
	}

	assumption := mustID(t, dir, "assume", "add", id, "B-D", "scope cannot change")
	if out := mustCLI(t, dir, "assume", "verdict", id, assumption, "holds"); strings.TrimSpace(out) != assumption {
		t.Errorf("assume verdict printed %q, want the assumption id %q", strings.TrimSpace(out), assumption)
	}

	injection := mustID(t, dir, "inject", "add", id, "--breaks", assumption, "cut the scope")
	if out := mustCLI(t, dir, "inject", "rm", id, injection); strings.TrimSpace(out) != injection {
		t.Errorf("inject rm printed %q, want the injection id %q", strings.TrimSpace(out), injection)
	}

	if out := mustCLI(t, dir, "delete", id); strings.TrimSpace(out) != id {
		t.Errorf("delete printed %q, want the cloud id %q", strings.TrimSpace(out), id)
	}
}

// TestCLIJSONWriteResult pins the shape a script reads back.
func TestCLIJSONWriteResult(t *testing.T) {
	dir := t.TempDir()
	id := mustID(t, dir, "new", "--title", "Release scope")

	out := mustCLI(t, dir, "assume", "add", id, "B-D", "scope cannot change", "--json")
	var result resultJSON
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("assume add --json is not JSON: %v", err)
	}
	if result.Kind != "assumption" {
		t.Errorf("kind = %q, want assumption", result.Kind)
	}
	if result.Cloud != ID(id) {
		t.Errorf("cloud = %q, want %q", result.Cloud, id)
	}
	if result.ID == "" || result.ID == id {
		t.Errorf("id = %q, want the new assumption id", result.ID)
	}
}

// TestCLIFlagsMayFollowOperands guards the interspersing pass: a flag written
// after an operand must not become part of the text.
func TestCLIFlagsMayFollowOperands(t *testing.T) {
	dir := t.TempDir()
	id := populate(t, dir)
	assumption := mustID(t, dir, "assume", "add", id, "B-D", "scope cannot change")

	// --breaks after the cloud, and the text after the flag.
	injection := mustID(t, dir, "inject", "add", id, "--breaks", assumption, "cut scope to a fixed date")

	doc := showJSON(t, dir, id)
	if len(doc.Injections) != 1 {
		t.Fatalf("injections = %d, want 1", len(doc.Injections))
	}
	if got := doc.Injections[0]; got.ID != injection {
		t.Errorf("injection id = %q, want %q", got.ID, injection)
	}
	if got := doc.Injections[0].Text; got != "cut scope to a fixed date" {
		t.Errorf("injection text = %q, want the text alone with no flag in it", got)
	}
	if got := doc.Injections[0].Breaks; len(got) != 1 || got[0] != assumption {
		t.Errorf("breaks = %v, want [%s]", got, assumption)
	}

	// The same for a flag after the operands of verdict.
	mustCLI(t, dir, "assume", "verdict", id, assumption, "invalid", "--evidence", "we hotfixed twice")
	if got := showJSON(t, dir, id).Assumptions[0].Evidence; got != "we hotfixed twice" {
		t.Errorf("evidence = %q, want the flag's value", got)
	}
}

// showJSON reads one cloud's reported document.
func showJSON(t *testing.T, dir, id string) cloudJSON {
	t.Helper()
	var doc cloudJSON
	if err := json.Unmarshal([]byte(mustCLI(t, dir, "show", id, "--json")), &doc); err != nil {
		t.Fatalf("show --json is not JSON: %v", err)
	}
	return doc
}

// TestCLIJSONAlwaysCarriesEveryField is the stability promise: a consumer never
// has to tell an absent field from an empty one.
func TestCLIJSONAlwaysCarriesEveryField(t *testing.T) {
	dir := t.TempDir()
	id := mustID(t, dir, "new", "--title", "Untouched")

	raw := mustCLI(t, dir, "show", id, "--json")
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		t.Fatalf("show --json is not a JSON object: %v", err)
	}
	for _, want := range []string{
		"id", "title", "parties", "status", "progress", "statements",
		"assumptions", "injections", "verified", "created", "updated",
		"steps", "issues",
	} {
		if _, ok := fields[want]; !ok {
			t.Errorf("show --json has no %q field", want)
		}
	}

	var statements map[string]string
	if err := json.Unmarshal(fields["statements"], &statements); err != nil {
		t.Fatalf("statements is not an object: %v", err)
	}
	for _, slot := range []string{"A", "B", "C", "D", "D'"} {
		if _, ok := statements[slot]; !ok {
			t.Errorf("statements has no %q key", slot)
		}
	}
	if got := strings.TrimSpace(string(fields["assumptions"])); got != "[]" {
		t.Errorf("assumptions = %s, want an empty array rather than null", got)
	}
	if got := strings.TrimSpace(string(fields["injections"])); got != "[]" {
		t.Errorf("injections = %s, want an empty array rather than null", got)
	}
	if got := strings.TrimSpace(string(fields["issues"])); !strings.HasPrefix(got, "[") {
		t.Errorf("issues = %s, want an array", got)
	}
}

// TestCLIReferencePrefixes covers naming a cloud by a prefix of its id, and the
// two ways a prefix can fail.
func TestCLIReferencePrefixes(t *testing.T) {
	dir := t.TempDir()
	store, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Ids chosen so the test controls where the prefixes diverge.
	first := resolved()
	first.ID = "aaaa11111111"
	second := resolved()
	second.ID = "aaaabbbbbbbb"
	writeRaw(t, store, *first)
	writeRaw(t, store, *second)

	if out := mustCLI(t, dir, "show", "aaaabbb"); !strings.Contains(out, "aaaabbbbbbbb") {
		t.Errorf("show by unique prefix = %q, want the matching cloud", out)
	}

	_, errOut, code := cli(t, dir, "show", "aaaa")
	if code != exitUsage {
		t.Errorf("exit = %d, want %d for an ambiguous prefix", code, exitUsage)
	}
	for _, want := range []string{"ambiguous", "aaaa11111111", "aaaabbbbbbbb"} {
		if !strings.Contains(errOut, want) {
			t.Errorf("stderr = %q, want it to name %q", errOut, want)
		}
	}

	if _, errOut, code := cli(t, dir, "show", "zzzz"); code != exitFailed {
		t.Errorf("exit = %d, want %d for a missing cloud (stderr %q)", code, exitFailed, errOut)
	}

	// The exact id always wins over a prefix that could also match it.
	if out := mustCLI(t, dir, "show", "aaaa11111111"); !strings.Contains(out, "aaaa11111111") {
		t.Errorf("show by exact id = %q", out)
	}
}

// TestCLIAssumptionPrefixes covers the same rule inside a cloud.
func TestCLIAssumptionPrefixes(t *testing.T) {
	dir := t.TempDir()
	id := populate(t, dir)
	assumption := mustID(t, dir, "assume", "add", id, "B-D", "scope cannot change")

	if out := mustCLI(t, dir, "assume", "verdict", id, assumption[:4], "holds"); strings.TrimSpace(out) != assumption {
		t.Errorf("verdict by prefix printed %q, want %q", strings.TrimSpace(out), assumption)
	}
	if _, errOut, code := cli(t, dir, "assume", "rm", id, ""); code != exitUsage {
		t.Errorf("exit = %d, want %d for an empty reference (stderr %q)", code, exitUsage, errOut)
	}
}

// TestCLIExitStatuses pins the three statuses a script branches on.
func TestCLIExitStatuses(t *testing.T) {
	dir := t.TempDir()
	id := populate(t, dir)

	tests := []struct {
		name string
		args []string
		want int
	}{
		{"no such command", []string{"bogus"}, exitUsage},
		{"unknown flag", []string{"list", "--nope"}, exitUsage},
		{"missing operand", []string{"show"}, exitUsage},
		{"too many operands", []string{"show", id, id}, exitUsage},
		{"unknown slot", []string{"set", id, "Q", "text"}, exitUsage},
		{"unknown arrow", []string{"assume", "add", id, "A-Z", "text"}, exitUsage},
		{"unknown verdict word", []string{"assume", "verdict", id, "abc", "maybe"}, exitUsage},
		{"unknown subcommand", []string{"assume", "frobnicate"}, exitUsage},
		{"missing cloud", []string{"show", "nosuchcloud"}, exitFailed},
		{"missing cloud on check", []string{"check", "nosuchcloud"}, exitFailed},
		{"unreadable from", []string{"new", "--from", filepath.Join(dir, "nope.json")}, exitFailed},
		{"conflicting flags", []string{"new", "--from", "-", "--title", "x"}, exitUsage},
		{"demo with a named library", []string{"--demo", "--dir", dir, "list"}, exitUsage},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, errOut, code := cliWithStdin(t, dir, "{}", tt.args...)
			if code != tt.want {
				t.Errorf("exit = %d, want %d (stderr %q)", code, tt.want, errOut)
			}
			if errOut == "" {
				t.Error("no diagnostic on stderr for a failed command")
			}
		})
	}
}

// TestCLIDiagnosticsStayOffStdout is the rule that lets a caller pipe the data.
func TestCLIDiagnosticsStayOffStdout(t *testing.T) {
	dir := t.TempDir()

	out, errOut, code := cli(t, dir, "show", "missing")
	if code == exitOK {
		t.Fatalf("exit = %d, want a failure", code)
	}
	if out != "" {
		t.Errorf("stdout = %q, want nothing but data", out)
	}
	if !strings.HasPrefix(errOut, "cloud-break: ") {
		t.Errorf("stderr = %q, want the cloud-break prefix", errOut)
	}
}

// TestCLICheckReportsAndFails covers the check contract: one line per finding,
// and an error fails the run.
func TestCLICheckReportsAndFails(t *testing.T) {
	dir := t.TempDir()
	id := mustID(t, dir, "new", "--title", "Half done")

	out, _, code := cli(t, dir, "check", id)
	if code != exitFailed {
		t.Errorf("exit = %d, want %d when a statement is missing", code, exitFailed)
	}
	if !strings.Contains(out, id+" error ") {
		t.Errorf("check output = %q, want a line beginning %q", out, id+" error ")
	}

	// A complete cloud with no assumptions is inconclusive, not broken: the run
	// passes and the warnings are still reported.
	complete := populate(t, dir)
	out, _, code = cli(t, dir, "check", complete)
	if code != exitOK {
		t.Errorf("exit = %d, want %d for warnings only", code, exitOK)
	}
	if !strings.Contains(out, complete+" warning ") {
		t.Errorf("check output = %q, want warnings for the bare arrows", out)
	}

	// JSON is the same data. The run still exits nonzero because the library
	// holds a cloud with missing statements.
	raw, _, _ := cli(t, dir, "check", "--json")
	var issues []issueJSON
	if err := json.Unmarshal([]byte(raw), &issues); err != nil {
		t.Fatalf("check --json is not JSON: %v", err)
	}
	if len(issues) == 0 {
		t.Error("check --json reported no issues, want the missing statements")
	}
	for _, issue := range issues {
		if issue.Cloud == "" || issue.Severity == "" || issue.Message == "" {
			t.Errorf("issue %+v is missing a field", issue)
		}
	}
}

// TestCLIVerifyIsEarned keeps the tool's discipline: a cloud is never marked
// dissolved until an invalid assumption is injected against.
func TestCLIVerifyIsEarned(t *testing.T) {
	dir := t.TempDir()
	id := populate(t, dir)

	_, errOut, code := cli(t, dir, "verify", id)
	if code != exitFailed {
		t.Errorf("exit = %d, want %d while nothing is broken", code, exitFailed)
	}
	if !strings.Contains(errOut, "nothing is broken") {
		t.Errorf("stderr = %q, want an explanation", errOut)
	}

	// An invalid verdict alone is not enough: the injection has to break it.
	assumption := mustID(t, dir, "assume", "add", id, "B-D", "scope cannot change")
	mustCLI(t, dir, "assume", "verdict", id, assumption, "invalid")
	if _, _, code := cli(t, dir, "verify", id); code != exitFailed {
		t.Errorf("exit = %d, want %d with no injection", code, exitFailed)
	}

	mustID(t, dir, "inject", "add", id, "--breaks", assumption, "cut the scope")
	mustCLI(t, dir, "verify", id)
	if got := statusOf(t, dir, id); got != "dissolved" {
		t.Errorf("status = %q, want dissolved", got)
	}

	// Verification can be withdrawn.
	mustCLI(t, dir, "verify", id, "--undo")
	if got := statusOf(t, dir, id); got != "dissolving" {
		t.Errorf("status after --undo = %q, want dissolving", got)
	}
	if showJSON(t, dir, id).Verified {
		t.Error("verified = true after --undo")
	}
}

// TestCLISetEmptyClearsAStatement covers the way to undo a statement.
func TestCLISetEmptyClearsAStatement(t *testing.T) {
	dir := t.TempDir()
	id := populate(t, dir)

	mustCLI(t, dir, "set", id, "A", "")
	if got := showJSON(t, dir, id).Statements["A"]; got != "" {
		t.Errorf("A = %q, want cleared", got)
	}
	if got := statusOf(t, dir, id); got != "draft" {
		t.Errorf("status = %q, want draft after clearing a statement", got)
	}

	// A missing text is a usage error rather than a silent clear.
	if _, _, code := cli(t, dir, "set", id, "A"); code != exitUsage {
		t.Errorf("exit = %d, want %d for a missing text", code, exitUsage)
	}
}

// TestCLIAssumeTypingIsForgiving covers the spellings a shell makes awkward.
func TestCLIAssumeTypingIsForgiving(t *testing.T) {
	for _, arrow := range []string{"C-D'", "C-D2", "c-d2", "CD'", "CD2"} {
		if got, ok := parseArrow(arrow); !ok || got != ArrowCD2 {
			t.Errorf("parseArrow(%q) = %q, %v; want %q", arrow, got, ok, ArrowCD2)
		}
	}
	for _, arrow := range []string{"A-B", "ab", "A-B-"} {
		_, ok := parseArrow(arrow)
		if want := arrow != "A-B-"; ok != want {
			t.Errorf("parseArrow(%q) ok = %v, want %v", arrow, ok, want)
		}
	}
	for _, slot := range []string{"A", "b", "C", "d", "D'", "D2"} {
		if _, ok := parseSlot(slot); !ok {
			t.Errorf("parseSlot(%q) rejected", slot)
		}
	}
	for _, word := range []string{"unassessed", "none", "holds", "hold", "questionable", "invalid", "INVALID"} {
		if _, ok := parseVerdict(word); !ok {
			t.Errorf("parseVerdict(%q) rejected", word)
		}
	}
	if _, ok := parseVerdict("probably"); ok {
		t.Error("parseVerdict accepted a word it does not define")
	}

	// D2 in a statement slot lands on D'.
	dir := t.TempDir()
	id := mustID(t, dir, "new", "--title", "Typing")
	mustCLI(t, dir, "set", id, "d2", "add every feature")
	if got := showJSON(t, dir, id).Statements["D'"]; got != "add every feature" {
		t.Errorf("D' = %q, want the text written through D2", got)
	}
}

// TestCLIChildrenFollowTheirParents covers the links that survive a deletion.
func TestCLIChildrenFollowTheirParents(t *testing.T) {
	dir := t.TempDir()
	id := populate(t, dir)
	first := mustID(t, dir, "assume", "add", id, "B-D", "first")
	second := mustID(t, dir, "assume", "add", id, "B-D", "second")
	mustID(t, dir, "inject", "add", id, "--breaks", first+","+second, "break both")

	mustCLI(t, dir, "assume", "rm", id, first)

	doc := showJSON(t, dir, id)
	if len(doc.Assumptions) != 1 || doc.Assumptions[0].ID != second {
		t.Errorf("assumptions = %+v, want only %s", doc.Assumptions, second)
	}
	if len(doc.Injections) != 1 {
		t.Fatalf("injections = %d, want the injection to survive", len(doc.Injections))
	}
	if got := doc.Injections[0].Breaks; len(got) != 1 || got[0] != second {
		t.Errorf("breaks = %v, want only the surviving assumption", got)
	}
}

// TestCLIExportRoundTrips moves a cloud between libraries.
func TestCLIExportRoundTrips(t *testing.T) {
	source := t.TempDir()
	id := dirWithAssumption(t, source)

	document := mustCLI(t, source, "export", id)

	target := t.TempDir()
	copied := strings.TrimSpace(mustCLIWithStdin(t, target, document, "new", "--from", "-"))
	if copied != id {
		t.Errorf("copied id = %q, want %q", copied, id)
	}

	// Importing stamps a new updated time, so the documents are compared by
	// what they say rather than by when they were written.
	from, to := showJSON(t, source, id), showJSON(t, target, id)
	if to.Title != from.Title {
		t.Errorf("title = %q, want %q", to.Title, from.Title)
	}
	if to.Parties != from.Parties {
		t.Errorf("parties = %v, want %v", to.Parties, from.Parties)
	}
	if fmt.Sprint(to.Statements) != fmt.Sprint(from.Statements) {
		t.Errorf("statements = %v, want %v", to.Statements, from.Statements)
	}
	if len(to.Assumptions) != len(from.Assumptions) || len(to.Injections) != len(from.Injections) {
		t.Errorf("copy holds %d assumptions and %d injections, want %d and %d",
			len(to.Assumptions), len(to.Injections), len(from.Assumptions), len(from.Injections))
	}
	if len(to.Assumptions) == 1 {
		if got, want := to.Assumptions[0], from.Assumptions[0]; got.Verdict != want.Verdict || got.Evidence != want.Evidence {
			t.Errorf("assumption = %+v, want %+v", got, want)
		}
	}

	// The same document cannot be created twice in one library.
	_, errOut, code := cliWithStdin(t, target, document, "new", "--from", "-")
	if code != exitFailed {
		t.Errorf("exit = %d, want %d for a duplicate id", code, exitFailed)
	}
	if !strings.Contains(errOut, "already exists") {
		t.Errorf("stderr = %q, want an already exists diagnostic", errOut)
	}
}

// mustCLIWithStdin is mustCLI for the commands that read standard input.
func mustCLIWithStdin(t *testing.T, dir, stdin string, args ...string) string {
	t.Helper()
	out, errOut, code := cliWithStdin(t, dir, stdin, args...)
	if code != exitOK {
		t.Fatalf("cloud-break %s: exit %d, stderr %q", strings.Join(args, " "), code, errOut)
	}
	return out
}

// dirWithAssumption prepares a library holding one dissolvable cloud and
// returns its id.
func dirWithAssumption(t *testing.T, dir string) string {
	t.Helper()
	id := populate(t, dir)
	assumption := mustID(t, dir, "assume", "add", id, "B-D", "scope cannot change")
	mustCLI(t, dir, "assume", "verdict", id, assumption, "invalid", "--evidence", "we hotfixed twice")
	mustID(t, dir, "inject", "add", id, "--breaks", assumption, "cut the scope")
	return id
}

// TestCLINewFromRejectsGarbage keeps a malformed document out of the library.
func TestCLINewFromRejectsGarbage(t *testing.T) {
	dir := t.TempDir()

	_, errOut, code := cliWithStdin(t, dir, "{not json", "new", "--from", "-")
	if code != exitFailed {
		t.Errorf("exit = %d, want %d", code, exitFailed)
	}
	if !strings.Contains(errOut, "parse the cloud document") {
		t.Errorf("stderr = %q, want a parse diagnostic", errOut)
	}
	if out := mustCLI(t, dir, "list"); out != "" {
		t.Errorf("list = %q, want the library untouched", out)
	}
}

// TestCLINewFromAssignsAnID covers a document copied from somewhere else.
func TestCLINewFromAssignsAnID(t *testing.T) {
	dir := t.TempDir()

	document := `{"title": "From a file", "parties": ["Us", "Them"],
		"statements": {"A": "ship on time", "B": "no regressions"}}`
	id := strings.TrimSpace(mustCLIWithStdin(t, dir, document, "new", "--from", "-"))
	if len(id) != 12 {
		t.Errorf("id = %q, want a 12 character id", id)
	}
	doc := showJSON(t, dir, id)
	if doc.Title != "From a file" || doc.Statements["A"] != "ship on time" {
		t.Errorf("document = %+v, want the statements from the file", doc)
	}
}

// TestCLIPartiesAndTitles covers the naming commands.
func TestCLIPartiesAndTitles(t *testing.T) {
	dir := t.TempDir()
	id := mustID(t, dir, "new", "--title", "Release scope", "--parties", "Eng,Sales")

	doc := showJSON(t, dir, id)
	if doc.Parties != [2]string{"Eng", "Sales"} {
		t.Errorf("parties = %v, want [Eng Sales]", doc.Parties)
	}

	mustCLI(t, dir, "parties", id, "Platform", "Sales")
	mustCLI(t, dir, "title", id, "Release", "scope", "for", "Q4")

	doc = showJSON(t, dir, id)
	if doc.Parties != [2]string{"Platform", "Sales"} {
		t.Errorf("parties = %v, want [Platform Sales]", doc.Parties)
	}
	if doc.Title != "Release scope for Q4" {
		t.Errorf("title = %q, want the joined text", doc.Title)
	}

	if _, _, code := cli(t, dir, "parties", id, "Only"); code != exitUsage {
		t.Errorf("exit = %d, want %d for one party", code, exitUsage)
	}
	if _, _, code := cli(t, dir, "new", "--parties", "one"); code != exitUsage {
		t.Errorf("exit = %d, want %d for a malformed --parties", code, exitUsage)
	}
}

// TestCLIRender pins the diagram: the statements appear inside it, and it
// refuses a width it cannot draw in.
func TestCLIRender(t *testing.T) {
	dir := t.TempDir()
	id := populate(t, dir)

	out := mustCLI(t, dir, "render", id)
	for _, want := range []string{
		"ship on time", "no regressions", "promised features",
		"freeze the scope", "add every feature",
		"A · Objective", "B · Need (Eng)", "D' · Want (Sales)",
		"assumptions A-B 0",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("render output does not contain %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "\x1b[") {
		t.Error("render emitted an escape sequence; piped output must be plain")
	}

	if narrow, _, code := cli(t, dir, "render", id, "--width", "30"); code != exitUsage || narrow != "" {
		t.Errorf("render --width 30 = %q, exit %d; want a usage error", narrow, code)
	}
	mustCLI(t, dir, "render", id, "--width", "100")
}

// TestCLIDemoLeavesTheLibraryAlone covers the throwaway library.
func TestCLIDemoLeavesTheLibraryAlone(t *testing.T) {
	dir := t.TempDir()

	out := mustCLIEnv(t, dir, "--demo", "list")
	if !strings.Contains(out, "example-") {
		t.Errorf("--demo list = %q, want the bundled examples", out)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("the named library holds %d entries, want the demo kept apart", len(entries))
	}

	// A demo cloud can be read and is thrown away with the run.
	example := strings.Fields(strings.SplitN(out, "\n", 2)[0])[0]
	if got := mustCLIEnv(t, dir, "--demo", "show", example); !strings.Contains(got, example) {
		t.Errorf("--demo show = %q, want the example cloud", got)
	}

	// Naming a library as well contradicts a throwaway one.
	if _, _, code := cli(t, dir, "--demo", "list"); code != exitUsage {
		t.Errorf("exit = %d, want %d for --demo with --dir", code, exitUsage)
	}
}

// mustCLIEnv runs the command line with no --dir, so the environment names the
// library, and fails the test unless it succeeds.
func mustCLIEnv(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, errOut, code := cliEnv(t, dir, args...)
	if code != exitOK {
		t.Fatalf("cloud-break %s: exit %d, stderr %q", strings.Join(args, " "), code, errOut)
	}
	return out
}

// cliEnv runs the command line against the library named by the environment.
func cliEnv(t *testing.T, dir string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	t.Setenv("CLOUD_BREAK_HOME", dir)

	var out, errOut strings.Builder
	s := streams{stdin: strings.NewReader(""), stdout: &out, stderr: &errOut}
	code = runCLI(args, s)
	return out.String(), errOut.String(), code
}

// TestCLIHelpTopics covers the documentation entry points.
func TestCLIHelpTopics(t *testing.T) {
	dir := t.TempDir()

	for _, args := range [][]string{
		{"--help"},
		{"-h"},
		{"help"},
		{"--help", "clouds"},
		{"--help", "agent"},
		{"help", "clouds"},
		{"help", "agent"},
		{"list", "--help"},
	} {
		out, errOut, code := cli(t, dir, args...)
		if code != exitOK {
			t.Errorf("%v: exit = %d, want %d (stderr %q)", args, code, exitOK, errOut)
		}
		if out == "" {
			t.Errorf("%v: nothing on stdout", args)
		}
	}

	if out := mustCLI(t, dir, "--help", "agent"); !strings.Contains(out, "exit status") && !strings.Contains(out, "status   0 success") {
		t.Errorf("the agent topic does not state the contract: %q", out)
	}

	_, errOut, code := cli(t, dir, "--help", "nonsense")
	if code != exitUsage {
		t.Errorf("exit = %d, want %d for an unknown topic", code, exitUsage)
	}
	if !strings.Contains(errOut, "unknown help topic") {
		t.Errorf("stderr = %q, want an unknown topic diagnostic", errOut)
	}

	if _, errOut, code := cli(t, dir, "help", "nonsense"); code != exitUsage {
		t.Errorf("help nonsense: exit = %d, want %d (stderr %q)", code, exitUsage, errOut)
	}
}

// TestCLIManPage covers --man, and the escaping that keeps the text out of
// roff's control.
func TestCLIManPage(t *testing.T) {
	out := mustCLI(t, t.TempDir(), "--man")

	for _, want := range []string{".TH CLOUD-BREAK 1", ".SH NAME", ".SH USAGE", ".SH CLOUDS", ".SH AGENT"} {
		if !strings.Contains(out, want) {
			t.Errorf("--man output has no %q", want)
		}
	}
	// A line that begins with a dot in the help text must be escaped, or roff
	// reads it as a request.
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, ".") && !strings.HasPrefix(line, ".TH") &&
			!strings.HasPrefix(line, ".SH") && !strings.HasPrefix(line, ".nf") &&
			!strings.HasPrefix(line, ".fi") && !strings.HasPrefix(line, `\&`) {
			t.Errorf("unescaped roff request in the manual: %q", line)
		}
	}
}

// TestCLIReadsStdinDocument is the pipeline shape an agent uses to add a cloud.
func TestCLIReadsStdinDocument(t *testing.T) {
	dir := t.TempDir()
	document := mustCLI(t, dir, "export", dirWithAssumption(t, dir))

	other := t.TempDir()
	id := strings.TrimSpace(mustCLIWithStdin(t, other, document, "new", "--from", "-"))

	if out := mustCLI(t, other, "list"); !strings.Contains(out, id) || !strings.Contains(out, "dissolving") {
		t.Errorf("list = %q, want the imported cloud in its own state", out)
	}
}

// TestCLIWriteErrorsAreReported keeps a failed open from passing silently.
func TestCLIWriteErrorsAreReported(t *testing.T) {
	dir := t.TempDir()

	// A file where the library directory should be cannot be opened, so every
	// command that touches the library fails with a diagnostic.
	file := filepath.Join(dir, "not-a-directory")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, errOut, code := cli(t, filepath.Join(file, "library"), "list")
	if code != exitFailed {
		t.Errorf("exit = %d, want %d (stderr %q)", code, exitFailed, errOut)
	}
	if out != "" {
		t.Errorf("stdout = %q, want nothing", out)
	}
	if !strings.Contains(errOut, "open the library") {
		t.Errorf("stderr = %q, want an open diagnostic", errOut)
	}
}

// TestCLIRendersTheDiagramOfADissolvedCloud covers the plain drawing end to
// end: the verdict ledger shows the assumptions that carried the work.
func TestCLIRendersTheDiagramOfADissolvedCloud(t *testing.T) {
	dir := t.TempDir()
	id := dirWithAssumption(t, dir)
	mustCLI(t, dir, "verify", id)

	out := mustCLI(t, dir, "render", id)
	if !strings.Contains(out, "dissolv") && statusOf(t, dir, id) != "dissolved" {
		t.Errorf("status = %q, want dissolved", statusOf(t, dir, id))
	}
	if !strings.Contains(out, "assumptions A-B 0 · A-C 0 · B-D 1") {
		t.Errorf("render ledger = %q, want the assumption counts", out)
	}
}

// TestInvocationStoreResolvesTheNamedLibrary keeps the store resolution out of
// the process environment when a directory is given.
func TestInvocationStoreResolvesTheNamedLibrary(t *testing.T) {
	t.Setenv("CLOUD_BREAK_HOME", filepath.Join(t.TempDir(), "ignored"))
	dir := t.TempDir()

	inv := &invocation{s: streams{stdin: strings.NewReader(""), stdout: io.Discard, stderr: io.Discard}}
	store, cleanup, err := inv.store(settings{dir: dir})
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	defer cleanup()
	if store.Dir() != dir {
		t.Errorf("dir = %q, want the named directory %q", store.Dir(), dir)
	}
}

// TestCLIUnknownFlagPrintsUsage covers the parser failure path.
func TestCLIUnknownFlagPrintsUsage(t *testing.T) {
	out, errOut, code := cli(t, t.TempDir(), "list", "--nope")
	if code != exitUsage {
		t.Errorf("exit = %d, want %d", code, exitUsage)
	}
	if out != "" {
		t.Errorf("stdout = %q, want nothing", out)
	}
	if !strings.Contains(errOut, "nope") {
		t.Errorf("stderr = %q, want the offending flag", errOut)
	}
	if !strings.Contains(errOut, "Usage:") {
		t.Errorf("stderr = %q, want the usage text after the diagnostic", errOut)
	}
}

// TestCLIStatusHelperMatchesTheModel keeps the reported status words in step
// with the model.
func TestCLIStatusHelperMatchesTheModel(t *testing.T) {
	for _, status := range []Status{StatusDraft, StatusOpen, StatusDissolving, StatusDissolved} {
		got, err := parseStatusFilter(status.String())
		if err != nil {
			t.Errorf("parseStatusFilter(%q) = %v", status.String(), err)
			continue
		}
		if got == nil || *got != status {
			t.Errorf("parseStatusFilter(%q) = %v, want %v", status.String(), got, status)
		}
	}
	if _, err := parseStatusFilter(""); err != nil {
		t.Errorf("parseStatusFilter(\"\") = %v, want no filter", err)
	}
}

// TestCLIExactCloudPrefersTheExactID guards the rule that an exact id always
// wins, so a cloud whose id prefixes another is still reachable.
func TestCLIExactCloudPrefersTheExactID(t *testing.T) {
	short := Cloud{ID: "abc"}
	long := Cloud{ID: "abcdef"}
	if got, ok := exactCloud([]Cloud{short, long}, "abc"); !ok || got.ID != short.ID {
		t.Errorf("exactCloud = %q, %v; want the exact match", got.ID, ok)
	}
	if _, ok := exactCloud([]Cloud{short, long}, "ab"); ok {
		t.Error("exactCloud accepted a prefix")
	}
}

// TestCLISaveReportsNormalizedDocument covers the read-back after a write.
func TestCLISaveReportsNormalizedDocument(t *testing.T) {
	store, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cloud := New("  Spaced   title  ")
	cloud.SetText(SlotA, "  multi   space  ")

	saved, err := saveCloud(store, *cloud)
	if err != nil {
		t.Fatalf("saveCloud: %v", err)
	}
	if saved.Title != "Spaced title" {
		t.Errorf("title = %q, want the normalized form", saved.Title)
	}
	if got := saved.Text(SlotA); got != "multi space" {
		t.Errorf("A = %q, want the normalized statement", got)
	}
}

// TestCLIErrorKindsChooseTheStatus makes the mapping explicit.
func TestCLIErrorKindsChooseTheStatus(t *testing.T) {
	tests := []struct {
		err  error
		want int
	}{
		{usageError{errors.New("bad")}, exitUsage},
		{flagError{errors.New("bad flag")}, exitUsage},
		{notFoundError{errors.New("gone")}, exitFailed},
		{checksFailed{2}, exitFailed},
		{fmt.Errorf("plain"), exitFailed},
	}
	for _, tt := range tests {
		if got := dispositionOf(tt.err).code; got != tt.want {
			t.Errorf("dispositionOf(%v) = %d, want %d", tt.err, got, tt.want)
		}
	}
	if got := dispositionOf(nil).code; got != exitOK {
		t.Errorf("dispositionOf(nil) = %d, want %d", got, exitOK)
	}
}

// TestCLICheckReportsFilesItCouldNotRead keeps a damaged document from making a
// validation run look complete.
func TestCLICheckReportsFilesItCouldNotRead(t *testing.T) {
	dir := t.TempDir()
	id := populate(t, dir)

	if err := os.WriteFile(filepath.Join(dir, "damaged.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, errOut, code := cli(t, dir, "check")
	if code != exitOK {
		t.Errorf("exit = %d, want %d: a damaged file is a warning, not a failed check (stderr %q)", code, exitOK, errOut)
	}
	if !strings.Contains(errOut, "skipped damaged.json") {
		t.Errorf("stderr = %q, want a note about the skipped file", errOut)
	}
	if !strings.Contains(out, id) {
		t.Errorf("stdout = %q, want the readable cloud still checked", out)
	}

	// Naming clouds explicitly checks only those, so nothing was skipped.
	if _, errOut, _ := cli(t, dir, "check", id); strings.Contains(errOut, "skipped") {
		t.Errorf("stderr = %q, want no skip note when clouds are named", errOut)
	}
}

// TestCLIDoubleDashEndsFlags covers storing text that looks like a flag.
func TestCLIDoubleDashEndsFlags(t *testing.T) {
	dir := t.TempDir()
	id := populate(t, dir)

	// Without "--" a help request wins, which is the documented behaviour of
	// asking for help anywhere on the line.
	if out, _, code := cli(t, dir, "set", id, "A", "--help"); code != exitOK || !strings.Contains(out, "Usage:") {
		t.Errorf("set with --help = %q, exit %d; want the usage text", out, code)
	}

	// After "--" the same word is an operand.
	mustCLI(t, dir, "set", id, "A", "--", "--help")
	if got := showJSON(t, dir, id).Statements["A"]; got != "--help" {
		t.Errorf("A = %q, want the literal text after --", got)
	}
}

// TestCLIBareInvocationNeedsATerminal keeps a pipe from hanging on a
// full-screen interface that will never appear.
func TestCLIBareInvocationNeedsATerminal(t *testing.T) {
	out, errOut, code := cli(t, t.TempDir())
	if code != exitUsage {
		t.Errorf("exit = %d, want %d", code, exitUsage)
	}
	if out != "" {
		t.Errorf("stdout = %q, want nothing", out)
	}
	if !strings.Contains(errOut, "needs a terminal") {
		t.Errorf("stderr = %q, want it to explain the terminal requirement", errOut)
	}
	if !strings.Contains(errOut, "cloud-break --help") {
		t.Errorf("stderr = %q, want it to point at the commands", errOut)
	}
}

// TestCLIAssumeEdits covers the two commands that change an assumption in place.
func TestCLIAssumeEdits(t *testing.T) {
	dir := t.TempDir()
	id := populate(t, dir)
	assumption := mustID(t, dir, "assume", "add", id, "B-D", "scope cannot change")

	if out := mustCLI(t, dir, "assume", "set", id, assumption, "scope can change with a flag"); strings.TrimSpace(out) != assumption {
		t.Errorf("assume set printed %q, want %q", strings.TrimSpace(out), assumption)
	}
	if got := showJSON(t, dir, id).Assumptions[0].Text; got != "scope can change with a flag" {
		t.Errorf("text = %q, want the reworded assumption", got)
	}

	mustCLI(t, dir, "assume", "evidence", id, assumption, "we shipped behind a flag last month")
	if got := showJSON(t, dir, id).Assumptions[0].Evidence; got != "we shipped behind a flag last month" {
		t.Errorf("evidence = %q, want the recorded evidence", got)
	}

	// An empty evidence clears it.
	mustCLI(t, dir, "assume", "evidence", id, assumption, "")
	if got := showJSON(t, dir, id).Assumptions[0].Evidence; got != "" {
		t.Errorf("evidence = %q, want it cleared", got)
	}
}

// TestCLIAmbiguousChildReference covers the diagnostic for a prefix that matches
// more than one assumption, and the one for a missing injection.
func TestCLIAmbiguousChildReference(t *testing.T) {
	dir := t.TempDir()
	id := populate(t, dir)

	store, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	cloud, err := store.Load(ID(id))
	if err != nil {
		t.Fatal(err)
	}
	// Ids chosen so a short prefix matches both.
	cloud.Assumptions = []Assumption{
		{ID: "aaaa1111", Arrow: ArrowBD, Text: "first"},
		{ID: "aaaabbbb", Arrow: ArrowBD, Text: "second"},
	}
	if err := store.Save(cloud); err != nil {
		t.Fatal(err)
	}

	_, errOut, code := cli(t, dir, "assume", "rm", id, "aaaa")
	if code != exitUsage {
		t.Errorf("exit = %d, want %d for an ambiguous assumption", code, exitUsage)
	}
	for _, want := range []string{"ambiguous", "aaaa1111", "aaaabbbb"} {
		if !strings.Contains(errOut, want) {
			t.Errorf("stderr = %q, want it to name %q", errOut, want)
		}
	}

	if _, errOut, code := cli(t, dir, "inject", "rm", id, "nothing"); code != exitFailed {
		t.Errorf("exit = %d, want %d for a missing injection (stderr %q)", code, exitFailed, errOut)
	}
	if _, _, code := cli(t, dir, "inject", "rm", id, ""); code != exitUsage {
		t.Errorf("exit = %d, want %d for an empty injection reference", code, exitUsage)
	}
}

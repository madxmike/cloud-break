package main

// The command line's output is a contract, so it is written through the types
// in this file rather than by marshalling the stored documents directly.
//
// Two shapes are offered for every read command. The text shape is
// line-oriented: one record per line, fields separated by single spaces, and
// the one field that may contain spaces (a title, a statement, an assumption)
// always last. The JSON shape is the same data with every field always present,
// so a consumer never has to tell "absent" from "empty".
//
// Nothing derived is left for the reader to compute: status, progress, the
// process steps and the validation issues travel with the cloud, and the links
// between assumptions and injections are reported from both ends.

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// cloudJSON is one cloud as the command line reports it.
type cloudJSON struct {
	ID          ID                `json:"id"`
	Title       string            `json:"title"`
	Parties     [2]string         `json:"parties"`
	Status      string            `json:"status"`
	Progress    progressJSON      `json:"progress"`
	Statements  map[string]string `json:"statements"`
	Assumptions []assumptionJSON  `json:"assumptions"`
	Injections  []injectionJSON   `json:"injections"`
	Verified    bool              `json:"verified"`
	Created     time.Time         `json:"created"`
	Updated     time.Time         `json:"updated"`
	Steps       []stepJSON        `json:"steps"`
	Issues      []issueJSON       `json:"issues"`
}

// progressJSON counts the statements filled in.
type progressJSON struct {
	Filled int `json:"filled"`
	Total  int `json:"total"`
}

// assumptionJSON is one assumption, with the injections that break it named
// from this end as well as the other.
type assumptionJSON struct {
	ID       string   `json:"id"`
	Arrow    string   `json:"arrow"`
	Text     string   `json:"text"`
	Verdict  string   `json:"verdict"`
	Evidence string   `json:"evidence"`
	BrokenBy []string `json:"broken_by"`
}

// injectionJSON is one injection: the change, and the assumptions it breaks.
type injectionJSON struct {
	ID        string   `json:"id"`
	Text      string   `json:"text"`
	Breaks    []string `json:"breaks"`
	Rationale string   `json:"rationale"`
}

// stepJSON is one stage of the process and whether the cloud has reached it.
type stepJSON struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Done   bool   `json:"done"`
}

// issueJSON is one validation finding.
type issueJSON struct {
	Cloud    ID     `json:"cloud"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// summaryJSON is one cloud as the library list reports it: enough to choose a
// cloud, not the whole document.
type summaryJSON struct {
	ID          ID           `json:"id"`
	Title       string       `json:"title"`
	Status      string       `json:"status"`
	Progress    progressJSON `json:"progress"`
	Assumptions int          `json:"assumptions"`
	Verified    bool         `json:"verified"`
	Updated     time.Time    `json:"updated"`
}

// resultJSON reports what a writing command changed.
type resultJSON struct {
	Kind    string `json:"kind"`
	Cloud   ID     `json:"cloud"`
	ID      string `json:"id"`
	Deleted bool   `json:"deleted,omitempty"`
}

// issueWords names a severity for output. The words are stable; the interface
// uses marks instead.
func issueWords(s Severity) string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	}
	return "info"
}

// newCloudJSON builds the reported document for a cloud.
func newCloudJSON(c Cloud) cloudJSON {
	filled, total := c.Progress()

	statements := make(map[string]string, len(SlotInfos()))
	for _, info := range SlotInfos() {
		statements[string(info.Slot)] = c.Text(info.Slot)
	}

	assumptions := make([]assumptionJSON, 0, len(c.Assumptions))
	for _, as := range c.Assumptions {
		brokenBy := []string{}
		for _, inj := range c.Injections {
			for _, id := range inj.Breaks {
				if id == as.ID {
					brokenBy = append(brokenBy, inj.ID)
					break
				}
			}
		}
		assumptions = append(assumptions, assumptionJSON{
			ID:       as.ID,
			Arrow:    string(as.Arrow),
			Text:     as.Text,
			Verdict:  as.Verdict.String(),
			Evidence: as.Evidence,
			BrokenBy: brokenBy,
		})
	}

	injections := make([]injectionJSON, 0, len(c.Injections))
	for _, inj := range c.Injections {
		breaks := inj.Breaks
		if breaks == nil {
			breaks = []string{}
		}
		injections = append(injections, injectionJSON{
			ID:        inj.ID,
			Text:      inj.Text,
			Breaks:    breaks,
			Rationale: inj.Rationale,
		})
	}

	steps := make([]stepJSON, 0, len(c.Steps()))
	for _, step := range c.Steps() {
		steps = append(steps, stepJSON{Number: step.Number, Title: step.Title, Done: step.Done})
	}

	issues := make([]issueJSON, 0, len(c.Validate()))
	for _, issue := range c.Validate() {
		issues = append(issues, issueJSON{
			Cloud:    c.ID,
			Severity: issueWords(issue.Severity),
			Message:  issue.Message,
		})
	}

	return cloudJSON{
		ID:          c.ID,
		Title:       c.Title,
		Parties:     c.Parties,
		Status:      c.Status().String(),
		Progress:    progressJSON{Filled: filled, Total: total},
		Statements:  statements,
		Assumptions: assumptions,
		Injections:  injections,
		Verified:    c.Verified,
		Created:     c.Created.UTC(),
		Updated:     c.Updated.UTC(),
		Steps:       steps,
		Issues:      issues,
	}
}

// newSummaryJSON builds the reported summary for a cloud.
func newSummaryJSON(c Cloud) summaryJSON {
	filled, total := c.Progress()
	return summaryJSON{
		ID:          c.ID,
		Title:       c.Title,
		Status:      c.Status().String(),
		Progress:    progressJSON{Filled: filled, Total: total},
		Assumptions: len(c.Assumptions),
		Verified:    c.Verified,
		Updated:     c.Updated.UTC(),
	}
}

// writeJSON writes one value as indented JSON followed by a newline. HTML
// escaping is off so statements keep their quotation marks readable.
func writeJSON(w io.Writer, value any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(value)
}

// writeSummaryLine writes one cloud as a single line:
//
//	<id> <status> <filled>/<total> <assumptions> <updated> <title>
//
// The title comes last because it is the only field that can contain spaces.
// Every free text is collapsed to one line, so a hand-edited document cannot
// break the one-record-per-line property.
func writeSummaryLine(w io.Writer, c Cloud) error {
	s := newSummaryJSON(c)
	_, err := fmt.Fprintf(w, "%s %s %d/%d %d %s %s\n",
		s.ID, s.Status, s.Progress.Filled, s.Progress.Total,
		s.Assumptions, s.Updated.Format(time.RFC3339), OneLine(c.Title))
	return err
}

// writeCloudBlock writes one cloud as a block of labelled sections, so a reader
// (or an agent reading the whole output) gets the statements, the assumptions,
// the injections, the process and the issues in one call.
func writeCloudBlock(w io.Writer, c Cloud) error {
	filled, total := c.Progress()
	verified := "no"
	if c.Verified {
		verified = "yes"
	}

	lines := []string{
		fmt.Sprintf("id          %s", c.ID),
		fmt.Sprintf("title       %s", OneLine(c.Title)),
		fmt.Sprintf("parties     %s / %s", c.Party(1), c.Party(2)),
		fmt.Sprintf("status      %s", c.Status()),
		fmt.Sprintf("progress    %d/%d statements", filled, total),
		fmt.Sprintf("verified    %s", verified),
		fmt.Sprintf("created     %s", c.Created.UTC().Format(time.RFC3339)),
		fmt.Sprintf("updated     %s", c.Updated.UTC().Format(time.RFC3339)),
		"",
		"statements",
	}
	for _, info := range SlotInfos() {
		text := OneLine(c.Text(info.Slot))
		if text == "" {
			text = "(empty)"
		}
		lines = append(lines, fmt.Sprintf("  %-2s  %s", info.Slot, text))
	}

	lines = append(lines, "", "assumptions")
	if len(c.Assumptions) == 0 {
		lines = append(lines, "  none")
	}
	for _, as := range c.Assumptions {
		lines = append(lines, fmt.Sprintf("  %s  %-5s  %-12s  %s",
			as.ID, as.Arrow, as.Verdict, OneLine(as.Text)))
		if as.Evidence != "" {
			lines = append(lines, fmt.Sprintf("    evidence  %s", OneLine(as.Evidence)))
		}
		var brokenBy []string
		for _, inj := range c.Injections {
			if contains(inj.Breaks, as.ID) {
				brokenBy = append(brokenBy, inj.ID)
			}
		}
		if len(brokenBy) > 0 {
			lines = append(lines, fmt.Sprintf("    broken by %s", strings.Join(brokenBy, ",")))
		}
	}

	lines = append(lines, "", "injections")
	if len(c.Injections) == 0 {
		lines = append(lines, "  none")
	}
	for _, inj := range c.Injections {
		breaks := strings.Join(inj.Breaks, ",")
		if breaks == "" {
			breaks = "-"
		}
		lines = append(lines, fmt.Sprintf("  %s  breaks %s  %s", inj.ID, breaks, OneLine(inj.Text)))
		if inj.Rationale != "" {
			lines = append(lines, fmt.Sprintf("    why it works  %s", OneLine(inj.Rationale)))
		}
	}

	lines = append(lines, "", "process")
	for _, step := range c.Steps() {
		state := "todo"
		if step.Done {
			state = "done"
		}
		lines = append(lines, fmt.Sprintf("  %d  %-4s  %s", step.Number, state, step.Title))
	}

	lines = append(lines, "", "issues")
	issues := c.Validate()
	if len(issues) == 0 {
		lines = append(lines, "  none")
	}
	for _, issue := range issues {
		lines = append(lines, fmt.Sprintf("  %-7s  %s", issueWords(issue.Severity), issue.Message))
	}

	_, err := io.WriteString(w, strings.Join(lines, "\n")+"\n")
	return err
}

// writeIssueLine writes one validation finding as a single line:
//
//	<cloud> <severity> <message>
func writeIssueLine(w io.Writer, issue issueJSON) error {
	_, err := fmt.Fprintf(w, "%s %s %s\n", issue.Cloud, issue.Severity, OneLine(issue.Message))
	return err
}

package main

//go:generate sh -c "go run . --man > man/cloud-break.1"

// The help texts are plain files so they are easy to edit and review, and are
// embedded so the command carries them with no runtime files. The man page is
// built from the same texts, so the manual and the help cannot drift apart.

import (
	_ "embed"
	"fmt"
	"io"
	"strings"
)

//go:embed help/usage.txt
var usageText string

//go:embed help/clouds.txt
var cloudsHelpText string

//go:embed help/agent.txt
var agentHelpText string

// helpTopics maps a topic name to its text, in the order the topics are listed.
var helpTopics = []struct {
	name string
	text string
}{
	{"clouds", cloudsHelpText},
	{"agent", agentHelpText},
}

func printUsage(w io.Writer) { fmt.Fprint(w, usageText) }

// printTopic writes the help for one topic and reports whether it knows it.
func printTopic(w io.Writer, topic string) bool {
	for _, t := range helpTopics {
		if t.name == topic {
			fmt.Fprint(w, t.text)
			return true
		}
	}
	return false
}

// topicNames lists the topics for a diagnostic.
func topicNames() string {
	names := make([]string, 0, len(helpTopics))
	for _, t := range helpTopics {
		names = append(names, t.name)
	}
	return strings.Join(names, ", ")
}

// help is the "help" command. Unlike --help it can report an unknown topic.
func (c *invocation) help(args []string) error {
	switch len(args) {
	case 0:
		printUsage(c.s.stdout)
		return nil
	case 1:
		if printTopic(c.s.stdout, args[0]) {
			return nil
		}
		return usageError{fmt.Errorf("unknown help topic %q (want %s)", args[0], topicNames())}
	}
	return usageError{fmt.Errorf("help takes at most one topic (want %s)", topicNames())}
}

// printDocRequest writes the documentation an argument asks for and reports
// whether it did, with the error that says how it ended: an unknown topic is a
// usage error, and a printed page is not. It runs before flag parsing because an
// explicit help request prints to standard output and exits 0, unlike a flag the
// parser rejects.
func printDocRequest(args []string, s streams) (bool, error) {
	for i, arg := range args {
		if arg == "--" {
			// Everything after "--" is an operand, so it is text to store
			// rather than a request for documentation.
			return false, nil
		}
		switch arg {
		case "-h", "-help", "--help":
			topic := ""
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				topic = args[i+1]
			}
			if topic == "" {
				printUsage(s.stdout)
				return true, nil
			}
			if !printTopic(s.stdout, topic) {
				return true, usageError{fmt.Errorf("unknown help topic %q (want %s)", topic, topicNames())}
			}
			return true, nil
		case "--man":
			printMan(s.stdout)
			return true, nil
		}
	}
	return false, nil
}

// printMan writes a roff manual page built from the embedded help texts.
func printMan(w io.Writer) {
	fmt.Fprint(w, ".TH CLOUD-BREAK 1\n.SH NAME\n")
	fmt.Fprint(w, `cloud-break \- build and process evaporating clouds`+"\n")
	for _, section := range []struct{ title, body string }{
		{"USAGE", usageText},
		{"CLOUDS", cloudsHelpText},
		{"AGENT", agentHelpText},
	} {
		fmt.Fprintf(w, ".SH %s\n.nf\n", section.title)
		fmt.Fprint(w, roffEscape(section.body))
		fmt.Fprint(w, ".fi\n")
	}
}

// roffEscape makes text safe inside a no-fill block. A backslash is an escape
// in roff, and a line beginning with "." or "'" would be read as a request.
func roffEscape(s string) string {
	var b strings.Builder
	for _, line := range strings.SplitAfter(s, "\n") {
		line = strings.ReplaceAll(line, `\`, `\\`)
		if strings.HasPrefix(line, ".") || strings.HasPrefix(line, "'") {
			line = `\&` + line
		}
		b.WriteString(line)
	}
	return b.String()
}

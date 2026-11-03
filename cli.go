package main

// The command line is a second front end onto the same library the interface
// edits. It follows a few rules so a script or an agent can rely on it:
//
//   - stdout carries data and nothing else; every diagnostic goes to stderr.
//   - every read command offers a line-oriented text shape and a --json shape,
//     and the JSON shape always carries the same fields.
//   - every command that changes something prints the identifier it acted on,
//     so the next command never has to guess.
//   - the exit status says what happened: 0 success, 1 the run failed or a
//     check found a problem, 2 the command cannot be used, 130 interrupted.
//   - a cloud, assumption or injection may be named by any unambiguous prefix
//     of its identifier, so hand-typed identifiers stay short.
//
// Flags may appear before or after the command name.

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/charmbracelet/x/term"
)

// streams are the process streams a command reads and writes. They are injected
// so the command line has no global state and can be tested.
type streams struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

// exit statuses. They are the contract a script gates on.
const (
	exitOK          = 0
	exitFailed      = 1   // the run failed, or a check found a problem
	exitUsage       = 2   // the command line cannot be used
	exitInterrupted = 130 // the run was interrupted
)

// settings are the flags every command accepts, wherever they appear on the
// line. A command's own flag set carries them so "cloud-break --dir d list" and
// "cloud-break list --dir d" mean the same thing.
type settings struct {
	dir  string
	demo bool
	json bool
}

// register declares the shared flags on a command's flag set. The current
// values become the defaults, so a flag given before the command name still
// applies unless the command repeats it.
func (s *settings) register(fs *flag.FlagSet) {
	fs.StringVar(&s.dir, "dir", s.dir, "library directory (default $CLOUD_BREAK_HOME, $XDG_DATA_HOME/cloud-break, ~/.local/share/cloud-break)")
	fs.BoolVar(&s.demo, "demo", s.demo, "use a throwaway library holding the example clouds")
	fs.BoolVar(&s.json, "json", s.json, "print machine-readable JSON")
}

// invocation is one run of the command line.
type invocation struct {
	s streams
}

// runCLI is the whole command line: it reports the error and returns the exit
// status main hands to the process.
func runCLI(args []string, s streams) int {
	inv := &invocation{s: s}

	// A help request prints to stdout and is not a failure, so it is answered
	// before anything else parses the line.
	if handled, err := printDocRequest(args, s); handled {
		return report(err, s.stderr)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	return report(inv.run(ctx, args), s.stderr)
}

// report writes an error's message and usage text and returns its exit status.
func report(err error, stderr io.Writer) int {
	d := dispositionOf(err)
	if d.message != "" {
		fmt.Fprintln(stderr, d.message)
	}
	if d.usage {
		printUsage(stderr)
	}
	return d.code
}

// run parses the line and dispatches it. A line with no command opens the
// interface, which is what the program did before it had commands.
func (c *invocation) run(ctx context.Context, args []string) error {
	root := flag.NewFlagSet("cloud-break", flag.ContinueOnError)
	root.SetOutput(io.Discard)
	root.Usage = func() {}

	var st settings
	st.register(root)
	showVersion := root.Bool("version", false, "print the version and exit")
	if err := root.Parse(args); err != nil {
		return flagError{err}
	}
	if *showVersion {
		fmt.Fprintf(c.s.stdout, "cloud-break %s\n", version)
		return nil
	}

	rest := root.Args()
	if len(rest) == 0 {
		if err := c.interactive(st); err != nil {
			return err
		}
		return ctx.Err()
	}

	return c.command(rest[0], rest[1:], st)
}

// command dispatches one subcommand with the flags that preceded it.
func (c *invocation) command(name string, args []string, st settings) error {
	switch name {
	case "list":
		return c.list(st, args)
	case "show":
		return c.show(st, args)
	case "render":
		return c.render(st, args)
	case "check":
		return c.check(st, args)
	case "new":
		return c.newCloud(st, args)
	case "set":
		return c.set(st, args)
	case "title":
		return c.title(st, args)
	case "parties":
		return c.parties(st, args)
	case "assume":
		return c.assume(st, args)
	case "inject":
		return c.inject(st, args)
	case "verify":
		return c.verify(st, args)
	case "delete":
		return c.delete(st, args)
	case "export":
		return c.export(st, args)
	case "version":
		fmt.Fprintf(c.s.stdout, "cloud-break %s\n", version)
		return nil
	case "help":
		return c.help(args)
	}
	return usageError{fmt.Errorf("unknown command %q (see 'cloud-break --help')", name)}
}

// interactive opens the library interface. It refuses to start when nothing is
// a terminal: a full-screen interface reading a pipe would hang the caller
// waiting for a window that never comes, so say so instead.
func (c *invocation) interactive(st settings) error {
	if !isTerminal(c.s.stdout) {
		return usageError{errors.New("the interface needs a terminal; run a command instead (see 'cloud-break --help')")}
	}
	store, cleanup, err := c.store(st)
	if err != nil {
		return err
	}
	defer cleanup()
	return runTUI(store)
}

// isTerminal reports whether w is a terminal. An injected writer never is, which
// keeps the interface from starting under a test or a pipe.
func isTerminal(w io.Writer) bool {
	file, ok := w.(*os.File)
	return ok && term.IsTerminal(file.Fd())
}

// store resolves the library a command works on. --demo always uses a
// throwaway library, so naming one as well contradicts it: quietly seeding the
// examples into a real library would be hard to undo.
func (c *invocation) store(st settings) (*Store, func(), error) {
	dir := st.dir
	cleanup := func() {}

	if st.demo {
		if strings.TrimSpace(dir) != "" {
			return nil, cleanup, usageError{errors.New("--demo works on a throwaway library, so it cannot be combined with --dir")}
		}
		temp, err := os.MkdirTemp("", "cloud-break-demo-")
		if err != nil {
			return nil, cleanup, fmt.Errorf("create the demo library: %w", err)
		}
		cleanup = func() { os.RemoveAll(temp) }
		dir = temp
	}

	store, err := OpenStore(dir)
	if err != nil {
		cleanup()
		return nil, func() {}, fmt.Errorf("open the library: %w", err)
	}
	if st.demo {
		if err := seedDemo(store.Dir()); err != nil {
			cleanup()
			return nil, func() {}, err
		}
	}
	return store, cleanup, nil
}

// newFlagSet returns a flag set for one command carrying the shared flags, with
// parsing errors reported as usage errors.
func (c *invocation) newFlagSet(name string, st *settings) *flag.FlagSet {
	fs := flag.NewFlagSet("cloud-break "+name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	st.register(fs)
	return fs
}

// parseCommand parses a command's flags and returns its operands, turning a
// parser error into a usage error. Flags are allowed anywhere on the line, so
// "inject add <cloud> --breaks id <text>" keeps its flag and its text apart.
func parseCommand(fs *flag.FlagSet, args []string) ([]string, error) {
	if err := fs.Parse(intersperse(fs, args)); err != nil {
		return nil, flagError{err}
	}
	return fs.Args(), nil
}

// intersperse moves flags and their values ahead of the operands. The flag
// package stops parsing at the first operand, which would leave a flag written
// after an operand to be read as part of it; reordering keeps a flag's position
// on the line from changing its meaning.
//
// A "--" ends flag parsing explicitly, so a text operand that begins with a
// dash can still be passed.
func intersperse(fs *flag.FlagSet, args []string) []string {
	var flags, operands []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			operands = append(operands, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			operands = append(operands, arg)
			continue
		}

		name, _, inline := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		declared := fs.Lookup(name)
		flags = append(flags, arg)
		// An unknown flag is left for the parser to reject; a declared one takes
		// the next argument as its value unless it was written "--flag=value" or
		// is a boolean.
		if inline || (declared != nil && !takesValue(declared)) {
			continue
		}
		if i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}
	return append(flags, operands...)
}

// takesValue reports whether a flag is declared with a value, as opposed to a
// boolean flag that stands alone.
func takesValue(f *flag.Flag) bool {
	boolean, ok := f.Value.(interface{ IsBoolFlag() bool })
	return !ok || !boolean.IsBoolFlag()
}

// lookupCloud finds the cloud named by ref: its identifier, or any unambiguous
// prefix of it. A reference that names nothing, or more than one cloud, is
// reported in terms the caller can act on.
func lookupCloud(store *Store, ref string) (Cloud, error) {
	ref = strings.ToLower(strings.TrimSpace(ref))
	if ref == "" {
		return Cloud{}, usageError{errors.New("a cloud is required; pass its id (see 'cloud-break list')")}
	}

	clouds, err := store.List()
	if err != nil {
		return Cloud{}, err
	}
	if c, ok := exactCloud(clouds, ref); ok {
		return c, nil
	}

	var matches []Cloud
	for _, c := range clouds {
		if strings.HasPrefix(string(c.ID), ref) {
			matches = append(matches, c)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return Cloud{}, notFoundError{fmt.Errorf("no cloud matches %q in %s", ref, store.Dir())}
	}
	ids := make([]string, 0, len(matches))
	for _, c := range matches {
		ids = append(ids, string(c.ID))
	}
	return Cloud{}, usageError{fmt.Errorf("cloud reference %q is ambiguous, it matches %s", ref, strings.Join(ids, ", "))}
}

// exactCloud returns the cloud whose identifier is ref.
func exactCloud(clouds []Cloud, ref string) (Cloud, bool) {
	for _, c := range clouds {
		if string(c.ID) == ref {
			return c, true
		}
	}
	return Cloud{}, false
}

// findAssumption resolves an assumption by identifier or unambiguous prefix.
func findAssumption(c Cloud, ref string) (Assumption, error) {
	if strings.TrimSpace(ref) == "" {
		return Assumption{}, usageError{fmt.Errorf("cloud %s: an assumption is required; 'cloud-break show %s' prints them", c.ID, c.ID)}
	}
	matches := prefixMatches(ref, len(c.Assumptions), func(i int) string { return c.Assumptions[i].ID })
	switch len(matches) {
	case 1:
		return c.Assumptions[matches[0]], nil
	case 0:
		return Assumption{}, notFoundError{fmt.Errorf("cloud %s has no assumption matching %q", c.ID, ref)}
	}
	return Assumption{}, ambiguousError("assumption", ref, matches, func(i int) string { return c.Assumptions[i].ID })
}

// findInjection resolves an injection by identifier or unambiguous prefix.
func findInjection(c Cloud, ref string) (Injection, error) {
	if strings.TrimSpace(ref) == "" {
		return Injection{}, usageError{fmt.Errorf("cloud %s: an injection is required; 'cloud-break show %s' prints them", c.ID, c.ID)}
	}
	matches := prefixMatches(ref, len(c.Injections), func(i int) string { return c.Injections[i].ID })
	switch len(matches) {
	case 1:
		return c.Injections[matches[0]], nil
	case 0:
		return Injection{}, notFoundError{fmt.Errorf("cloud %s has no injection matching %q", c.ID, ref)}
	}
	return Injection{}, ambiguousError("injection", ref, matches, func(i int) string { return c.Injections[i].ID })
}

// prefixMatches returns the indexes whose identifier is, or begins with, ref.
// An empty ref matches nothing, so a missing identifier cannot silently pick
// the first record.
func prefixMatches(ref string, count int, id func(int) string) []int {
	ref = strings.ToLower(strings.TrimSpace(ref))
	if ref == "" {
		return nil
	}
	var matches []int
	for i := range count {
		got := strings.ToLower(id(i))
		if got == ref {
			return []int{i}
		}
		if strings.HasPrefix(got, ref) {
			matches = append(matches, i)
		}
	}
	return matches
}

// ambiguousError names every record a reference could mean.
func ambiguousError(kind, ref string, matches []int, id func(int) string) error {
	ids := make([]string, 0, len(matches))
	for _, i := range matches {
		ids = append(ids, id(i))
	}
	return usageError{fmt.Errorf("%s reference %q is ambiguous, it matches %s", kind, ref, strings.Join(ids, ", "))}
}

// saveCloud writes a cloud and returns the document as stored, so output and
// later commands see the normalized form.
func saveCloud(store *Store, c Cloud) (Cloud, error) {
	if err := store.Save(c); err != nil {
		return Cloud{}, err
	}
	return store.Load(c.ID)
}

// wrote reports the identifier a writing command acted on. In text mode that is
// the identifier alone, one line, so a command can be composed in a shell.
func (c *invocation) wrote(st settings, kind string, cloud ID, id string) error {
	if !st.json {
		_, err := fmt.Fprintln(c.s.stdout, id)
		return err
	}
	return writeJSON(c.s.stdout, resultJSON{Kind: kind, Cloud: cloud, ID: id})
}

// removed reports a deletion, which has no surviving identifier to name.
func (c *invocation) removed(st settings, cloud ID) error {
	if !st.json {
		_, err := fmt.Fprintln(c.s.stdout, cloud)
		return err
	}
	return writeJSON(c.s.stdout, resultJSON{Kind: "cloud", Cloud: cloud, ID: string(cloud), Deleted: true})
}

// flagError is a command line the flag parser rejected.
type flagError struct{ err error }

func (e flagError) Error() string { return e.err.Error() }
func (e flagError) Unwrap() error { return e.err }

// usageError is a parsed command line the command cannot act on: a missing
// operand, an unknown word, or an ambiguous reference.
type usageError struct{ err error }

func (e usageError) Error() string { return e.err.Error() }
func (e usageError) Unwrap() error { return e.err }

// notFoundError is a reference the library does not contain.
type notFoundError struct{ err error }

func (e notFoundError) Error() string { return e.err.Error() }
func (e notFoundError) Unwrap() error { return e.err }

// checksFailed reports a check that ran and found errors, so a script can gate
// on the exit status.
type checksFailed struct{ errors int }

func (e checksFailed) Error() string {
	return fmt.Sprintf("validation found %d errors", e.errors)
}

// disposition is how the command line reports an error: the exit status, the
// line for standard error (empty for none), and whether to print the usage text.
type disposition struct {
	code    int
	message string
	usage   bool
}

// dispositionOf maps an error to how it is reported. The error kind is assigned
// where the error is found, so this table does not need to know which command
// produced it.
func dispositionOf(err error) disposition {
	if err == nil {
		return disposition{code: exitOK}
	}

	var flagErr flagError
	if errors.As(err, &flagErr) {
		return disposition{code: exitUsage, message: "cloud-break: " + err.Error(), usage: true}
	}
	var usage usageError
	if errors.As(err, &usage) {
		return disposition{code: exitUsage, message: "cloud-break: " + err.Error()}
	}
	if errors.Is(err, context.Canceled) {
		return disposition{code: exitInterrupted, message: "cloud-break: interrupted"}
	}
	return disposition{code: exitFailed, message: "cloud-break: " + err.Error()}
}

// Command cloud-break builds and processes evaporating clouds, the conflict
// resolution diagrams from Goldratt's Theory of Constraints thinking processes.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// version is stamped at build time with -ldflags "-X main.version=...".
var version = "0.1.0"

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "cloud-break:", err)
		os.Exit(1)
	}
}

// options are the parsed command line.
type options struct {
	dir         string
	showVersion bool
}

func parseArgs(args []string, stderr io.Writer) (options, error) {
	fs := flag.NewFlagSet("cloud-break", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var opts options
	fs.StringVar(&opts.dir, "dir", "", "library directory (default: $XDG_DATA_HOME/cloud-break)")
	fs.BoolVar(&opts.showVersion, "version", false, "print the version and exit")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "cloud-break %s - build and process evaporating clouds\n\n", version)
		fmt.Fprintln(stderr, "usage: cloud-break [flags]")
		fmt.Fprintln(stderr)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	return opts, nil
}

func run(args []string, stdout, stderr io.Writer) error {
	opts, err := parseArgs(args, stderr)
	if err != nil {
		return err
	}
	if opts.showVersion {
		fmt.Fprintln(stdout, "cloud-break", version)
		return nil
	}

	store, err := OpenStore(opts.dir)
	if err != nil {
		return err
	}
	return runTUI(store)
}

// libraryDir resolves where clouds live: the flag, then $CLOUD_BREAK_HOME,
// then $XDG_DATA_HOME/cloud-break, then ~/.local/share/cloud-break.
func libraryDir(flagValue string) (string, error) {
	if v := strings.TrimSpace(flagValue); v != "" {
		return filepath.Abs(v)
	}
	if v := strings.TrimSpace(os.Getenv("CLOUD_BREAK_HOME")); v != "" {
		return filepath.Abs(v)
	}
	if v := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); v != "" {
		return filepath.Join(v, "cloud-break"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate home directory: %w", err)
	}
	return filepath.Join(home, ".local", "share", "cloud-break"), nil
}

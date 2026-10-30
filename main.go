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

func run(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("cloud-break", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		dir         = fs.String("dir", "", "library directory (default: $XDG_DATA_HOME/cloud-break)")
		showVersion = fs.Bool("version", false, "print the version and exit")
	)
	fs.Usage = func() {
		fmt.Fprintf(stderr, "cloud-break %s - build and process evaporating clouds\n\n", version)
		fmt.Fprintln(stderr, "usage: cloud-break [flags]")
		fmt.Fprintln(stderr)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *showVersion {
		fmt.Fprintln(stdout, "cloud-break", version)
		return nil
	}

	lib, err := libraryDir(*dir)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "cloud-break %s\nlibrary: %s\n", version, lib)
	return nil
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

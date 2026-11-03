// Command cloud-break builds and processes evaporating clouds, the conflict
// resolution diagrams from Goldratt's Theory of Constraints thinking processes.
//
// With no command it opens the interface; with a command it acts on the library
// from a script. See "cloud-break --help".
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// version is stamped at build time with -ldflags "-X main.version=...".
var version = "0.2.0"

func main() {
	s := streams{stdin: os.Stdin, stdout: os.Stdout, stderr: os.Stderr}
	os.Exit(runCLI(os.Args[1:], s))
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

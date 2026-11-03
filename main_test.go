package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLibraryDirPrecedence(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLOUD_BREAK_HOME", "")
	t.Setenv("XDG_DATA_HOME", "")

	t.Run("flag wins", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "flagdir")
		got, err := libraryDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		if got != dir {
			t.Errorf("got %q, want %q", got, dir)
		}
	})

	t.Run("CLOUD_BREAK_HOME beats XDG", func(t *testing.T) {
		custom := filepath.Join(t.TempDir(), "custom")
		t.Setenv("CLOUD_BREAK_HOME", custom)
		t.Setenv("XDG_DATA_HOME", filepath.Join(t.TempDir(), "xdg"))
		got, err := libraryDir("")
		if err != nil {
			t.Fatal(err)
		}
		if got != custom {
			t.Errorf("got %q, want %q", got, custom)
		}
	})

	t.Run("XDG is honoured", func(t *testing.T) {
		t.Setenv("CLOUD_BREAK_HOME", "")
		xdg := filepath.Join(t.TempDir(), "xdg")
		t.Setenv("XDG_DATA_HOME", xdg)
		got, err := libraryDir("")
		if err != nil {
			t.Fatal(err)
		}
		if want := filepath.Join(xdg, "cloud-break"); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("fallback is under the home directory", func(t *testing.T) {
		t.Setenv("CLOUD_BREAK_HOME", "")
		t.Setenv("XDG_DATA_HOME", "")
		got, err := libraryDir("")
		if err != nil {
			t.Fatal(err)
		}
		if want := filepath.Join(home, ".local", "share", "cloud-break"); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

// A help request is not a failure, and it is documentation, so it goes to
// standard output rather than standard error.
func TestHelpGoesToStdoutAndSucceeds(t *testing.T) {
	out, errOut, code := cli(t, t.TempDir(), "--help")
	if code != exitOK {
		t.Errorf("exit = %d, want %d (stderr %s)", code, exitOK, errOut)
	}
	if !strings.Contains(out, "cloud-break - build and process evaporating clouds") {
		t.Errorf("usage not on stdout: %q", out)
	}
	if errOut != "" {
		t.Errorf("stderr = %q, want nothing", errOut)
	}
}

func TestVersionNeedsNoTerminal(t *testing.T) {
	out, errOut, code := cli(t, t.TempDir(), "--version")
	if code != exitOK {
		t.Fatalf("exit = %d, want %d (stderr %s)", code, exitOK, errOut)
	}
	if !strings.Contains(out, version) {
		t.Errorf("output %q does not mention version %q", out, version)
	}
}

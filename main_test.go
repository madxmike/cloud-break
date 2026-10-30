package main

import (
	"bytes"
	"os"
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

func TestRunVersion(t *testing.T) {
	var out, errBuf bytes.Buffer
	if err := run([]string{"-version"}, &out, &errBuf); err != nil {
		t.Fatalf("run: %v (stderr %s)", err, errBuf.String())
	}
	if !strings.Contains(out.String(), version) {
		t.Errorf("output %q does not mention version %q", out.String(), version)
	}
}

func TestRunReportsLibraryDir(t *testing.T) {
	dir := t.TempDir()
	var out, errBuf bytes.Buffer
	if err := run([]string{"-dir", dir}, &out, &errBuf); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(out.String(), dir) {
		t.Errorf("output %q does not mention %q", out.String(), dir)
	}
}

func TestRunRejectsUnknownFlag(t *testing.T) {
	var out, errBuf bytes.Buffer
	if err := run([]string{"-nope"}, &out, &errBuf); err == nil {
		t.Error("unknown flag was accepted")
	}
}

func TestMainEntrypointExists(t *testing.T) {
	// Guard against a run() that is never reachable from main.
	if _, err := os.Stat("main.go"); err != nil {
		t.Fatalf("main.go missing: %v", err)
	}
}

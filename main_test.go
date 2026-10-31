package main

import (
	"bytes"
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

func TestParseArgs(t *testing.T) {
	var stderr bytes.Buffer

	opts, err := parseArgs([]string{"-dir", "/tmp/lib", "-version"}, &stderr)
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	if opts.dir != "/tmp/lib" {
		t.Errorf("dir = %q", opts.dir)
	}
	if !opts.showVersion {
		t.Error("showVersion not set")
	}

	if _, err := parseArgs([]string{"-nope"}, &stderr); err == nil {
		t.Error("unknown flag was accepted")
	}
	if !strings.Contains(stderr.String(), "usage: cloud-break") {
		t.Errorf("usage not printed, stderr = %q", stderr.String())
	}
}

func TestParseArgsDemo(t *testing.T) {
	var stderr bytes.Buffer
	opts, err := parseArgs([]string{"-demo"}, &stderr)
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	if !opts.demo {
		t.Error("demo flag not set")
	}
}

func TestRunHelpIsNotAFailure(t *testing.T) {
	var out, errBuf bytes.Buffer
	if err := run([]string{"-h"}, &out, &errBuf); err != nil {
		t.Errorf("run(-h) = %v, want nil", err)
	}
	if !strings.Contains(errBuf.String(), "usage: cloud-break") {
		t.Errorf("usage was not printed: %q", errBuf.String())
	}
}

func TestRunVersionNeedsNoTerminal(t *testing.T) {
	var out, errBuf bytes.Buffer
	if err := run([]string{"-version"}, &out, &errBuf); err != nil {
		t.Fatalf("run: %v (stderr %s)", err, errBuf.String())
	}
	if !strings.Contains(out.String(), version) {
		t.Errorf("output %q does not mention version %q", out.String(), version)
	}
}

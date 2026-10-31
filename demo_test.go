package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSeedDemoWritesEveryExample(t *testing.T) {
	dir := t.TempDir()
	if err := seedDemo(dir); err != nil {
		t.Fatalf("seedDemo: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("seeded nothing")
	}

	store, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	clouds, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(clouds) != len(entries) {
		t.Errorf("listed %d clouds from %d files", len(clouds), len(entries))
	}

	for _, c := range clouds {
		if len(c.Statements) < 4 {
			t.Errorf("%s: only %d statements", c.Title, len(c.Statements))
		}
		if len(c.Assumptions) == 0 {
			t.Errorf("%s: no claims to look at", c.Title)
		}
		if c.ID == "" || c.Title == "" {
			t.Errorf("incomplete example: %+v", c)
		}
	}
}

func TestSeedDemoLeavesAnExistingCloudAlone(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "example-on-call.json")
	mine := []byte(`{"id":"example-on-call","title":"Mine, edited"}`)
	if err := os.WriteFile(path, mine, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := seedDemo(dir); err != nil {
		t.Fatalf("seedDemo: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Mine, edited") {
		t.Error("seeding overwrote a cloud that was already there")
	}
}

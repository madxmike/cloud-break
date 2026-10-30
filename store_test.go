package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	s, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	return s
}

// writeRaw stores a cloud exactly as given, so tests control timestamps.
func writeRaw(t *testing.T, s *Store, c Cloud) {
	t.Helper()
	path, err := s.Path(c.ID)
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestOpenStoreCreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "library")
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	if s.Dir() != dir {
		t.Errorf("Dir = %q, want %q", s.Dir(), dir)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Errorf("directory not created: %v", err)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	s := testStore(t)
	c := resolved()
	id := c.AddAssumption(ArrowBD, "review catches everything")
	c.AssumptionByID(id).Verdict = Invalid
	c.AssumptionByID(id).Evidence = "three regressions last quarter"
	c.AddInjection([]string{id}, "regression test per change")
	c.SetParties("Sales", "Engineering")

	if err := s.Create(*c); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := s.Load(c.ID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Title != c.Title {
		t.Errorf("Title = %q, want %q", got.Title, c.Title)
	}
	if got.Statements[SlotA] != c.Statements[SlotA] {
		t.Errorf("A = %q", got.Statements[SlotA])
	}
	if got.Parties != [2]string{"Sales", "Engineering"} {
		t.Errorf("Parties = %v", got.Parties)
	}
	if len(got.Assumptions) != 1 || got.Assumptions[0].Evidence == "" {
		t.Errorf("assumptions lost: %+v", got.Assumptions)
	}
	if len(got.Injections) != 1 || got.Injections[0].Breaks[0] != id {
		t.Errorf("injection lost: %+v", got.Injections)
	}
}

func TestSaveNormalizesText(t *testing.T) {
	s := testStore(t)
	c := New("  ship\nfriday ")
	c.SetText(SlotA, "  the   release\t on time ")
	c.AddAssumption(ArrowBD, "line one\nline two")
	c.AddInjection([]string{"x"}, "  idea\nhere ")

	if err := s.Save(*c); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := s.Load(c.ID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Title != "ship friday" {
		t.Errorf("Title = %q, want one line", got.Title)
	}
	if got.Statements[SlotA] != "the release on time" {
		t.Errorf("A = %q, want collapsed whitespace", got.Statements[SlotA])
	}
	if got.Assumptions[0].Text != "line one line two" {
		t.Errorf("assumption = %q", got.Assumptions[0].Text)
	}
	if got.Injections[0].Text != "idea here" {
		t.Errorf("injection = %q", got.Injections[0].Text)
	}
}

func TestSaveDropsEmptyStatements(t *testing.T) {
	s := testStore(t)
	c := New("t")
	c.SetText(SlotA, "only this")
	c.SetText(SlotB, "   ")

	if err := s.Save(*c); err != nil {
		t.Fatalf("Save: %v", err)
	}
	data, err := os.ReadFile(mustPath(t, s, c.ID))
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	statements, _ := raw["statements"].(map[string]any)
	if len(statements) != 1 {
		t.Errorf("statements = %v, want only A", statements)
	}
	if _, ok := statements["B"]; ok {
		t.Error("empty statement B was written to disk")
	}
}

func TestSaveStampsUpdated(t *testing.T) {
	s := testStore(t)
	c := New("t")
	c.Updated = time.Now().Add(-48 * time.Hour)
	before := c.Updated
	if err := s.Save(*c); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := s.Load(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Updated.After(before) {
		t.Errorf("Updated = %v, want later than %v", got.Updated, before)
	}
	if got.Created.IsZero() {
		t.Error("Created was not filled in")
	}
}

func TestCreateRefusesOverwrite(t *testing.T) {
	s := testStore(t)
	c := New("t")
	if err := s.Create(*c); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	if err := s.Create(*c); !errors.Is(err, ErrExists) {
		t.Errorf("second Create = %v, want ErrExists", err)
	}
}

func TestLoadMissingCloud(t *testing.T) {
	s := testStore(t)
	if _, err := s.Load("nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Load = %v, want ErrNotFound", err)
	}
}

func TestPathRejectsTraversal(t *testing.T) {
	s := testStore(t)
	for _, id := range []ID{"", ".", "..", "../escape", "a/b", `a\b`, "a\x00b"} {
		if _, err := s.Path(id); err == nil {
			t.Errorf("Path(%q) was accepted", id)
		}
	}
}

func TestListSortsByUpdatedDescending(t *testing.T) {
	s := testStore(t)
	now := time.Now().UTC()
	for i, name := range []string{"oldest", "middle", "newest"} {
		c := *New(name)
		c.ID = ID(name)
		c.Updated = now.Add(time.Duration(i-2) * time.Hour)
		c.Created = c.Updated
		writeRaw(t, s, c)
	}

	clouds, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(clouds) != 3 {
		t.Fatalf("List returned %d clouds, want 3", len(clouds))
	}
	want := []string{"newest", "middle", "oldest"}
	for i, title := range want {
		if clouds[i].Title != title {
			t.Errorf("position %d = %q, want %q", i, clouds[i].Title, title)
		}
	}
}

func TestListSkipsNoise(t *testing.T) {
	s := testStore(t)
	good := *New("good")
	good.ID = "good"
	good.Updated = time.Now().UTC()
	writeRaw(t, s, good)

	dir := s.Dir()
	if err := os.Mkdir(filepath.Join(dir, "subdir"+jsonExt), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".tmp-123"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "broken"+jsonExt), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	clouds, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(clouds) != 1 || clouds[0].Title != "good" {
		t.Errorf("List = %+v, want just the readable cloud", clouds)
	}
}

func TestListEmptyLibrary(t *testing.T) {
	s := testStore(t)
	clouds, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(clouds) != 0 {
		t.Errorf("List = %v, want empty", clouds)
	}
}

func TestSaveLeavesNoTemporaryFiles(t *testing.T) {
	s := testStore(t)
	if err := s.Save(*New("t")); err != nil {
		t.Fatalf("Save: %v", err)
	}
	entries, err := os.ReadDir(s.Dir())
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".tmp-") {
			t.Errorf("temporary file left behind: %s", entry.Name())
		}
	}
}

func TestDeleteRemovesCloud(t *testing.T) {
	s := testStore(t)
	c := New("t")
	if err := s.Save(*c); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := s.Delete(c.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Load(c.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("Load after Delete = %v, want ErrNotFound", err)
	}
	if err := s.Delete(c.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("second Delete = %v, want ErrNotFound", err)
	}
}

func TestDeleteRejectsUnsafeID(t *testing.T) {
	s := testStore(t)
	if err := s.Delete("../etc/passwd"); err == nil {
		t.Error("Delete accepted a traversing id")
	}
}

func mustPath(t *testing.T, s *Store, id ID) string {
	t.Helper()
	path, err := s.Path(id)
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	return path
}

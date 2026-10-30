package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const jsonExt = ".json"

var (
	// ErrNotFound is returned when a cloud does not exist in the library.
	ErrNotFound = errors.New("cloud not found")
	// ErrExists is returned when creating a cloud whose ID is already taken.
	ErrExists = errors.New("cloud already exists")
)

// Store is a directory of cloud documents. Each cloud is one JSON file, named
// after its ID, written atomically so an interrupted save cannot truncate it.
type Store struct {
	dir string
}

// OpenStore prepares a library directory, creating it when missing.
func OpenStore(dir string) (*Store, error) {
	if strings.TrimSpace(dir) == "" {
		var err error
		if dir, err = libraryDir(""); err != nil {
			return nil, err
		}
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve library directory: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("create library %s: %w", abs, err)
	}
	return &Store{dir: abs}, nil
}

// Dir is the directory the library lives in.
func (s *Store) Dir() string { return s.dir }

// Path returns the file backing a cloud. IDs are restricted to plain file
// names so a hand-edited document cannot point outside the library.
func (s *Store) Path(id ID) (string, error) {
	name := string(id)
	if name == "" || name == "." || name == ".." {
		return "", fmt.Errorf("invalid cloud id %q", id)
	}
	if strings.ContainsAny(name, `/\`) || strings.ContainsRune(name, 0) {
		return "", fmt.Errorf("invalid cloud id %q", id)
	}
	return filepath.Join(s.dir, name+jsonExt), nil
}

// List returns every readable cloud, most recently updated first. Files that
// cannot be parsed are skipped: one damaged document must not hide the rest of
// the library.
func (s *Store) List() ([]Cloud, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("read library %s: %w", s.dir, err)
	}
	clouds := make([]Cloud, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || strings.HasPrefix(name, ".") || !strings.HasSuffix(name, jsonExt) {
			continue
		}
		c, err := s.Load(ID(strings.TrimSuffix(name, jsonExt)))
		if err != nil {
			continue
		}
		clouds = append(clouds, c)
	}
	sort.Slice(clouds, func(i, j int) bool {
		if clouds[i].Updated.Equal(clouds[j].Updated) {
			return clouds[i].Title < clouds[j].Title
		}
		return clouds[i].Updated.After(clouds[j].Updated)
	})
	return clouds, nil
}

// Load reads one cloud.
func (s *Store) Load(id ID) (Cloud, error) {
	var c Cloud
	path, err := s.Path(id)
	if err != nil {
		return c, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return c, fmt.Errorf("cloud %s: %w", id, ErrNotFound)
		}
		return c, fmt.Errorf("read cloud %s: %w", id, err)
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return c, fmt.Errorf("parse cloud %s: %w", id, err)
	}
	if c.ID == "" {
		c.ID = id
	}
	return c, nil
}

// Save writes a cloud, creating or replacing it.
func (s *Store) Save(c Cloud) error {
	path, err := s.Path(c.ID)
	if err != nil {
		return err
	}
	c.Normalize()
	c.Updated = time.Now().UTC()
	if c.Created.IsZero() {
		c.Created = c.Updated
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("encode cloud %s: %w", c.ID, err)
	}
	return s.writeAtomic(path, append(data, '\n'))
}

// Create saves a new cloud, refusing to overwrite an existing one.
func (s *Store) Create(c Cloud) error {
	path, err := s.Path(c.ID)
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("cloud %s: %w", c.ID, ErrExists)
	}
	return s.Save(c)
}

// Delete removes a cloud from the library.
func (s *Store) Delete(id ID) error {
	path, err := s.Path(id)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("cloud %s: %w", id, ErrNotFound)
		}
		return fmt.Errorf("delete cloud %s: %w", id, err)
	}
	return nil
}

// writeAtomic writes to a temporary file in the same directory and renames it
// into place, so a crash mid-write leaves the previous version intact.
func (s *Store) writeAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(s.dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	name := tmp.Name()
	defer os.Remove(name)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("flush %s: %w", filepath.Base(path), err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close %s: %w", filepath.Base(path), err)
	}
	if err := os.Chmod(name, 0o644); err != nil {
		return fmt.Errorf("mode %s: %w", filepath.Base(path), err)
	}
	if err := os.Rename(name, path); err != nil {
		return fmt.Errorf("install %s: %w", filepath.Base(path), err)
	}
	return nil
}

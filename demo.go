package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// demoLibrary holds the example clouds so a binary can start with content
// without needing the repository beside it.
//
//go:embed examples/library/*.json
var demoLibrary embed.FS

// seedDemo writes the bundled examples into dir. A cloud that already exists is
// left alone, so seeding never overwrites work in a library that is kept.
func seedDemo(dir string) error {
	entries, err := fs.ReadDir(demoLibrary, "examples/library")
	if err != nil {
		return fmt.Errorf("read the bundled examples: %w", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		target := filepath.Join(dir, name)
		if _, err := os.Stat(target); err == nil {
			continue
		}
		data, err := demoLibrary.ReadFile("examples/library/" + name)
		if err != nil {
			return fmt.Errorf("read example %s: %w", name, err)
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return fmt.Errorf("write example %s: %w", name, err)
		}
	}
	return nil
}

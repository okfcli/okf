// Package initbundle creates a new empty OKF bundle with the standard
// directory structure.
package initbundle

import (
	"fmt"
	"os"
	"path/filepath"
)

// standardSubdirs are the conventional OKF bundle subdirectories.
var standardSubdirs = []string{"tables", "datasets", "playbooks"}

// Create makes a new bundle directory with standard subdirectories,
// a root index.md, and a .gitignore. It fails if the directory already exists.
func Create(dir string) error {
	if info, err := os.Stat(dir); err == nil {
		if info.IsDir() {
			return fmt.Errorf("directory already exists: %s", dir)
		}
		return fmt.Errorf("path exists and is not a directory: %s", dir)
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	for _, sub := range standardSubdirs {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0755); err != nil {
			return fmt.Errorf("create subdir %s: %w", sub, err)
		}
	}

	// Root index.md
	indexContent := "# Index\n\nBundle root.\n\n## Subdirectories\n\n- [tables/](tables/index.md)\n- [datasets/](datasets/index.md)\n- [playbooks/](playbooks/index.md)\n"
	if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte(indexContent), 0644); err != nil {
		return fmt.Errorf("write index.md: %w", err)
	}

	// .gitignore
	gitignore := "# OKF bundle\n*.bak\n.DS_Store\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(gitignore), 0644); err != nil {
		return fmt.Errorf("write .gitignore: %w", err)
	}

	return nil
}

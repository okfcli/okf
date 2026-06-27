// Package bundle walks an OKF bundle directory and loads all concept documents.
package bundle

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/okfcli/okf/internal/concept"
)

// Bundle is a loaded OKF knowledge bundle.
type Bundle struct {
	Root      string                 // absolute path to the bundle root
	Concepts  []*concept.Concept     // all concept documents, sorted by ID
	conceptByID map[string]*concept.Concept
	Reserved  []*concept.Concept     // index.md / log.md files (parsed if present)
}

// Load walks a bundle directory and parses every .md file.
// Reserved filenames (index.md, log.md) are separated into Reserved.
func Load(root string) (*Bundle, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", root, err)
	}
	info, err := os.Stat(absRoot)
	if err != nil {
		return nil, fmt.Errorf("stat bundle root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("bundle root %s is not a directory", absRoot)
	}

	b := &Bundle{
		Root:        absRoot,
		conceptByID: make(map[string]*concept.Concept),
	}

	err = filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Skip hidden directories like .git
			if d.Name() != filepath.Base(absRoot) && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}

		relPath, err := filepath.Rel(absRoot, path)
		if err != nil {
			return fmt.Errorf("rel path %s: %w", path, err)
		}
		relPath = filepath.ToSlash(relPath)

		// Reserved filenames are loaded separately; they may lack frontmatter.
		if concept.ReservedNames[strings.ToLower(d.Name())] {
			c, perr := concept.Parse(path, relPath)
			if perr == nil {
				b.Reserved = append(b.Reserved, c)
			} else if errors.Is(perr, concept.ErrNoFrontmatter) {
				// Generated index.md files have no frontmatter. Still load
				// them as a Concept with empty Frontmatter and the raw file
				// content as Body so callers can discover them via Reserved.
				raw, rerr := os.ReadFile(path)
				if rerr != nil {
					return fmt.Errorf("read reserved %s: %w", relPath, rerr)
				}
				b.Reserved = append(b.Reserved, &concept.Concept{
					ID:    concept.ConceptID(relPath),
					Path:  path,
					Body:  string(raw),
				})
			}
			return nil
		}

		c, err := concept.Parse(path, relPath)
		if err != nil {
			return fmt.Errorf("parse %s: %w", relPath, err)
		}
		b.Concepts = append(b.Concepts, c)
		b.conceptByID[c.ID] = c
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(b.Concepts, func(i, j int) bool {
		return b.Concepts[i].ID < b.Concepts[j].ID
	})
	return b, nil
}

// Get returns a concept by ID, or nil if not found.
func (b *Bundle) Get(id string) *concept.Concept {
	return b.conceptByID[id]
}

// HasConcept reports whether a concept with the given ID exists.
func (b *Bundle) HasConcept(id string) bool {
	_, ok := b.conceptByID[id]
	return ok
}

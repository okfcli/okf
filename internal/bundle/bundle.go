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
	Root         string             // absolute path to the bundle root
	Concepts     []*concept.Concept // all concept documents, sorted by ID
	conceptByID  map[string]*concept.Concept
	Reserved     []*concept.Concept // index.md / log.md files (parsed if present)
	reservedByID map[string]*concept.Concept
}

// ParseError reports a .md file whose contents could not be parsed as a
// concept: no frontmatter block, an unclosed one, or invalid YAML. Path is
// bundle-relative so callers can name the file (issue #27). It is distinct
// from a filesystem failure, which is returned unwrapped as an I/O error.
type ParseError struct {
	Path string
	Err  error
}

func (e *ParseError) Error() string { return "parse " + e.Path + ": " + e.Err.Error() }

// Unwrap exposes the underlying parse failure for errors.Is / errors.As.
func (e *ParseError) Unwrap() error { return e.Err }

// wrapParse classifies a concept parse failure: a filesystem error stays an
// I/O error, anything else is a ParseError naming the file.
func wrapParse(relPath string, err error) error {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return fmt.Errorf("read %s: %w", relPath, err)
	}
	return &ParseError{Path: relPath, Err: err}
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
		Root:         absRoot,
		conceptByID:  make(map[string]*concept.Concept),
		reservedByID: make(map[string]*concept.Concept),
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
		// Skip hidden files (editor drafts, backups) the same way hidden
		// directories are skipped: they are not part of the bundle.
		if !strings.HasSuffix(d.Name(), ".md") || strings.HasPrefix(d.Name(), ".") {
			return nil
		}

		relPath, err := filepath.Rel(absRoot, path)
		if err != nil {
			return fmt.Errorf("rel path %s: %w", path, err)
		}
		relPath = filepath.ToSlash(relPath)

		// Reserved filenames are loaded separately; they may lack frontmatter.
		// ParseReserved reads the file once and tolerates a missing frontmatter
		// block (e.g. generated index.md), but still surfaces malformed
		// frontmatter and other errors rather than silently dropping the file.
		if concept.ReservedNames[strings.ToLower(d.Name())] {
			c, perr := concept.ParseReserved(path, relPath)
			if perr != nil {
				return wrapParse(relPath, perr)
			}
			b.Reserved = append(b.Reserved, c)
			b.reservedByID[c.ID] = c
			return nil
		}

		c, err := concept.Parse(path, relPath)
		if err != nil {
			return wrapParse(relPath, err)
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

// HasReserved reports whether a reserved file (index.md or log.md) with the
// given ID exists, e.g. "sub/index" for /sub/index.md.
func (b *Bundle) HasReserved(id string) bool {
	_, ok := b.reservedByID[id]
	return ok
}

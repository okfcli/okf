// Package concept parses a single OKF concept document: YAML frontmatter
// delimited by --- plus a markdown body.
package concept

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Frontmatter holds the recognized OKF frontmatter keys. Producers may include
// arbitrary extra keys; they are preserved in Extensions.
type Frontmatter struct {
	Type        string            `yaml:"type"`
	Title       string            `yaml:"title"`
	Description string            `yaml:"description"`
	Resource    string            `yaml:"resource"`
	Tags        []string          `yaml:"tags"`
	Timestamp   time.Time         `yaml:"timestamp"`
	Links       []string          `yaml:"links"`
	Extensions  map[string]any    `yaml:",inline"`
}

// Concept is a parsed concept document.
type Concept struct {
	// ID is the file path within the bundle with .md removed, e.g. "tables/users".
	ID         string
	Path       string       // absolute path to the .md file on disk
	Frontmatter Frontmatter
	Body       string
	RawFront   string       // raw YAML block for round-tripping
}

// Reserved filenames per OKF spec §3.1 — they are NOT concepts.
var ReservedNames = map[string]bool{
	"index.md": true,
	"log.md":   true,
}

// ErrNoFrontmatter is returned when a .md file has no YAML frontmatter block.
var ErrNoFrontmatter = errors.New("no YAML frontmatter block found")

// Parse reads and parses a concept document from path. relPath is the path
// relative to the bundle root, used to derive the concept ID.
func Parse(path, relPath string) (*Concept, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return ParseBytes(raw, relPath, path)
}

// ParseBytes parses concept content from a byte slice.
func ParseBytes(raw []byte, relPath, absPath string) (*Concept, error) {
	front, body, err := splitFrontmatter(raw)
	if err != nil {
		return nil, err
	}

	var fm Frontmatter
	if err := yaml.Unmarshal([]byte(front), &fm); err != nil {
		return nil, fmt.Errorf("parse frontmatter in %s: %w", relPath, err)
	}

	id := conceptID(relPath)
	return &Concept{
		ID:          id,
		Path:        absPath,
		Frontmatter: fm,
		Body:        body,
		RawFront:    front,
	}, nil
}

// ConceptID converts a relative file path to a concept ID by stripping the .md
// suffix and normalizing separators to forward slashes.
// e.g. "tables/users.md" -> "tables/users"
func ConceptID(relPath string) string {
	return conceptID(relPath)
}

// conceptID strips .md and normalizes path separators to forward slashes.
func conceptID(relPath string) string {
	id := strings.TrimSuffix(relPath, ".md")
	id = filepath.ToSlash(id)
	return id
}

// splitFrontmatter separates the YAML frontmatter block from the markdown body.
// The frontmatter is delimited by --- on its own line at the start of the file
// and a closing --- on its own line.
func splitFrontmatter(raw []byte) (front, body string, err error) {
	content := string(raw)
	// Strip a leading BOM if present.
	content = strings.TrimPrefix(content, "\uFEFF")
	if !strings.HasPrefix(content, "---\n") && !strings.HasPrefix(content, "---\r\n") {
		return "", "", ErrNoFrontmatter
	}
	// Find the closing delimiter.
	rest := content[3:] // skip opening "---"
	if strings.HasPrefix(rest, "\r\n") {
		rest = rest[2:]
	} else {
		rest = rest[1:]
	}
	// Search line by line for a line that is exactly "---" (allowing \r).
	lines := strings.Split(rest, "\n")
	closeIdx := -1
	for i, line := range lines {
		trimmed := strings.TrimRight(line, "\r")
		if trimmed == "---" {
			closeIdx = i
			break
		}
	}
	if closeIdx == -1 {
		return "", "", errors.New("frontmatter block not closed: missing closing ---")
	}
	front = strings.Join(lines[:closeIdx], "\n")
	body = strings.Join(lines[closeIdx+1:], "\n")
	body = strings.TrimPrefix(body, "\n")
	body = strings.TrimPrefix(body, "\r")
	return front, body, nil
}

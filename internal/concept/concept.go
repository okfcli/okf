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
	Type        string     `yaml:"type"`
	Title       string     `yaml:"title"`
	Description string     `yaml:"description"`
	Resource    string     `yaml:"resource"`
	Tags        TagList    `yaml:"tags"`
	Timestamp   time.Time  `yaml:"timestamp"` // legacy v0.1; superseded by generated.at (OKF §13.1)
	Links       StringList `yaml:"links"`

	// Provenance, trust, and lifecycle families (OKF v0.2 §5).
	Sources     []Source     `yaml:"sources"`
	UsageWindow *UsageWindow `yaml:"usage_window"`
	Generated   *Generated   `yaml:"generated"`
	Verified    VerifiedList `yaml:"verified"`
	Status      string       `yaml:"status"`
	StaleAfter  string       `yaml:"stale_after"`

	// Attested Computation contract (OKF v0.2 §10).
	Runtime     string      `yaml:"runtime"`
	Parameters  []Parameter `yaml:"parameters"`
	Computation string      `yaml:"computation"`
	Executor    *Executor   `yaml:"executor"`
	Attester    *Attester   `yaml:"attester"`

	Extensions map[string]any `yaml:",inline"`
}

// StringList is a []string that unmarshals from YAML flexibly: it accepts both
// a single scalar string (links: /b) and a sequence of strings (links: [/b, /c]).
// A missing/empty value stays a nil/empty slice, and a malformed shape (e.g. a
// mapping) is tolerated as an empty slice rather than failing the whole parse -
// so one badly-shaped concept cannot take down an entire bundle load.
type StringList []string

// UnmarshalYAML implements yaml.Unmarshaler, accepting scalar-or-sequence.
func (s *StringList) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		// A single string, or an explicit null. Treat null as empty.
		if value.Tag == "!!null" {
			*s = nil
			return nil
		}
		var single string
		if err := value.Decode(&single); err != nil {
			// Non-string scalar (e.g. a number) - tolerate as empty.
			*s = nil
			return nil
		}
		*s = StringList{single}
		return nil
	case yaml.SequenceNode:
		var list []string
		if err := value.Decode(&list); err != nil {
			// A sequence of non-strings - tolerate as empty rather than
			// failing the entire bundle load.
			*s = nil
			return nil
		}
		*s = StringList(list)
		return nil
	default:
		// Mapping or any other unexpected shape - tolerate as empty.
		*s = nil
		return nil
	}
}

// TagList is a []string that unmarshals from YAML flexibly. The spec form is
// a sequence (§4.1), but reference bundles in the wild emit a comma-separated
// scalar ("a, b, c"); consumers MUST NOT reject such documents (§11), so the
// scalar form is split on commas. Malformed shapes are tolerated as empty.
type TagList []string

// UnmarshalYAML implements yaml.Unmarshaler, accepting sequence-or-scalar.
func (t *TagList) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.SequenceNode:
		var list []string
		if err := value.Decode(&list); err != nil {
			*t = nil
			return nil
		}
		*t = TagList(list)
		return nil
	case yaml.ScalarNode:
		if value.Tag == "!!null" {
			*t = nil
			return nil
		}
		var single string
		if err := value.Decode(&single); err != nil {
			*t = nil
			return nil
		}
		var tags []string
		for _, part := range strings.Split(single, ",") {
			if p := strings.TrimSpace(part); p != "" {
				tags = append(tags, p)
			}
		}
		*t = TagList(tags)
		return nil
	default:
		*t = nil
		return nil
	}
}

// Concept is a parsed concept document.
type Concept struct {
	// ID is the file path within the bundle with .md removed, e.g. "tables/users".
	ID          string
	Path        string // absolute path to the .md file on disk
	Frontmatter Frontmatter
	Body        string
	RawFront    string // raw YAML block for round-tripping
}

// Reserved filenames per OKF spec §3.1 - they are NOT concepts.
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

// ParseReserved reads and parses a reserved document (index.md, log.md) from
// path. Unlike Parse, a missing frontmatter block is not an error: such files
// (e.g. those generated by `okf index`) are returned as a Concept with empty
// Frontmatter and the raw file content as Body, so callers can still discover
// them. The file is read exactly once. Malformed-but-present frontmatter still
// returns an error rather than being silently dropped.
func ParseReserved(path, relPath string) (*Concept, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	c, err := ParseBytes(raw, relPath, path)
	if err != nil {
		if errors.Is(err, ErrNoFrontmatter) {
			return &Concept{
				ID:   conceptID(relPath),
				Path: path,
				Body: string(raw),
			}, nil
		}
		return nil, err
	}
	return c, nil
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

// Package search filters concepts in a bundle by tag, type, or body text.
package search

import (
	"strings"

	"github.com/okfcli/okf/internal/bundle"
	"github.com/okfcli/okf/internal/concept"
)

// Filters holds optional filter criteria. Empty fields match everything.
type Filters struct {
	Tag  string // match if concept has this tag
	Type string // match if concept frontmatter type equals this
	Text string // match if text appears in title, description, or body (case-insensitive)
}

// Search returns concepts matching all provided filters.
// Empty filters return all concepts.
func Search(b *bundle.Bundle, f Filters) []*concept.Concept {
	var results []*concept.Concept
	for _, c := range b.Concepts {
		if !matchTag(c, f.Tag) {
			continue
		}
		if !matchType(c, f.Type) {
			continue
		}
		if !matchText(c, f.Text) {
			continue
		}
		results = append(results, c)
	}
	return results
}

func matchTag(c *concept.Concept, tag string) bool {
	if tag == "" {
		return true
	}
	for _, t := range c.Frontmatter.Tags {
		if strings.EqualFold(t, tag) {
			return true
		}
	}
	return false
}

func matchType(c *concept.Concept, typ string) bool {
	if typ == "" {
		return true
	}
	return strings.EqualFold(c.Frontmatter.Type, typ)
}

func matchText(c *concept.Concept, text string) bool {
	if text == "" {
		return true
	}
	lower := strings.ToLower(text)
	// Search title, description, and body.
	if strings.Contains(strings.ToLower(c.Frontmatter.Title), lower) {
		return true
	}
	if strings.Contains(strings.ToLower(c.Frontmatter.Description), lower) {
		return true
	}
	if strings.Contains(strings.ToLower(c.Body), lower) {
		return true
	}
	return false
}

// Package validate checks an OKF bundle against the OKF spec.
package validate

import (
	"fmt"
	"strings"
	"time"

	"github.com/okfcli/okf/internal/bundle"
	"github.com/okfcli/okf/internal/concept"
)

// Severity is the severity of a validation finding.
type Severity int

const (
	SeverityError   Severity = iota // spec violation
	SeverityWarning                 // recommended field missing or style issue
)

func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "ERROR"
	case SeverityWarning:
		return "WARN"
	default:
		return "??"
	}
}

// Finding is a single validation result.
type Finding struct {
	ConceptID string
	Severity  Severity
	Message   string
}

// Report is the full validation output.
type Report struct {
	Findings []Finding
	Errors   int
	Warnings int
}

// HasErrors reports whether the report contains any errors.
func (r *Report) HasErrors() bool { return r.Errors > 0 }

// Validate runs all checks against a bundle and returns a report.
func Validate(b *bundle.Bundle) *Report {
	r := &Report{}
	for _, c := range b.Concepts {
		validateFrontmatter(r, c)
		validateBody(r, c)
	}
	validateLinks(r, b)
	return r
}

// validateFrontmatter checks required and recommended frontmatter fields.
func validateFrontmatter(r *Report, c *concept.Concept) {
	fm := c.Frontmatter

	// type is REQUIRED (OKF spec §4.1)
	if strings.TrimSpace(fm.Type) == "" {
		r.add(c.ID, SeverityError, "frontmatter: 'type' is required (OKF §4.1)")
	}

	// Recommended fields (OKF spec §4.1)
	if fm.Title == "" {
		r.add(c.ID, SeverityWarning, "frontmatter: 'title' is recommended")
	}
	if fm.Description == "" {
		r.add(c.ID, SeverityWarning, "frontmatter: 'description' is recommended")
	}
	if len(fm.Tags) == 0 {
		r.add(c.ID, SeverityWarning, "frontmatter: 'tags' is recommended (empty or missing)")
	}

	// timestamp, if present, should be a valid ISO 8601 datetime.
	// yaml.v3 already parses it into time.Time; a zero value with a non-empty
	// raw would indicate a parse issue, but we accept zero as "not set".
	if !fm.Timestamp.IsZero() && fm.Timestamp.After(time.Now().Add(24 * 365 * time.Hour)) {
		r.add(c.ID, SeverityWarning, "frontmatter: 'timestamp' is more than a year in the future")
	}
}

// validateBody checks the markdown body for structural issues.
func validateBody(r *Report, c *concept.Concept) {
	if strings.TrimSpace(c.Body) == "" {
		r.add(c.ID, SeverityWarning, "body is empty — structural markdown is recommended (OKF §4.2)")
	}
}

// validateLinks checks that all cross-links within the bundle resolve to an
// existing concept. Both absolute (/path/to/concept.md) and relative links
// are checked.
//
// Relative links (without a leading /) resolve from the concept's own
// directory: a link [X](organizations/cloaked) in pages/about.md targets
// pages/organizations/cloaked, not organizations/cloaked. When such a
// relative link is broken but the same target would resolve as an absolute
// path, the error message suggests the absolute form (e.g.
// /organizations/cloaked) so authors can fix the link.
func validateLinks(r *Report, b *bundle.Bundle) {
	for _, c := range b.Concepts {
		links := extractLinks(c.Body)
		for _, link := range links {
			target := resolveLink(c.ID, link)
			if target == "" {
				continue // external URL or non-concept link, skip
			}
			if !b.HasConcept(target) {
				// The link didn't resolve as written. Check whether the raw
				// target would resolve as an absolute (bundle-root-relative)
				// path; if so, the author likely intended an absolute link.
				if absTarget := absoluteTarget(link.Target); absTarget != "" && b.HasConcept(absTarget) {
					r.add(c.ID, SeverityError, fmt.Sprintf(
						"broken link: [%s] -> %s (relative links resolve from the current concept's directory; use /%s for an absolute path)",
						link.Text, link.Target, absTarget))
				} else {
					r.add(c.ID, SeverityError, fmt.Sprintf(
						"broken link: [%s] -> %s (concept %s not found)",
						link.Text, link.Target, target))
				}
			}
		}
	}
}

// absoluteTarget converts a raw link target into the concept ID it would
// have if treated as an absolute (bundle-root-relative) path. It strips a
// leading /, removes any fragment (#) or query (?), and drops a trailing
// .md suffix. It returns "" for external URLs or empty targets.
func absoluteTarget(raw string) string {
	target := strings.TrimSpace(raw)
	if target == "" || isExternalURL(target) {
		return ""
	}
	// Strip any fragment (#section) or query (?query).
	if idx := strings.IndexAny(target, "#?"); idx != -1 {
		target = target[:idx]
	}
	if target == "" {
		return ""
	}
	target = strings.TrimPrefix(target, "/")
	target = strings.TrimSuffix(target, ".md")
	if target == "" {
		return ""
	}
	return concept.ConceptID(target)
}

// Link represents a markdown link found in a concept body.
type Link struct {
	Text   string
	Target string
}

// ExtractLinks parses all markdown links [text](target) from a body.
//
// Link targets may be absolute or relative. Absolute targets begin with /
// and resolve against the bundle root (e.g. /tables/users.md -> the
// concept tables/users regardless of where the linking concept lives).
// Relative targets lack a leading / and resolve against the linking
// concept's own directory: a target organizations/cloaked in a concept at
// pages/about resolves to pages/organizations/cloaked, not
// organizations/cloaked. External URLs (http://, https://, mailto:, and
// pure-fragment links like #section) are returned as links but are ignored
// by concept resolution.
func ExtractLinks(body string) []Link {
	return extractLinks(body)
}

// extractLinks parses all markdown links [text](target) from a body.
func extractLinks(body string) []Link {
	var links []Link
	i := 0
	for i < len(body) {
		// Find '['
		bracket := strings.IndexByte(body[i:], '[')
		if bracket == -1 {
			break
		}
		start := i + bracket
		closeBracket := strings.IndexByte(body[start:], ']')
		if closeBracket == -1 {
			break
		}
		textEnd := start + closeBracket
		// Check for '(' immediately after ']'
		if textEnd+1 >= len(body) || body[textEnd+1] != '(' {
			i = textEnd + 1
			continue
		}
		closeParen := strings.IndexByte(body[textEnd+2:], ')')
		if closeParen == -1 {
			break
		}
		target := body[textEnd+2 : textEnd+2+closeParen]
		text := body[start+1 : textEnd]
		links = append(links, Link{Text: text, Target: target})
		i = textEnd + 2 + closeParen + 1
	}
	return links
}

// isExternalURL reports whether a link target is an external URL (http://, https://, etc.)
func isExternalURL(target string) bool {
	target = strings.TrimSpace(target)
	return strings.HasPrefix(target, "http://") ||
		strings.HasPrefix(target, "https://") ||
		strings.HasPrefix(target, "mailto:") ||
		strings.HasPrefix(target, "#")
}

// ResolveLink converts a link target to a concept ID, or returns "" if the
// link is external or otherwise not a concept reference.
func ResolveLink(fromConceptID string, link Link) string {
	return resolveLink(fromConceptID, link)
}

// resolveLink converts a link target to a concept ID, or returns "" if the
// link is external or otherwise not a concept reference.
func resolveLink(fromConceptID string, link Link) string {
	target := strings.TrimSpace(link.Target)
	if target == "" || isExternalURL(target) {
		return ""
	}
	// Strip any fragment (#section) or query (?query).
	if idx := strings.IndexAny(target, "#?"); idx != -1 {
		target = target[:idx]
	}
	if target == "" {
		return ""
	}
	// Absolute bundle-relative link: starts with /
	if strings.HasPrefix(target, "/") {
		target = strings.TrimPrefix(target, "/")
		return concept.ConceptID(target)
	}
	// Relative link: resolve from the concept's directory.
	dir := dirOf(fromConceptID)
	resolved := normalizePath(dir + "/" + target)
	return concept.ConceptID(resolved)
}

// dirOf returns the directory portion of a concept ID ("tables/users" -> "tables").
func dirOf(id string) string {
	idx := strings.LastIndexByte(id, '/')
	if idx == -1 {
		return ""
	}
	return id[:idx]
}

// normalizePath resolves . and .. in a slash path.
func normalizePath(p string) string {
	var parts []string
	for _, seg := range strings.Split(p, "/") {
		switch seg {
		case "", ".":
			continue
		case "..":
			if len(parts) > 0 {
				parts = parts[:len(parts)-1]
			}
		default:
			parts = append(parts, seg)
		}
	}
	return strings.Join(parts, "/")
}

func (r *Report) add(id string, sev Severity, msg string) {
	r.Findings = append(r.Findings, Finding{ConceptID: id, Severity: sev, Message: msg})
	switch sev {
	case SeverityError:
		r.Errors++
	case SeverityWarning:
		r.Warnings++
	}
}

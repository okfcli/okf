// Package backlinks finds all concepts that link to a given concept.
package backlinks

import (
	"github.com/okfcli/okf/internal/bundle"
	"github.com/okfcli/okf/internal/validate"
)

// Backlinks returns the IDs of all concepts that link to the given concept ID.
// Results are not sorted. If the concept doesn't exist, returns nil.
// Duplicate links from the same concept are deduplicated.
func Backlinks(b *bundle.Bundle, conceptID string) []string {
	if !b.HasConcept(conceptID) {
		return nil
	}

	seen := make(map[string]bool)
	var results []string

	for _, c := range b.Concepts {
		if c.ID == conceptID {
			continue // don't self-report
		}
		bodyLinks := validate.ExtractLinks(c.Body)
		fmLinks := validate.ExtractFrontmatterLinks(c.Frontmatter.Links)
		for _, link := range append(bodyLinks, fmLinks...) {
			target := validate.ResolveLink(c.ID, link)
			if target == conceptID {
				if !seen[c.ID] {
					seen[c.ID] = true
					results = append(results, c.ID)
				}
				break // one link from this concept is enough
			}
		}
	}

	return results
}

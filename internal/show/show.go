// Package show retrieves a single concept from a bundle for display.
package show

import (
	"fmt"

	"github.com/okfcli/okf/internal/bundle"
	"github.com/okfcli/okf/internal/concept"
)

// Show returns the concept with the given ID from the bundle.
// Returns an error if the concept is not found.
func Show(b *bundle.Bundle, conceptID string) (*concept.Concept, error) {
	c := b.Get(conceptID)
	if c == nil {
		return nil, fmt.Errorf("concept not found: %s", conceptID)
	}
	return c, nil
}

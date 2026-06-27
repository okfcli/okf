// Package graph builds the cross-link graph from an OKF bundle.
// Per OKF spec §5, concepts link to each other via markdown links, expressing
// relationships richer than the parent/child directory hierarchy.
package graph

import (
	"fmt"
	"sort"
	"strings"

	"github.com/okfcli/okf/internal/bundle"
	"github.com/okfcli/okf/internal/concept"
	"github.com/okfcli/okf/internal/validate"
)

// Graph is the directed cross-link graph of a bundle.
type Graph struct {
	Nodes     []Node
	Edges     []Edge
	// Backlinks maps concept ID -> list of concept IDs that link to it.
	Backlinks map[string][]string
}

// Node is a concept in the graph.
type Node struct {
	ID   string
	Type string
}

// Edge is a directed link from one concept to another.
type Edge struct {
	From string
	To   string
}

// Build constructs the cross-link graph from a bundle.
func Build(b *bundle.Bundle) *Graph {
	g := &Graph{Backlinks: make(map[string][]string)}

	nodeSet := make(map[string]bool)
	for _, c := range b.Concepts {
		g.Nodes = append(g.Nodes, Node{ID: c.ID, Type: c.Frontmatter.Type})
		nodeSet[c.ID] = true
	}

	edgeSet := make(map[string]bool) // "from\x00to" dedup
	for _, c := range b.Concepts {
		// Collect body links and frontmatter links, then process them
		// uniformly. The edgeSet dedup handles overlaps.
		bodyLinks := validate.ExtractLinks(c.Body)
		fmLinks := validate.ExtractFrontmatterLinks(c.Frontmatter.Links)
		for _, link := range append(bodyLinks, fmLinks...) {
			target := resolveLink(c.ID, link)
			if target == "" || !b.HasConcept(target) {
				continue
			}
			key := c.ID + "\x00" + target
			if !edgeSet[key] {
				edgeSet[key] = true
				g.Edges = append(g.Edges, Edge{From: c.ID, To: target})
			}
			g.Backlinks[target] = appendUnique(g.Backlinks[target], c.ID)
		}
	}

	sort.Slice(g.Nodes, func(i, j int) bool { return g.Nodes[i].ID < g.Nodes[j].ID })
	sort.Slice(g.Edges, func(i, j int) bool {
		if g.Edges[i].From != g.Edges[j].From {
			return g.Edges[i].From < g.Edges[j].From
		}
		return g.Edges[i].To < g.Edges[j].To
	})
	return g
}

// Stats returns summary statistics about the graph.
type Stats struct {
	NodeCount    int
	EdgeCount    int
	IsolatedNodes int  // nodes with no inbound or outbound edges
	MaxBacklinks int  // highest number of backlinks on any single concept
}

// Stats computes summary statistics.
func (g *Graph) Stats() Stats {
	s := Stats{NodeCount: len(g.Nodes), EdgeCount: len(g.Edges)}

	degree := make(map[string]int)
	for _, e := range g.Edges {
		degree[e.From]++
		degree[e.To]++
	}
	for _, n := range g.Nodes {
		if degree[n.ID] == 0 {
			s.IsolatedNodes++
		}
	}
	for _, links := range g.Backlinks {
		if len(links) > s.MaxBacklinks {
			s.MaxBacklinks = len(links)
		}
	}
	return s
}

func appendUnique(slice []string, val string) []string {
	for _, s := range slice {
		if s == val {
			return slice
		}
	}
	return append(slice, val)
}

func resolveLink(fromConceptID string, link validate.Link) string {
	return validate.ResolveLink(fromConceptID, link)
}

// Summary returns a human-readable summary string.
func (g *Graph) Summary() string {
	s := g.Stats()
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("nodes: %d  edges: %d  isolated: %d  max-backlinks: %d\n",
		s.NodeCount, s.EdgeCount, s.IsolatedNodes, s.MaxBacklinks))
	if s.NodeCount > 0 && s.EdgeCount > 0 {
		density := float64(s.EdgeCount) / float64(s.NodeCount*(s.NodeCount-1)) * 100
		sb.WriteString(fmt.Sprintf("graph density: %.2f%%\n", density))
	}
	return sb.String()
}

// silence unused import warning for concept (used transitively via bundle).
var _ = concept.ConceptID

package graph

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/okfcli/okf/internal/bundle"
)

func loadBundle(t *testing.T, files map[string]string) *bundle.Bundle {
	t.Helper()
	dir := t.TempDir()
	for path, content := range files {
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	b, err := bundle.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return b
}

func hasEdge(g *Graph, from, to string) bool {
	for _, e := range g.Edges {
		if e.From == from && e.To == to {
			return true
		}
	}
	return false
}

// §5.1: when a sources[].resource points at another OKF concept, the
// derivation edge exists in the bundle graph.
func TestBuild_SourceResourceDerivationEdge(t *testing.T) {
	b := loadBundle(t, map[string]string{
		"metrics/revenue.md": "---\ntype: Metric\ntitle: R\nsources:\n  - id: dash\n    resource: /dashboards/exec.md\n---\n\nbody",
		"dashboards/exec.md": "---\ntype: Dashboard\ntitle: D\n---\n\nbody",
	})
	g := Build(b)
	if !hasEdge(g, "metrics/revenue", "dashboards/exec") {
		t.Fatalf("expected derivation edge metrics/revenue -> dashboards/exec, edges = %+v", g.Edges)
	}
}

// §6.2: computation and executor.resource are path-valued and produce edges
// when they resolve to concepts; external URLs and non-concept files do not.
func TestBuild_ComputationContractEdges(t *testing.T) {
	b := loadBundle(t, map[string]string{
		"computations/revenue.md":        "---\ntype: Attested Computation\ntitle: R\nruntime: bigquery\nexecutor:\n  resource: /references/skills/run-on-bq.md\nattester:\n  resource: /references/attesters/rev.py\nsources:\n  - id: pol\n    resource: https://wiki.example/policy\n---\n\n# Computation\n\n    SELECT 1\n",
		"references/skills/run-on-bq.md": "---\ntype: Skill\ntitle: S\n---\n\nbody",
	})
	g := Build(b)
	if !hasEdge(g, "computations/revenue", "references/skills/run-on-bq") {
		t.Fatalf("expected edge to executor skill concept, edges = %+v", g.Edges)
	}
	// The .py attester and the external URL are not concepts: no edges, no panic.
	if len(g.Edges) != 1 {
		t.Fatalf("expected exactly 1 edge, got %+v", g.Edges)
	}
}

// A scope-descriptor resource ("all queries in project X") must not produce
// an edge or a bogus node.
func TestBuild_ScopeDescriptorResourceIgnored(t *testing.T) {
	b := loadBundle(t, map[string]string{
		"a.md": "---\ntype: T\ntitle: A\nsources:\n  - resource: all queries in BigQuery project X\n---\n\nbody",
	})
	g := Build(b)
	if len(g.Edges) != 0 {
		t.Fatalf("expected no edges, got %+v", g.Edges)
	}
}

// Bare root-relative contract paths (as used by upstream bundles and the
// spec's §10.2 example) still produce derivation edges via root fallback.
func TestBuild_RootRelativeContractPathEdge(t *testing.T) {
	b := loadBundle(t, map[string]string{
		"computations/c.md": "---\ntype: Attested Computation\ntitle: C\nruntime: bigquery\nexecutor:\n  resource: skills/run.md\n---\n\n# Computation\n\n    SELECT 1\n",
		"skills/run.md":     "---\ntype: Skill\ntitle: S\n---\n\nbody",
	})
	g := Build(b)
	if !hasEdge(g, "computations/c", "skills/run") {
		t.Fatalf("expected root-relative fallback edge, edges = %+v", g.Edges)
	}
}

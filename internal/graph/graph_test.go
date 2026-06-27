package graph

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/okfcli/okf/internal/bundle"
)

func TestBuild_BodyLinks(t *testing.T) {
	b := testBundle(t, map[string]string{
		"a.md": "---\ntype: T\ntitle: A\n---\n\nLinks to [B](b.md).",
		"b.md": "---\ntype: T\ntitle: B\n---\n\nbody",
	})

	g := Build(b)
	if len(g.Edges) != 1 {
		t.Fatalf("got %d edges, want 1", len(g.Edges))
	}
	if g.Edges[0].From != "a" || g.Edges[0].To != "b" {
		t.Errorf("edge = %v, want a->b", g.Edges[0])
	}
}

func TestBuild_FrontmatterLinksOnly(t *testing.T) {
	b := testBundle(t, map[string]string{
		"a.md": "---\ntype: T\ntitle: A\nlinks:\n  - /b\n---\n\nNo body links here.",
		"b.md": "---\ntype: T\ntitle: B\n---\n\nbody",
	})

	g := Build(b)
	if len(g.Edges) != 1 {
		t.Fatalf("got %d edges, want 1 (from frontmatter link)", len(g.Edges))
	}
	if g.Edges[0].From != "a" || g.Edges[0].To != "b" {
		t.Errorf("edge = %v, want a->b", g.Edges[0])
	}
	// Backlinks should also be populated.
	if len(g.Backlinks["b"]) != 1 || g.Backlinks["b"][0] != "a" {
		t.Errorf("backlinks[b] = %v, want [a]", g.Backlinks["b"])
	}
}

func TestBuild_FrontmatterAndBodyDeduped(t *testing.T) {
	b := testBundle(t, map[string]string{
		"a.md": "---\ntype: T\ntitle: A\nlinks:\n  - /b\n---\n\nAlso links in [body](b.md).",
		"b.md": "---\ntype: T\ntitle: B\n---\n\nbody",
	})

	g := Build(b)
	if len(g.Edges) != 1 {
		t.Fatalf("got %d edges, want 1 (deduped frontmatter+body)", len(g.Edges))
	}
	if g.Edges[0].From != "a" || g.Edges[0].To != "b" {
		t.Errorf("edge = %v, want a->b", g.Edges[0])
	}
}

// --- helpers ---

func testBundle(t *testing.T, files map[string]string) *bundle.Bundle {
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

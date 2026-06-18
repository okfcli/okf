package backlinks

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/okfcli/okf/internal/bundle"
)

func TestBacklinks_FindsIncomingLinks(t *testing.T) {
	b := testBundle(t, map[string]string{
		"a.md": "---\ntype: T\ntitle: A\n---\n\nLinks to [B](b.md).",
		"b.md": "---\ntype: T\ntitle: B\n---\n\nLinks to [C](c.md).",
		"c.md": "---\ntype: T\ntitle: C\n---\n\nNo links out.",
	})

	links := Backlinks(b, "c")
	if len(links) != 1 {
		t.Fatalf("got %d backlinks, want 1", len(links))
	}
	if links[0] != "b" {
		t.Errorf("got %s, want b", links[0])
	}
}

func TestBacklinks_MultipleIncoming(t *testing.T) {
	b := testBundle(t, map[string]string{
		"a.md": "---\ntype: T\ntitle: A\n---\n\nLinks to [C](c.md).",
		"b.md": "---\ntype: T\ntitle: B\n---\n\nLinks to [C](c.md).",
		"c.md": "---\ntype: T\ntitle: C\n---\n\nbody",
	})

	links := Backlinks(b, "c")
	if len(links) != 2 {
		t.Fatalf("got %d backlinks, want 2", len(links))
	}
	sorted := append([]string(nil), links...)
	sort.Strings(sorted)
	if sorted[0] != "a" || sorted[1] != "b" {
		t.Errorf("got %v, want [a b]", sorted)
	}
}

func TestBacklinks_NoneIncoming(t *testing.T) {
	b := testBundle(t, map[string]string{
		"a.md": "---\ntype: T\ntitle: A\n---\n\nbody",
		"b.md": "---\ntype: T\ntitle: B\n---\n\nLinks to [A](a.md).",
	})

	links := Backlinks(b, "b")
	if len(links) != 0 {
		t.Fatalf("got %d backlinks, want 0", len(links))
	}
}

func TestBacklinks_NestedPaths(t *testing.T) {
	b := testBundle(t, map[string]string{
		"tables/users.md":    "---\ntype: Table\ntitle: Users\n---\n\nLinks to [orders](orders.md).",
		"tables/orders.md":   "---\ntype: Table\ntitle: Orders\n---\n\nbody",
		"playbooks/check.md": "---\ntype: Playbook\ntitle: Check\n---\n\nCheck [orders](/tables/orders.md).",
	})

	links := Backlinks(b, "tables/orders")
	if len(links) != 2 {
		t.Fatalf("got %d backlinks, want 2", len(links))
	}
	sorted := append([]string(nil), links...)
	sort.Strings(sorted)
	if sorted[0] != "playbooks/check" || sorted[1] != "tables/users" {
		t.Errorf("got %v, want [playbooks/check tables/users]", sorted)
	}
}

func TestBacklinks_ConceptNotFound(t *testing.T) {
	b := testBundle(t, map[string]string{
		"a.md": "---\ntype: T\ntitle: A\n---\n\nbody",
	})

	links := Backlinks(b, "nonexistent")
	if len(links) != 0 {
		t.Fatalf("got %d backlinks, want 0 for nonexistent concept", len(links))
	}
}

func TestBacklinks_DeduplicatesLinks(t *testing.T) {
	b := testBundle(t, map[string]string{
		"a.md": "---\ntype: T\ntitle: A\n---\n\nLinks to [B1](b.md) and [B2](b.md).",
		"b.md": "---\ntype: T\ntitle: B\n---\n\nbody",
	})

	links := Backlinks(b, "b")
	if len(links) != 1 {
		t.Fatalf("got %d backlinks, want 1 (deduplicated)", len(links))
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

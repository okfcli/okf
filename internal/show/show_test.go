package show

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/okfcli/okf/internal/bundle"
	"github.com/okfcli/okf/internal/concept"
)

func TestShow_ReturnsConcept(t *testing.T) {
	b := testBundle(t, map[string]string{
		"tables/users.md": "---\ntype: Table\ntitle: Users\ndescription: User accounts\ntags: [auth, users]\n---\n\n# Users\n\nOne row per user.",
	})

	got, err := Show(b, "tables/users")
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if got.ID != "tables/users" {
		t.Errorf("ID = %q, want tables/users", got.ID)
	}
	if got.Frontmatter.Title != "Users" {
		t.Errorf("Title = %q, want Users", got.Frontmatter.Title)
	}
	if got.Frontmatter.Type != "Table" {
		t.Errorf("Type = %q, want Table", got.Frontmatter.Type)
	}
	if got.Body == "" {
		t.Error("Body should be non-empty")
	}
}

func TestShow_NotFound(t *testing.T) {
	b := testBundle(t, map[string]string{
		"tables/users.md": "---\ntype: Table\ntitle: Users\n---\n\nbody",
	})

	_, err := Show(b, "tables/nonexistent")
	if err == nil {
		t.Fatal("expected error for missing concept, got nil")
	}
}

func TestShow_IncludesRawFrontmatter(t *testing.T) {
	b := testBundle(t, map[string]string{
		"datasets/ga4.md": "---\ntype: BigQuery Dataset\ntitle: GA4\ntags: [analytics]\n---\n\n# GA4",
	})

	got, err := Show(b, "datasets/ga4")
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if got.RawFront == "" {
		t.Error("RawFront should be non-empty for round-tripping")
	}
}

// --- test helpers ---

func testBundle(t *testing.T, files map[string]string) *bundle.Bundle {
	t.Helper()
	dir := t.TempDir()
	for path, content := range files {
		full := dir + "/" + path
		// create parent dirs
		import_writeFile(t, full, content)
	}
	b, err := bundle.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return b
}

func import_writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// avoid extra imports in helpers
var _ = concept.ConceptID

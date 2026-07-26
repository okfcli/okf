package index

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFiles(t *testing.T, files map[string]string) string {
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
	return dir
}

func TestGenerate_WritesIndexes(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"tables/users.md": "---\ntype: Table\ntitle: Users\ndescription: d\n---\n\nbody",
	})
	if err := Generate(dir); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for _, p := range []string{"index.md", "tables/index.md"} {
		if _, err := os.Stat(filepath.Join(dir, p)); err != nil {
			t.Errorf("missing %s: %v", p, err)
		}
	}
}

// §12: a bundle-root index.md may declare okf_version; regeneration must
// preserve an existing declaration rather than dropping it.
func TestGenerate_PreservesRootOkfVersion(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"index.md":        "---\nokf_version: \"0.2\"\n---\n# Index\n\nstale content\n",
		"tables/users.md": "---\ntype: Table\ntitle: Users\ndescription: d\n---\n\nbody",
	})
	if err := Generate(dir); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "index.md"))
	if err != nil {
		t.Fatalf("read root index: %v", err)
	}
	s := string(got)
	if !strings.HasPrefix(s, "---\n") || !strings.Contains(s, `okf_version: "0.2"`) {
		t.Errorf("root index.md lost its okf_version declaration:\n%s", s)
	}
	if strings.Contains(s, "stale content") {
		t.Errorf("root index.md body was not regenerated:\n%s", s)
	}
}

// A root index with no prior declaration must not gain one implicitly (the
// bundle may target v0.1), and non-root indexes never carry frontmatter (§8).
func TestGenerate_NoImplicitOkfVersion(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"tables/users.md": "---\ntype: Table\ntitle: Users\ndescription: d\n---\n\nbody",
	})
	if err := Generate(dir); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	root, _ := os.ReadFile(filepath.Join(dir, "index.md"))
	if strings.Contains(string(root), "okf_version") {
		t.Errorf("root index.md gained an implicit okf_version:\n%s", root)
	}
	sub, _ := os.ReadFile(filepath.Join(dir, "tables", "index.md"))
	if strings.HasPrefix(string(sub), "---\n") {
		t.Errorf("non-root index.md must not carry frontmatter:\n%s", sub)
	}
}

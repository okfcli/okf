package initbundle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreate_MakesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "mybundle")
	if err := Create(dir); err != nil {
		t.Fatalf("Create: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("not a directory")
	}
}

func TestCreate_CreatesStandardSubdirs(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "mybundle")
	if err := Create(dir); err != nil {
		t.Fatalf("Create: %v", err)
	}
	for _, sub := range []string{"tables", "datasets", "playbooks"} {
		p := filepath.Join(dir, sub)
		info, err := os.Stat(p)
		if err != nil {
			t.Errorf("missing subdir %s: %v", sub, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s is not a directory", sub)
		}
	}
}

func TestCreate_CreatesRootIndex(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "mybundle")
	if err := Create(dir); err != nil {
		t.Fatalf("Create: %v", err)
	}
	index := filepath.Join(dir, "index.md")
	if _, err := os.Stat(index); err != nil {
		t.Fatalf("missing index.md: %v", err)
	}
}

func TestCreate_FailsIfExists(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "mybundle")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	err := Create(dir)
	if err == nil {
		t.Fatal("expected error when dir exists, got nil")
	}
}

func TestCreate_CreatesGitignore(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "mybundle")
	if err := Create(dir); err != nil {
		t.Fatalf("Create: %v", err)
	}
	gi := filepath.Join(dir, ".gitignore")
	if _, err := os.Stat(gi); err != nil {
		t.Fatalf("missing .gitignore: %v", err)
	}
}

// §12: a freshly created bundle targets OKF v0.2 via a root index.md
// frontmatter declaration.
func TestCreate_RootIndexDeclaresOKFVersion(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "mybundle")
	if err := Create(dir); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "index.md"))
	if err != nil {
		t.Fatalf("read index.md: %v", err)
	}
	s := string(got)
	if !strings.HasPrefix(s, "---\n") || !strings.Contains(s, `okf_version: "0.2"`) {
		t.Errorf("root index.md should declare okf_version \"0.2\" (OKF §12):\n%s", s)
	}
}

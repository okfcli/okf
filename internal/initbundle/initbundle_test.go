package initbundle

import (
	"os"
	"path/filepath"
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

package export

import (
	"os"
	"path/filepath"
	"testing"
)

func TestArchive_CreatesArchiveAndManifest(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "tables", "users.md"), "---\ntype: Table\ntitle: Users\n---\n\n# Users\n")
	writeFile(t, filepath.Join(root, "tables", "orders.md"), "---\ntype: Table\ntitle: Orders\n---\n\n# Orders\n")
	writeFile(t, filepath.Join(root, "index.md"), "# Index\n")

	outPath := filepath.Join(t.TempDir(), "bundle.okf")
	manifest, err := Archive(root, outPath)
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}

	if manifest.FileCount != 3 {
		t.Errorf("file_count = %d, want 3", manifest.FileCount)
	}
	if manifest.SHA256 == "" {
		t.Error("sha256 is empty")
	}

	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("archive not written: %v", err)
	}
	if info.Size() == 0 {
		t.Error("archive is empty")
	}
}

func TestArchive_Deterministic(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.md"), "---\ntype: T\ntitle: A\n---\n\nbody A\n")
	writeFile(t, filepath.Join(root, "b.md"), "---\ntype: T\ntitle: B\n---\n\nbody B\n")

	out1 := filepath.Join(t.TempDir(), "a1.okf")
	out2 := filepath.Join(t.TempDir(), "a2.okf")

	m1, err := Archive(root, out1)
	if err != nil {
		t.Fatalf("first Archive: %v", err)
	}
	m2, err := Archive(root, out2)
	if err != nil {
		t.Fatalf("second Archive: %v", err)
	}

	if m1.SHA256 != m2.SHA256 {
		t.Errorf("non-deterministic: %s != %s", m1.SHA256, m2.SHA256)
	}
}

func TestArchive_SkipsHiddenDirs(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "visible.md"), "---\ntype: T\ntitle: V\n---\n\nbody\n")
	writeFile(t, filepath.Join(root, ".git", "config"), "hidden\n")

	outPath := filepath.Join(t.TempDir(), "out.okf")
	manifest, err := Archive(root, outPath)
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if manifest.FileCount != 1 {
		t.Errorf("file_count = %d, want 1 (hidden dir should be skipped)", manifest.FileCount)
	}
}

func TestArchive_NotADirectory(t *testing.T) {
	f := filepath.Join(t.TempDir(), "notdir.txt")
	writeFile(t, f, "data")
	_, err := Archive(f, f+".okf")
	if err == nil {
		t.Fatal("expected error for non-directory")
	}
}

func TestArchive_ManifestFileHashes(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.md"), "---\ntype: T\ntitle: A\n---\n\nbody\n")

	outPath := filepath.Join(t.TempDir(), "out.okf")
	manifest, err := Archive(root, outPath)
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if len(manifest.Files) != 1 {
		t.Fatalf("files = %d, want 1", len(manifest.Files))
	}
	if manifest.Files[0].SHA256 == "" {
		t.Error("file sha256 is empty")
	}
	if manifest.Files[0].Path != "a.md" {
		t.Errorf("path = %q, want a.md", manifest.Files[0].Path)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// Package export bundles an entire OKF knowledge bundle into a single
// self-contained archive file. The archive is a deterministic tar of every
// file inside the bundle root (sorted by path), so the same bundle always
// produces the same byte-identical archive — which is essential for signing.
package export

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Manifest describes the contents of an exported archive.
type Manifest struct {
	Bundle   string         `json:"bundle"`           // absolute path to the bundle root
	Files    []ManifestFile `json:"files"`            // every file in the archive
	FileCount int           `json:"file_count"`       // total files
	TotalBytes int64        `json:"total_bytes"`       // sum of file sizes
	SHA256   string         `json:"sha256"`            // hex digest of the archive bytes
	Archive  string         `json:"archive"`           // path to the .okf file
	Created  string         `json:"created"`           // RFC 3339 timestamp
}

// ManifestFile is a single entry in the manifest.
type ManifestFile struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// Archive creates a deterministic .okf (tar.gz) archive of the bundle at root.
// The output is written to outPath. Returns a Manifest describing the archive.
func Archive(root, outPath string) (*Manifest, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", root, err)
	}
	info, err := os.Stat(absRoot)
	if err != nil {
		return nil, fmt.Errorf("stat bundle root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("bundle root %s is not a directory", absRoot)
	}

	// Collect all files (sorted for determinism).
	files, err := collectFiles(absRoot)
	if err != nil {
		return nil, err
	}

	// Write the archive to a buffer first so we can compute the SHA-256.
	var buf strings.Builder
	h := sha256.New()
	mw := io.MultiWriter(&buf, h)

	gw := gzip.NewWriter(mw)
	tw := tar.NewWriter(gw)

	var manifestFiles []ManifestFile
	var totalBytes int64

	for _, relPath := range files {
		absPath := filepath.Join(absRoot, relPath)
		fi, err := os.Stat(absPath)
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", relPath, err)
		}

		data, err := os.ReadFile(absPath)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", relPath, err)
		}

		fileHash := sha256.Sum256(data)
		manifestFiles = append(manifestFiles, ManifestFile{
			Path:   filepath.ToSlash(relPath),
			Size:   fi.Size(),
			SHA256: hex.EncodeToString(fileHash[:]),
		})
		totalBytes += fi.Size()

		hdr := &tar.Header{
			Name:    filepath.ToSlash(relPath),
			Size:    fi.Size(),
			Mode:    int64(fi.Mode()),
			ModTime: time.Time{}, // zero time for determinism
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, fmt.Errorf("tar header %s: %w", relPath, err)
		}
		if _, err := tw.Write(data); err != nil {
			return nil, fmt.Errorf("tar write %s: %w", relPath, err)
		}
	}

	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("close tar writer: %w", err)
	}
	if err := gw.Close(); err != nil {
		return nil, fmt.Errorf("close gzip writer: %w", err)
	}

	archiveBytes := []byte(buf.String())
	archiveHash := h.Sum(nil)

	// Write to output file.
	if err := os.WriteFile(outPath, archiveBytes, 0o644); err != nil {
		return nil, fmt.Errorf("write archive %s: %w", outPath, err)
	}

	return &Manifest{
		Bundle:     absRoot,
		Files:      manifestFiles,
		FileCount:  len(manifestFiles),
		TotalBytes: totalBytes,
		SHA256:     hex.EncodeToString(archiveHash),
		Archive:    outPath,
		Created:    time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// collectFiles returns a sorted list of relative file paths inside root.
// Hidden directories (starting with .) are skipped.
func collectFiles(root string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() != filepath.Base(root) && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("rel path %s: %w", path, err)
		}
		paths = append(paths, relPath)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(paths, func(i, j int) bool {
		return paths[i] < paths[j]
	})
	return paths, nil
}

// ManifestToJSON returns the manifest as pretty-printed JSON.
func ManifestToJSON(m *Manifest) ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}

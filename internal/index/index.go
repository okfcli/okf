// Package index generates index.md files for OKF bundle directories.
// Per OKF spec §6, index.md provides a directory listing for progressive
// disclosure — agents and humans navigate one level at a time.
package index

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/okfcli/okf/internal/concept"
)

// Generate writes index.md files into every subdirectory of the bundle root
// that contains concept documents. Existing index.md files are overwritten.
// The bundle root itself gets an index.md too.
func Generate(root string) error {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	return filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		// Skip hidden directories.
		if d.Name() != filepath.Base(absRoot) && strings.HasPrefix(d.Name(), ".") {
			return filepath.SkipDir
		}
		return generateForDir(absRoot, path)
	})
}

func generateForDir(root, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	var concepts []conceptInfo
	var subdirs []string
	for _, e := range entries {
		if e.IsDir() {
			if strings.HasPrefix(e.Name(), ".") {
				continue
			}
			// Only include subdirs that contain .md files (recursively).
			if hasMarkdownFiles(filepath.Join(dir, e.Name())) {
				subdirs = append(subdirs, e.Name())
			}
			continue
		}
		if !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		if concept.ReservedNames[strings.ToLower(e.Name())] {
			continue // don't list index.md/log.md in themselves
		}
		info, err := parseConceptInfo(filepath.Join(dir, e.Name()))
		if err != nil {
			return fmt.Errorf("parse %s: %w", e.Name(), err)
		}
		concepts = append(concepts, info)
	}

	if len(concepts) == 0 && len(subdirs) == 0 {
		return nil // nothing to index
	}

	sort.Slice(concepts, func(i, j int) bool { return concepts[i].title < concepts[j].title })
	sort.Strings(subdirs)

	var sb strings.Builder
	sb.WriteString("# Index\n\n")
	relDir, _ := filepath.Rel(root, dir)
	relDir = filepath.ToSlash(relDir)
	if relDir == "." {
		sb.WriteString("Bundle root.\n\n")
	} else {
		sb.WriteString(fmt.Sprintf("Directory: `%s/`\n\n", relDir))
	}

	if len(concepts) > 0 {
		sb.WriteString("## Concepts\n\n")
		sb.WriteString("| Title | Type | Description |\n")
		sb.WriteString("|-------|------|-------------|\n")
		for _, c := range concepts {
			desc := c.description
			if len(desc) > 80 {
				desc = desc[:77] + "..."
			}
			sb.WriteString(fmt.Sprintf("| [%s](%s) | %s | %s |\n", c.title, c.filename, c.typeName, desc))
		}
		sb.WriteString("\n")
	}

	if len(subdirs) > 0 {
		sb.WriteString("## Subdirectories\n\n")
		for _, s := range subdirs {
			sb.WriteString(fmt.Sprintf("- [%s/](%s/index.md)\n", s, s))
		}
		sb.WriteString("\n")
	}

	indexPath := filepath.Join(dir, "index.md")
	return os.WriteFile(indexPath, []byte(sb.String()), 0644)
}

type conceptInfo struct {
	filename    string
	title       string
	typeName    string
	description string
}

func parseConceptInfo(path string) (conceptInfo, error) {
	c, err := concept.Parse(path, filepath.Base(path))
	if err != nil {
		return conceptInfo{filename: filepath.Base(path), title: filepath.Base(path)}, nil
	}
	title := c.Frontmatter.Title
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(path), ".md")
	}
	return conceptInfo{
		filename:    filepath.Base(path),
		title:       title,
		typeName:    c.Frontmatter.Type,
		description: c.Frontmatter.Description,
	}, nil
}

// hasMarkdownFiles reports whether dir (recursively) contains any .md files
// other than reserved ones.
func hasMarkdownFiles(dir string) bool {
	found := false
	filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".md") && !concept.ReservedNames[strings.ToLower(d.Name())] {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

package concept

import (
	"reflect"
	"testing"
)

func TestParse_LinksScalar(t *testing.T) {
	raw := []byte("---\ntype: T\ntitle: A\nlinks: /b\n---\n\nbody")
	c, err := ParseBytes(raw, "a.md", "/tmp/a.md")
	if err != nil {
		t.Fatalf("ParseBytes: %v", err)
	}
	want := StringList{"/b"}
	if !reflect.DeepEqual(c.Frontmatter.Links, want) {
		t.Errorf("Links = %v, want %v", c.Frontmatter.Links, want)
	}
}

func TestParse_LinksSequence(t *testing.T) {
	raw := []byte("---\ntype: T\ntitle: A\nlinks:\n  - /b\n  - /c\n---\n\nbody")
	c, err := ParseBytes(raw, "a.md", "/tmp/a.md")
	if err != nil {
		t.Fatalf("ParseBytes: %v", err)
	}
	want := StringList{"/b", "/c"}
	if !reflect.DeepEqual(c.Frontmatter.Links, want) {
		t.Errorf("Links = %v, want %v", c.Frontmatter.Links, want)
	}
}

func TestParse_LinksMissing(t *testing.T) {
	raw := []byte("---\ntype: T\ntitle: A\n---\n\nbody")
	c, err := ParseBytes(raw, "a.md", "/tmp/a.md")
	if err != nil {
		t.Fatalf("ParseBytes: %v", err)
	}
	if len(c.Frontmatter.Links) != 0 {
		t.Errorf("Links = %v, want empty", c.Frontmatter.Links)
	}
}

func TestParse_LinksMalformedShapeTolerated(t *testing.T) {
	// A mapping (or otherwise unexpected) shape must not fail the parse - it is
	// tolerated as an empty list so one bad concept cannot kill a bundle load.
	raw := []byte("---\ntype: T\ntitle: A\nlinks:\n  target: /b\n---\n\nbody")
	c, err := ParseBytes(raw, "a.md", "/tmp/a.md")
	if err != nil {
		t.Fatalf("ParseBytes should tolerate malformed links, got error: %v", err)
	}
	if len(c.Frontmatter.Links) != 0 {
		t.Errorf("Links = %v, want empty for malformed shape", c.Frontmatter.Links)
	}
}

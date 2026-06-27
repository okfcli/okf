package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okfcli/okf/internal/bundle"
)

func TestExtractLinks(t *testing.T) {
	body := `See [events table](/tables/events_.md) for details.
External: [docs](https://example.com/docs).
Relative: [local](../tables/events_.md).
Fragment: [section](#schema).
Empty: [](something.md).`

	links := extractLinks(body)
	// All 5 links have non-empty targets (including the empty-text one).
	want := 5
	if len(links) != want {
		t.Fatalf("got %d links, want %d: %+v", len(links), want, links)
	}

	// First link is absolute.
	if links[0].Target != "/tables/events_.md" {
		t.Errorf("link[0] target = %q, want /tables/events_.md", links[0].Target)
	}
}

func TestResolveLink_Absolute(t *testing.T) {
	link := Link{Text: "events", Target: "/tables/events_.md"}
	got := resolveLink("datasets/ga4", link)
	want := "tables/events_"
	if got != want {
		t.Errorf("resolveLink absolute = %q, want %q", got, want)
	}
}

func TestResolveLink_Relative(t *testing.T) {
	link := Link{Text: "events", Target: "../tables/events_.md"}
	got := resolveLink("datasets/ga4", link)
	want := "tables/events_"
	if got != want {
		t.Errorf("resolveLink relative = %q, want %q", got, want)
	}
}

func TestResolveLink_External(t *testing.T) {
	link := Link{Text: "docs", Target: "https://example.com/docs"}
	got := resolveLink("datasets/ga4", link)
	if got != "" {
		t.Errorf("resolveLink external = %q, want empty (skip)", got)
	}
}

func TestResolveLink_Fragment(t *testing.T) {
	link := Link{Text: "schema", Target: "#schema"}
	got := resolveLink("datasets/ga4", link)
	if got != "" {
		t.Errorf("resolveLink fragment = %q, want empty (skip)", got)
	}
}

func TestResolveLink_WithFragment(t *testing.T) {
	link := Link{Text: "events", Target: "/tables/events_.md#schema"}
	got := resolveLink("datasets/ga4", link)
	want := "tables/events_"
	if got != want {
		t.Errorf("resolveLink with fragment = %q, want %q", got, want)
	}
}

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"datasets/../tables/events_.md", "tables/events_.md"},
		{"a/b/./c", "a/b/c"},
		{"a/b/c", "a/b/c"},
		{"./a", "a"},
		{"", ""},
	}
	for _, tt := range tests {
		got := normalizePath(tt.in)
		if got != tt.want {
			t.Errorf("normalizePath(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// --- validateLinks integration tests ---

func TestValidateLinks_RelativeLinkSuggestsAbsolutePath(t *testing.T) {
	// A concept at pages/about has [Cloaked](organizations/cloaked). The
	// relative target resolves to pages/organizations/cloaked (broken), but
	// the absolute target organizations/cloaked exists. The error should
	// suggest the absolute path.
	b := testBundle(t, map[string]string{
		"pages/about.md":             "---\ntype: Page\ntitle: About\n---\n\nSee [Cloaked](organizations/cloaked).",
		"organizations/cloaked.md":   "---\ntype: Org\ntitle: Cloaked\n---\n\nbody",
	})

	r := &Report{}
	validateLinks(r, b)

	var msg string
	for _, f := range r.Findings {
		if f.ConceptID == "pages/about" && strings.Contains(f.Message, "broken link") {
			msg = f.Message
			break
		}
	}
	if msg == "" {
		t.Fatalf("no broken-link finding for pages/about: %+v", r.Findings)
	}
	if !strings.Contains(msg, "use /organizations/cloaked for an absolute path") {
		t.Errorf("error %q does not suggest absolute path", msg)
	}
}

func TestValidateLinks_NonexistentConceptNoAbsolutePathSuggestion(t *testing.T) {
	// A link to a genuinely nonexistent concept should report "broken link:"
	// but must NOT mention "absolute path".
	b := testBundle(t, map[string]string{
		"pages/about.md": "---\ntype: Page\ntitle: About\n---\n\nSee [Ghost](/nonexistent/ghost).",
	})

	r := &Report{}
	validateLinks(r, b)

	var msg string
	for _, f := range r.Findings {
		if f.ConceptID == "pages/about" && strings.Contains(f.Message, "broken link") {
			msg = f.Message
			break
		}
	}
	if msg == "" {
		t.Fatalf("no broken-link finding for pages/about: %+v", r.Findings)
	}
	if !strings.Contains(msg, "broken link:") {
		t.Errorf("error %q does not contain 'broken link:'", msg)
	}
	if strings.Contains(msg, "absolute path") {
		t.Errorf("error %q should not mention 'absolute path' for a genuinely missing concept", msg)
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

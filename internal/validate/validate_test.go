package validate

import (
	"testing"
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

func TestExtractFrontmatterLinks(t *testing.T) {
	links := ExtractFrontmatterLinks([]string{"/tables/events_", "users", "", "/playbooks/check"})
	if len(links) != 3 {
		t.Fatalf("got %d links, want 3 (empty skipped): %+v", len(links), links)
	}
	if links[0].Text != "" || links[0].Target != "/tables/events_" {
		t.Errorf("link[0] = %+v, want Text=\"\" Target=/tables/events_", links[0])
	}
	if links[1].Target != "users" {
		t.Errorf("link[1] target = %q, want users", links[1].Target)
	}
	if links[2].Target != "/playbooks/check" {
		t.Errorf("link[2] target = %q, want /playbooks/check", links[2].Target)
	}
}

func TestExtractFrontmatterLinks_Empty(t *testing.T) {
	if links := ExtractFrontmatterLinks(nil); len(links) != 0 {
		t.Fatalf("got %d links, want 0", len(links))
	}
	if links := ExtractFrontmatterLinks([]string{}); len(links) != 0 {
		t.Fatalf("got %d links, want 0", len(links))
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

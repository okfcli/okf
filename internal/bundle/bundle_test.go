package bundle

import (
	"testing"
)

func TestLoad_ValidBundle(t *testing.T) {
	b, err := Load("../../testdata/valid")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	// 3 concept documents (ga4, events_, freshness).
	if len(b.Concepts) != 3 {
		t.Fatalf("got %d concepts, want 3", len(b.Concepts))
	}
	// Check concept IDs are derived correctly.
	wantIDs := map[string]bool{
		"datasets/ga4":       false,
		"tables/events_":     false,
		"playbooks/freshness": false,
	}
	for _, c := range b.Concepts {
		if _, ok := wantIDs[c.ID]; !ok {
			t.Errorf("unexpected concept ID: %s", c.ID)
		}
		wantIDs[c.ID] = true
	}
	for id, found := range wantIDs {
		if !found {
			t.Errorf("missing concept: %s", id)
		}
	}
}

func TestLoad_HasConcept(t *testing.T) {
	b, err := Load("../../testdata/valid")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if !b.HasConcept("tables/events_") {
		t.Error("HasConcept(tables/events_) = false, want true")
	}
	if b.HasConcept("nonexistent") {
		t.Error("HasConcept(nonexistent) = true, want false")
	}
}

func TestLoad_FrontmatterParsed(t *testing.T) {
	b, err := Load("../../testdata/valid")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	c := b.Get("datasets/ga4")
	if c == nil {
		t.Fatal("concept datasets/ga4 not found")
	}
	if c.Frontmatter.Type != "BigQuery Dataset" {
		t.Errorf("type = %q, want 'BigQuery Dataset'", c.Frontmatter.Type)
	}
	if c.Frontmatter.Title != "GA4 Ecommerce" {
		t.Errorf("title = %q, want 'GA4 Ecommerce'", c.Frontmatter.Title)
	}
	if len(c.Frontmatter.Tags) != 3 {
		t.Errorf("tags len = %d, want 3", len(c.Frontmatter.Tags))
	}
}

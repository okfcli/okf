package enrich

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/okfcli/okf/internal/llm"
	"github.com/okfcli/okf/internal/source"
)

// fakeSource is a test Source that returns canned ConceptInputs.
type fakeSource struct {
	inputs []source.ConceptInput
}

func (f *fakeSource) Name() string { return "fake" }
func (f *fakeSource) Inspect() ([]source.ConceptInput, error) {
	return f.inputs, nil
}

// fakeChatFn returns a canned ConceptDoc JSON for any input.
func fakeChatFn(ctx context.Context, messages []llm.ChatMessage, schema *llm.JSONSchemaDef) (string, error) {
	return `{
		"type": "PostgreSQL Table",
		"title": "Users",
		"description": "User accounts table.",
		"tags": ["auth", "users"],
		"body": "# Schema\n\n| Column | Type |\n|--------|------|\n| id | integer |\n| email | text |\n\n# Examples\n\nSELECT * FROM users;"
	}`, nil
}

func TestRun_BasicEnrichment(t *testing.T) {
	src := &fakeSource{
		inputs: []source.ConceptInput{
			{
				ID:     "public.users",
				Type:   "PostgreSQL Table",
				Title:  "users",
				Schema: `{"schema":"public","table":"users","columns":[{"name":"id","type":"integer"}]}`,
			},
		},
	}

	outDir := t.TempDir()
	report, err := Run(context.Background(), Options{
		Source: src,
		ChatFn: fakeChatFn,
		OutDir: outDir,
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if report.Total != 1 {
		t.Errorf("Total = %d, want 1", report.Total)
	}
	if report.Enriched != 1 {
		t.Errorf("Enriched = %d, want 1", report.Enriched)
	}
	if report.Errors != 0 {
		t.Errorf("Errors = %d, want 0", report.Errors)
	}

	// Verify the file was written.
	filePath := filepath.Join(outDir, "public_users.md")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatalf("expected file %s to exist", filePath)
	}

	// Verify the file content has frontmatter + body.
	content, _ := os.ReadFile(filePath)
	s := string(content)
	if !contains(s, "type: PostgreSQL Table") {
		t.Errorf("file missing frontmatter type: %s", s[:200])
	}
	if !contains(s, "# Schema") {
		t.Errorf("file missing body schema section: %s", s[:200])
	}
}

func TestRun_LLMError(t *testing.T) {
	src := &fakeSource{
		inputs: []source.ConceptInput{
			{ID: "test", Type: "Test", Title: "test", Schema: "{}"},
		},
	}

	errChatFn := func(ctx context.Context, messages []llm.ChatMessage, schema *llm.JSONSchemaDef) (string, error) {
		return "", nil // empty content triggers error
	}

	outDir := t.TempDir()
	report, err := Run(context.Background(), Options{
		Source: src,
		ChatFn: errChatFn,
		OutDir: outDir,
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if report.Errors != 1 {
		t.Errorf("Errors = %d, want 1", report.Errors)
	}
	if report.Enriched != 0 {
		t.Errorf("Enriched = %d, want 0", report.Enriched)
	}
}

func TestRun_MaxConcepts(t *testing.T) {
	src := &fakeSource{
		inputs: []source.ConceptInput{
			{ID: "a", Type: "T", Title: "a", Schema: "{}"},
			{ID: "b", Type: "T", Title: "b", Schema: "{}"},
			{ID: "c", Type: "T", Title: "c", Schema: "{}"},
		},
	}

	outDir := t.TempDir()
	report, err := Run(context.Background(), Options{
		Source:      src,
		ChatFn:      fakeChatFn,
		OutDir:      outDir,
		MaxConcepts: 2,
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if report.Total != 2 {
		t.Errorf("Total = %d, want 2 (capped)", report.Total)
	}
}

func TestRenderConceptDoc(t *testing.T) {
	doc := ConceptDoc{
		Type:        "Test Type",
		Title:       "Test Title",
		Description: "A test concept.",
		Tags:        []string{"a", "b"},
		Body:        "# Schema\n\n| Col | Type |\n|-----|------|\n| id | int |",
	}
	input := source.ConceptInput{Resource: "test://resource"}

	md := renderConceptDoc(doc, input)
	if !contains(md, "type: Test Type") {
		t.Errorf("missing type in frontmatter")
	}
	if !contains(md, "tags: [a, b]") {
		t.Errorf("missing tags in frontmatter")
	}
	if !contains(md, "resource: test://resource") {
		t.Errorf("missing resource in frontmatter")
	}
	if !contains(md, "# Schema") {
		t.Errorf("missing body")
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Package enrich is the agentic core of okf. It takes raw metadata from a
// Source, sends each concept through an LLM to produce an OKF concept document,
// writes the documents to an output bundle, and validates the result.
//
// The enrichment loop:
//  1. source.Inspect() → []ConceptInput (raw table schemas, API endpoints, etc.)
//  2. For each ConceptInput, call the LLM with a system prompt that instructs
//     it to write an OKF concept document (frontmatter + markdown body).
//  3. Parse the LLM's structured JSON output into a ConceptDoc.
//  4. Write the .md file to the output bundle directory.
//  5. Run validate on the resulting bundle.
package enrich

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/okfcli/okf/internal/bundle"
	"github.com/okfcli/okf/internal/llm"
	"github.com/okfcli/okf/internal/source"
	"github.com/okfcli/okf/internal/validate"
)

// Options configures an enrichment run.
type Options struct {
	// Source is the metadata provider (postgres, openapi, etc.).
	Source source.Source
	// ChatFn is the LLM call function (injectable for testing).
	ChatFn llm.ChatFunc
	// OutDir is the bundle output directory.
	OutDir string
	// MaxConcepts caps the number of concepts to enrich (0 = no limit).
	MaxConcepts int
	// Trace, if true, emits per-concept NDJSON progress events.
	Trace bool
}

// ConceptDoc is the structured output the LLM returns for each concept.
// It maps directly to an OKF concept document (frontmatter + body).
type ConceptDoc struct {
	Type        string   `json:"type"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Body        string   `json:"body"`
}

// Result is the output of a single concept enrichment.
type Result struct {
	ID        string `json:"id"`
	Status    string `json:"status"` // "ok", "error", "skipped"
	File      string `json:"file,omitempty"`
	Error     string `json:"error,omitempty"`
	TokensIn  int    `json:"tokens_in,omitempty"`
	TokensOut int    `json:"tokens_out,omitempty"`
}

// Report is the full enrichment output (JSON-serializable).
type Report struct {
	Source     string         `json:"source"`
	StartedAt  time.Time      `json:"started_at"`
	FinishedAt time.Time      `json:"finished_at"`
	Total      int            `json:"total"`
	Enriched   int            `json:"enriched"`
	Errors     int            `json:"errors"`
	Results    []Result       `json:"results"`
	Validation *validate.Report `json:"validation,omitempty"`
}

// conceptSchema is the JSON Schema the LLM must conform to when producing
// a concept document.
var conceptSchema = &llm.JSONSchemaDef{
	Name: "okf_concept",
	Schema: map[string]any{
		"type": "object",
		"properties": map[string]any{
			"type":        map[string]any{"type": "string"},
			"title":       map[string]any{"type": "string"},
			"description": map[string]any{"type": "string"},
			"tags": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"body": map[string]any{"type": "string"},
		},
		"required": []string{"type", "title", "description", "body"},
	},
	Strict: false,
}

const systemPrompt = `You are a data catalog enrichment agent. Your job is to take raw metadata
about a data asset (a database table, an API endpoint, a metric, etc.) and
produce an OKF (Open Knowledge Format) concept document — a markdown file with
YAML frontmatter and a structured body.

The OKF spec requires:
- frontmatter with a "type" field (required)
- frontmatter with "title", "description", "tags" (recommended)
- a markdown body with structural sections: # Schema, # Examples, # Citations

You will receive the raw metadata as JSON. Produce a concept document that:
1. Has a clear "type" (e.g. "PostgreSQL Table", "API Endpoint")
2. Has a human-readable "title" and one-line "description"
3. Has relevant "tags" for categorization
4. Has a "body" containing:
   - A # Schema section with a markdown table of columns/fields
   - A # Examples section with a sample query or usage example
   - A # Notes section with any relationships, constraints, or context

Be concise but thorough. The body is markdown (not HTML). Use tables for schemas.
If the metadata includes foreign keys, note the relationships in the body.

Respond with JSON matching the schema: {type, title, description, tags, body}.`

// Run executes the enrichment loop and returns a Report.
func Run(ctx context.Context, opts Options) (*Report, error) {
	if opts.Source == nil {
		return nil, fmt.Errorf("source is required")
	}
	if opts.ChatFn == nil {
		return nil, fmt.Errorf("chat function is required")
	}
	if opts.OutDir == "" {
		return nil, fmt.Errorf("output directory is required")
	}

	report := &Report{
		Source:    opts.Source.Name(),
		StartedAt: time.Now().UTC(),
	}

	// Step 1: Inspect the source.
	inputs, err := opts.Source.Inspect()
	if err != nil {
		return nil, fmt.Errorf("inspect source: %w", err)
	}

	if opts.MaxConcepts > 0 && len(inputs) > opts.MaxConcepts {
		inputs = inputs[:opts.MaxConcepts]
	}

	report.Total = len(inputs)

	// Ensure output directory exists.
	if err := os.MkdirAll(opts.OutDir, 0755); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}

	// Step 2: Enrich each concept.
	for _, input := range inputs {
		result := enrichConcept(ctx, input, opts)
		report.Results = append(report.Results, result)
		if result.Status == "ok" {
			report.Enriched++
		} else {
			report.Errors++
		}

		if opts.Trace {
			emitTrace(result)
		}
	}

	// Step 3: Validate the resulting bundle.
	b, err := bundle.Load(opts.OutDir)
	if err == nil {
		report.Validation = validate.Validate(b)
	}

	report.FinishedAt = time.Now().UTC()
	return report, nil
}

func enrichConcept(ctx context.Context, input source.ConceptInput, opts Options) Result {
	result := Result{ID: input.ID}

	// Build the user prompt from the concept input.
	userPrompt := fmt.Sprintf("Concept ID: %s\nType hint: %s\nTitle: %s\n\nRaw metadata:\n%s",
		input.ID, input.Type, input.Title, input.Schema)

	messages := []llm.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	// Call the LLM with the concept schema.
	content, err := opts.ChatFn(ctx, messages, conceptSchema)
	if err != nil {
		result.Status = "error"
		result.Error = err.Error()
		return result
	}

	// Parse the structured output.
	var doc ConceptDoc
	if err := json.Unmarshal([]byte(content), &doc); err != nil {
		result.Status = "error"
		result.Error = fmt.Sprintf("parse LLM output: %v (raw: %q)", err, truncate(content, 200))
		return result
	}

	// Validate required fields.
	if doc.Type == "" {
		doc.Type = input.Type
	}
	if doc.Title == "" {
		doc.Title = input.Title
	}
	if doc.Tags == nil {
		doc.Tags = []string{}
	}

	// Write the concept document.
	filename := sanitizeFilename(input.ID) + ".md"
	outPath := filepath.Join(opts.OutDir, filename)

	md := renderConceptDoc(doc, input)
	if err := os.WriteFile(outPath, []byte(md), 0644); err != nil {
		result.Status = "error"
		result.Error = fmt.Sprintf("write file: %v", err)
		return result
	}

	result.Status = "ok"
	result.File = filename
	return result
}

// renderConceptDoc produces the final markdown file (frontmatter + body).
func renderConceptDoc(doc ConceptDoc, input source.ConceptInput) string {
	var sb strings.Builder
	sb.WriteString("---\n")
	fmt.Fprintf(&sb, "type: %s\n", doc.Type)
	fmt.Fprintf(&sb, "title: %s\n", doc.Title)
	fmt.Fprintf(&sb, "description: %s\n", doc.Description)
	if len(doc.Tags) > 0 {
		fmt.Fprintf(&sb, "tags: [%s]\n", strings.Join(doc.Tags, ", "))
	}
	if input.Resource != "" {
		fmt.Fprintf(&sb, "resource: %s\n", input.Resource)
	}
	fmt.Fprintf(&sb, "timestamp: %s\n", time.Now().UTC().Format(time.RFC3339))
	sb.WriteString("---\n\n")
	sb.WriteString(doc.Body)
	sb.WriteString("\n")
	return sb.String()
}

func sanitizeFilename(id string) string {
	s := strings.ReplaceAll(id, ".", "_")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, " ", "_")
	return strings.ToLower(s)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func emitTrace(r Result) {
	b, _ := json.Marshal(r)
	fmt.Fprintln(os.Stderr, string(b))
}

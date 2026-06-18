// Package source defines the Source interface for data systems that okf enrich
// can ingest metadata from. Each source inspects a live system (a database,
// an API spec, a catalog) and returns a list of ConceptInputs — raw metadata
// that the enrichment LLM turns into OKF concept documents.
package source

import (
	"fmt"
	"strings"
)

// Source is a metadata provider. Implementations include postgres, openapi,
// and (future) gws (Google Workspace), dbt, Snowflake.
type Source interface {
	// Name returns the source type identifier ("postgres", "openapi", etc.).
	Name() string
	// Inspect connects to the source and returns raw metadata for each
	// concept the source advertises.
	Inspect() ([]ConceptInput, error)
}

// ConceptInput is raw metadata about a single concept (table, endpoint, etc.)
// that the enrichment LLM will turn into an OKF concept document.
type ConceptInput struct {
	// ID is the concept identifier within the source (e.g. "public.users",
	// "GET /api/v1/users"). Used for the output filename.
	ID string `json:"id"`
	// Type is the OKF concept type hint (e.g. "PostgreSQL Table", "API Endpoint").
	Type string `json:"type"`
	// Title is a human-readable name for the concept.
	Title string `json:"title"`
	// Schema is the structured metadata (columns, fields, parameters, etc.)
	// serialized as a JSON string for the LLM prompt.
	Schema string `json:"schema"`
	// Resource is the canonical URI for the underlying asset, if available.
	Resource string `json:"resource,omitempty"`
}

// Opener opens a source from a connection string or file path.
// Registered by init() in each source driver package.
type Opener func(dsn string) (Source, error)

var openers = make(map[string]Opener)

// Register registers a source opener under a name.
func Register(name string, fn Opener) {
	openers[name] = fn
}

// Open opens a source by type name and connection string.
func Open(name, dsn string) (Source, error) {
	fn, ok := openers[name]
	if !ok {
		available := make([]string, 0, len(openers))
		for k := range openers {
			available = append(available, k)
		}
		return nil, fmt.Errorf("unknown source %q (available: %s)", name, strings.Join(available, ", "))
	}
	return fn(dsn)
}

// AvailableSources returns the names of all registered source types.
func AvailableSources() []string {
	names := make([]string, 0, len(openers))
	for k := range openers {
		names = append(names, k)
	}
	return names
}

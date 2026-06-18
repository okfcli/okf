// Package postgres is a source driver that introspects a PostgreSQL database
// via information_schema and returns one ConceptInput per table.
package postgres

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	_ "github.com/lib/pq" // database/sql driver

	"github.com/okfcli/okf/internal/source"
)

func init() {
	source.Register("postgres", Open)
}

// PostgresSource introspects a PostgreSQL database.
type PostgresSource struct {
	dsn string
	db  *sql.DB
}

// Open creates a PostgresSource from a connection string.
// Format: "postgres://user:pass@host:5432/dbname?sslmode=disable"
func Open(dsn string) (source.Source, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	return &PostgresSource{dsn: dsn, db: db}, nil
}

// Name returns "postgres".
func (p *PostgresSource) Name() string { return "postgres" }

// Inspect queries information_schema for all tables in non-system schemas
// and returns a ConceptInput per table with column metadata.
func (p *PostgresSource) Inspect() ([]source.ConceptInput, error) {
	defer p.db.Close()

	// Fetch all tables in non-system schemas.
	tableRows, err := p.db.Query(`
		SELECT table_schema, table_name
		FROM information_schema.tables
		WHERE table_schema NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
		  AND table_type = 'BASE TABLE'
		ORDER BY table_schema, table_name
	`)
	if err != nil {
		return nil, fmt.Errorf("query tables: %w", err)
	}
	defer tableRows.Close()

	var tables []struct {
		Schema string
		Name   string
	}
	for tableRows.Next() {
		var t struct {
			Schema string
			Name   string
		}
		if err := tableRows.Scan(&t.Schema, &t.Name); err != nil {
			return nil, fmt.Errorf("scan table: %w", err)
		}
		tables = append(tables, t)
	}

	var inputs []source.ConceptInput
	for _, t := range tables {
		input, err := p.inspectTable(t.Schema, t.Name)
		if err != nil {
			return nil, fmt.Errorf("inspect %s.%s: %w", t.Schema, t.Name, err)
		}
		inputs = append(inputs, input)
	}
	return inputs, nil
}

// Column describes a single column in a table.
type Column struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Nullable   bool   `json:"nullable"`
	Default    string `json:"default,omitempty"`
	IsPrimary  bool   `json:"is_primary"`
	IsForeign  bool   `json:"is_foreign,omitempty"`
	References string `json:"references,omitempty"` // "schema.table.column" if FK
}

// TableSchema is the structured metadata for a table, serialized into the
// ConceptInput.Schema field.
type TableSchema struct {
	Schema  string   `json:"schema"`
	Table   string   `json:"table"`
	Columns []Column `json:"columns"`
}

func (p *PostgresSource) inspectTable(schema, table string) (source.ConceptInput, error) {
	// Column metadata.
	colRows, err := p.db.Query(`
		SELECT
			c.column_name,
			c.data_type,
			c.is_nullable = 'YES',
			COALESCE(c.column_default, ''),
			EXISTS(
				SELECT 1 FROM information_schema.table_constraints tc
				JOIN information_schema.key_column_usage kcu
				  ON tc.constraint_name = kcu.constraint_name
				 AND tc.table_schema = kcu.table_schema
				WHERE tc.constraint_type = 'PRIMARY KEY'
				  AND tc.table_schema = c.table_schema
				  AND tc.table_name = c.table_name
				  AND kcu.column_name = c.column_name
			) AS is_primary
		FROM information_schema.columns c
		WHERE c.table_schema = $1 AND c.table_name = $2
		ORDER BY c.ordinal_position
	`, schema, table)
	if err != nil {
		return source.ConceptInput{}, fmt.Errorf("query columns: %w", err)
	}
	defer colRows.Close()

	var cols []Column
	for colRows.Next() {
		var c Column
		if err := colRows.Scan(&c.Name, &c.Type, &c.Nullable, &c.Default, &c.IsPrimary); err != nil {
			return source.ConceptInput{}, fmt.Errorf("scan column: %w", err)
		}
		cols = append(cols, c)
	}

	// Foreign keys.
	fkRows, err := p.db.Query(`
		SELECT
			kcu.column_name,
			ccu.table_schema || '.' || ccu.table_name || '.' || ccu.column_name AS references
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
		  ON tc.constraint_name = kcu.constraint_name
		 AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage ccu
		  ON tc.constraint_name = ccu.constraint_name
		WHERE tc.constraint_type = 'FOREIGN KEY'
		  AND tc.table_schema = $1 AND tc.table_name = $2
	`, schema, table)
	if err != nil {
		return source.ConceptInput{}, fmt.Errorf("query foreign keys: %w", err)
	}
	defer fkRows.Close()

	fkMap := make(map[string]string)
	for fkRows.Next() {
		var col, ref string
		if err := fkRows.Scan(&col, &ref); err != nil {
			return source.ConceptInput{}, fmt.Errorf("scan FK: %w", err)
		}
		fkMap[col] = ref
	}

	for i := range cols {
		if ref, ok := fkMap[cols[i].Name]; ok {
			cols[i].IsForeign = true
			cols[i].References = ref
		}
	}

	ts := TableSchema{
		Schema:  schema,
		Table:   table,
		Columns: cols,
	}
	schemaJSON, _ := json.MarshalIndent(ts, "", "  ")

	id := schema + "." + table
	return source.ConceptInput{
		ID:       id,
		Type:     "PostgreSQL Table",
		Title:    table,
		Schema:   string(schemaJSON),
		Resource: fmt.Sprintf("postgres://%s", id),
	}, nil
}

// SanitizeID converts a "schema.table" identifier to a safe filename component.
func SanitizeID(id string) string {
	return strings.ReplaceAll(id, ".", "_")
}

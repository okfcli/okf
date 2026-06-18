// Command okf is a CLI toolkit for the Open Knowledge Format (OKF).
// It validates, lints, indexes, and inspects OKF bundles.
//
// okf is designed agentic-first: every command supports --json for
// machine-readable output, and `okf schema` emits a complete machine-readable
// description of every command, its flags, args, and output format.
// An external AI agent can discover and drive the entire CLI from that one
// command.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/okfcli/okf/internal/bundle"
	"github.com/okfcli/okf/internal/graph"
	"github.com/okfcli/okf/internal/index"
	"github.com/okfcli/okf/internal/validate"
)

var version = "dev"

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		printUsage()
		os.Exit(0)
	}

	cmd := args[0]
	rest := args[1:]

	switch cmd {
	case "version", "--version", "-v":
		fmt.Printf("okf %s\n", version)
	case "help", "--help", "-h":
		printUsage()
	case "schema":
		runSchema(rest)
	case "validate":
		runValidate(rest, true)
	case "lint":
		runValidate(rest, false)
	case "index":
		runIndex(rest)
	case "graph":
		runGraph(rest)
	case "list":
		runList(rest)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(2)
	}
}

const usage = `okf — Open Knowledge Format toolkit (v%s)

Usage:
  okf <command> [--json] <bundle-path>

Commands:
  schema [command]            Print machine-readable CLI metadata as JSON
  validate [--json] <bundle>  Validate a bundle against the OKF spec
  lint [--json] <bundle>      Check recommended fields and style (warnings only)
  index [--json] <bundle>     Generate index.md files (progressive disclosure)
  graph [--json] <bundle>     Print cross-link graph statistics
  list [--json] <bundle>      List all concepts in the bundle
  version                     Print version

All commands accept --json for structured, machine-readable output.
Run ` + "`okf schema`" + ` for a complete machine-readable description of every
command, its flags, arguments, and output format.

OKF spec: https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md
`

func printUsage() {
	fmt.Printf(usage, version)
}

// --- shared helpers ---

func mustBundle(args []string) (*bundle.Bundle, []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "error: bundle path required")
		os.Exit(2)
	}
	b, err := bundle.Load(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	return b, args[1:]
}

func parseJSONFlag(args []string) (jsonMode bool, rest []string) {
	rest = make([]string, 0, len(args))
	for _, a := range args {
		if a == "--json" || a == "-j" {
			jsonMode = true
			continue
		}
		rest = append(rest, a)
	}
	return
}

func outputJSON(v any) {
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: marshal: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(out))
}

// --- validate / lint ---

func runValidate(args []string, strict bool) {
	jsonMode, rest := parseJSONFlag(args)
	b, _ := mustBundle(rest)
	r := validate.Validate(b)

	if jsonMode {
		findings := make([]map[string]any, 0, len(r.Findings))
		for _, f := range r.Findings {
			if !strict && f.Severity == validate.SeverityError {
				continue
			}
			findings = append(findings, map[string]any{
				"concept_id": f.ConceptID,
				"severity":   f.Severity.String(),
				"message":    f.Message,
			})
		}
		outputJSON(map[string]any{
			"command":  map[string]string{"name": commandName(strict)},
			"bundle":   b.Root,
			"findings": findings,
			"errors":   r.Errors,
			"warnings": r.Warnings,
			"valid":    !r.HasErrors(),
		})
		if strict && r.HasErrors() {
			os.Exit(1)
		}
		return
	}

	// human-readable
	if len(r.Findings) == 0 {
		fmt.Printf("✓ bundle valid — %d concepts, 0 errors, 0 warnings\n", len(b.Concepts))
		return
	}
	for _, f := range r.Findings {
		if !strict && f.Severity == validate.SeverityError {
			continue
		}
		fmt.Printf("  %s  %s: %s\n", f.Severity, f.ConceptID, f.Message)
	}
	summary := fmt.Sprintf("%d errors, %d warnings", r.Errors, r.Warnings)
	if strict && r.HasErrors() {
		fmt.Fprintf(os.Stderr, "\n✗ FAIL — %s\n", summary)
		os.Exit(1)
	}
	fmt.Printf("\n✓ %s\n", summary)
}

func commandName(strict bool) string {
	if strict {
		return "validate"
	}
	return "lint"
}

// --- index ---

func runIndex(args []string) {
	jsonMode, rest := parseJSONFlag(args)
	if len(rest) == 0 {
		fmt.Fprintln(os.Stderr, "error: bundle path required")
		os.Exit(2)
	}
	root := rest[0]
	if err := index.Generate(root); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Count generated index files for JSON output.
	var indexFiles []string
	b, err := bundle.Load(root)
	if err == nil {
		for _, r := range b.Reserved {
			if strings.HasSuffix(r.ID, "index") || r.ID == "index" {
				indexFiles = append(indexFiles, r.Path)
			}
		}
	}

	if jsonMode {
		outputJSON(map[string]any{
			"command":        map[string]string{"name": "index"},
			"bundle":         root,
			"indexes_written": indexFiles,
			"count":          len(indexFiles),
		})
		return
	}
	fmt.Printf("✓ index.md files generated in %s\n", root)
}

// --- graph ---

func runGraph(args []string) {
	jsonMode, rest := parseJSONFlag(args)
	b, _ := mustBundle(rest)
	g := graph.Build(b)
	s := g.Stats()

	if jsonMode {
		density := 0.0
		if s.NodeCount > 1 && s.EdgeCount > 0 {
			density = float64(s.EdgeCount) / float64(s.NodeCount*(s.NodeCount-1)) * 100
		}
		nodes := make([]map[string]string, 0, len(g.Nodes))
		for _, n := range g.Nodes {
			nodes = append(nodes, map[string]string{"id": n.ID, "type": n.Type})
		}
		edges := make([]map[string]string, 0, len(g.Edges))
		for _, e := range g.Edges {
			edges = append(edges, map[string]string{"from": e.From, "to": e.To})
		}
		outputJSON(map[string]any{
			"command":      map[string]string{"name": "graph"},
			"bundle":       b.Root,
			"nodes":        nodes,
			"edges":        edges,
			"node_count":   s.NodeCount,
			"edge_count":   s.EdgeCount,
			"isolated":     s.IsolatedNodes,
			"max_backlinks": s.MaxBacklinks,
			"density_pct":  density,
		})
		return
	}
	fmt.Print(g.Summary())
}

// --- list ---

func runList(args []string) {
	jsonMode, rest := parseJSONFlag(args)
	b, _ := mustBundle(rest)

	if jsonMode {
		concepts := make([]map[string]string, 0, len(b.Concepts))
		for _, c := range b.Concepts {
			concepts = append(concepts, map[string]string{
				"id":    c.ID,
				"type":  c.Frontmatter.Type,
				"title": c.Frontmatter.Title,
			})
		}
		outputJSON(map[string]any{
			"command":  map[string]string{"name": "list"},
			"bundle":   b.Root,
			"concepts": concepts,
			"count":    len(b.Concepts),
		})
		return
	}

	if len(b.Concepts) == 0 {
		fmt.Println("(no concepts found)")
		return
	}
	fmt.Printf("%-50s  %-20s  %s\n", "ID", "TYPE", "TITLE")
	fmt.Println(strings.Repeat("-", 90))
	for _, c := range b.Concepts {
		title := c.Frontmatter.Title
		if len(title) > 30 {
			title = title[:27] + "..."
		}
		fmt.Printf("%-50s  %-20s  %s\n", c.ID, c.Frontmatter.Type, title)
	}
	fmt.Printf("\n%d concepts\n", len(b.Concepts))
}

// --- schema ---

// schemaFlag describes a single flag for machine consumption.
type schemaFlag struct {
	Name        string `json:"name"`
	Short       string `json:"short,omitempty"`
	Type        string `json:"type"`
	Default     string `json:"default"`
	Description string `json:"description"`
}

// schemaArg describes a positional argument.
type schemaArg struct {
	Name     string `json:"name"`
	Required bool   `json:"required"`
}

// schemaCommand describes a single CLI command for machine consumption.
type schemaCommand struct {
	Name      string       `json:"name"`
	Short     string       `json:"short"`
	Long      string       `json:"long,omitempty"`
	Flags     []schemaFlag `json:"flags"`
	Args      []schemaArg  `json:"args"`
	Stdout    string       `json:"stdout"`
	ExitCodes []int        `json:"exit_codes"`
}

// schemaRoot is the top-level schema output.
type schemaRoot struct {
	Name        string          `json:"name"`
	Version     string          `json:"version"`
	Description string          `json:"description"`
	Commands    []schemaCommand `json:"commands"`
	ExitCodes   []exitCodeDoc   `json:"exit_codes"`
}

type exitCodeDoc struct {
	Code        int    `json:"code"`
	Description string `json:"description"`
}

var exitCodeDocs = []exitCodeDoc{
	{0, "success"},
	{1, "validation errors found (validate) or runtime error"},
	{2, "usage error (missing args, unknown command)"},
}

func runSchema(args []string) {
	// `okf schema <command>` describes a single command.
	if len(args) > 0 && args[0] != "--json" && args[0] != "-j" {
		cmd := findSchemaCommand(args[0])
		if cmd == nil {
			fmt.Fprintf(os.Stderr, "error: unknown command %q\n", args[0])
			os.Exit(2)
		}
		outputJSON(cmd)
		return
	}
	outputJSON(buildSchemaRoot())
}

func buildSchemaRoot() schemaRoot {
	return schemaRoot{
		Name:        "okf",
		Version:     version,
		Description: "Go CLI toolkit for the Open Knowledge Format (OKF)",
		Commands:    allSchemaCommands(),
		ExitCodes:   exitCodeDocs,
	}
}

func allSchemaCommands() []schemaCommand {
	jsonFlag := schemaFlag{Name: "json", Short: "j", Type: "bool", Default: "false", Description: "output structured JSON instead of human-readable text"}
	return []schemaCommand{
		{
			Name:  "schema",
			Short: "Print machine-readable CLI metadata as JSON",
			Long:  "Outputs a JSON document describing every command, its flags, arguments, output format, and exit codes. Pass a command name to describe just that command.",
			Flags: nil,
			Args: []schemaArg{
				{Name: "command", Required: false},
			},
			Stdout:    "json",
			ExitCodes: []int{0, 2},
		},
		{
			Name:  "validate",
			Short: "Validate a bundle against the OKF spec",
			Long:  "Checks every concept for required frontmatter (type), recommended fields (title, description, tags), non-empty body, and valid cross-links.",
			Flags: []schemaFlag{jsonFlag},
			Args: []schemaArg{
				{Name: "bundle", Required: true},
			},
			Stdout:    "text|json",
			ExitCodes: []int{0, 1, 2},
		},
		{
			Name:  "lint",
			Short: "Check recommended fields and style (warnings only)",
			Long:  "Same checks as validate but only emits warnings — errors are suppressed. Exits 0 even with warnings.",
			Flags: []schemaFlag{jsonFlag},
			Args: []schemaArg{
				{Name: "bundle", Required: true},
			},
			Stdout:    "text|json",
			ExitCodes: []int{0, 2},
		},
		{
			Name:  "index",
			Short: "Generate index.md files (progressive disclosure)",
			Long:  "Writes index.md into every directory containing concept documents, providing progressive disclosure per OKF spec §6.",
			Flags: []schemaFlag{jsonFlag},
			Args: []schemaArg{
				{Name: "bundle", Required: true},
			},
			Stdout:    "text|json",
			ExitCodes: []int{0, 1, 2},
		},
		{
			Name:  "graph",
			Short: "Print cross-link graph statistics",
			Long:  "Builds the directed cross-link graph from concept markdown links and prints summary statistics (nodes, edges, isolated concepts, density).",
			Flags: []schemaFlag{jsonFlag},
			Args: []schemaArg{
				{Name: "bundle", Required: true},
			},
			Stdout:    "text|json",
			ExitCodes: []int{0, 1, 2},
		},
		{
			Name:  "list",
			Short: "List all concepts in the bundle",
			Long:  "Lists every concept document with its ID, type, and title.",
			Flags: []schemaFlag{jsonFlag},
			Args: []schemaArg{
				{Name: "bundle", Required: true},
			},
			Stdout:    "text|json",
			ExitCodes: []int{0, 1, 2},
		},
		{
			Name:      "version",
			Short:     "Print version",
			Flags:     nil,
			Args:      nil,
			Stdout:    "text",
			ExitCodes: []int{0},
		},
	}
}

func findSchemaCommand(name string) *schemaCommand {
	for _, c := range allSchemaCommands() {
		if c.Name == name {
			return &c
		}
	}
	return nil
}

// silence unused import warning for flag (used transitively for future expansion).
var _ = flag.NewFlagSet

// Command okf is a CLI toolkit for the Open Knowledge Format (OKF).
// It validates, lints, indexes, and inspects OKF bundles.
//
// okf is designed agentic-first: all output is JSON on stdout by default,
// `okf schema` emits a complete machine-readable description of every command,
// and all errors are emitted as JSON envelopes with stable exit codes.
// An external AI agent can discover and drive the entire CLI from that one
// command.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/okfcli/okf/internal/bundle"
	"github.com/okfcli/okf/internal/cerr"
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
		outputJSON(map[string]any{"name": "okf", "version": version})
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
		exitErr(cerr.Usage("unknown command: %s", cmd))
	}
}

const usage = `okf — Open Knowledge Format toolkit (v%s)

Usage:
  okf <command> <bundle-path>

All output is JSON on stdout. Diagnostics go to stderr.

Commands:
  schema [command]            Print machine-readable CLI metadata as JSON
  validate <bundle>           Validate a bundle against the OKF spec
  lint <bundle>               Check recommended fields and style (warnings only)
  index <bundle>              Generate index.md files (progressive disclosure)
  graph <bundle>              Print cross-link graph statistics
  list <bundle>               List all concepts in the bundle
  version                     Print version

Exit codes:
  0  success
  1  validation error (spec violation, broken link, bad input)
  2  filesystem or I/O error
  3  internal error (unexpected)
  4  usage error (missing args, unknown command)

OKF spec: https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md
`

func printUsage() {
	fmt.Fprintf(os.Stderr, usage, version)
}

// --- shared helpers ---

func outputJSON(v any) {
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: marshal: %v\n", err)
		os.Exit(cerr.ExitCodeInternal)
	}
	fmt.Println(string(out))
}

// exitErr prints a structured JSON error envelope to stdout and exits with
// the mapped exit code.
func exitErr(err error) {
	e := cerr.From(err)
	if e == nil {
		os.Exit(0)
	}
	b, _ := e.ToJSON()
	fmt.Println(string(b))
	fmt.Fprintf(os.Stderr, "error[%s]: %s\n", e.Kind, e.Message)
	os.Exit(e.ExitCode())
}

func mustBundle(args []string) *bundle.Bundle {
	if len(args) == 0 {
		exitErr(cerr.Usage("bundle path required"))
	}
	b, err := bundle.Load(args[0])
	if err != nil {
		exitErr(cerr.IO(err, "load bundle %s", args[0]))
	}
	return b
}

// --- validate / lint ---

func runValidate(args []string, strict bool) {
	if len(args) == 0 {
		exitErr(cerr.Usage("bundle path required"))
	}
	b, err := bundle.Load(args[0])
	if err != nil {
		exitErr(cerr.IO(err, "load bundle %s", args[0]))
	}
	r := validate.Validate(b)

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
		"command":  commandName(strict),
		"bundle":   b.Root,
		"findings": findings,
		"errors":   r.Errors,
		"warnings": r.Warnings,
		"valid":    !r.HasErrors(),
	})

	if strict && r.HasErrors() {
		os.Exit(cerr.ExitCodeValidation)
	}
}

func commandName(strict bool) string {
	if strict {
		return "validate"
	}
	return "lint"
}

// --- index ---

func runIndex(args []string) {
	if len(args) == 0 {
		exitErr(cerr.Usage("bundle path required"))
	}
	root := args[0]
	if err := index.Generate(root); err != nil {
		exitErr(cerr.IO(err, "generate index in %s", root))
	}

	// Collect generated index file paths.
	var indexFiles []string
	b, err := bundle.Load(root)
	if err == nil {
		for _, r := range b.Reserved {
			if r.ID == "index" || strings.HasSuffix(r.ID, "/index") {
				indexFiles = append(indexFiles, r.Path)
			}
		}
	}

	outputJSON(map[string]any{
		"command":         "index",
		"bundle":          root,
		"indexes_written": indexFiles,
		"count":           len(indexFiles),
	})
}

// --- graph ---

func runGraph(args []string) {
	b := mustBundle(args)
	g := graph.Build(b)
	s := g.Stats()

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
		"command":       "graph",
		"bundle":        b.Root,
		"nodes":         nodes,
		"edges":         edges,
		"node_count":    s.NodeCount,
		"edge_count":    s.EdgeCount,
		"isolated":      s.IsolatedNodes,
		"max_backlinks": s.MaxBacklinks,
		"density_pct":   density,
	})
}

// --- list ---

func runList(args []string) {
	b := mustBundle(args)

	concepts := make([]map[string]string, 0, len(b.Concepts))
	for _, c := range b.Concepts {
		concepts = append(concepts, map[string]string{
			"id":    c.ID,
			"type":  c.Frontmatter.Type,
			"title": c.Frontmatter.Title,
		})
	}

	outputJSON(map[string]any{
		"command":  "list",
		"bundle":   b.Root,
		"concepts": concepts,
		"count":    len(b.Concepts),
	})
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
	ExitCodes   []cerr.ExitCodeDoc `json:"exit_codes"`
}

func runSchema(args []string) {
	// `okf schema <command>` describes a single command.
	if len(args) > 0 {
		cmd := findSchemaCommand(args[0])
		if cmd == nil {
			exitErr(cerr.Usage("unknown command %q", args[0]))
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
		ExitCodes:   cerr.ExitCodeDocs,
	}
}

func allSchemaCommands() []schemaCommand {
	return []schemaCommand{
		{
			Name:   "schema",
			Short:  "Print machine-readable CLI metadata as JSON",
			Long:   "Outputs a JSON document describing every command, its flags, arguments, output format, and exit codes. Pass a command name to describe just that command.",
			Args:   []schemaArg{{Name: "command", Required: false}},
			Stdout: "json",
			ExitCodes: []int{cerr.ExitCodeOK, cerr.ExitCodeUsage},
		},
		{
			Name:   "validate",
			Short:  "Validate a bundle against the OKF spec",
			Long:   "Checks every concept for required frontmatter (type), recommended fields (title, description, tags), non-empty body, and valid cross-links. Exits 1 if any errors are found.",
			Args:   []schemaArg{{Name: "bundle", Required: true}},
			Stdout: "json",
			ExitCodes: []int{cerr.ExitCodeOK, cerr.ExitCodeValidation, cerr.ExitCodeIO, cerr.ExitCodeUsage},
		},
		{
			Name:   "lint",
			Short:  "Check recommended fields and style (warnings only)",
			Long:   "Same checks as validate but only emits warnings — errors are suppressed. Exits 0 even with warnings.",
			Args:   []schemaArg{{Name: "bundle", Required: true}},
			Stdout: "json",
			ExitCodes: []int{cerr.ExitCodeOK, cerr.ExitCodeIO, cerr.ExitCodeUsage},
		},
		{
			Name:   "index",
			Short:  "Generate index.md files (progressive disclosure)",
			Long:   "Writes index.md into every directory containing concept documents, providing progressive disclosure per OKF spec §6.",
			Args:   []schemaArg{{Name: "bundle", Required: true}},
			Stdout: "json",
			ExitCodes: []int{cerr.ExitCodeOK, cerr.ExitCodeIO, cerr.ExitCodeUsage},
		},
		{
			Name:   "graph",
			Short:  "Print cross-link graph statistics",
			Long:   "Builds the directed cross-link graph from concept markdown links and prints nodes, edges, and summary statistics.",
			Args:   []schemaArg{{Name: "bundle", Required: true}},
			Stdout: "json",
			ExitCodes: []int{cerr.ExitCodeOK, cerr.ExitCodeIO, cerr.ExitCodeUsage},
		},
		{
			Name:   "list",
			Short:  "List all concepts in the bundle",
			Long:   "Lists every concept document with its ID, type, and title.",
			Args:   []schemaArg{{Name: "bundle", Required: true}},
			Stdout: "json",
			ExitCodes: []int{cerr.ExitCodeOK, cerr.ExitCodeIO, cerr.ExitCodeUsage},
		},
		{
			Name:   "version",
			Short:  "Print version",
			Stdout: "json",
			ExitCodes: []int{cerr.ExitCodeOK},
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

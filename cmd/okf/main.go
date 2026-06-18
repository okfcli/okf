// Command okf is a CLI toolkit for the Open Knowledge Format (OKF).
// It validates, lints, indexes, and inspects OKF bundles.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/okfcli/okf/internal/bundle"
	"github.com/okfcli/okf/internal/enrich"
	"github.com/okfcli/okf/internal/graph"
	"github.com/okfcli/okf/internal/index"
	"github.com/okfcli/okf/internal/llm"
	"github.com/okfcli/okf/internal/source"
	"github.com/okfcli/okf/internal/validate"

	// Source drivers — register via init().
	_ "github.com/okfcli/okf/internal/source/postgres"
)

var version = "dev"

const usage = `okf — Open Knowledge Format toolkit (v%s)

Usage:
  okf <command> [flags] <bundle-path>

Commands:
  validate <bundle>           Validate a bundle against the OKF spec
  lint <bundle>               Check recommended fields and style (warnings only)
  index <bundle>              Generate index.md files (progressive disclosure)
  graph <bundle>              Print cross-link graph statistics
  list <bundle>               List all concepts in the bundle
  enrich --source <type>      Enrich a bundle from a data source using an LLM
    --dsn <conn-string>
    --out <bundle-dir>
    [--base-url <url>]        LLM API endpoint (default: https://api.openai.com/v1)
    [--api-key <key>]         LLM API key (or set OKF_API_KEY / OPENAI_API_KEY)
    [--model <model>]         LLM model name (default: gpt-4o)
    [--max <N>]               Cap number of concepts to enrich
    [--trace]                 Emit per-concept progress to stderr
  version                     Print version

Examples:
  okf validate ./bundles/ga4
  okf enrich --source postgres --dsn "postgres://user:***@localhost/mydb" --out ./bundles/mydb
  okf enrich --source postgres --dsn "$DATABASE_URL" --out ./bundles/mydb --model llama3.2 --base-url http://localhost:11434/v1

OKF spec: https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md
`

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Printf(usage, version)
		os.Exit(0)
	}

	cmd := args[0]
	rest := args[1:]

	switch cmd {
	case "version", "--version", "-v":
		fmt.Printf("okf %s\n", version)
		return
	case "help", "--help", "-h":
		fmt.Printf(usage, version)
		return
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
	case "enrich":
		runEnrich(rest)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		fmt.Printf(usage, version)
		os.Exit(2)
	}
}

func mustBundle(args []string) *bundle.Bundle {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "error: bundle path required")
		os.Exit(2)
	}
	b, err := bundle.Load(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	return b
}

func runValidate(args []string, strict bool) {
	b := mustBundle(args)
	r := validate.Validate(b)

	if len(r.Findings) == 0 {
		conceptCount := len(b.Concepts)
		fmt.Printf("✓ bundle valid — %d concepts, 0 errors, 0 warnings\n", conceptCount)
		return
	}

	for _, f := range r.Findings {
		if !strict && f.Severity == validate.SeverityError {
			continue // lint mode: warnings only
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

func runIndex(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "error: bundle path required")
		os.Exit(2)
	}
	if err := index.Generate(args[0]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ index.md files generated in %s\n", args[0])
}

func runGraph(args []string) {
	b := mustBundle(args)
	g := graph.Build(b)
	fmt.Print(g.Summary())
}

func runList(args []string) {
	b := mustBundle(args)
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

func runEnrich(args []string) {
	fs := newFlagSet("enrich")
	sourceName := fs.string("source", "", "source type (postgres, openapi)")
	dsn := fs.string("dsn", "", "source connection string")
	outDir := fs.string("out", "", "output bundle directory")
	baseURL := fs.string("base-url", "https://api.openai.com/v1", "LLM API endpoint")
	apiKey := fs.string("api-key", "", "LLM API key (or OKF_API_KEY / OPENAI_API_KEY)")
	model := fs.string("model", "gpt-4o", "LLM model name")
	maxConcepts := fs.int("max", 0, "max concepts to enrich (0 = no limit)")
	trace := fs.bool("trace", false, "emit per-concept progress to stderr")
	fs.parse(args)

	if *sourceName == "" {
		fmt.Fprintln(os.Stderr, "error: --source is required")
		fmt.Fprintf(os.Stderr, "available sources: %s\n", strings.Join(source.AvailableSources(), ", "))
		os.Exit(2)
	}
	if *dsn == "" {
		fmt.Fprintln(os.Stderr, "error: --dsn is required")
		os.Exit(2)
	}
	if *outDir == "" {
		fmt.Fprintln(os.Stderr, "error: --out is required")
		os.Exit(2)
	}

	// Resolve API key from flag or env.
	key := *apiKey
	if key == "" {
		key = os.Getenv("OKF_API_KEY")
	}
	if key == "" {
		key = os.Getenv("OPENAI_API_KEY")
	}

	// Open the source.
	src, err := source.Open(*sourceName, *dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: open source: %v\n", err)
		os.Exit(1)
	}

	// Create the LLM client.
	client := &llm.Client{
		BaseURL: *baseURL,
		APIKey:  key,
		Model:   *model,
	}

	// Run enrichment.
	report, err := enrich.Run(context.Background(), enrich.Options{
		Source:      src,
		ChatFn:      client.ChatFn(),
		OutDir:      *outDir,
		MaxConcepts: *maxConcepts,
		Trace:       *trace,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Output the report as JSON to stdout.
	out, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: marshal report: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(out))

	if report.Errors > 0 {
		os.Exit(1)
	}
}

// flagSet is a lightweight wrapper around flag.FlagSet for subcommand parsing.
type flagSet struct {
	fs *flag.FlagSet
}

func newFlagSet(name string) *flagSet {
	return &flagSet{fs: flag.NewFlagSet(name, flag.ExitOnError)}
}

func (f *flagSet) string(name, def, usage string) *string {
	return f.fs.String(name, def, usage)
}

func (f *flagSet) int(name string, def int, usage string) *int {
	return f.fs.Int(name, def, usage)
}

func (f *flagSet) bool(name string, def bool, usage string) *bool {
	return f.fs.Bool(name, def, usage)
}

func (f *flagSet) parse(args []string) {
	f.fs.Parse(args)
}

// Command okf is a CLI toolkit for the Open Knowledge Format (OKF).
// It validates, lints, indexes, and inspects OKF bundles.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/okfcli/okf/internal/bundle"
	"github.com/okfcli/okf/internal/graph"
	"github.com/okfcli/okf/internal/index"
	"github.com/okfcli/okf/internal/validate"
)

var version = "dev"

const usage = `okf — Open Knowledge Format toolkit (v%s)

Usage:
  okf <command> [flags] <bundle-path>

Commands:
  validate <bundle>     Validate a bundle against the OKF spec (required fields, link integrity)
  lint <bundle>         Check recommended fields and style (warnings only)
  index <bundle>        Generate index.md files in every directory (progressive disclosure)
  graph <bundle>        Print cross-link graph statistics
  list <bundle>         List all concepts in the bundle
  version               Print version

Examples:
  okf validate ./bundles/ga4
  okf lint ./my-bundle
  okf index ./my-bundle
  okf graph ./bundles/stackoverflow

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

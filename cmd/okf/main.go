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
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/okfcli/okf/internal/backlinks"
	"github.com/okfcli/okf/internal/bundle"
	"github.com/okfcli/okf/internal/cerr"
	"github.com/okfcli/okf/internal/graph"
	"github.com/okfcli/okf/internal/index"
	"github.com/okfcli/okf/internal/initbundle"
	"github.com/okfcli/okf/internal/search"
	"github.com/okfcli/okf/internal/show"
	"github.com/okfcli/okf/internal/validate"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

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
		outputJSON(map[string]any{"name": "okf", "version": version, "commit": commit, "date": date})
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
	case "show":
		runShow(rest)
	case "search":
		runSearch(rest)
	case "init":
		runInit(rest)
	case "backlinks":
		runBacklinks(rest)
	default:
		exitErr(cerr.Usage("unknown command: %s", cmd))
	}
}

const usage = `okf - Open Knowledge Format toolkit (v%s)

Usage:
  okf <command> <bundle-path>

All output is JSON on stdout. Diagnostics go to stderr.

Commands:
  schema [command]            Print machine-readable CLI metadata as JSON
  init <bundle>               Create a new empty OKF bundle
  validate <bundle>           Validate a bundle against the OKF spec
  lint <bundle>               Check recommended fields and style (warnings only)
  index <bundle>              Generate index.md files (progressive disclosure)
  list <bundle>               List all concepts in the bundle
  show <bundle> <concept-id> Show a single concept's full content
  search <bundle> [filters]  Search concepts by tag, type, or text
  backlinks <bundle> <id>    List concepts that link to a given concept
  graph <bundle>             Print cross-link graph statistics
  version                     Print version

Exit codes:
  0  success
  1  validation error (spec violation, bad input)
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

// rejectFlags returns a usage error when any argument looks like a flag.
// Commands that take no flags call it so a mistyped or unsupported flag is
// reported as a usage mistake rather than being treated as a bundle path
// and failing as I/O (issue #31).
func rejectFlags(cmd string, args []string) *cerr.Error {
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			return cerr.Usage("unknown %s flag: %s", cmd, a)
		}
	}
	return nil
}

// loadBundle loads the bundle at path and maps loader failures onto the
// error kinds the CLI documents. A .md file that is not a concept violates
// OKF §11, so it is a validation error that names the file (issue #27);
// anything else is a filesystem error.
func loadBundle(path string) (*bundle.Bundle, *cerr.Error) {
	b, err := bundle.Load(path)
	if err == nil {
		return b, nil
	}
	var pe *bundle.ParseError
	if errors.As(err, &pe) {
		e := cerr.Validation("load bundle %s: %v", path, err)
		e.Hint = "every non-reserved .md file in a bundle needs a YAML frontmatter block (OKF §11); add frontmatter to " + pe.Path + " or move it out of the bundle"
		return nil, e
	}
	return nil, cerr.IO(err, "load bundle %s", path)
}

func mustBundle(cmd string, args []string) *bundle.Bundle {
	if err := rejectFlags(cmd, args); err != nil {
		exitErr(err)
	}
	if len(args) == 0 {
		exitErr(cerr.Usage("bundle path required"))
	}
	b, err := loadBundle(args[0])
	if err != nil {
		exitErr(err)
	}
	return b
}

// --- validate / lint ---

func runValidate(args []string, strict bool) {
	// Flags may appear before or after the bundle path
	// (okf validate --format sarif bundle, okf validate bundle --format sarif).
	bundlePath := ""
	format := "json"
	exitZero := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--format":
			if i+1 >= len(args) {
				exitErr(cerr.Usage("--format requires a value"))
			}
			format = args[i+1]
			i++
		case "--exit-zero":
			// lint always exits 0, so the flag only exists on validate.
			if !strict {
				exitErr(cerr.Usage("unknown lint flag: --exit-zero"))
			}
			exitZero = true
		default:
			if strings.HasPrefix(args[i], "-") {
				exitErr(cerr.Usage("unknown %s flag: %s", commandName(strict), args[i]))
			}
			if bundlePath != "" {
				exitErr(cerr.Usage("unexpected argument: %s", args[i]))
			}
			bundlePath = args[i]
		}
	}
	if bundlePath == "" {
		exitErr(cerr.Usage("bundle path required"))
	}
	if format != "json" && format != "sarif" {
		exitErr(cerr.Usage("--format must be json or sarif, got %q", format))
	}

	b, lerr := loadBundle(bundlePath)
	if lerr != nil {
		exitErr(lerr)
	}
	r := validate.Validate(b)

	// lint reports warnings only; validate reports everything.
	kept := make([]validate.Finding, 0, len(r.Findings))
	for _, f := range r.Findings {
		if !strict && f.Severity == validate.SeverityError {
			continue
		}
		kept = append(kept, f)
	}

	if format == "sarif" {
		out, err := validate.SARIF(kept, version)
		if err != nil {
			exitErr(cerr.Internal(err, "marshal sarif"))
		}
		fmt.Println(string(out))
	} else {
		findings := make([]map[string]any, 0, len(kept))
		for _, f := range kept {
			findings = append(findings, map[string]any{
				"concept_id": f.ConceptID,
				"rule":       f.RuleID,
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
	}

	if strict && r.HasErrors() && !exitZero {
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
	if err := rejectFlags("index", args); err != nil {
		exitErr(err)
	}
	if len(args) == 0 {
		exitErr(cerr.Usage("bundle path required"))
	}
	root := args[0]
	// Load before writing anything so a bundle that does not parse (issue
	// #27) is reported by name instead of being half-indexed and then
	// silently miscounted below.
	if _, lerr := loadBundle(root); lerr != nil {
		exitErr(lerr)
	}
	if err := index.Generate(root); err != nil {
		exitErr(cerr.IO(err, "generate index in %s", root))
	}

	// Collect generated index file paths.
	var indexFiles []string
	b, err := bundle.Load(root)
	if err != nil {
		exitErr(cerr.IO(err, "reload bundle %s after indexing", root))
	}
	for _, r := range b.Reserved {
		if r.ID == "index" || strings.HasSuffix(r.ID, "/index") {
			indexFiles = append(indexFiles, r.Path)
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
	b := mustBundle("graph", args)
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
	b := mustBundle("list", args)

	concepts := make([]map[string]string, 0, len(b.Concepts))
	for _, c := range b.Concepts {
		concepts = append(concepts, map[string]string{
			"id":         c.ID,
			"type":       c.Frontmatter.Type,
			"title":      c.Frontmatter.Title,
			"status":     c.Frontmatter.EffectiveStatus(),
			"trust_tier": c.Frontmatter.TrustTier(),
		})
	}

	outputJSON(map[string]any{
		"command":  "list",
		"bundle":   b.Root,
		"concepts": concepts,
		"count":    len(b.Concepts),
	})
}

// --- show ---

func runShow(args []string) {
	if err := rejectFlags("show", args); err != nil {
		exitErr(err)
	}
	if len(args) < 2 {
		exitErr(cerr.Usage("usage: okf show <bundle> <concept-id>"))
	}
	b := mustBundle("show", args[:1])
	c, err := show.Show(b, args[1])
	if err != nil {
		exitErr(cerr.Validation("%s", err))
	}

	var tags []string
	if c.Frontmatter.Tags != nil {
		tags = c.Frontmatter.Tags
	}

	fm := c.Frontmatter
	out := map[string]any{
		"id":          c.ID,
		"path":        c.Path,
		"type":        fm.Type,
		"title":       fm.Title,
		"description": fm.Description,
		"resource":    fm.Resource,
		"tags":        tags,
		"body":        c.Body,
		// Trust and lifecycle (OKF v0.2 §5): always present so agents can
		// branch on them without probing for key existence.
		"status":     fm.EffectiveStatus(),
		"trust_tier": fm.TrustTier(),
		"stale":      fm.IsStale(time.Now()),
	}
	if fm.Generated != nil {
		out["generated"] = map[string]any{"by": fm.Generated.By, "at": fm.Generated.At}
	}
	if len(fm.Verified) > 0 {
		verified := make([]map[string]any, 0, len(fm.Verified))
		for _, v := range fm.Verified {
			verified = append(verified, map[string]any{"by": v.By, "at": v.At})
		}
		out["verified"] = verified
	}
	if fm.StaleAfter != "" {
		out["stale_after"] = fm.StaleAfter
	}
	if len(fm.Sources) > 0 {
		sources := make([]map[string]any, 0, len(fm.Sources))
		for _, s := range fm.Sources {
			src := map[string]any{"resource": s.Resource}
			if s.ID != "" {
				src["id"] = s.ID
			}
			if s.Title != "" {
				src["title"] = s.Title
			}
			if s.Author != "" {
				src["author"] = s.Author
			}
			if s.UsageCount != nil {
				src["usage_count"] = *s.UsageCount
			}
			if s.LastModified != "" {
				src["last_modified"] = s.LastModified
			}
			if s.UsageWindow != nil {
				src["usage_window"] = map[string]string{"from": s.UsageWindow.From, "to": s.UsageWindow.To}
			}
			sources = append(sources, src)
		}
		out["sources"] = sources
	}
	if fm.UsageWindow != nil {
		out["usage_window"] = map[string]string{"from": fm.UsageWindow.From, "to": fm.UsageWindow.To}
	}
	// Attested Computation contract (OKF v0.2 §10).
	if fm.Runtime != "" {
		out["runtime"] = fm.Runtime
	}
	if len(fm.Parameters) > 0 {
		params := make([]map[string]any, 0, len(fm.Parameters))
		for _, p := range fm.Parameters {
			params = append(params, map[string]any{"name": p.Name, "type": p.Type, "required": p.Required})
		}
		out["parameters"] = params
	}
	if fm.Computation != "" {
		out["computation"] = fm.Computation
	}
	if fm.Executor != nil {
		out["executor"] = map[string]any{"resource": fm.Executor.Resource, "receipt": fm.Executor.Receipt}
	}
	if fm.Attester != nil {
		out["attester"] = map[string]any{"resource": fm.Attester.Resource}
	}

	outputJSON(map[string]any{
		"command": "show",
		"bundle":  b.Root,
		"concept": out,
	})
}

// --- search ---

func runSearch(args []string) {
	if len(args) == 0 {
		exitErr(cerr.Usage("usage: okf search <bundle> [--tag <tag>] [--type <type>] [--text <query>]"))
	}
	if err := rejectFlags("search", args[:1]); err != nil {
		exitErr(err)
	}
	bundlePath := args[0]
	rest := args[1:]

	f := search.Filters{}
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--tag":
			if i+1 >= len(rest) {
				exitErr(cerr.Usage("--tag requires a value"))
			}
			f.Tag = rest[i+1]
			i++
		case "--type":
			if i+1 >= len(rest) {
				exitErr(cerr.Usage("--type requires a value"))
			}
			f.Type = rest[i+1]
			i++
		case "--text":
			if i+1 >= len(rest) {
				exitErr(cerr.Usage("--text requires a value"))
			}
			f.Text = rest[i+1]
			i++
		default:
			exitErr(cerr.Usage("unknown search flag: %s", rest[i]))
		}
	}

	b, lerr := loadBundle(bundlePath)
	if lerr != nil {
		exitErr(lerr)
	}

	results := search.Search(b, f)
	concepts := make([]map[string]string, 0, len(results))
	for _, c := range results {
		concepts = append(concepts, map[string]string{
			"id":    c.ID,
			"type":  c.Frontmatter.Type,
			"title": c.Frontmatter.Title,
		})
	}

	outputJSON(map[string]any{
		"command": "search",
		"bundle":  b.Root,
		"filters": f,
		"results": concepts,
		"count":   len(results),
	})
}

// --- init ---

func runInit(args []string) {
	if err := rejectFlags("init", args); err != nil {
		exitErr(err)
	}
	if len(args) == 0 {
		exitErr(cerr.Usage("usage: okf init <bundle-path>"))
	}
	dir := args[0]
	if err := initbundle.Create(dir); err != nil {
		exitErr(cerr.IO(err, "create bundle %s", dir))
	}

	var created []string
	for _, sub := range []string{"tables", "datasets", "playbooks"} {
		created = append(created, sub+"/")
	}

	outputJSON(map[string]any{
		"command":   "init",
		"bundle":    dir,
		"created":   created,
		"index":     "index.md",
		"gitignore": ".gitignore",
	})
}

// --- backlinks ---

func runBacklinks(args []string) {
	if err := rejectFlags("backlinks", args); err != nil {
		exitErr(err)
	}
	if len(args) < 2 {
		exitErr(cerr.Usage("usage: okf backlinks <bundle> <concept-id>"))
	}
	b := mustBundle("backlinks", args[:1])
	conceptID := args[1]
	links := backlinks.Backlinks(b, conceptID)

	if links == nil {
		links = []string{} // emit empty array, not null
	}

	outputJSON(map[string]any{
		"command":    "backlinks",
		"bundle":     b.Root,
		"concept_id": conceptID,
		"backlinks":  links,
		"count":      len(links),
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
	Name        string             `json:"name"`
	Version     string             `json:"version"`
	Description string             `json:"description"`
	Commands    []schemaCommand    `json:"commands"`
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
			Name:      "schema",
			Short:     "Print machine-readable CLI metadata as JSON",
			Long:      "Outputs a JSON document describing every command, its flags, arguments, output format, and exit codes. Pass a command name to describe just that command.",
			Args:      []schemaArg{{Name: "command", Required: false}},
			Stdout:    "json",
			ExitCodes: []int{cerr.ExitCodeOK, cerr.ExitCodeUsage},
		},
		{
			Name:      "init",
			Short:     "Create a new empty OKF bundle",
			Long:      "Creates a bundle directory with standard subdirectories (tables, datasets, playbooks), a root index.md, and a .gitignore. Fails if the directory already exists.",
			Args:      []schemaArg{{Name: "bundle", Required: true}},
			Stdout:    "json",
			ExitCodes: []int{cerr.ExitCodeOK, cerr.ExitCodeValidation, cerr.ExitCodeIO, cerr.ExitCodeUsage},
		},
		{
			Name:  "validate",
			Short: "Validate a bundle against the OKF spec",
			Long:  "Checks every concept against OKF v0.2: required frontmatter (type), recommended fields (title, description, tags), non-empty body, valid cross-links, the provenance/trust/lifecycle families (sources, generated, verified, status, stale_after), the Attested Computation contract (runtime, parameters, computation, executor, attester), reserved-file structure (index.md, log.md), and legacy v0.1 constructs (timestamp, # Citations). Every finding carries a stable rule ID (okf/<family>/<check>). With --format sarif, stdout is a SARIF 2.1.0 document for CI code scanning. Exits 1 if any errors are found, unless --exit-zero is set.",
			Flags: []schemaFlag{
				{Name: "format", Type: "string", Default: "json", Description: "output format: json or sarif (SARIF 2.1.0)"},
				{Name: "exit-zero", Type: "bool", Default: "false", Description: "always exit 0, even when errors are found (for CI steps that upload findings after the run)"},
			},
			Args:      []schemaArg{{Name: "bundle", Required: true}},
			Stdout:    "json|sarif",
			ExitCodes: []int{cerr.ExitCodeOK, cerr.ExitCodeValidation, cerr.ExitCodeIO, cerr.ExitCodeUsage},
		},
		{
			Name:  "lint",
			Short: "Check recommended fields and style (warnings only)",
			Long:  "Same checks as validate but only emits warnings - errors are suppressed. Findings carry the same stable rule IDs, and --format sarif emits SARIF 2.1.0. Exits 0 even with warnings.",
			Flags: []schemaFlag{
				{Name: "format", Type: "string", Default: "json", Description: "output format: json or sarif (SARIF 2.1.0)"},
			},
			Args:      []schemaArg{{Name: "bundle", Required: true}},
			Stdout:    "json|sarif",
			ExitCodes: []int{cerr.ExitCodeOK, cerr.ExitCodeValidation, cerr.ExitCodeIO, cerr.ExitCodeUsage},
		},
		{
			Name:      "index",
			Short:     "Generate index.md files (progressive disclosure)",
			Long:      "Writes index.md into every directory containing concept documents, providing progressive disclosure per OKF spec §6.",
			Args:      []schemaArg{{Name: "bundle", Required: true}},
			Stdout:    "json",
			ExitCodes: []int{cerr.ExitCodeOK, cerr.ExitCodeValidation, cerr.ExitCodeIO, cerr.ExitCodeUsage},
		},
		{
			Name:      "list",
			Short:     "List all concepts in the bundle",
			Long:      "Lists every concept document with its ID, type, title, lifecycle status, and trust tier (unverified, machine-confirmed, or human-reviewed).",
			Args:      []schemaArg{{Name: "bundle", Required: true}},
			Stdout:    "json",
			ExitCodes: []int{cerr.ExitCodeOK, cerr.ExitCodeValidation, cerr.ExitCodeIO, cerr.ExitCodeUsage},
		},
		{
			Name:  "show",
			Short: "Show a single concept's full content",
			Long:  "Returns the concept's ID, file path, frontmatter (type, title, description, resource, tags), trust and lifecycle state (status, trust_tier, stale, generated, verified, stale_after), provenance (sources, usage_window), the Attested Computation contract when present (runtime, parameters, computation, executor, attester), and markdown body as JSON.",
			Args: []schemaArg{
				{Name: "bundle", Required: true},
				{Name: "concept-id", Required: true},
			},
			Stdout:    "json",
			ExitCodes: []int{cerr.ExitCodeOK, cerr.ExitCodeValidation, cerr.ExitCodeIO, cerr.ExitCodeUsage},
		},
		{
			Name:  "search",
			Short: "Search concepts by tag, type, or text",
			Long:  "Filters concepts in a bundle by tag (--tag), frontmatter type (--type), or full-text search in title, description, and body (--text). Multiple filters are AND-combined. With no filters, returns all concepts.",
			Flags: []schemaFlag{
				{Name: "tag", Type: "string", Default: "", Description: "filter by tag (case-insensitive)"},
				{Name: "type", Type: "string", Default: "", Description: "filter by frontmatter type (case-insensitive)"},
				{Name: "text", Type: "string", Default: "", Description: "full-text search in title, description, and body (case-insensitive)"},
			},
			Args: []schemaArg{
				{Name: "bundle", Required: true},
			},
			Stdout:    "json",
			ExitCodes: []int{cerr.ExitCodeOK, cerr.ExitCodeValidation, cerr.ExitCodeIO, cerr.ExitCodeUsage},
		},
		{
			Name:  "backlinks",
			Short: "List concepts that link to a given concept",
			Long:  "Returns the IDs of all concepts in the bundle that contain a markdown link to the specified concept. Deduplicates multiple links from the same source.",
			Args: []schemaArg{
				{Name: "bundle", Required: true},
				{Name: "concept-id", Required: true},
			},
			Stdout:    "json",
			ExitCodes: []int{cerr.ExitCodeOK, cerr.ExitCodeValidation, cerr.ExitCodeIO, cerr.ExitCodeUsage},
		},
		{
			Name:      "graph",
			Short:     "Print cross-link graph statistics",
			Long:      "Builds the directed cross-link graph from concept markdown links and prints nodes, edges, and summary statistics.",
			Args:      []schemaArg{{Name: "bundle", Required: true}},
			Stdout:    "json",
			ExitCodes: []int{cerr.ExitCodeOK, cerr.ExitCodeValidation, cerr.ExitCodeIO, cerr.ExitCodeUsage},
		},
		{
			Name:      "version",
			Short:     "Print version",
			Stdout:    "json",
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

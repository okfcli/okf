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

	"github.com/okfcli/okf/internal/backlinks"
	"github.com/okfcli/okf/internal/bundle"
	"github.com/okfcli/okf/internal/cerr"
	"github.com/okfcli/okf/internal/export"
	"github.com/okfcli/okf/internal/graph"
	"github.com/okfcli/okf/internal/index"
	"github.com/okfcli/okf/internal/initbundle"
	"github.com/okfcli/okf/internal/search"
	"github.com/okfcli/okf/internal/show"
	"github.com/okfcli/okf/internal/sign"
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
	case "export":
		runExport(rest)
	case "sign":
		runSign(rest)
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
  init <bundle>               Create a new empty OKF bundle
  validate <bundle>           Validate a bundle against the OKF spec
  lint <bundle>               Check recommended fields and style (warnings only)
  index <bundle>              Generate index.md files (progressive disclosure)
  list <bundle>               List all concepts in the bundle
  show <bundle> <concept-id> Show a single concept's full content
  search <bundle> [filters]  Search concepts by tag, type, or text
  backlinks <bundle> <id>    List concepts that link to a given concept
  graph <bundle>             Print cross-link graph statistics
  export <bundle> [-o file]  Export entire bundle as a .okf tar.gz archive
  sign <archive> <action>    Post-quantum sign/verify with ML-KEM-768 via HPKE
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

// --- show ---

func runShow(args []string) {
	if len(args) < 2 {
		exitErr(cerr.Usage("usage: okf show <bundle> <concept-id>"))
	}
	b := mustBundle(args[:1])
	c, err := show.Show(b, args[1])
	if err != nil {
		exitErr(cerr.Validation("%s", err))
	}

	var tags []string
	if c.Frontmatter.Tags != nil {
		tags = c.Frontmatter.Tags
	}

	outputJSON(map[string]any{
		"command": "show",
		"bundle":  b.Root,
		"concept": map[string]any{
			"id":          c.ID,
			"path":        c.Path,
			"type":        c.Frontmatter.Type,
			"title":       c.Frontmatter.Title,
			"description": c.Frontmatter.Description,
			"resource":    c.Frontmatter.Resource,
			"tags":        tags,
			"body":        c.Body,
		},
	})
}

// --- search ---

func runSearch(args []string) {
	if len(args) == 0 {
		exitErr(cerr.Usage("usage: okf search <bundle> [--tag <tag>] [--type <type>] [--text <query>]"))
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

	b, err := bundle.Load(bundlePath)
	if err != nil {
		exitErr(cerr.IO(err, "load bundle %s", bundlePath))
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
		"command":  "search",
		"bundle":   b.Root,
		"filters":  f,
		"results":  concepts,
		"count":    len(results),
	})
}

// --- init ---

func runInit(args []string) {
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
	if len(args) < 2 {
		exitErr(cerr.Usage("usage: okf backlinks <bundle> <concept-id>"))
	}
	b := mustBundle(args[:1])
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

// --- export ---

func runExport(args []string) {
	if len(args) == 0 {
		exitErr(cerr.Usage("usage: okf export <bundle> [-o <output-file>]"))
	}
	bundlePath := args[0]
	outPath := bundlePath + ".okf"
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-o", "--output":
			if i+1 >= len(args) {
				exitErr(cerr.Usage("-o requires a value"))
			}
			outPath = args[i+1]
			i++
		default:
			exitErr(cerr.Usage("unknown export flag: %s", args[i]))
		}
	}

	manifest, err := export.Archive(bundlePath, outPath)
	if err != nil {
		exitErr(cerr.IO(err, "export bundle %s", bundlePath))
	}

	manifestJSON, err := export.ManifestToJSON(manifest)
	if err != nil {
		exitErr(cerr.Internal(err, "marshal manifest"))
	}

	outputJSON(map[string]any{
		"command":  "export",
		"bundle":   manifest.Bundle,
		"archive":  manifest.Archive,
		"manifest": json.RawMessage(manifestJSON),
	})
}

// --- sign ---

func runSign(args []string) {
	if len(args) == 0 {
		exitErr(cerr.Usage("usage: okf sign <archive> <keygen|sign|verify> [options]"))
	}
	archivePath := args[0]
	if len(args) < 2 {
		exitErr(cerr.Usage("usage: okf sign <archive> <keygen|sign|verify> [options]"))
	}
	action := args[1]
	rest := args[2:]

	switch action {
	case "keygen":
		runSignKeygen()
	case "sign":
		runSignSign(archivePath, rest)
	case "verify":
		runSignVerify(archivePath, rest)
	default:
		exitErr(cerr.Usage("unknown sign action: %s (use keygen, sign, or verify)", action))
	}
}

func runSignKeygen() {
	kp, err := sign.GenerateKeyPair()
	if err != nil {
		exitErr(cerr.Internal(err, "generate key pair"))
	}

	kpJSON, err := sign.KeyPairToJSON(kp)
	if err != nil {
		exitErr(cerr.Internal(err, "marshal key pair"))
	}

	outputJSON(map[string]any{
		"command":  "sign",
		"action":   "keygen",
		"algorithm": "ML-KEM-768",
		"keypair":   json.RawMessage(kpJSON),
	})
}

func runSignSign(archivePath string, rest []string) {
	var pubKeyHex, sigOutPath string
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--pub", "--public-key":
			if i+1 >= len(rest) {
				exitErr(cerr.Usage("--pub requires a value"))
			}
			pubKeyHex = rest[i+1]
			i++
		case "-o", "--output":
			if i+1 >= len(rest) {
				exitErr(cerr.Usage("-o requires a value"))
			}
			sigOutPath = rest[i+1]
			i++
		default:
			exitErr(cerr.Usage("unknown sign flag: %s", rest[i]))
		}
	}
	if pubKeyHex == "" {
		exitErr(cerr.Usage("usage: okf sign <archive> sign --pub <public-key-hex> [-o <sig.json>]"))
	}

	sig, err := sign.Sign(archivePath, pubKeyHex)
	if err != nil {
		exitErr(cerr.Internal(err, "sign archive %s", archivePath))
	}

	sigJSON, err := sign.SignatureToJSON(sig)
	if err != nil {
		exitErr(cerr.Internal(err, "marshal signature"))
	}

	// If -o is given, write the signature to a file for later verification.
	if sigOutPath != "" {
		if err := os.WriteFile(sigOutPath, sigJSON, 0o644); err != nil {
			exitErr(cerr.IO(err, "write signature %s", sigOutPath))
		}
	}

	outputJSON(map[string]any{
		"command":   "sign",
		"action":    "sign",
		"archive":   archivePath,
		"signature": json.RawMessage(sigJSON),
	})
}

func runSignVerify(archivePath string, rest []string) {
	var privKeyHex, sigPath string
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--priv", "--private-key":
			if i+1 >= len(rest) {
				exitErr(cerr.Usage("--priv requires a value"))
			}
			privKeyHex = rest[i+1]
			i++
		case "--sig", "--signature":
			if i+1 >= len(rest) {
				exitErr(cerr.Usage("--sig requires a value"))
			}
			sigPath = rest[i+1]
			i++
		default:
			exitErr(cerr.Usage("unknown verify flag: %s", rest[i]))
		}
	}
	if privKeyHex == "" {
		exitErr(cerr.Usage("usage: okf sign <archive> verify --priv <private-key-hex> --sig <signature.json>"))
	}
	if sigPath == "" {
		exitErr(cerr.Usage("usage: okf sign <archive> verify --priv <private-key-hex> --sig <signature.json>"))
	}

	sigData, err := os.ReadFile(sigPath)
	if err != nil {
		exitErr(cerr.IO(err, "read signature %s", sigPath))
	}

	var sig sign.Signature
	if err := json.Unmarshal(sigData, &sig); err != nil {
		exitErr(cerr.Validation("parse signature: %s", err))
	}

	if err := sign.Verify(archivePath, &sig, privKeyHex); err != nil {
		exitErr(cerr.Validation("%s", err))
	}

	outputJSON(map[string]any{
		"command":  "sign",
		"action":   "verify",
		"archive":  archivePath,
		"verified": true,
		"algorithm": sig.Algorithm,
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
			Name:   "init",
			Short:  "Create a new empty OKF bundle",
			Long:   "Creates a bundle directory with standard subdirectories (tables, datasets, playbooks), a root index.md, and a .gitignore. Fails if the directory already exists.",
			Args:   []schemaArg{{Name: "bundle", Required: true}},
			Stdout: "json",
			ExitCodes: []int{cerr.ExitCodeOK, cerr.ExitCodeIO, cerr.ExitCodeUsage},
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
			Name:   "list",
			Short:  "List all concepts in the bundle",
			Long:   "Lists every concept document with its ID, type, and title.",
			Args:   []schemaArg{{Name: "bundle", Required: true}},
			Stdout: "json",
			ExitCodes: []int{cerr.ExitCodeOK, cerr.ExitCodeIO, cerr.ExitCodeUsage},
		},
		{
			Name:   "show",
			Short:  "Show a single concept's full content",
			Long:   "Returns the concept's ID, file path, frontmatter (type, title, description, resource, tags), and markdown body as JSON.",
			Args: []schemaArg{
				{Name: "bundle", Required: true},
				{Name: "concept-id", Required: true},
			},
			Stdout: "json",
			ExitCodes: []int{cerr.ExitCodeOK, cerr.ExitCodeValidation, cerr.ExitCodeIO, cerr.ExitCodeUsage},
		},
		{
			Name:   "search",
			Short:  "Search concepts by tag, type, or text",
			Long:   "Filters concepts in a bundle by tag (--tag), frontmatter type (--type), or full-text search in title, description, and body (--text). Multiple filters are AND-combined. With no filters, returns all concepts.",
			Flags: []schemaFlag{
				{Name: "tag", Type: "string", Default: "", Description: "filter by tag (case-insensitive)"},
				{Name: "type", Type: "string", Default: "", Description: "filter by frontmatter type (case-insensitive)"},
				{Name: "text", Type: "string", Default: "", Description: "full-text search in title, description, and body (case-insensitive)"},
			},
			Args: []schemaArg{
				{Name: "bundle", Required: true},
			},
			Stdout: "json",
			ExitCodes: []int{cerr.ExitCodeOK, cerr.ExitCodeIO, cerr.ExitCodeUsage},
		},
		{
			Name:   "backlinks",
			Short:  "List concepts that link to a given concept",
			Long:   "Returns the IDs of all concepts in the bundle that contain a markdown link to the specified concept. Deduplicates multiple links from the same source.",
			Args: []schemaArg{
				{Name: "bundle", Required: true},
				{Name: "concept-id", Required: true},
			},
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
			Name:  "export",
			Short: "Export entire bundle as a .okf tar.gz archive",
			Long:  "Creates a deterministic tar.gz archive of every file in the bundle (sorted by path for reproducibility). Outputs a manifest with per-file SHA-256 hashes and total archive hash.",
			Flags: []schemaFlag{
				{Name: "output", Short: "o", Type: "string", Default: "<bundle>.okf", Description: "output archive path"},
			},
			Args: []schemaArg{{Name: "bundle", Required: true}},
			Stdout: "json",
			ExitCodes: []int{cerr.ExitCodeOK, cerr.ExitCodeIO, cerr.ExitCodeUsage},
		},
		{
			Name:  "sign",
			Short: "Post-quantum sign/verify archives with ML-KEM-768 via HPKE",
			Long:  "Subcommands: 'keygen' generates an ML-KEM-768 key pair. 'sign' seals the archive hash with HPKE using the public key. 'verify' opens the HPKE ciphertext with the private key and confirms the archive hash matches. Uses crypto/hpke with ML-KEM-768 (FIPS 203) — no external dependencies.",
			Args: []schemaArg{
				{Name: "archive", Required: true},
				{Name: "action", Required: true},
			},
			Flags: []schemaFlag{
				{Name: "pub", Type: "string", Default: "", Description: "hex-encoded ML-KEM-768 public key (for sign)"},
				{Name: "priv", Type: "string", Default: "", Description: "hex-encoded ML-KEM-768 private key (for verify)"},
				{Name: "sig", Type: "string", Default: "", Description: "path to signature JSON file (for verify)"},
			},
			Stdout: "json",
			ExitCodes: []int{cerr.ExitCodeOK, cerr.ExitCodeValidation, cerr.ExitCodeInternal, cerr.ExitCodeIO, cerr.ExitCodeUsage},
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

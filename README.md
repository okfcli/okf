# okf

A Go CLI toolkit for the [Open Knowledge Format (OKF)](https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md) — a vendor-neutral format for representing data catalog knowledge as plain markdown files with YAML frontmatter.

`okf` validates, lints, indexes, and inspects OKF bundles. One static binary, no runtime dependencies, fast enough to validate millions of concepts.

## Agentic-first design

`okf` is designed to be driven by external AI agents, not to call AI internally. Three mechanisms make this possible:

1. **`okf schema`** — emits a complete machine-readable description of every command: its name, description, flags, arguments, output format, and exit codes. An AI agent runs this once and knows the entire CLI surface.

2. **JSON by default** — all output is structured JSON on stdout. No `--json` flag needed, no screen-scraping. Diagnostics go to stderr.

3. **Structured error envelopes** — all errors emit as `{"error": {"kind":..., "code":..., "reason":..., "message":...}}` on stdout with a stable exit code.

```bash
# An AI agent discovers the CLI:
okf schema

# Then drives it — JSON on stdout, always:
okf validate ./my-bundle
okf list ./my-bundle
okf graph ./my-bundle
```

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | success |
| 1 | validation error (spec violation, broken link, bad input) |
| 2 | filesystem or I/O error |
| 3 | internal error (unexpected) |
| 4 | usage error (missing args, unknown command) |

Errors are emitted as JSON on stdout:

```json
{
  "error": {
    "kind": "io",
    "code": 500,
    "reason": "ioError",
    "message": "load bundle /nonexistent"
  }
}
```

## Install

```bash
go install github.com/okfcli/okf/cmd/okf@latest
```

Or build from source:

```bash
git clone https://github.com/okfcli/okf.git
cd okf
make build
# binary at bin/okf
```

## Usage

```bash
okf schema                     # machine-readable CLI metadata (JSON)
okf schema validate            # describe a single command (JSON)
okf validate ./bundles/ga4     # validate against the OKF spec (JSON)
okf lint ./my-bundle           # warnings only (JSON)
okf index ./my-bundle          # generate index.md files (JSON)
okf graph ./bundles/so         # cross-link graph statistics (JSON)
okf list ./my-bundle           # list all concepts (JSON)
okf version                    # version (JSON)
```

## Commands

### `okf schema [command]`

Prints machine-readable CLI metadata as JSON. With no argument, describes every command. With a command name, describes just that command. This is the entry point for AI agents discovering the CLI.

### `okf validate <bundle>`

Checks a bundle against the OKF spec:
- **Required fields**: every concept must have a `type` in its frontmatter (OKF §4.1)
- **Link integrity**: all cross-links (`[text](/path/to/concept.md)`) must resolve to an existing concept
- **Reserved filenames**: `index.md` and `log.md` are handled correctly (OKF §3.1)

Exits with code 1 if any errors are found.

### `okf lint <bundle>`

Checks recommended fields and style (warnings only, no errors):
- `title`, `description`, `tags` are recommended per OKF §4.1
- Non-empty body with structural markdown per OKF §4.2
- Timestamp sanity checks

### `okf index <bundle>`

Generates `index.md` files in every directory containing concepts. Each index lists the concepts in that directory (with title, type, description) and links to subdirectory indexes. This implements the progressive disclosure pattern from OKF §6.

### `okf graph <bundle>`

Builds the cross-link graph and outputs nodes, edges, and summary statistics: node count, edge count, isolated nodes, max backlinks, and graph density.

### `okf list <bundle>`

Lists all concepts in the bundle with their ID, type, and title.

## What is OKF?

OKF is an open format from Google for representing knowledge — the metadata, context, and curated insight that surrounds data and systems. A bundle is a directory of markdown files with YAML frontmatter:

```
my-bundle/
├── index.md                  # directory listing (auto-generated)
├── datasets/
│   ├── index.md
│   └── ga4.md                # a concept
├── tables/
│   ├── index.md
│   └── events_.md            # a concept
└── playbooks/
    └── freshness.md           # a concept
```

Each concept document has YAML frontmatter and a markdown body:

```markdown
---
type: BigQuery Table
title: Customer Orders
description: One row per completed customer order.
tags: [sales, orders, revenue]
timestamp: 2026-05-28T14:30:00Z
---

# Schema

| Column | Type | Description |
|--------|------|-------------|
| `order_id` | STRING | Globally unique order identifier. |

# Joins

Joined with [customers](/tables/customers.md) on `customer_id`.
```

The format is intentionally minimal: no schema registry, no central authority, no required tooling. If you can `cat` a file, you can read OKF; if you can `git clone` a repo, you can ship it.

## Project Status

Early development. The CLI surface (schema, validate, lint, index, graph, list) is functional. Planned:

- `okf serve` — local HTTP server to browse a bundle interactively
- `okf render` — export a bundle as a self-contained HTML file
- OKF library package (`okf-go`) for embedding in Go applications

## License

Apache 2.0 — matching the upstream [Google knowledge-catalog](https://github.com/GoogleCloudPlatform/knowledge-catalog) repository.

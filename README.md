# okf

A Go CLI toolkit for the [Open Knowledge Format (OKF)](https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md) — a vendor-neutral format for representing data catalog knowledge as plain markdown files with YAML frontmatter.

`okf` creates, validates, lints, indexes, searches, and inspects OKF bundles. One static binary, no runtime dependencies, fast enough to validate millions of concepts.

## Agentic-first design

`okf` is designed to be driven by external AI agents, not to call AI internally. Three mechanisms make this possible:

1. **`okf schema`** — emits a complete machine-readable description of every command: its name, description, flags, arguments, output format, and exit codes. An AI agent runs this once and knows the entire CLI surface.

2. **JSON by default** — all output is structured JSON on stdout. No `--json` flag needed, no screen-scraping. Diagnostics go to stderr.

3. **Structured error envelopes** — all errors emit as `{"error": {"kind":..., "code":..., "reason":..., "message":...}}` on stdout with a stable exit code.

```bash
# An AI agent discovers the CLI:
okf schema

# Then drives it — JSON on stdout, always:
okf init ./my-bundle
okf validate ./my-bundle
okf search ./my-bundle --tag auth
okf show ./my-bundle tables/users
okf backlinks ./my-bundle tables/users
```

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | success |
| 1 | validation error (spec violation, broken link, concept not found) |
| 2 | filesystem or I/O error |
| 3 | internal error (unexpected) |
| 4 | usage error (missing args, unknown command) |

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

## Commands

### `okf schema [command]`

Prints machine-readable CLI metadata as JSON. With no argument, describes every command. With a command name, describes just that command.

### `okf init <bundle>`

Creates a new bundle directory with standard subdirectories (tables, datasets, playbooks), a root index.md, and a .gitignore. Fails if the directory already exists.

### `okf validate <bundle>`

Checks a bundle against the OKF spec:
- **Required fields**: every concept must have a `type` in its frontmatter (OKF §4.1)
- **Link integrity**: all cross-links must resolve to an existing concept
- **Reserved filenames**: `index.md` and `log.md` are handled correctly (OKF §3.1)

Exits 1 if any errors are found.

### `okf lint <bundle>`

Checks recommended fields and style (warnings only):
- `title`, `description`, `tags` recommended per OKF §4.1
- Non-empty body with structural markdown per OKF §4.2
- Timestamp sanity checks

### `okf index <bundle>`

Generates `index.md` files in every directory containing concepts (progressive disclosure per OKF §6).

### `okf list <bundle>`

Lists all concepts with their ID, type, and title.

### `okf show <bundle> <concept-id>`

Returns a single concept's full content as JSON: ID, file path, frontmatter (type, title, description, resource, tags), and the complete markdown body. This is how an AI agent reads a concept's content.

### `okf search <bundle> [--tag <tag>] [--type <type>] [--text <query>]`

Filters concepts by tag, frontmatter type, or full-text search (title, description, body — case-insensitive). Multiple filters are AND-combined. With no filters, returns all concepts.

### `okf backlinks <bundle> <concept-id>`

Returns the IDs of all concepts that link to the given concept. Deduplicates multiple links from the same source. An AI agent uses this to understand reverse relationships — "who depends on this concept?"

### `okf graph <bundle>`

Builds the cross-link graph and outputs nodes, edges, and summary statistics (node count, edge count, isolated nodes, max backlinks, density).

### `okf version`

Prints version as JSON.

## Use cases

### 1. AI-driven documentation pipeline

An AI agent creates a bundle, writes concept documents, validates them, and generates navigation:

```bash
okf init ./bundles/mydb                          # create empty bundle
# ... AI writes .md files into tables/, datasets/ ...
okf validate ./bundles/mydb                       # catch spec violations
okf index ./bundles/mydb                          # generate index.md files
okf graph ./bundles/mydb                          # verify cross-link structure
```

The CLI is the quality gate. The AI reads validation findings as JSON, fixes each one, and re-validates.

### 2. Knowledge catalog audit

An AI agent inspects an existing bundle for health and quality:

```bash
okf list ./bundles/catalog                        # inventory all concepts
okf lint ./bundles/catalog                        # flag missing recommended fields
okf graph ./bundles/catalog                       # find isolated concepts (orphans)
okf search ./bundles/catalog --type Table         # filter by type
okf search ./bundles/catalog --text "deprecated"  # find stale concepts
okf backlinks ./bundles/catalog tables/users      # who depends on this?
```

The agent can identify structural problems (broken links, orphans, missing metadata) without reading a single file.

### 3. CI quality gate

Every PR that touches an OKF bundle runs validate. If it exits 1, the AI reads the findings and either fixes or comments:

```bash
okf validate ./bundles/ga4
# exit 1 -> parse findings array -> fix each -> re-validate
```

### 4. Onboarding assistant

A new engineer joins. An AI agent walks them through the knowledge map:

```bash
okf list ./bundles/team                           # what's in here?
okf show ./bundles/team tables/users              # read a key concept
okf backlinks ./bundles/team tables/users         # what depends on it?
okf search ./bundles/team --tag auth              # find related concepts
```

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

## Project Status

Early development. The CLI surface (schema, init, validate, lint, index, list, show, search, backlinks, graph, version) is functional with 35 tests. Planned:

- `okf serve` — local HTTP server to browse a bundle interactively
- `okf render` — export a bundle as a self-contained HTML file
- OKF library package (`okf-go`) for embedding in Go applications

## License

Apache 2.0 — matching the upstream [Google knowledge-catalog](https://github.com/GoogleCloudPlatform/knowledge-catalog) repository.

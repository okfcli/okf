# okf

A Go CLI toolkit for the [Open Knowledge Format (OKF)](https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md) — a vendor-neutral format for representing data catalog knowledge as plain markdown files with YAML frontmatter.

`okf` validates, lints, indexes, and inspects OKF bundles. One static binary, no runtime dependencies, fast enough to validate millions of concepts.

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
okf validate ./bundles/ga4           # validate against the OKF spec
okf lint ./my-bundle                  # warnings only (recommended fields)
okf index ./my-bundle                 # generate index.md files (progressive disclosure)
okf graph ./bundles/stackoverflow     # cross-link graph statistics
okf list ./my-bundle                  # list all concepts
okf enrich --source postgres \        # enrich a bundle from a database using an LLM
  --dsn "postgres://user:***@localhost/mydb" \
  --out ./bundles/mydb
okf version
```

## Commands

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

Generates `index.md` files in every directory containing concepts. Each index lists the concepts in that directory (with title, type, description) and links to subdirectory indexes. This implements the progressive disclosure pattern from OKF §6 — agents and humans navigate one level at a time instead of loading the entire bundle.

### `okf graph <bundle>`

Builds the cross-link graph and prints statistics: node count, edge count, isolated nodes, max backlinks, and graph density.

### `okf list <bundle>`

Lists all concepts in the bundle with their ID, type, and title.

### `okf enrich --source <type> --dsn <conn> --out <dir>`

The agentic command. Ingests metadata from a data source, sends each concept
through an LLM to produce an OKF concept document, writes the documents to an
output bundle, and validates the result. Outputs a JSON report.

**Model-agnostic** — works with any OpenAI-compatible LLM provider:
- OpenAI: `--base-url https://api.openai.com/v1 --api-key sk-... --model gpt-4o`
- OpenRouter: `--base-url https://openrouter.ai/api/v1 --api-key sk-... --model anthropic/claude-sonnet-4`
- Ollama (local): `--base-url http://localhost:11434/v1 --model llama3.2` (no API key needed)
- Any OpenAI-compatible endpoint

**Sources:**
- `postgres` — introspects a PostgreSQL database via `information_schema`, one concept per table (columns, types, PKs, FKs)

**Example:**
```bash
okf enrich --source postgres \
  --dsn "postgres://user:***@localhost:5432/mydb?sslmode=disable" \
  --out ./bundles/mydb \
  --model gpt-4o \
  --trace
```

Output is a JSON report:
```json
{
  "source": "postgres",
  "total": 42,
  "enriched": 42,
  "errors": 0,
  "results": [{"id": "public.users", "status": "ok", "file": "public_users.md"}],
  "validation": {"errors": 0, "warnings": 3}
}
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

The format is intentionally minimal: no schema registry, no central authority, no required tooling. If you can `cat` a file, you can read OKF; if you can `git clone` a repo, you can ship it.

## Project Status

Early development. The CLI surface (validate, lint, index, graph, list) is functional. Planned:

- `okf serve` — local HTTP server to browse a bundle interactively
- `okf render` — export a bundle as a self-contained HTML file (like the Python visualizer)
- OKF library package (`okf-go`) for embedding in Go applications
- Source connectors (PostgreSQL, OpenAPI, dbt) to generate OKF from existing systems

## License

Apache 2.0 — matching the upstream [Google knowledge-catalog](https://github.com/GoogleCloudPlatform/knowledge-catalog) repository.

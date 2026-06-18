# okf

A Go CLI toolkit for the [Open Knowledge Format (OKF)](https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md) — a vendor-neutral format for representing data catalog knowledge as plain markdown files with YAML frontmatter.

`okf` creates, validates, lints, indexes, searches, and inspects OKF knowledge bundles. One static binary, no runtime dependencies, fast enough to validate millions of concepts.

## Why okf?

Google's reference OKF implementation is Python + Gemini + BigQuery — vendor-locked to Google's cloud. `okf` is the vendor-neutral alternative: a single Go binary that works anywhere, speaks JSON natively, and is designed to be driven by any AI agent on any provider.

**Agentic-first** means an AI agent can discover, understand, and drive the entire CLI without reading documentation or scraping text output. Three mechanisms make this work:

1. **`okf schema`** — emits a complete machine-readable description of every command: name, description, flags, arguments, output format, exit codes. One call and the agent knows the full CLI surface.

2. **JSON by default** — every command outputs structured JSON on stdout. No `--json` flag, no screen-scraping. Diagnostics go to stderr.

3. **Structured error envelopes** — all errors emit `{"error": {"kind":..., "code":..., "reason":..., "message":...}}` on stdout with a stable exit code. An agent can branch on the `kind` field to decide what to do next.

## Quick start

```bash
# Install
go install github.com/okfcli/okf/cmd/okf@latest

# Or build from source
git clone https://github.com/okfcli/okf.git && cd okf && make build

# Create a bundle, add concepts, validate
okf init ./my-bundle
# ... write .md files into tables/, datasets/, playbooks/ ...
okf validate ./my-bundle
okf index ./my-bundle
okf graph ./my-bundle
```

## Use cases

### 1. AI-driven documentation pipeline

An AI agent creates a bundle, writes concept documents from a database schema or API spec, validates them, and generates navigation — all autonomously.

```bash
okf init ./bundles/mydb                              # start from scratch
# AI writes tables/users.md, tables/orders.md, datasets/ga4.md, ...
okf validate ./bundles/mydb                           # catch spec violations
okf lint ./bundles/mydb                               # flag missing recommended fields
okf index ./bundles/mydb                              # generate index.md files
okf graph ./bundles/mydb                              # verify cross-link structure
```

The CLI is the quality gate. The AI reads validation findings as JSON, fixes each one, re-validates. No human in the loop unless something needs a judgment call.

### 2. Knowledge catalog audit

A company inherits a large OKF bundle from a migration or acquisition. An AI agent inspects it for health and quality without reading a single file:

```bash
okf list ./bundles/catalog                            # what's in here?
okf lint ./bundles/catalog                            # what's missing recommended fields?
okf graph ./bundles/catalog                           # find orphan concepts (isolated nodes)
okf search ./bundles/catalog --type Table             # filter by type
okf search ./bundles/catalog --text "deprecated"      # find stale concepts
okf backlinks ./bundles/catalog tables/users           # who depends on this concept?
okf show ./bundles/catalog tables/users               # read the full concept content
```

The agent produces a health report: orphans to fix, missing metadata to add, broken links to repair, dependency hotspots to document.

### 3. CI quality gate

Every PR that touches an OKF bundle runs `okf validate`. If it exits non-zero, the AI reads the findings and either fixes or comments on the PR:

```yaml
# .github/workflows/okf-check.yml
- run: okf validate ./bundles/ga4
# exit 1 -> parse findings -> fix or comment -> re-validate
```

The JSON error envelope makes this trivial to automate:

```json
{
  "error": {
    "kind": "validation",
    "code": 400,
    "reason": "validationError",
    "message": "broken link: [Users] -> users.md (concept tables/users not found)"
  }
}
```

### 4. Onboarding assistant

A new engineer joins a team. An AI agent walks them through the team's knowledge map:

```bash
okf list ./bundles/team                               # inventory
okf graph ./bundles/team                              # structure overview
okf show ./bundles/team tables/users                  # read a key concept
okf backlinks ./bundles/team tables/users             # what depends on it?
okf search ./bundles/team --tag auth                  # find related concepts
```

Progressive disclosure (index.md) lets the agent navigate level by level instead of loading the entire bundle.

## Commands

| Command | Description |
|---------|-------------|
| `okf schema [command]` | Print machine-readable CLI metadata as JSON |
| `okf init <bundle>` | Create a new empty OKF bundle |
| `okf validate <bundle>` | Validate a bundle against the OKF spec (exit 1 on errors) |
| `okf lint <bundle>` | Check recommended fields and style (warnings only) |
| `okf index <bundle>` | Generate index.md files (progressive disclosure) |
| `okf list <bundle>` | List all concepts with ID, type, title |
| `okf show <bundle> <concept-id>` | Show a single concept's full content as JSON |
| `okf search <bundle> [--tag] [--type] [--text]` | Search concepts by tag, type, or text |
| `okf backlinks <bundle> <concept-id>` | List concepts that link to a given concept |
| `okf graph <bundle>` | Print cross-link graph with nodes, edges, and stats |
| `okf version` | Print version |

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | success |
| 1 | validation error (spec violation, broken link, concept not found) |
| 2 | filesystem or I/O error |
| 3 | internal error (unexpected) |
| 4 | usage error (missing args, unknown command) |

## What is OKF?

OKF is an open format from Google for representing knowledge — the metadata, context, and curated insight that surrounds data and systems. A bundle is a directory of markdown files with YAML frontmatter:

```
my-bundle/
├── index.md                  # directory listing (auto-generated)
├── tables/
│   ├── index.md
│   ├── users.md              # a concept
│   └── orders.md             # a concept
├── datasets/
│   └── ga4.md                # a concept
└── playbooks/
    └── freshness.md          # a concept
```

Each concept has YAML frontmatter and a markdown body with cross-links:

```markdown
---
type: Table
title: Customer Orders
description: One row per completed customer order.
tags: [sales, orders, revenue]
---

# Schema

| Column | Type | Description |
|--------|------|-------------|
| order_id | UUID | Primary key |
| user_id | UUID | FK to [Users](/tables/users.md) |

# Joins

Joined with [users](/tables/users.md) on user_id.
```

The format is intentionally minimal: no schema registry, no central authority, no required tooling. If you can `cat` a file, you can read OKF; if you can `git clone` a repo, you can ship it.

## Project status

Early development. The CLI surface is functional with 35 tests:

- `schema`, `init`, `validate`, `lint`, `index`, `list`, `show`, `search`, `backlinks`, `graph`, `version`

Planned:

- `okf serve` — local HTTP server to browse a bundle interactively
- `okf render` — export a bundle as a self-contained HTML file
- `okf-go` — Go library package for embedding in applications

## License

Apache 2.0 — matching the upstream [Google knowledge-catalog](https://github.com/GoogleCloudPlatform/knowledge-catalog) repository.
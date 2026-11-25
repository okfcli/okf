---
name: okf
description: >-
  Create, validate, lint, index, search and graph Open Knowledge Format (OKF
  0.2) bundles with the `okf` CLI: directories of markdown files with YAML
  frontmatter (`type` required) used as agent-readable knowledge bases. Use
  when the user asks to document or catalog knowledge as OKF, add OKF
  frontmatter to a file, create or fix an `index.md`/`log.md`, audit a bundle's
  health, or find what links to a concept.
---

# Skill: okf — working with OKF bundles

An OKF bundle is a directory of `.md` files. Each file is a **concept** with a
YAML frontmatter block; only `type` is required. The `okf` CLI is the quality
gate: it emits JSON on stdout, diagnostics on stderr, and a stable exit code.

> Spec: <https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md>
> Field cheat sheet: [references/frontmatter.md](references/frontmatter.md) ·
> Copy-paste templates: [references/templates.md](references/templates.md)

Never guess the CLI surface. Run `okf schema` (all commands) or
`okf schema <command>` (one command) and read the JSON.

---

## 1 — The loop

1. **Orient.** `okf list <bundle>` shows every concept with id, type, title,
   status and trust tier. Read the bundle's `index.md` files first: local
   conventions beat this guide.
2. **Reuse vocabulary.** `okf search <bundle> --type <Type>` before inventing a
   new `type`; `type` values are free text and unregistered, so consistency is
   what makes filtering useful.
3. **Write** the concept (section 3).
4. **Gate.** `okf validate <bundle>`, fix every `ERROR` finding, re-run until
   `"valid": true` (exit 0). Then `okf lint <bundle>` for recommended fields
   and broken links.
5. **Regenerate navigation.** `okf index <bundle>` rewrites the `index.md`
   files.

---

## 2 — Commands

| Goal | Command |
|---|---|
| New bundle (`tables/`, `datasets/`, `playbooks/`, root `index.md` with `okf_version: "0.2"`) | `okf init <bundle>` (fails if the directory exists) |
| Spec check; exit 1 on errors | `okf validate <bundle> [--format json\|sarif] [--exit-zero]` |
| Recommended fields and style (warnings only, exit 0) | `okf lint <bundle> [--format json\|sarif]` |
| Generate `index.md` in every folder with concepts | `okf index <bundle>` |
| Inventory | `okf list <bundle>` |
| Read one concept as JSON | `okf show <bundle> <concept-id>` |
| Filter concepts | `okf search <bundle> [--tag T] [--type T] [--text S]` (AND-combined) |
| Who links to this concept? | `okf backlinks <bundle> <concept-id>` |
| Link structure, orphans | `okf graph <bundle>` (see `isolated`) |

A **concept id** is the file path relative to the bundle root without `.md`
(`tables/users`). Exit codes: `0` ok · `1` validation · `2` I/O · `3` internal ·
`4` usage. Errors arrive as `{"error": {"kind", "code", "reason", "message"}}`
on stdout: branch on `kind`, not on message text. Every finding has a stable
rule id (`okf/<family>/<check>`, e.g. `okf/links/broken`).

Only `okf index` and `okf init` write files.

---

## 3 — Writing a concept

Minimal valid concept:

```yaml
---
type: Reference
---
```

Default to the recommended set:

```yaml
---
type: Table                      # REQUIRED, free text, reuse existing values
title: Customer Orders
description: One row per completed customer order.   # one sentence
tags: [sales, orders]
---
```

Rules of thumb:

- Body is structured markdown (headings, lists, tables, fenced code), not
  prose walls. Conventional headings: `# Schema`, `# Examples`, `# Computation`.
- Link related concepts with bundle-absolute paths: `[users](/tables/users.md)`.
  The relationship's meaning comes from the surrounding sentence. Broken links
  are only warnings (they may be knowledge not yet written) — but fix the ones
  you can.
- Quote `title`/`description` if they contain `:` or `#`.
- Extra frontmatter keys are allowed; keep any you did not write.
- `index.md` and `log.md` are **reserved names**: never use them for concepts.
  Frontmatter in `index.md` is allowed only at the bundle root, and only for
  `okf_version`.

### Optional trust / provenance / lifecycle fields

| Field | Shape |
|---|---|
| `status` | `draft` \| `stable` (default) \| `deprecated` |
| `generated` | `{ by: <actor>, at: <ISO 8601 UTC> }` |
| `verified` | `{ by, at }` or a list of them |
| `sources` | list of `{ resource (required), id, title, author, usage_count, last_modified }` |
| `stale_after` | see the note below |

Actors: `human:<id>` · `process:<id>` · `<producer>/<version>` (agents).
The trust tier `human-reviewed` is derived from a `human:` verifier, so:

- record yourself as `generated: { by: <producer>/<version>, at: ... }`;
- **never write a `human:` entry in `verified` unless that person actually
  confirmed the content.**

Per-claim attribution uses a footnote whose label is a `sources[].id`
(`text.[^id]` … `[^id]: Source title`).

> **`stale_after` format.** The spec (§5.5) says ISO 8601 datetime, but
> `okf` v0.5.0 currently rejects that and demands `YYYY-MM-DD`
> (`okf/lifecycle/stale-after-invalid`, tracked in okfcli/okf#34). Use the
> date form until that is fixed, or `validate` will fail.

---

## 4 — Gotchas

- **Any non-reserved `.md` without frontmatter aborts the whole load**
  (`validation` error naming the file), it is not just one finding. Keep
  READMEs, `AGENTS.md` and other non-concepts outside the bundle directory, or
  give them a `type`.
- `okf validate` and `okf lint` are the same checks; lint hides errors.
  Use `--exit-zero` only when a later step must still run (e.g. SARIF upload).
- `okf index` regenerates `index.md` files and preserves an existing
  `okf_version`. Do not hand-maintain listings that `index` will overwrite.
- Legacy v0.1 constructs (`timestamp`, body `# Citations`) still load with
  migration warnings; prefer `generated.at` and `sources`.

---

## 5 — Audits

```bash
okf list <bundle>                       # inventory, status, trust tiers
okf lint <bundle>                       # missing title/description/tags, broken links
okf graph <bundle>                      # isolated > 0 means orphan concepts
okf search <bundle> --text deprecated   # stale or retired knowledge
okf backlinks <bundle> <concept-id>     # blast radius before changing a concept
```

Summarize findings grouped by `rule`, fix the mechanical ones, and leave
judgment calls (what a concept *means*) to the user.

## 6 — CI

```yaml
- run: okf validate ./bundles/ga4                  # fails the job on errors
# or, to surface findings as PR annotations:
- run: okf validate --format sarif --exit-zero ./bundles/ga4 > okf.sarif
- uses: github/codeql-action/upload-sarif@v3
  with: { sarif_file: okf.sarif }
```

# OKF 0.2 frontmatter cheat sheet

Summary of the [spec](https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md);
the spec wins on any disagreement.

## Base fields

| Field | Status | Notes |
|---|---|---|
| `type` | **required** | Free text, not centrally registered (`Table`, `Playbook`, `Metric`, `Reference`…) |
| `title` | recommended | Falls back to the filename |
| `description` | recommended | One sentence; used by indexes, search, previews |
| `resource` | recommended | Canonical URI of the underlying asset; omit for abstract concepts |
| `tags` | recommended | List of short strings |

## Provenance, trust, lifecycle (all optional)

| Field | Shape | Notes |
|---|---|---|
| `sources` | `[{ resource, id?, title?, author?, usage_count?, last_modified? }]` | `resource` is a URL, bundle path, `references/...` path, or a scope descriptor |
| `usage_window` | `{ from, to }` | Sibling of `sources`; frames every `usage_count` |
| `generated` | `{ by, at }` | How/when the current content was produced |
| `verified` | `{ by, at }` or list | Who confirmed the content; independent of `generated` |
| `status` | `draft` \| `stable` \| `deprecated` | Absent means `stable` |
| `stale_after` | instant | Stale when `now >= stale_after`. Spec: datetime; `okf` 0.5.0: `YYYY-MM-DD` (okfcli/okf#34) |

Timestamps are ISO 8601 with an explicit offset (`2026-06-30T14:00:00Z`).

### Actors (`generated.by`, `verified[].by`)

`human:<id>` · `process:<id>` · `<producer>/<version>`

### Trust tier (derived, never stored)

| `verified` | Tier |
|---|---|
| absent | `unverified` |
| only non-`human:` actors | `machine-confirmed` |
| any `human:` actor | `human-reviewed` |

## Attested Computation

`type: Attested Computation` describes a sanctioned way to *compute* a value.
Required: `runtime`. Also: `parameters` (`[{ name, type, required }]`),
`computation` (file path; otherwise the fenced block under `# Computation`),
`executor` (`resource`, `receipt`), `attester` (`resource`). Agents supply
parameter values only; they do not edit the computation. Concepts that use the
value just link to it. See spec §10.

## Reserved files

- `index.md` — folder listing (`okf index` generates it). No frontmatter except
  `okf_version: "0.2"` at the bundle root.
- `log.md` — history, newest first, `## YYYY-MM-DD` headings.

## Links

- Absolute (recommended): `[x](/tables/users.md)`, relative to the bundle root.
- Relative: `[x](./other.md)`.
- Broken links are tolerated by the spec; `okf` reports them as warnings.
- Path-valued fields (`resource`, `sources[].resource`, `computation`,
  `executor.resource`, `attester.resource`) that resolve to concepts become
  edges in `okf graph` / `okf backlinks`.

## v0.1 → v0.2

`timestamp` → `generated.at` · body `# Citations` → `sources`.

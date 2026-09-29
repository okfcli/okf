# OKF concept templates

Copy, adjust, delete what you do not need. Only `type` is required. Run
`okf validate <bundle>` after writing.

## Generic concept

```markdown
---
type: Reference
title: Readable name
description: One sentence summarizing the concept.
tags: [topic]
---

# Context

# Details

- ...

See also [related concept](/folder/related.md).
```

## Table (bound to a resource)

````markdown
---
type: Table
title: Customer Orders
description: One row per completed customer order.
resource: https://example.com/warehouse/sales/orders
tags: [sales, orders]
---

# Schema

| Column | Type | Description |
|--------|------|-------------|
| order_id | UUID | Primary key |
| user_id | UUID | FK to [Users](/tables/users.md) |

# Examples

```sql
SELECT order_id FROM orders LIMIT 10;
```
````

## Playbook

```markdown
---
type: Playbook
title: Freshness alert triage
description: Steps to triage a freshness alert on the orders pipeline.
tags: [oncall]
status: draft
---

# Trigger

# Steps

1. ...
```

## Meeting note

```markdown
---
type: Meeting Note
title: Design review with <person>
description: One-line summary of what was decided.
tags: [review]
date: 2026-09-29
---

# Context

# Decisions

# Next steps
```

## Concept with provenance and trust

```markdown
---
type: Reference
title: Revenue recognition
description: How recognized revenue is defined.
status: stable
generated: { by: claude-code/opus-5, at: 2026-09-29T17:00:00Z }
# add `verified` only after a person confirms:
# verified: { by: human:<id>, at: 2026-09-30T09:00:00Z }
sources:
  - id: rev-policy
    resource: https://wiki.example.com/finance/revenue-recognition
    title: Revenue recognition policy
---

Revenue is booked when the service is delivered.[^rev-policy]

[^rev-policy]: Revenue recognition policy
```

## `log.md`

```markdown
# Directory Update Log

## 2026-09-29
* **Creation**: Added [Customer Orders](/tables/orders.md).

## 2026-09-15
* **Initialization**: Created the bundle.
```

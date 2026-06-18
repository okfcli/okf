---
type: BigQuery Table
title: Events
description: One row per GA4 event (pageview, purchase, etc.) partitioned by event date.
resource: https://console.cloud.google.com/bigquery?p=bigquery-public-data&d=ga4_obfuscated_sample_ecommerce&t=events_
tags: [analytics, events, ga4]
timestamp: 2026-05-28T14:30:00Z
---

# Schema

| Column         | Type      | Description                              |
|----------------|-----------|------------------------------------------|
| `event_date`   | DATE      | The date the event was logged.           |
| `event_timestamp` | INT64 | The time (microseconds) the event was logged. |
| `event_name`   | STRING    | The name of the event.                   |
| `user_pseudo_id` | STRING  | Unique per-user id (resets on cookie clear). |
| `user_id`      | STRING    | The signed-in user id, if available.     |

# Partitions

Partitioned on `event_date` (daily). Clustered on `user_pseudo_id`.

# Joins

Join with the [GA4 dataset](/datasets/ga4.md) for dataset-level metadata.

# Citations

[1] [GA4 BigQuery Export schema](https://support.google.com/analytics/answer/9358801)

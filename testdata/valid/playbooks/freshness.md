---
type: Playbook
title: Investigate a freshness alert
description: Steps to triage a data freshness alert on the GA4 events pipeline.
tags: [oncall, incident]
timestamp: 2026-04-12T09:00:00Z
---

# Trigger

A freshness alert fires when [events_](/tables/events_.md) lags more than 30
minutes behind its expected SLA.

# Steps

1. Check the ingestion job dashboard.
2. Compare the latest `event_date` in [events_](/tables/events_.md) against yesterday.
3. If stale, restart the GA4 export sync and notify the data platform on-call.

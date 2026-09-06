---
id: godoclive-c0dzc7
title: Load generated API data before the single-file UI starts
project: godoclive
state: proposed
kind: fix
parent: null
spawned_by: agent/codex-audit-20260906
approved_by: null
claimed_by: null
lease_until: null
branch: null
pr: null
depends_on: []
budget:
  attempts: 3
  attempts_used: 0
  max_tokens: 400000
priority: 1
created: 2026-09-06T12:43:24Z
updated: 2026-09-06T12:43:24Z
---

## Why
Audit of 4755a0a: generate --format single on testdata/chi-basic selects the embedded Pet Store API demo (9 endpoints) instead of generated Chi Basic API data (6 endpoints). Folder mode selects the correct data. generator.go inlines app.js before injectAPIData appends API_DATA at the end of body; execution of the generated startup scripts reproduces this.

## Done when
- [ ] Single-file generated documentation initializes with the analyzed project and endpoint set.
- [ ] A regression check executes the generated startup script order for both output formats.

## Out of scope
- UI redesign and analysis changes.

## Log
<!-- append-only; newest last -->
- 2026-09-06T12:43Z agent/codex-audit-20260906: created

---
id: godoclive-aqi79y
title: Watch Go files in directories created after startup
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
priority: 2
created: 2026-09-06T12:44:50Z
updated: 2026-09-06T12:44:50Z
---

## Why
Audit 4755a0a: run watch on a temporary copy of testdata/stdlib-basic. Add admin/main.go copied from the existing fixture: regeneration count stays 1. Save original main.go: count becomes 2. Save admin/main.go again: count stays 2. addWatchDirs runs only at startup and directory creation events never add watches.

## Done when
- [ ] Creating a new package directory and Go file triggers regeneration.
- [ ] Subsequent changes inside the new directory also trigger regeneration.

## Out of scope
- Unrelated analyzer features or UI redesign.

## Log
<!-- append-only; newest last -->
- 2026-09-06T12:44Z agent/codex-audit-20260906: created

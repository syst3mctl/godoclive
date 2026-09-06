---
id: godoclive-0tcgti
title: Reuse analysis results across GitHub Action outputs
project: godoclive
state: proposed
kind: chore
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
priority: 3
created: 2026-09-06T12:44:55Z
updated: 2026-09-06T12:44:55Z
---

## Why
Audit 4755a0a: action.yml runs validate twice, then openapi, then generate when both spec and docs are enabled. Each invokes RunPipeline and reloads the same unchanged packages. On the pinned 27-route upstream corpus, median warm analysis is 168.41ms and 18.81MB allocated; four passes imply roughly 674ms and 75.3MB cumulative allocation for analysis alone, excluding process startup and generation.

## Done when
- [ ] Coverage JSON and human summary are derived from one validation result.
- [ ] Generating both docs and OpenAPI reuses one analysis result while preserving action outputs and gates.

## Out of scope
- Unrelated analyzer features or UI redesign.

## Log
<!-- append-only; newest last -->
- 2026-09-06T12:44Z agent/codex-audit-20260906: created

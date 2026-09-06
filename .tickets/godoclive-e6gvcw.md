---
id: godoclive-e6gvcw
title: Accept overlapping response alternatives in the emitted JSON Schema
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
created: 2026-09-06T12:44:38Z
updated: 2026-09-06T12:44:38Z
---

## Why
Audit 4755a0a: invoke existing testdata/validation GetArticle with ?format=summary through httptest. Runtime returns {"id":"","title":""}, but Draft202012Validator rejects the emitted oneOf because this object validates against both ArticleSummary and Article. converter.go:274 uses exclusive oneOf for overlapping response shapes.

## Done when
- [ ] Both actual GetArticle response variants validate against the generated response schema.
- [ ] Alternative-schema regression tests validate handler output instead of only checking oneOf structure.

## Out of scope
- Unrelated analyzer features or UI redesign.

## Log
<!-- append-only; newest last -->
- 2026-09-06T12:44Z agent/codex-audit-20260906: created

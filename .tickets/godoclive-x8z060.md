---
id: godoclive-x8z060
title: Assign distinct OpenAPI operation IDs to trailing-slash routes
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
created: 2026-09-06T12:44:44Z
updated: 2026-09-06T12:44:44Z
---

## Why
Audit 4755a0a of pinned gothinkster/golang-gin-realworld-example-app commit 626c372d259472148d93303f74aa9b9a1cdcef24: 27 routes yield seven duplicate operationId pairs, including getApiArticles for GET /api/articles and GET /api/articles/. toOperationID drops empty path segments. OpenAPI 3.1 requires operationId uniqueness; current upstream corpus gate passes.

## Done when
- [ ] All 27 upstream corpus operations have unique deterministic IDs.
- [ ] A generation-level corpus assertion rejects duplicate operation IDs without removing distinct routes.

## Out of scope
- Unrelated analyzer features or UI redesign.

## Log
<!-- append-only; newest last -->
- 2026-09-06T12:44Z agent/codex-audit-20260906: created

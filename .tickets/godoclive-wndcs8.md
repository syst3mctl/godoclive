---
id: godoclive-wndcs8
title: Keep unknown custom JSON marshaler output unconstrained in OpenAPI
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
created: 2026-09-06T12:44:41Z
updated: 2026-09-06T13:13:05Z
---

## Why
Audit 4755a0a: testdata/validation Money.MarshalJSON returns the string "0.00". Actual GetArticle output contains that string, but Article.fee is emitted as type object. The mapper deliberately returns KindInterface for an unknown marshaler, and converter.go:337 incorrectly constrains it to object. JSON Schema validation rejects the actual fee value.

## Done when
- [ ] Unknown custom JSON marshaler output accepts the actual scalar fixture payload without exposing private struct fields.
- [ ] Concrete known types remain constrained to their correct wire types.

## Out of scope
- Unrelated analyzer features or UI redesign.

## Log
<!-- append-only; newest last -->
- 2026-09-06T12:44Z agent/codex-audit-20260906: created
- 2026-09-06T13:13Z agent/codex-fixes-20260906: Implemented under the user-approved combined delivery ticket godoclive-rnrffj in PR #37; review and completion are tracked on that ticket.

---
id: godoclive-l1qk2s
title: Preserve array response schemas in Echo Fiber and net/http
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
created: 2026-09-06T12:44:35Z
updated: 2026-09-06T13:13:00Z
---

## Why
Audit 4755a0a: existing testdata/echo-basic and fiber-basic listUsers return []User and stdlib-basic ListUsers encodes []UserResponse. All three generated GET /users 200 schemas are type object, not array; Echo validate --json still reports 8/8 resolved and coverage 100. typeRefDef stores an opaque slice name that pipeline type lookup cannot resolve.

## Done when
- [ ] Existing list handlers produce array schemas with expanded item fields across all three frameworks.
- [ ] Coverage reports unresolved response shapes when mapping cannot preserve their known wire type.

## Out of scope
- Unrelated analyzer features or UI redesign.

## Log
<!-- append-only; newest last -->
- 2026-09-06T12:44Z agent/codex-audit-20260906: created
- 2026-09-06T13:13Z agent/codex-fixes-20260906: Implemented under the user-approved combined delivery ticket godoclive-rnrffj in PR #37; review and completion are tracked on that ticket.

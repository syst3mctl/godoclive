---
id: godoclive-97215m
title: Broadcast live reload events to every connected documentation client
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
created: 2026-09-06T12:44:47Z
updated: 2026-09-06T12:44:47Z
---

## Why
Audit 4755a0a: a temporary integration test connects two HTTP clients to the existing ServeWithSSE implementation, sends one reload event, and receives exactly one notification. generator.go:536 has all connections consume the same channel, distributing events instead of broadcasting them.

## Done when
- [ ] A single regeneration notifies both of two connected SSE clients.
- [ ] Disconnected clients are removed without blocking active clients.

## Out of scope
- Unrelated analyzer features or UI redesign.

## Log
<!-- append-only; newest last -->
- 2026-09-06T12:44Z agent/codex-audit-20260906: created

---
id: godoclive-rnrffj
title: Deliver the approved September audit corrections in one PR
project: godoclive
state: claimed
kind: fix
parent: null
spawned_by: agent/codex-fixes-20260906
approved_by: nicolas
claimed_by: agent/codex-fixes-20260906
lease_until: 2026-09-06T14:25:24Z
branch: null
pr: null
depends_on: []
budget:
  attempts: 3
  attempts_used: 0
  max_tokens: 400000
priority: 1
created: 2026-09-06T12:55:04Z
updated: 2026-09-06T12:55:24Z
---

## Why
The user explicitly requested implementation of all nine confirmed findings on one new branch and one PR. This delivery ticket consolidates godoclive-c0dzc7, godoclive-l1qk2s, godoclive-e6gvcw, godoclive-wndcs8, godoclive-x8z060, godoclive-97215m, godoclive-aqi79y, godoclive-c3yohq and godoclive-0tcgti, whose Why sections contain executed reproductions against 4755a0a and the pinned upstream RealWorld corpus.

## Done when
- [ ] Single and folder documentation initialize using generated API data.
- [ ] Existing Echo Fiber and stdlib list responses preserve array item schemas and unresolved shapes are reported.
- [ ] Both existing article response variants and the custom Money marshaler payload conform to emitted schemas.
- [ ] All upstream RealWorld operations have unique deterministic operation IDs.
- [ ] One reload notifies every connected client and disconnected clients do not block delivery.
- [ ] Go files in new directories trigger regeneration on creation and later edits.
- [ ] Benchmarks use accurately named consistent timing boundaries.
- [ ] The GitHub Action reuses validation results and combines HTML and OpenAPI generation while preserving outputs and gates.
- [ ] Regression checks, full race tests, vet, build and make ci pass locally; one PR contains the completed checklist.

## Out of scope
- Unrelated analyzer features, UI redesign, merging the PR, and changes to existing ticket acceptance criteria.

## Log
<!-- append-only; newest last -->
- 2026-09-06T12:55Z agent/codex-fixes-20260906: created
- 2026-09-06T12:55Z nicolas: promoted to ready: Human approval in this conversation: Start implementing fixes for each confirmed findings in new branch and then open a PR.
- 2026-09-06T12:55Z agent/codex-fixes-20260906: claimed, lease until 2026-09-06T14:25:24Z

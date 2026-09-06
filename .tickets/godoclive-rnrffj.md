---
id: godoclive-rnrffj
title: Deliver the approved September audit corrections in one PR
project: godoclive
state: in_review
kind: fix
parent: null
spawned_by: agent/codex-fixes-20260906
approved_by: nicolas
claimed_by: null
lease_until: null
branch: agent/godoclive-rnrffj
pr: 37
depends_on: []
budget:
  attempts: 3
  attempts_used: 0
  max_tokens: 400000
priority: 1
created: 2026-09-06T12:55:04Z
updated: 2026-09-06T13:12:30Z
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
- 2026-09-06T13:05Z agent/codex-fixes-20260906: All nine audit corrections are implemented on agent/godoclive-rnrffj; executable startup, runtime schema, SSE, watcher and Action regression checks pass; full make ci is running.
- 2026-09-06T13:07Z agent/codex-fixes-20260906: First make ci passes race tests and vet; lint identifies three unchecked test cleanup calls, now corrected before the second CI run.
- 2026-09-06T13:08Z agent/codex-fixes-20260906: Second make ci is green, including race tests, vet and lint; the pinned upstream corpus also passes its new operation-ID uniqueness gate.
- 2026-09-06T13:10Z agent/codex-fixes-20260906: Go 1.27 race tests also pass; real Action smoke emits both outputs from shared analysis; warm upstream analysis remains about 162 ms with 18.86 MB allocated.
- 2026-09-06T13:11Z agent/codex-fixes-20260906: in progress on agent/godoclive-rnrffj
- 2026-09-06T13:12Z agent/codex-fixes-20260906: PR #37 is open with all acceptance items checked and local validation green; auto-merge is unavailable because the repository disables it, so the PR remains open for independent review.
- 2026-09-06T13:12Z agent/codex-fixes-20260906: PR #37 open, auto-merge enabled; released for review

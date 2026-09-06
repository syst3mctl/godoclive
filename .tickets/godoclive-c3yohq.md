---
id: godoclive-c3yohq
title: Make pipeline and generator benchmarks measure their stated work
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
created: 2026-09-06T12:44:52Z
updated: 2026-09-06T12:44:52Z
---

## Why
Audit 4755a0a: BenchmarkRunPipeline_LoadVsProcess claims to isolate post-load processing, but PipelineFromCache calls RunPipeline and reloads packages on every iteration. Generator standalone benchmarks time TempDir creation, while Folder_vs_Single also times RemoveAll. Measured folder medians are 1.059ms versus 1.251ms with 326 versus 367 allocations, for different timed work.

## Done when
- [ ] Benchmark names and comments accurately distinguish loading, full warm-cache analysis, and any isolated processing measurement.
- [ ] Generator comparisons use a consistent documented boundary for temporary-directory creation and cleanup.

## Out of scope
- Unrelated analyzer features or UI redesign.

## Log
<!-- append-only; newest last -->
- 2026-09-06T12:44Z agent/codex-audit-20260906: created

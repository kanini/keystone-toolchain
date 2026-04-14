---
schema: v1
id: f2hqgw
title: Implement keystone-hub sync M1 through ready-adapter path
status: closed
type: task
priority: p1
deps: [1sa48m, kfj483]
return_to: 8jpmde
tags: [hub, sync, tdd]
created_at: "2026-04-13T04:06:55Z"
closed_at: "2026-04-13T17:08:06Z"
---
<!-- ksmem:managed: direct edits bypass validation; use ksmem commands -->
Implement the first real sync slice for kstoolchain by proving one truthful end-to-end path: keystone-hub only, through the generic ready-adapter flow.

## Context

The completed 8jpmde review round converged on a narrow M1:
- fix the existing status truth bug before adding sync
- make the hub adapter contract truthful
- add ready/candidate/blocked rollout semantics
- implement stage-probe-promote for the ready set only
- write current.json atomically and have status read it back truthfully
- require a dogfooding checkpoint on the machine with the live stale kshub warning

## Plan

1. Fix the status truth bug so a missing PATH resolution can never remain CURRENT. 2. Tighten manifest and state validation where sync depends on it. 3. Make the keystone-hub adapter truthfully materialize staged output. 4. Implement SavePersistedState plus ready-set sync with stage-probe-promote. 5. Add tests starting with the status truth fix and the sync round-trip. 6. Run the dogfooding checkpoint and save its artifact bundle.

## Decisions

M1 proves one real path only: keystone-hub.

Sync success is judged on ready adapters only.

Status remains broad and can continue to show candidate and blocked adapters.

## Evidence

Key review invariants for this slice:
- stage-probe-promote is mandatory
- current.json is evidence memory and must be written atomically
- dirty worktrees fail closed in v1
- launchd, broad suite sync, advanced LKG history, and multi-output activation stay out of M1
- PATH truth is part of the dogfooding checkpoint, not an afterthought

Dogfood checkpoint completed with a temp config and a temp managed bin, using the real /Users/rc/git/keystone-hub checkout. Durable local artifact bundle saved under .keystone/local/dogfood/sync-m1-hub/.

Observed results:
- clean hub sync succeeded and produced a truthful CURRENT result for the ready-set sync surface
- command -v kshub resolved to the temp managed bin path after PATH was prepended
- the staged and promoted kshub reported commit e5e59c8 from the live hub repo
- dirtying a tracked hub file caused sync to fail closed with DIRTY_SKIPPED and exit code 1
- the previously promoted kshub kept running after the dirty rerun

Saved artifact files:
- 01-sync-clean.txt
- 02-status-after-clean.json
- 03-command-v-kshub.txt
- 04-kshub-version.txt
- 05-sync-dirty.txt
- 06-status-after-dirty.json
- 07-kshub-version-after-dirty.txt
- certification.md

Follow-up exposed by dogfood:
- the broad status surface still shows candidate and blocked rows as SHADOWED when their tools resolve outside the temp managed bin, even though overall truth for the ready set is CURRENT. This needs a clearer out-of-scope presentation in a later slice.

Dogfood follow-up issue filed as sdz8vc: clarify out-of-scope rows in broad kstoolchain status output.

Post-implementation review surfaced and this slice now fixes three functional regressions:
- sync exit code is now based on ready-set sync outcome, not post-sync PATH state, so a successful sync can return 0 even while the report truthfully shows SHADOWED
- promotion now falls back from os.Rename to copy-then-rename when the source and target live on different filesystems
- dirty fail-closed ready repos now keep DIRTY_SKIPPED even when repo HEAD has moved past the active build

Regression checks run after the fixes:
- go test ./...
- make test
- go vet ./...
- git diff --check
- real temp-config sync run with unchanged PATH produced SHADOWED output and exit=0

Managed-bin reconfiguration bug fixed. Real workflow check after rebuilding the binary:
- sync to config A with managed_bin_dir=A and shared state_dir succeeded
- sync to config B with managed_bin_dir=B and the same state_dir also succeeded
- the second sync rewrote current.json for the new managed bin instead of failing on the previous path
- both A/bin/kshub and B/bin/kshub existed after their respective syncs, and status under config B remained truthful about SHADOWED because PATH still preferred /Users/rc/go/bin/kshub

Regression intent: managed_bin_dir drift is now treated as stale persisted state for the active config, not a fatal error.

## Journal

- 2026-04-13T17:08:06Z | Code on main. All regression checks passed (go test ./..., make test, go vet, git diff --check). Dogfood artifact bundle saved under .keystone/local/dogfood/sync-m1-hub/. Three post-dogfood regressions fixed: sync exit code based on ready-set outcome not PATH state, cross-filesystem promotion fallback, DIRTY_SKIPPED persists when HEAD moves. Managed-bin reconfiguration drift treated as stale state rather than fatal error. Follow-up filed as sdz8vc.

## Lessons

- Dogfood on the real machine with a temp config exposed issues that tests missed: the exit-code/PATH coupling, cross-filesystem rename, and dirty-skip drift. Run dogfood before marking M1 complete, not after.

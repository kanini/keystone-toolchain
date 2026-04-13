---
schema: v1
id: 8jpmde
title: Bootstrap keystone-toolchain scaffold and initial status surface
status: open
type: task
priority: p1
deps: [1sa48m]
tags: [review-loop, scaffold, status, toolchain]
created_at: "2026-04-13T01:18:12Z"
---
<!-- ksmem:managed: direct edits bypass validation; use ksmem commands -->
Build the first real keystone-toolchain implementation slice on top of the new CLI scaffold. Scope: version/provenance surface, managed-bin and state-dir defaults, tracked adapter manifest, live PATH-aware status command, tests, CI, and repo docs. This work is no longer trivial and needs repo-local stone tracking plus a focused review loop before it is treated as complete.

## Context

## Plan

1. Bootstrap the repo scaffold with contract/runtime/CLI layers. 2. Add a tracked adapter manifest. 3. Wire status to report manifest truth plus live PATH resolution. 4. Add tests and CI. 5. Run a focused repo-local review loop on this implementation slice before treating it as done.

## Decisions

## Evidence

Local implementation exists in keystone-toolchain and is pushed on main. The status surface already exposes a real machine truth: the suite is currently SHADOWED because tools resolve from ~/go/bin or ~/bin instead of ~/.keystone/toolchain/active/bin.

Review loop 1/5 and 2/5 captured under .keystone/local/8jpmde-sync-m1-review-responses/.

Convergence so far:
- current scaffold plus status slice is solid enough to build on
- first sync milestone should target keystone-hub
- stage-probe-promote belongs in M1, with persisted state landing now
- dirty worktrees should fail closed in v1
- dogfooding checkpoint must be a hard gate

High-signal guidance from 1/5:
- fix the keystone-hub Makefile so INSTALL_BIN_DIR is honored via GOBIN or an explicit build output path
- dedupe path normalization between manifest.go and config.go before sync grows further
- stage on the same filesystem as the managed bin dir to avoid EXDEV on os.Rename
- capture subprocess stdout and stderr on failure so adapter debugging is truthful
- bias M1 toward a single-repo keystone-hub sync, rebuilding blindly rather than adding stale-skip logic yet

High-signal guidance from 2/5:
- proceed now; the pack is ready and the scaffold is ready
- current.json must land in M1 because status depends on persisted truth
- template expansion for install_cmd is acceptable for M1
- managed bin dir creation and PATH requirement docs are low-risk stewardship worth fixing now

Live design tension after 2/5:
- 1/5 wants explicit single-repo keystone-hub scope for M1
- 2/5 is comfortable driving all candidate repos through the same minimal flow, with keystone-hub as the primary proof target
- no conflict yet on the hard invariants; only on how narrow the first slice should be

Review 4/5 was a duplicate of 1/5, not a new line of argument.

What it changes:
- higher confidence in a narrow M1 centered on keystone-hub
- higher confidence that the first stewardship fix is the keystone-hub Makefile install contract
- higher confidence that same-filesystem staging and subprocess output capture are hard requirements, not polish

What it does not change:
- the only live design tension remains milestone width, not the contract direction

Review 3/5 Opus captured under .keystone/local/8jpmde-sync-m1-review-responses/03-opus.md.

Useful additions from Opus:
- strongest framing for M1 is the round-trip proof: sync writes current.json and status reads it back truthfully
- first test should prove that writer-reader contract end to end
- stage directory should live under StateDir and current.json should always be written atomically
- blocked adapters should be skipped, not attempted
- template expansion deserves its own small tested unit
- PATH reality should be part of the dogfooding checkpoint because SHADOWED after a successful sync is still truthful

Internal drift inside the review:
- the executive judgment and most of the review say prove one repo: keystone-hub
- the later simplification answer drifts toward iterating all non-blocked candidates without a filter
- I am treating the one-repo framing as the stronger center because it is repeated more clearly and aligns with the current narrowing direction

Review 5/5 Gemini 3.1 Pro captured under .keystone/local/8jpmde-sync-m1-review-responses/05-gemini-3-1-pro.md.

Useful additions from 5/5:
- recommends keeping persisted-state read and write together by adding SavePersistedState near LoadPersistedState
- reinforces that current.json is evidence memory, not optional bookkeeping
- adds a concrete narrowing move if we do not want a repo-selection flag yet: temporarily mark non-hub adapters blocked in adapters.yaml
- suggests leaving RepoHead and ActiveBuild blank in M1 if provenance capture adds drag
- wants failure output buffered and truncated into the persisted reason field

Live tension after 5/5:
- several reviews want keystone-hub-only M1
- two reviews are willing to keep the engine loop broad if the manifest is narrowed to hub in practice
- no reviewer is asking for multi-repo proof, launchd, or advanced LKG in this slice

Final review response from GPT Pro captured under .keystone/local/8jpmde-sync-m1-review-responses/06-gpt-pro.md.

Round synthesis after all received responses:
- strong convergence on a narrow M1 centered on keystone-hub
- strong convergence on stage-probe-promote, atomic current.json, fail-closed dirty handling, and a hard dogfood gate
- strong convergence that launchd, broad candidate syncing, advanced LKG history, and multi-output activation stay out of this slice

Most important new guidance from GPT Pro:
- fix the existing status truth bug where a persisted CURRENT state can remain CURRENT even when the tool is not on PATH
- the current hub adapter contract is false today and must be made truthful before sync lands
- narrow sync through manifest readiness rather than a new CLI repo-selection flag
- keep status broad and sync narrow: status can show full inventory while sync judges success on ready adapters only
- strict state validation should land now, not later
- if STALE_LKG and CONTRACT_DRIFT remain public state names, they need real derivation or they should be trimmed back

My current implementation bias after the round:
- add a small rollout-state seam in the manifest: ready, candidate, blocked
- mark only keystone-hub ready for M1
- make the hub adapter truthfully materialize staged output
- fix status so non-resolved PATH can never remain CURRENT
- implement sync for the ready set only
- keep current.json read/write and state validation together
- prove the round trip first: sync writes truth, status reads truth

## Journal

- 2026-04-13T01:18:25Z | Process correction: this slice moved ahead of repo-local stone capture. The gap is now fixed. Upstream review-looped design existed in Blueprint, but repo-local stones were missing until now.

## Lessons

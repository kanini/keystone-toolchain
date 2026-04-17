---
schema: v1
id: 5r6j4d
title: Define sync result contract distinct from status truth surface
status: closed
type: decision
priority: p1
deps: []
tags: [contract, status, sync, ux]
created_at: "2026-04-17T14:55:39Z"
closed_at: "2026-04-17T20:46:36Z"
---
<!-- ksmem:managed: direct edits bypass validation; use ksmem commands -->
`sync` currently performs mutation work but presents its result mostly as a status report. That makes the operator experience confusing: the backend treats sync as an action, but the CLI renders it like a truth dump.

This decision stone should define the canonical `sync` contract in both text and JSON. It needs to answer:
- what `sync` is trying to do
- what a successful sync result looks like
- what belongs in the default summary versus verbose detail
- how `sync` should describe updates, skips, blockers, and next actions without collapsing back into a broad status render
- which fields and schema belong specifically to `sync` rather than `status`

The goal is to make `sync` read like an action result and `status` read like an inspection surface.

## Context

The review loop for `5r6j4d` is now complete.

Round three exposed one grounded remaining issue: the stone was still borrowing broad `StatusSummary` semantics that do not survive ready-set narrowing. After replacing those with sync-native summary rules, removing `summary.unchanged_repo_count`, tightening `summary.updated_repo_count` to an explicit durable tuple comparison, reserving a distinct blocked exit, and clarifying the live-head boundary, the final micro ready-check passed.

All three reviewers in the final micro round judged the stone ready. Gemini judged it ready as written. Opus judged it ready with two small precision edits around the post-derivation meaning of terminal repo state and the same-slice `InitReport.ReadySet` rename. Codex also judged it ready and agreed the remaining edits were review-loop cleanup rather than design changes.

Accepted synthesis:
- keep the `SyncReport` wrapper architecture, post-reload terminal truth, and result-bearing non-success unchanged
- keep `repo_head` as persisted classified-input provenance only; live observed HEAD may inform derived stale state or reason text, but must never overwrite that field
- keep sync summary counts sync-native rather than borrowed from broad `StatusSummary`
- keep delegated init aligned in the same implementation slice rather than allowing a temporary reporting split
- treat the remaining work as implementation, not more review

No further review round is required for `5r6j4d` unless implementation uncovers a direct contradiction in code or doctrine.

## Plan

1. Treat the sync-result review loop as complete.

2. Implement the contract in one slice that keeps the ready-set action surface aligned across `sync`, delegated `init`, and the shared status projection seams.

3. Implementation gates:
- `summary.blocked_repo_count` counts ready repos whose terminal post-derivation `RepoStatus.state` is not `CURRENT`
- `summary.updated_repo_count` compares the pre-mutation persisted snapshot loaded for this invocation with the post-mutation persisted-state reload using the same resolved ready-set manifest, on the tuple `(present, active_build, active_source_kind)`
- `summary.unchanged_repo_count` remains absent from `v1alpha1`
- `contract.ExitReadySetBlocked` is introduced as the single distinct non-zero exit for `completed_with_blockers`
- `InitReport.ReadySet` changes from `*StatusReport` to `*SyncReport` in the same implementation slice that introduces `SyncReport`
- `CollectReadySetManualActions` or its replacement keeps blocker-action selection aligned with the existing `status` / delegated-init repo-state mapping
- `BuildStatusReport` stops overwriting `repo_head` with live setup observation; live observed HEAD may still be used locally to derive stale state or reason text without changing provenance projection
- sync replaces the current `SyncExitCode` path rather than patching it incrementally
- the contract layer gains an explicit result-bearing non-success path instead of bypassing `contract.Success` / `contract.Failure`

4. Validation gates for implementation:
- sync JSON emits `SyncReport`, not raw `StatusReport`
- `completed_with_blockers` is `ok=false` with `result` present, `error` absent, and exit `contract.ExitReadySetBlocked`
- command-level failures still emit no sync-result body
- `status` and `sync.final_status` project the same repo state, reason, PATH-shadow behavior, and `repo_head` meaning for the same ready subset
- delegated init emits the same ready-set reporting contract as sync
- default text renders the sync result summary and one next action without collapsing back into the broad status dump

## Decisions

Accepted synthesis from the final review loop on 2026-04-17:

Adjacent doctrine remains unchanged:
- `status` remains the broad inspection truth surface
- `sync` remains the narrow ready-set mutation command
- dirty-policy and persisted provenance truth remain owned by `a2bwaz`
- setup/overlay truth remains outside this slice
- command-boundary doctrine remains owned by `1vqbne`

Top-level sync result schema:
- schema string: `kstoolchain.sync-report/v1alpha1`
- top-level JSON shape in this slice:
  - `schema`
  - `outcome`
  - `summary`
  - optional `primary_next_action`
  - `final_status`, which is the ready-set-only terminal `StatusReport` detail
- `secondary_notes` is removed from `v1alpha1`
- `SyncReport` is the top-level result for `sync`; raw `StatusReport` is nested detail, not the command result itself
- `InitReport.ReadySet` must change from `*StatusReport` to `*SyncReport` in the same implementation slice that introduces `SyncReport`; landing `SyncReport` without updating delegated init reporting would reopen the `1vqbne` invariant that init and sync must not drift into separate ready-set execution or reporting models

Truth ownership inside `SyncReport`:
- terminal sync truth is the post-sync reload of persisted state projected through the same ready-set `StatusReport` logic used by `status`
- `final_status` is built from the same resolved ready-set manifest snapshot used for the invocation plus the post-sync persisted-state reload and live PATH audit
- active clean-vs-dirty truth inside `final_status` is read from the persisted active promoted-build pair owned by `a2bwaz`; `outcome` and `primary_next_action` must not re-inspect the worktree for that truth
- `status` remains the only broad tracked-inventory inspection surface; `sync` does not include non-ready adapters in its result body
- repo states, reasons, PATH-shadow behavior, and provenance field meanings for the same resolved ready input must match what `status` would project for the ready subset; cross-surface drift between `status` and `sync.final_status` is a contract violation
- top-level sync summary counts are sync-specific action-result truth and do not inherit broad `StatusSummary` semantics by field name alone

Field-semantics freeze for provenance projection:
- `final_status.repos[].repo_head` means the persisted classified-input head from accepted persisted state, not live observed current HEAD from setup probing
- `status` and `sync.final_status` must not overwrite `repo_head` with live setup-probe values during report assembly
- live observed HEAD may still inform derived terminal state or reason text such as `STALE_LKG`, but it is not persisted provenance truth and must never be projected through `repo_head`
- if a future slice needs both persisted classified-input head and live observed current HEAD on the surface at once, the live observation must use a distinct field name rather than overloading `repo_head`

Outcome vocabulary and durable update truth:
- `succeeded`
  - `final_status.summary.overall == CURRENT`
  - no ready output ends `SHADOWED`
  - `summary.updated_repo_count > 0`
- `no_change`
  - `final_status.summary.overall == CURRENT`
  - no ready output ends `SHADOWED`
  - `summary.updated_repo_count == 0`
  - this is the canonical duplicate-action / idempotent-rerun outcome; repeated sync on an already-current ready set must converge here and must not emit a shell-reload hint
- `completed_with_blockers`
  - `final_status.summary.overall != CURRENT`
  - this includes `SETUP_BLOCKED`, `DIRTY_SKIPPED`, `FAILED`, `STALE_LKG`, `CONTRACT_DRIFT`, `UNKNOWN`, and output-driven `SHADOWED`
- command-level failure before terminal result assembly is not a `SyncReport` outcome; it remains a top-level `AppError` path with no sync-result body

Summary derivation rules:
- `summary.ready_repo_count` must equal `final_status.summary.repo_count`
- `summary.blocked_repo_count` is the count of ready repos in `final_status.repos` whose terminal post-derivation `RepoStatus.state` is not `CURRENT`; it is sync-specific action-result truth and must not reuse `final_status.summary.blocked_repo_count`, which remains blocked-adapter inventory truth on the broad status surface
- `summary.shadowed_output_count` is counted from `final_status.repos[].outputs[]` where `state == SHADOWED`
- `summary.updated_repo_count` is derived by comparing the pre-mutation persisted state snapshot loaded for this invocation with the post-mutation persisted-state reload for the same resolved ready-set manifest, on the tuple `(present, active_build, active_source_kind)` for each ready repo
- absence participates in the tuple comparison
- a ready repo counts as updated only when that active promoted-build tuple changed durably across the invocation; stage, probe, attempted promotion, classified-input changes, or live setup observation alone do not count
- `summary.unchanged_repo_count` is removed from `v1alpha1`; unchanged detail remains derivable from `ready_repo_count`, `updated_repo_count`, `blocked_repo_count`, and `final_status`
- `summary.updated_repo_count` is informational only; terminal `outcome`, `ok`, and exit code are still derived from the closed mapping below, not from this count alone

Closed mapping for this slice:
- `succeeded` -> envelope `ok=true`, result present, error absent, exit `contract.ExitOK`
- `no_change` -> envelope `ok=true`, result present, error absent, exit `contract.ExitOK`
- `completed_with_blockers` -> envelope `ok=false`, result present, error absent, exit reserved distinct non-zero constant `contract.ExitReadySetBlocked`
- `contract.ExitReadySetBlocked` is introduced by this slice and must be numerically distinct from `contract.ExitOK`, `contract.ExitValidation`, and every existing `AppError` exit; it is used only for `completed_with_blockers`
- command-level `AppError` -> envelope `ok=false`, result absent, error present, exit `appErr.Exit`
- `result` and `error` are mutually exclusive for `sync`; warnings may accompany either path
- zero ready adapters is a top-level validation `AppError` and emits no `SyncReport`
- post-mutation persisted-state reload failure, terminal result-assembly failure, or any failure after mutation but before `SyncReport` is fully built is a top-level `AppError` and emits no `SyncReport`

Result-versus-error boundary:
- terminal ready-set outcomes emit `SyncReport`
- overlay/config/state-load/state-write failures before terminal result assembly emit only the top-level error envelope and no sync-result body
- the envelope invariant for this command is: `ok` is set from the outcome mapping above rather than from presence of `error`; `result` and `error` remain mutually exclusive
- the existing `contract.Success` / `contract.Failure` split is insufficient for this command and must be extended rather than bypassed so `ok=false` with populated `result` and nil `error` is an explicit supported path
- sync must not fabricate a partial result body for command-level failures that occur before post-sync reload/result assembly succeeds

Primary next-action and shell-hint boundary:
- `primary_next_action` is omitted when terminal ready-set truth yields no operator action
- on `succeeded` with `summary.updated_repo_count > 0`, the shell reload hint is the `primary_next_action`
- on `no_change`, no shell reload hint is emitted
- otherwise `primary_next_action` is the first stable deduped blocker action derived from `final_status` in ready-manifest order
- except for the successful-update shell reload hint, blocker actions must reuse the same repo-state-to-action mapping used by `status` and delegated `init`; sync must not invent a second manual-action catalog
- exact prose remains downstream wording work, but the structural slot and selection rule are owned here

Text and JSON boundary:
- JSON schema is stable and does not change under `--verbose`
- default text is terse and action-oriented and renders only the top-level sync result, compact sync-native counts, operator-visible blocker notes, and one `next:` line from `primary_next_action`
- verbose widens text only and may render the per-repo detail from `final_status`
- default sync text must not reuse the broad `status` header or full tracked-inventory dump

Rejected directions after the final review loop:
- do not keep raw `StatusReport` as the top-level sync result
- do not let `--verbose` mutate JSON schema
- do not decide sync success from pre-reload mutation facts alone
- do not emit both `result` and `error` for the same sync outcome
- do not reuse broad `StatusSummary.blocked_repo_count` semantics for the sync result summary
- do not restore `summary.unchanged_repo_count` to `v1alpha1`
- do not overload `repo_head` with both persisted provenance meaning and live setup observation in the same field
- do not let sync invent a second blocker-action catalog separate from `status` and delegated `init`
- do not land `SyncReport` without the same-slice delegated-init reporting update

## Evidence

Current code-truth evidence still constrains this slice:
- `internal/cli/root.go` still renders sync through `toolchain.RenderStatusText(report)` and only appends one success hint line, proving the action/result split still has not landed.
- `internal/service/service.go` still exposes the key seam: sync mutates, reloads persisted state, narrows to ready adapters, and then returns a report; init still embeds delegated ready-set output as `*StatusReport`.
- `internal/toolchain/status.go` still contains both the broad `StatusReport` shape and the current `SyncExitCode` helper, which is why the review kept focusing on terminal truth and exact exit mapping instead of superficial text changes.
- `internal/contract/contract.go` still exposes the unresolved helper split: `contract.Success` implies `ok=true` with result, `contract.Failure` implies `ok=false` with error and no result. Sync still needs an explicit third shape: `ok=false` with result and no error.
- `internal/toolchain/init.go` still demonstrates the action-result precedent (`InitReport`) and the current delegated ready-set embedding seam (`ReadySet *StatusReport`).
- `internal/toolchain/status.go` still overwrites ready-adapter `repo_head` from live setup probing during report assembly after first reading persisted state. That is now a concrete required code change because this stone freezes `repo_head` as persisted classified-input provenance only.
- `internal/toolchain/status.go` also confirms the summary-count seam that round three had to fix: `StatusSummary.blocked_repo_count` increments only when `adapter_status == blocked`. On a ready-only manifest, that count stays `0` even when ready repos end blocked by `DIRTY_SKIPPED`, `FAILED`, `SETUP_BLOCKED`, `STALE_LKG`, or `SHADOWED`. That is why the top-level sync summary now owns its own blocked count semantics.

External review round 3 evidence:
- Pack `CCJC94` delivered 10 docs and 14,951 tokens.
- Full-pack delivery check passed for all three reviewers:
  - Gemini `exec_599b604e15bfa59ee98ae17e`: 17,575 input tokens
  - Opus `exec_7a68e814cce1132af5a8782c`: 25,235 input tokens
  - Codex `exec_412e2af7eedbe5bb464cbe50`: 380,820 total / 48,916 new input tokens
- Round-three accepted synthesis:
  - keep the wrapper architecture, post-reload terminal truth, delegated init alignment, and result-bearing non-success unchanged
  - replace borrowed broad summary semantics with sync-native summary rules
  - remove `summary.unchanged_repo_count` from `v1alpha1`
  - tighten `summary.updated_repo_count` to the explicit `(present, active_build, active_source_kind)` tuple comparison
  - reserve `contract.ExitReadySetBlocked` as a distinct exit constant for `completed_with_blockers`
  - clarify that live observed HEAD may still inform derived stale state or reason text, but must never overwrite `repo_head`
  - clarify that blocker-action selection reuses the same repo-state-to-action mapping as `status` and delegated `init`
- Round-three outcome:
  - not ready yet after that review
  - one final micro ready-check required

External review round 4 evidence:
- Pack `EZM1YT` delivered 8 docs and 14,142 tokens.
- Full-pack delivery check passed for all three reviewers:
  - Gemini `exec_dd5e81cefb3af07988e4901b`: 16,818 input tokens
  - Opus `exec_7868acf026aeeb6e9833249f`: 24,089 input tokens
  - Codex `exec_9c88034b45d044a9a1480e08`: 322,529 total / 45,537 new input tokens
- All three reviewers judged the revised stone ready.
- Final accepted synthesis:
  - the remaining sync-summary and live-head ambiguities are closed
  - `summary.blocked_repo_count` should be read as terminal post-derivation `RepoStatus.state != CURRENT`
  - `InitReport.ReadySet` must change in the same implementation slice that introduces `SyncReport`
  - the current `repoState.RepoHead = setup.RepoHead` line in `status.go` is evidence of the exact overwrite this stone now forbids and must be removed during implementation
  - no further design iteration is needed; the remaining work is code on the known seams in `service.go`, `status.go`, `contract.go`, and `init.go`

Review-loop outcome:
- `5r6j4d` is implementation-ready after the final micro round
- no further external review round is required unless implementation reveals a direct contradiction in code or doctrine

Implemented the sync action-result contract in code. Added `SyncReport` (`kstoolchain.sync-report/v1alpha1`), explicit result-bearing non-success envelope support with `contract.ExitReadySetBlocked`, pre/post durable update comparison on `(present, active_build, active_source_kind)`, and terse/verbose sync text rendering that no longer reuses the broad status dump. `BuildStatusReport` now preserves persisted `repo_head` meaning instead of overwriting it with live setup HEAD, and delegated `init` now carries `*SyncReport` so the ready-set reporting model stays aligned with `sync`. Validation run: `go test ./internal/toolchain ./internal/service ./internal/cli ./internal/contract` and `go test ./...`, both passing.

## Journal

- 2026-04-17T19:05:42Z | rewrote section decisions (old_lines=0 new_lines=12): Record operator steering on sync-result UX and JSON contract direction before formal review.

- 2026-04-17T19:12:10Z | rewrote section context (old_lines=0 new_lines=20): Formalize 5r6j4d into a review-ready sync-result contract stone grounded in the current code seam and adjacent settled doctrine.

- 2026-04-17T19:12:10Z | rewrote section plan (old_lines=0 new_lines=29): Add a review-loop plan that freezes the sync-result boundary, truth owners, and validation gates before implementation.

- 2026-04-17T19:12:10Z | rewrote section evidence (old_lines=0 new_lines=26): Capture the current code, docs, and adjacent-stone evidence that makes the sync-result contract review necessary now.

- 2026-04-17T19:24:57Z | rewrote section context (old_lines=20 new_lines=23): Synthesize the first external review round and narrow 5r6j4d to the remaining sync-result contract seams before round two.

- 2026-04-17T19:24:57Z | rewrote section plan (old_lines=29 new_lines=35): Replace the exploratory plan with the narrowed second-round contract freeze for sync-result semantics.

- 2026-04-17T19:24:57Z | rewrote section decisions (old_lines=12 new_lines=60): Record the accepted synthesis from review round one and replace provisional operator steering with a stronger first-pass sync-result contract.

- 2026-04-17T19:24:57Z | rewrote section evidence (old_lines=26 new_lines=29): Refresh evidence with current code truth plus the first external review round and the accepted synthesis it produced.

- 2026-04-17T19:24:57Z | Review round 1 completed on 2026-04-17.

Pack:
- pack id: `YVHF25`
- spec: `.keystone/local/5r6j4d-sync-result-review.yaml`
- rendered pack: `.keystone/local/5r6j4d-sync-result-review-pack.md`
- docs: 11
- size: 24,502 tokens / 106,046 bytes

Deterministic context build:
- recipe: `.ctx/recipes/5r6j4d-sync-result-review/recipe.yaml`
- run id: `2026-04-17T19-13-54Z`
- explain: `.ctx/runs/5r6j4d-sync-result-review/2026-04-17T19-13-54Z/explain.md`
- manifest: `.ctx/runs/5r6j4d-sync-result-review/2026-04-17T19-13-54Z/manifest.json`
- budget used: 12 files, 37,559 / 45,000 tokens

Execution ids:
- Gemini: `exec_377f828fedea8bff173764b8`
- Opus: `exec_018b8c58d7d90f917d0c408f`
- Codex: `exec_c1446f0991153ff7603d1dda`

Payload verification:
- Gemini input tokens: 27,980
- Opus input tokens: 41,732
- Codex input tokens: 41,844
- all three executions cleared the full-pack delivery check and were kept in synthesis

Accepted synthesis:
- keep the review boundary narrow and leave dirty-policy, setup/overlay, and init ownership doctrine untouched
- add a dedicated top-level sync result schema instead of returning raw `StatusReport`
- prefer a sync-specific wrapper around ready-set `StatusReport` detail rather than a second repo-detail vocabulary
- derive full success from the post-reload ready-set terminal projection, including PATH-shadow as non-success
- distinguish terminal sync-result outcomes from top-level command errors
- freeze JSON independently from `--verbose`
- make no-change/duplicate-action behavior explicit
- update delegated init reporting in lockstep with the new sync contract

Review outcome:
- not ready to implement yet
- another narrow review round is required after the accepted contract clarifications are written into the stone

- 2026-04-17T19:27:28Z | rewrote section decisions (old_lines=60 new_lines=85): Freeze a concrete proposed SyncReport contract for the second review round based on the accepted synthesis from round one.

- 2026-04-17T19:38:03Z | rewrote section context (old_lines=23 new_lines=28): Synthesize the second external review round and narrow 5r6j4d to the final remaining contract seams before a ready-check round.

- 2026-04-17T19:38:03Z | rewrote section plan (old_lines=35 new_lines=33): Replace the second-round plan with the final contract-freeze steps needed before 5r6j4d can be declared implementation-ready.

- 2026-04-17T19:38:03Z | rewrote section decisions (old_lines=85 new_lines=88): Integrate the accepted synthesis from round two and freeze a near-final SyncReport contract for the ready-check review.

- 2026-04-17T19:38:03Z | rewrote section evidence (old_lines=31 new_lines=35): Refresh evidence with the second external review round and the accepted near-final synthesis for 5r6j4d.

- 2026-04-17T19:38:03Z | Review round 2 completed on 2026-04-17.

Pack:
- pack id: `692WRW`
- spec: `.keystone/local/5r6j4d-sync-result-review-r2.yaml`
- rendered pack: `.keystone/local/5r6j4d-sync-result-review-r2-pack.md`
- docs: 11
- size: 12,569 tokens / 53,955 bytes

Deterministic context build:
- recipe: `.ctx/recipes/5r6j4d-sync-result-review-r2/recipe.yaml`
- run id: `2026-04-17T19-29-18Z`
- explain: `.ctx/runs/5r6j4d-sync-result-review-r2/2026-04-17T19-29-18Z/explain.md`
- manifest: `.ctx/runs/5r6j4d-sync-result-review-r2/2026-04-17T19-29-18Z/manifest.json`
- budget used: 9 files, 32,390 / 38,000 tokens

Execution ids:
- Gemini: `exec_faeee9fdfbe64c83e2be056b`
- Opus: `exec_f7554aaf20fb35c4371322aa`
- Codex: `exec_a76a1f8619bba57b7a1fd121`

Payload verification:
- Gemini input tokens: 14,742
- Opus input tokens: 21,344
- Codex input tokens: 29,911
- all three executions cleared the full-pack delivery check and were kept in synthesis

Accepted synthesis:
- keep the SyncReport wrapper architecture and nested ready-set final_status
- remove `secondary_notes` from v1alpha1
- make `result` and `error` mutually exclusive for sync
- derive `succeeded` versus `no_change` from durable active-pair change plus post-reload terminal truth
- keep zero-ready-adapter and post-reload/result-assembly failure on the top-level AppError path with no result body
- reserve a distinct non-zero exit constant for `completed_with_blockers`
- keep one more narrow ready-check round focused on the explicit outcome/exit/envelope table and durable update derivation

Review outcome:
- not ready to implement yet
- one final narrow review round is still required after the accepted clarifications above are written into the stone

- 2026-04-17T19:53:36Z | Implementation follow-on from `a2bwaz`: `status` still overwrites ready-adapter `repo_head` from the live setup probe while `v1alpha2` now makes persisted provenance-pair meaning load-bearing. When `5r6j4d` freezes the sync/status projection contract, it should explicitly decide whether rendered `repo_head` is live repo observation, persisted classified-input truth, or two separate fields, so the status/sync surfaces do not blur provenance semantics.

- 2026-04-17T19:55:34Z | rewrote section context (old_lines=28 new_lines=20): Add the repo_head projection seam to the final ready-check context so the review judges the actual remaining contract edge.

- 2026-04-17T19:55:34Z | rewrote section plan (old_lines=33 new_lines=39): Add the final repo_head projection freeze to the ready-check plan for 5r6j4d.

- 2026-04-17T19:55:34Z | rewrote section decisions (old_lines=88 new_lines=95): Freeze repo_head projection semantics alongside the near-final SyncReport contract before the final ready-check round.

- 2026-04-17T19:55:34Z | rewrote section evidence (old_lines=35 new_lines=37): Add the repo_head projection seam to the evidence set before the final ready-check review.

- 2026-04-17T20:10:35Z | rewrote section context (old_lines=20 new_lines=9): Synthesize review round 3 and narrow the remaining gap to sync-native summary semantics and the live-head boundary.

- 2026-04-17T20:11:04Z | rewrote section plan (old_lines=39 new_lines=38): Replace the old ready-check plan with the final sync-summary and live-head freeze before one last micro review.

- 2026-04-17T20:11:49Z | rewrote section decisions (old_lines=95 new_lines=100): Integrate review round 3 synthesis, replace borrowed status-summary semantics with sync-native summary rules, and freeze the live-head boundary.

- 2026-04-17T20:12:10Z | rewrote section evidence (old_lines=37 new_lines=33): Refresh evidence with the round 3 review split and the confirmed sync-summary seam in current status.go.

- 2026-04-17T20:18:51Z | rewrote section context (old_lines=9 new_lines=14): Close the review loop after the final micro ready-check and record that the contract is now implementation-ready.

- 2026-04-17T20:19:02Z | rewrote section plan (old_lines=38 new_lines=22): Replace the old next-round plan with implementation gates now that the review loop has converged.

- 2026-04-17T20:19:51Z | rewrote section decisions (old_lines=100 new_lines=101): Absorb the final micro-round precision edits and mark the sync-result contract as implementation-ready.

- 2026-04-17T20:20:17Z | rewrote section evidence (old_lines=33 new_lines=44): Refresh evidence with the final micro ready-check, delivery proof, and accepted convergence on implementation readiness.

- 2026-04-17T20:20:30Z | Final micro review round completed on 2026-04-17.

Pack:
- pack id: `EZM1YT`
- spec: `.keystone/local/5r6j4d-sync-result-review-r4.yaml`
- rendered pack: `.keystone/local/5r6j4d-sync-result-review-r4-pack.md`
- docs: 8
- size: 14,142 tokens / 59,798 bytes

Deterministic context build:
- recipe: `.ctx/recipes/5r6j4d-sync-result-review-r4/recipe.yaml`
- run id: `2026-04-17T20-13-15Z`
- explain: `.ctx/runs/5r6j4d-sync-result-review-r4/2026-04-17T20-13-15Z/explain.md`
- manifest: `.ctx/runs/5r6j4d-sync-result-review-r4/2026-04-17T20-13-15Z/manifest.json`
- budget used: 8 files, 31,895 / 32,000 tokens

Execution ids:
- Gemini: `exec_dd5e81cefb3af07988e4901b`
- Opus: `exec_7868acf026aeeb6e9833249f`
- Codex: `exec_9c88034b45d044a9a1480e08`

Payload verification:
- Gemini input tokens: 16,818
- Opus input tokens: 24,089
- Codex input tokens: 322,529 total / 45,537 new
- all three executions cleared the full-pack delivery check and were kept in synthesis

Accepted synthesis:
- keep the wrapper architecture, post-reload terminal truth, delegated init alignment, and result-bearing non-success unchanged
- keep sync summary counts sync-native rather than borrowed from broad `StatusSummary`
- keep `repo_head` as persisted classified-input provenance only; live observed HEAD may inform derived stale state or reason text but must never overwrite that field
- keep `InitReport.ReadySet` aligned in the same implementation slice that introduces `SyncReport`
- treat the remaining work as implementation on the known `service.go`, `status.go`, `contract.go`, and `init.go` seams

Review outcome:
- implementation-ready after this revision
- no further external review round is required unless implementation uncovers a direct contradiction in repo truth

- 2026-04-17T20:46:36Z | Implemented and validated. `sync` now reports a dedicated action result instead of reusing the broad status surface, and delegated `init` stays aligned in the same slice.

## Lessons

- When a command can finish with actionable blockers after valid mutation work, give it an explicit result-bearing non-success contract instead of reusing either a broad inspection report or a pure error envelope.

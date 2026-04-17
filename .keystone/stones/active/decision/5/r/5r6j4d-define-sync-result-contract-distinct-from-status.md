---
schema: v1
id: 5r6j4d
title: Define sync result contract distinct from status truth surface
status: open
type: decision
priority: p1
deps: []
tags: [contract, status, sync, ux]
created_at: "2026-04-17T14:55:39Z"
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

The second external review round converged that `5r6j4d` is no longer missing architecture. It is missing only a few contract-grade freeze points.

What reviewers now agree on:
- the dedicated top-level `SyncReport` wrapper is the right shape
- raw `StatusReport` should remain only nested ready-set detail, not the top-level sync result
- terminal sync truth must come from the post-sync reload of persisted state projected through the same ready-set `StatusReport` logic used by `status`
- `SHADOWED` remains non-success and blocks full success
- JSON must stay stable under `--verbose`
- delegated `init` must move to the same sync contract in lockstep

The remaining disagreements are now narrow and explicit:
- how the envelope expresses terminal non-success with a populated result body
- whether `completed_with_blockers` needs its own reserved non-zero exit constant rather than reusing the generic validation exit
- how `updated_repo_count` is derived durably enough to split `succeeded` from `no_change`
- whether under-defined fields like `secondary_notes` should remain in `v1alpha1`

Accepted synthesis from round two:
- keep the `SyncReport` wrapper architecture
- keep `result` and `error` mutually exclusive for `sync`
- make `completed_with_blockers` a terminal sync-result outcome with `ok=false`, populated `result`, and no top-level error envelope
- keep pre-ready-set failures, zero-ready-adapter failure, and post-mutation reload/result-assembly failure on the top-level `AppError` path with no `SyncReport`
- derive `succeeded` versus `no_change` from durable post-run truth, not from attempted work or other loose mutation facts
- remove `secondary_notes` from `v1alpha1` unless a real truth owner is defined

So the next round, if needed, is a final ready-check over three specific freezes:
- durable update-truth derivation for `updated_repo_count`
- the closed `outcome -> ok -> exit -> result/error` table
- the structural next-action and shell-hint rules after `secondary_notes` removal

## Plan

1. Freeze durable action truth:
- `updated_repo_count` is derived from durable pre-sync versus post-sync active promoted build-pair change for each ready repo, not from attempted work
- `succeeded` versus `no_change` is derived from `final_status.summary.overall` plus that durable `updated_repo_count`

2. Freeze the terminal outcome table completely:
- `succeeded` -> `ok=true`, exit 0, result present, error absent
- `no_change` -> `ok=true`, exit 0, result present, error absent
- `completed_with_blockers` -> `ok=false`, reserved non-zero ready-set-blocked exit, result present, error absent
- top-level `AppError` -> `ok=false`, appErr exit, result absent, error present
- zero ready adapters -> top-level validation `AppError`, no `SyncReport`
- post-mutation reload/result-assembly failure -> top-level `AppError`, no `SyncReport`

3. Freeze structural surface rules:
- delete `secondary_notes` from `v1alpha1`
- keep one structural `primary_next_action` slot only
- on `succeeded` with `updated_repo_count > 0`, the shell reload hint is the primary next action
- on `no_change`, omit the shell reload hint
- otherwise derive `primary_next_action` deterministically from ready-set terminal truth in stable ready-manifest order

4. Freeze cross-surface alignment:
- `SyncReport.final_status` must match what `status` would project for the same ready subset
- delegated `init` must embed `*SyncReport` and reuse the same outcome/exit/manual-action rules
- no sync path may fabricate a terminal result body from pre-reload data

5. Run one final ready-check review round after these edits.

Validation gates for the next round:
- durable update-count derivation is explicit
- `result` and `error` exclusivity is explicit
- zero-ready-adapter and post-reload-failure paths are explicit
- `completed_with_blockers` has a named reserved exit constant distinct from generic validation failure
- `secondary_notes` is gone from `v1alpha1`
- shell-hint and `primary_next_action` rules are explicit and deterministic

## Decisions

Accepted synthesis from the second external review round on 2026-04-17:

Adjacent doctrine remains unchanged:
- `status` remains the broad inspection truth surface
- `sync` remains the narrow ready-set mutation command
- dirty-policy and persisted provenance truth remain owned by `a2bwaz`
- setup/overlay truth and `SETUP_BLOCKED` remain owned by `gqksyd`
- command-boundary doctrine remains owned by `1vqbne`

Top-level sync result schema:
- schema string: `kstoolchain.sync-report/v1alpha1`
- top-level JSON shape in this slice:
  - `schema`
  - `outcome`
  - `summary`
  - optional `primary_next_action`
  - `final_status`, which is the ready-set-only terminal `StatusReport` detail
- `secondary_notes` is removed from `v1alpha1`; it does not yet have a strong enough truth owner to freeze as contract surface
- `SyncReport` is the top-level result for `sync`; raw `StatusReport` is nested detail, not the command result itself
- `InitReport.ReadySet` must change from `*StatusReport` to `*SyncReport` when this contract lands so delegated ready-set reporting stays aligned with the sync surface

Truth ownership inside `SyncReport`:
- terminal sync truth is the post-sync reload of persisted state projected through the same ready-set `StatusReport` logic used by `status`
- `final_status` is built from the same resolved ready-set manifest snapshot used for the invocation plus the post-sync persisted-state reload and live PATH audit
- full success, blockers, and `primary_next_action` selection are derived from that post-reload ready-set terminal truth
- active clean-vs-dirty facts inside `final_status` are read from the persisted active promoted build pair owned by `a2bwaz`; `primary_next_action` selection and `outcome` classification must not re-inspect the worktree
- `status` remains the only broad tracked-inventory inspection surface; `sync` does not include non-ready adapters in its result body
- `final_status` state vocabulary, reasons, and counts for the same resolved adapter input must match what `status` would render for the ready subset; cross-surface drift between `status` and `sync.final_status` is a contract violation

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
- `summary.blocked_repo_count` must equal `final_status.summary.blocked_repo_count`
- `summary.shadowed_output_count` is counted from `final_status.repos[].outputs[]` where `state == SHADOWED`
- `summary.updated_repo_count` is derived by comparing the accepted pre-sync persisted active promoted build pair (`active_build` + `active_source_kind`) for each ready repo with the post-sync reloaded persisted active promoted build pair for that same repo
- a ready repo counts as updated only when that active promoted build pair changed durably across the invocation; stage, probe, attempted promotion, or classified-input changes alone do not count
- `summary.unchanged_repo_count` equals `summary.ready_repo_count - summary.updated_repo_count`
- `summary.updated_repo_count` is informational only; terminal `outcome`, `ok`, and exit code are still derived from the closed mapping below, not from this count alone

Closed mapping for this slice:
- `succeeded` -> envelope `ok=true`, result present, error absent, exit `contract.ExitOK`
- `no_change` -> envelope `ok=true`, result present, error absent, exit `contract.ExitOK`
- `completed_with_blockers` -> envelope `ok=false`, result present, error absent, exit reserved distinct non-zero constant `contract.ExitReadySetBlocked`
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
- otherwise `primary_next_action` is the first stable deduped manual action derived from `final_status` in ready-manifest order
- exact prose remains downstream wording work, but the structural slot and selection rule are owned here

Text and JSON boundary:
- JSON schema is stable and does not change under `--verbose`
- default text is terse and action-oriented and should render only the top-level sync result, compact counts, operator-visible blocker notes, and one `next:` line from `primary_next_action`
- verbose widens text only and may render the per-repo detail from `final_status`
- default sync text must not reuse the broad `status` header or full tracked-inventory dump

Rejected directions after round two:
- do not keep raw `StatusReport` as the top-level sync result
- do not let `--verbose` mutate JSON schema
- do not decide sync success from pre-reload mutation facts alone
- do not emit both `result` and `error` for the same sync outcome
- do not leave init delegated ready-set output on a separate schema path once sync changes its top-level contract

## Evidence

Current code-truth evidence still constrains this slice:
- `internal/cli/root.go` still renders sync through `toolchain.RenderStatusText(report)` and only appends one success hint line, proving the action/result split still has not landed.
- `internal/service/service.go` still exposes the key seam: sync mutates, reloads persisted state, narrows to ready adapters, and then returns a report; init still embeds delegated ready-set output as `*StatusReport`.
- `internal/toolchain/status.go` still contains both the broad `StatusReport` shape and the current `SyncExitCode` helper, which is why the review kept focusing on terminal truth and exact exit mapping instead of superficial text changes.
- `internal/contract/contract.go` still exposes the unresolved helper split: `contract.Success` implies `ok=true` with result, `contract.Failure` implies `ok=false` with error and no result. Round-two reviewers converged that sync needs an explicit third shape: `ok=false` with result and no error.
- `internal/toolchain/init.go` still demonstrates the action-result precedent (`InitReport`) and the current delegated ready-set embedding seam (`ReadySet *StatusReport`).

External review round 2 evidence:
- Pack `692WRW` delivered 11 docs and 12,569 tokens.
- Full-pack delivery check passed for all three reviewers:
  - Gemini `exec_faeee9fdfbe64c83e2be056b`: 14,742 input tokens
  - Opus `exec_f7554aaf20fb35c4371322aa`: 21,344 input tokens
  - Codex `exec_a76a1f8619bba57b7a1fd121`: 29,911 input tokens
- All three reviewers again judged the stone not ready, but all three also converged that the architecture is now right and the remaining work is purely contract freeze.

Accepted reviewer convergence from round two:
- Gemini confirmed the wrapper architecture, outcome vocabulary, terminal truth owner, and JSON stability, and narrowed its remaining concern to the exact shape of `ok=false` terminal non-success in the envelope.
- Opus confirmed the wrapper architecture, terminal truth owner, and removal of drift with `final_status`, and narrowed its remaining concerns to exact exit-code reservation, zero-ready-adapter path, and explicit envelope invariants.
- Codex confirmed the wrapper architecture and boundary, and narrowed its remaining concern to durable update-truth derivation, result/error exclusivity, and removal of under-defined fields like `secondary_notes`.
- Opus and Codex both converged that `completed_with_blockers` should not share structural path with top-level `AppError`.
- Gemini and Codex both converged that `succeeded` versus `no_change` must depend on durable post-run truth rather than attempted work.
- Gemini and Opus both converged that the remaining work is no longer another architecture round; it is just freezing the envelope/result mapping tightly enough to implement.

Accepted synthesis from round two:
- keep the `SyncReport` wrapper and nested ready-set `final_status`
- remove `secondary_notes` from `v1alpha1`
- make `result` and `error` mutually exclusive for sync
- derive `succeeded` versus `no_change` from durable update truth plus post-reload terminal state
- keep zero-ready-adapter and post-reload/result-assembly failure on the top-level `AppError` path with no sync result body
- reserve a distinct non-zero exit constant for `completed_with_blockers` so shell callers can distinguish terminal ready-set non-success from top-level command validation failures

Round-two outcome:
- `5r6j4d` is still not implementation-ready
- only one narrow ready-check review round should remain after these accepted clarifications are reviewed
- if the next round agrees with the explicit table above, the stone should be ready to implement without another redesign pass

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

## Lessons

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

The first external review round converged quickly on one conclusion: the stone boundary is correct, but the contract is still under-specified for implementation.

What reviewers agreed on:
- `sync` really is a different command from `status`; the problem is no longer discovery, it is contract freeze.
- this stone must stay narrow and must not reopen dirty-policy, setup/overlay, rollout, or init ownership doctrine.
- a sync-specific result surface is required; continuing to return raw `StatusReport` would leave the surface too easy to drift back into broad inspection semantics.
- the missing gaps are load-bearing, not cosmetic: outcome vocabulary, `ok`/exit mapping, terminal truth owner, result-versus-error boundary, no-change semantics, and init alignment.

Accepted synthesis from round one:
- `sync` should return a dedicated `SyncReport` schema rather than reusing raw `StatusReport` as the top-level result.
- the safest low-duplication shape is a sync-specific wrapper over ready-set terminal detail, not a second invented per-repo vocabulary. In other words: sync owns top-level action-result semantics; the nested per-repo detail reuses ready-set `StatusReport` truth.
- terminal sync truth must be derived from the post-sync reload of persisted state plus the same PATH audit/status projection used by `status`, not from pre-reload mutation facts alone.
- full success must be decided from that post-reload ready-set truth, which means PATH-shadowed outputs cannot read as full success.
- command-level pre-ready-set failures remain top-level `AppError` failures and emit no sync-result body.
- terminal non-success outcomes still need a sync-result body, which means the current `contract.Success` / `contract.Failure` helper split is too weak for this command and must be narrowed or extended.
- JSON shape must stay stable across `--verbose`; verbose is a text-rendering concern, not a schema mutation.
- `init` delegated ready-set reporting must be updated in lockstep once `sync` stops returning raw `StatusReport`.

So the next review round is not another broad design pass. It is a contract-freeze pass over four specific things:
- the sync outcome-class vocabulary, including explicit no-change behavior
- the closed mapping from outcome to envelope `ok` to exit code
- the exact top-level `SyncReport` shape and its relationship to nested ready-set detail
- the emitted-result versus top-level-error boundary

## Plan

1. Freeze the top-level sync result contract explicitly:
- `sync` returns a dedicated `SyncReport` schema, distinct from raw `StatusReport`
- the result is a wrapper around ready-set terminal detail rather than a new per-repo detail vocabulary
- the nested terminal detail is a ready-set-only `StatusReport` projection built after sync reloads persisted state

2. Freeze the action outcome table:
- explicit outcome classes owned by this stone
- explicit no-change / duplicate-action behavior
- closed mapping from outcome class to top-level envelope `ok`
- closed mapping from outcome class to exit code
- explicit statement that full success is computed from post-reload ready-set truth and therefore includes PATH-shadow as non-success

3. Freeze the result-versus-error boundary:
- terminal ready-set outcomes emit `SyncReport`
- pre-ready-set config/overlay/state-load/state-write failures emit top-level `AppError` only and no sync-result body
- this stone must state how terminal non-success outcomes keep a result body even when top-level `ok=false`

4. Freeze the surface boundaries:
- JSON schema is stable and unaffected by `--verbose`
- default text is terse and action-oriented
- verbose widens text only
- one structural `primary_next_action` slot exists, with exact prose still left to downstream wording work
- delegated init reporting must consume the same `SyncReport` contract

5. Run one narrower second review round after those edits.

Validation gates for the next round:
- one clear truth owner for terminal sync success and blocker truth
- one named `SyncReport` schema
- one closed outcome -> `ok` -> exit mapping
- explicit no-change semantics
- explicit rule that command-level failures emit no sync result
- explicit rule that PATH-shadowed ready outputs block full success
- explicit rule that JSON does not change under `--verbose`
- explicit rule that `init` delegated ready-set output stays aligned with the new sync contract

## Decisions

Accepted synthesis from the first external review round on 2026-04-17:

- Keep all adjacent doctrine boundaries unchanged:
  - `status` remains the broad inspection truth surface
  - `sync` remains the narrow ready-set mutation command
  - dirty-policy and persisted provenance truth remain owned by `a2bwaz`
  - setup/overlay truth and `SETUP_BLOCKED` remain owned by `gqksyd`
  - command-boundary doctrine remains owned by `1vqbne`

- `sync` needs its own top-level result schema. Reusing raw `StatusReport` at the top level is rejected because it leaves the contract too easy to drift back into broad-inventory semantics.

- The accepted direction is a dedicated sync wrapper rather than a second invented repo-detail vocabulary:
  - top-level sync semantics are action-result semantics owned by this stone
  - nested terminal repo/output detail reuses ready-set-only `StatusReport` truth after the sync path reloads persisted state

Proposed contract to validate in round two:

Top-level sync result schema:
- schema string: `kstoolchain.sync-report/v1alpha1`
- top-level JSON shape:
  - `schema`
  - `outcome`
  - `summary`
  - `primary_next_action`
  - optional `secondary_notes`
  - `final_status`, which is the ready-set-only terminal `StatusReport` detail
- `SyncReport` is the top-level result for `sync`; raw `StatusReport` is nested detail, not the command result itself
- `InitReport.ReadySet` must change from `*StatusReport` to `*SyncReport` when this contract lands so delegated ready-set reporting stays aligned with the sync surface

Truth ownership inside `SyncReport`:
- terminal sync truth is the post-sync reload of persisted state projected through the same ready-set `StatusReport` logic used by `status`
- full success, blockers, and primary next-action selection are derived from that post-reload ready-set terminal truth
- pre-reload mutation facts may inform invocation-local counts such as `updated_repo_count`, but they are not the truth owner for top-level `outcome`, top-level `ok`, or exit code
- PATH-shadowed ready outputs are part of terminal truth and therefore prevent full success
- `status` remains the only broad tracked-inventory inspection surface; `sync` does not include non-ready adapters in its default result body

Outcome vocabulary proposed for validation in round two:
- `succeeded`
  - every ready repo ends `CURRENT` in `final_status`
  - no ready output ends `SHADOWED`
  - at least one ready repo advanced in the current invocation
- `no_change`
  - every ready repo ends `CURRENT` in `final_status`
  - no ready output ends `SHADOWED`
  - no ready repo advanced in the current invocation
- `completed_with_blockers`
  - the command reached a terminal ready-set result, but at least one ready repo or ready output ends non-success in `final_status`
  - this includes `SETUP_BLOCKED`, `DIRTY_SKIPPED`, `FAILED`, `STALE_LKG`, `CONTRACT_DRIFT`, `UNKNOWN`, and output-driven `SHADOWED`
- command-level failure before terminal result assembly is not a `SyncReport` outcome; it remains a top-level `AppError` path with no sync-result body

Closed mapping proposed for validation in round two:
- `succeeded` -> envelope `ok=true`, exit 0
- `no_change` -> envelope `ok=true`, exit 0
- `completed_with_blockers` -> envelope `ok=false`, non-zero exit, with populated `SyncReport`
- command-level `AppError` -> envelope `ok=false`, non-zero exit, no `SyncReport`
- zero ready adapters stays a command-level non-success, not a green no-op sync result

Result-versus-error boundary:
- terminal ready-set outcomes emit `SyncReport`
- overlay/config/state-load/state-write failures before terminal result assembly emit only the top-level error envelope and no sync-result body
- terminal non-success outcomes still need a result body, so the current `contract.Success` / `contract.Failure` helper split is insufficient for this command and must be narrowed or extended during implementation
- sync must not fabricate a partial result body for command-level failures that occur before post-sync reload/result assembly succeeds

Summary and boundary fields proposed for validation in round two:
- `summary.ready_repo_count`
- `summary.updated_repo_count`
- `summary.unchanged_repo_count`
- `summary.blocked_repo_count`
- `summary.shadowed_output_count`
- `primary_next_action` is a single structural slot selected from terminal ready-set truth; exact prose remains downstream wording work
- `secondary_notes` may carry compact supporting notes, but must not compete with `primary_next_action`

Text and JSON boundary:
- JSON schema is stable and does not change under `--verbose`
- default text is terse and action-oriented and should render only the top-level sync result, compact counts, operator-visible blocker notes, and one `next:` line from `primary_next_action`
- verbose widens text only and may render the per-repo detail from `final_status`
- default sync text must not reuse the broad `status` header or full tracked-inventory dump
- shell reload hints belong only on `succeeded` runs that actually advanced managed outputs in the current invocation; they do not belong on `no_change` or `completed_with_blockers`

Round-one rejected directions remain in force:
- do not keep raw `StatusReport` as the top-level sync result
- do not let `--verbose` mutate JSON schema
- do not decide sync success from pre-reload mutation facts alone
- do not let top-level command errors and terminal sync-result outcomes share the same structural path
- do not leave init delegated ready-set output on a separate schema path once sync changes its top-level contract

## Evidence

Current code-truth evidence still matters:
- `internal/cli/root.go` documents distinct command intent, but `sync` still renders `toolchain.RenderStatusText(report)` just like `status`, differing only by one appended success hint line.
- `internal/service/service.go` still distinguishes behavior operationally: `StatusReport()` inspects only, while `executeReadySetSync()` mutates, reloads persisted state, narrows to ready adapters, and then returns a `StatusReport` plus `SyncExitCode`.
- `internal/toolchain/status.go` still holds the broad report shape and renderer, while `SyncExitCode` currently checks persisted repo state rather than the full PATH-derived terminal report truth. That is exactly why reviewers pushed to freeze terminal success off the post-reload ready-set projection instead of the pre-existing helper alone.
- `internal/contract/contract.go` still exposes the load-bearing helper mismatch: `contract.Success(...)` assumes `ok=true` for any non-error result, while `contract.Failure(...)` drops the result payload entirely. Reviewers converged that `sync` needs a terminal non-success path with `ok=false` and a populated result body.
- `internal/toolchain/init.go` plus the init CLI path prove the repo already supports dedicated action-result surfaces (`InitReport` plus `RenderInitText`), so a sync-specific wrapper is a natural extension rather than a new pattern.

Current docs-truth evidence still holds:
- `README.md` and `BUILDING.md` both describe `sync` as a narrow ready-set mutation command and `status` as the inspection/truth surface. That makes the shared rendering path a real contract mismatch, not just a formatting nit.

External review round 1 evidence:
- Pack `YVHF25` delivered 11 docs and 24,502 tokens.
- Full-pack delivery check passed for all three reviewers:
  - Gemini `exec_377f828fedea8bff173764b8`: 27,980 input tokens
  - Opus `exec_018b8c58d7d90f917d0c408f`: 41,732 input tokens
  - Codex `exec_c1446f0991153ff7603d1dda`: 41,844 input tokens
- All three reviewers judged the stone not ready, but their disagreements were narrowing, not architectural.

Accepted reviewer convergence from round one:
- Gemini identified the contract-envelope hole directly: terminal non-success needs `ok=false` with a real result body, which the current helper split does not provide.
- Opus identified the missing contract table directly: outcome-class vocabulary, outcome -> `ok` -> exit mapping, explicit schema identity, and explicit no-change behavior all still need to be frozen.
- Codex identified the terminal-truth seam directly: full success must be computed from the post-reload ready-set projection, not just persisted repo rows or pre-reload mutation facts, and this includes PATH-shadow as non-success.
- Opus and Codex both converged on the structural simplification that the sync surface should be a sync-specific wrapper over ready-set status detail rather than a second independent repo-detail schema.
- Gemini and Codex both converged that delegated `init` output must change in lockstep once sync has a distinct top-level contract.

Round-one outcome:
- `5r6j4d` is not implementation-ready yet
- the remaining work is now narrow and explicit
- another review round is required after the accepted contract clarifications above are written into the stone

Dogfood confirmation from 2026-04-17 on /home/aj/git/keystone-toolchain: with isolated HOME and a clean real /home/aj/git/keystone-hub checkout, real `kstoolchain init` bootstrapped the machine, wrote the overlay, delegated once into the shared ready-set path, wrote current.json, and then exited non-zero because the delegated terminal ready-set truth was SHADOWED. In the same environment, `kstoolchain sync` later exited 0 while still rendering SHADOWED because persisted repo state was CURRENT and PATH still resolved `kshub` to /home/aj/.local/share/mise/installs/go/1.26.0/bin/kshub instead of the managed bin. Once PATH was explicitly prepended with the managed bin, `kstoolchain status` became CURRENT. This is a concrete repro of the current mismatch between sync exit semantics and post-reload visible truth.

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

## Lessons

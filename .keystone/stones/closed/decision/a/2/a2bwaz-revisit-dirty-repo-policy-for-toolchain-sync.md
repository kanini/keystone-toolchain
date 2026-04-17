---
schema: v1
id: a2bwaz
title: Revisit dirty-repo policy for toolchain sync
status: closed
type: decision
priority: p3
deps: []
tags: [dirty-policy, review-loop, sync, ux]
created_at: "2026-04-17T14:55:40Z"
closed_at: "2026-04-17T19:53:36Z"
---
<!-- ksmem:managed: direct edits bypass validation; use ksmem commands -->
Current reviewed doctrine says dirty ready repos fail closed during sync. That trust posture is explicit in the local design memory and currently implemented.

Operator feedback raises a legitimate follow-up question: is the current fail-closed policy still the right default for a multi-repo toolchain manager, or should sync bias toward updating everything clean while classifying dirty repos as skipped blockers rather than suite-level failure?

This stone exists only to reopen that doctrine deliberately rather than letting the question hide inside implementation tweaks.

Questions to settle:
- should dirty repos still make sync exit non-zero by default
- should the suite be considered partially successful when clean repos update and dirty repos are skipped
- do we need a stricter mode and a more permissive default mode, or is one global policy enough
- what tradeoff best matches the trust posture of a toolchain manager used across many repos

If we keep fail-closed, the result should still sharpen the explanation. If we change policy, the change should be explicit and reviewed.

## Context

The fifth external review round reduced the remaining disagreement to one narrow compatibility boundary and three small precision edits.

Gemini judged the stone fully ready as written. Opus judged it ready with three surgical clarifications: classify git sampling or `git status --porcelain` failure as not trustworthy classification, bind setup classification ahead of dirtiness classification, and state that `last_attempt_at` is a suite-level timestamp rather than part of either per-repo provenance pair. Codex still judged it not ready, but only because the stone named `kstoolchain.state/v1alpha2` without explicitly freezing what a `v1alpha2` reader does when it encounters an existing `kstoolchain.state/v1alpha1` file.

That remaining Codex concern is in scope and accepted here. The current code already exact-matches the schema string and fails closed on mismatch; the stone now needs to own that behavior explicitly rather than leaving implementation to invent migration, backfill, or treat-as-missing fallback semantics.

The accepted round-5 synthesis therefore is:
- keep the round-4 truth model, schema bump, provenance-pair rules, and promotion-boundary revalidation unchanged
- explicitly state that `kstoolchain.state/v1alpha2` readers reject legacy `kstoolchain.state/v1alpha1` files with no compatibility read path, source-kind backfill, or treat-as-missing fallback
- explicitly state that git sampling or `git status --porcelain` failure is not trustworthy classification and therefore preserves the prior classified input pair exactly
- explicitly bind setup classification before dirtiness classification so `SETUP_BLOCKED` and `DIRTY_SKIPPED` remain mutually exclusive in one run
- explicitly state that `last_attempt_at` is suite-level and not part of either per-repo provenance pair

With those edits, implementation no longer needs to invent any compatibility or classification behavior inside this slice. Broader interrupted-attempt durability, locking, and crash-recovery protocol remain outside this stone with `hq97he` and `c580kd`.

## Plan

1. Freeze the compatibility boundary completely:
- dirty-source provenance bumps the persisted schema from `kstoolchain.state/v1alpha1` to `kstoolchain.state/v1alpha2`
- readers that recognize `kstoolchain.state/v1alpha2` also reject legacy `kstoolchain.state/v1alpha1` files
- this slice provides no read-time migration, no `active_source_kind` / `last_attempt_source_kind` backfill from legacy fields, and no treat-as-missing fallback for legacy state
- when a legacy `kstoolchain.state/v1alpha1` file is present, `status` and `sync` fail closed through the existing invalid-state-file path until the operator removes that file and a fresh `kstoolchain.state/v1alpha2` file is written
- old readers reject `kstoolchain.state/v1alpha2`
- missing, unknown, or empty source-kind values remain invalid under the new schema and never default to clean

2. Freeze trustworthy classification rules precisely:
- a trustworthy classification requires both an observed git head and a definitive clean-or-dirty verdict from one observation record
- git sampling or `git status --porcelain` failure is not trustworthy classification and preserves the prior classified input pair exactly, including pair absence
- setup classification precedes dirtiness classification, so `SETUP_BLOCKED` and `DIRTY_SKIPPED` are mutually exclusive in one run
- `last_attempt_at` remains a suite-level timestamp and is not part of either per-repo provenance pair

3. Freeze provenance write rules and promotion-boundary revalidation:
- classified input pair updates only after trustworthy classification
- active promoted build pair updates only on successful promotion
- immediately before the artifact promotion step from the stage dir into the managed bin dir, re-observe git head and re-run the same `git status --porcelain` cleanliness check used at classification
- `clean_head` active truth still requires the same head and a still-clean worktree at that boundary
- `dirty_worktree` active truth still requires the same head plus an already-recorded dirty classified input pair from the current run
- if the promotion-boundary check fails, stay on the existing stale-last-known-good or failed path and do not persist a new active pair

4. Keep validation and ownership boundaries explicit:
- validation remains a closed persisted repo-state / provenance matrix for the states this slice writes
- prior-state preservation rules apply only after the prior state file has already been accepted as valid `kstoolchain.state/v1alpha2`
- `5r6j4d` still owns sync/status projection field names, envelope `ok`, warnings, and exact exit-code semantics
- `zs7xv0` still owns operator wording and grouping
- `hq97he` and `c580kd` still own broader interrupted-attempt durability, locking, and crash-recovery protocol

5. End the review loop for this stone after these edits. The next work is implementation plus the separate sync-attempt-integrity track, not another dirty-policy review round.

Validation gates:
- the legacy `v1alpha1` read boundary is explicit and fail-closed
- trustworthy classification requires one observation record with a definitive verdict
- classification failure explicitly preserves the prior classified input pair
- setup-before-dirty ordering and suite-level `last_attempt_at` semantics are explicit
- promotion-boundary revalidation location and checks remain explicit
- validation matrix remains closed for sync-written states in this slice
- interrupted-attempt durability remains explicitly outside this stone

## Decisions

Accepted synthesis from the fifth external review round on 2026-04-17:

- The product direction remains fully settled and unchanged:
  - default dirty handling remains protective, repo-scoped, and non-zero
  - clean ready repos continue when another ready repo is dirty
  - the only expert override in this slice is one-shot suite-wide `sync --allow-dirty`
  - no config default, env var, named mode, or repo-targeted dirty override is added here

- `DIRTY_SKIPPED` remains the repo-level persisted state for default-mode dirty blockers. Completed-with-blockers remains a sync-run outcome owned by `5r6j4d`, not a repo-state enum.

- Dirty-built success still persists as `CURRENT` plus provenance. Reject new repo-state constants such as `DIRTY_PROMOTED` or `CURRENT_DIRTY`.

- Dirty-source provenance bumps the persisted schema from `kstoolchain.state/v1alpha1` to `kstoolchain.state/v1alpha2`.
  - readers that do not recognize `kstoolchain.state/v1alpha2` reject the state file
  - readers that do recognize `kstoolchain.state/v1alpha2` also reject persisted files with schema `kstoolchain.state/v1alpha1`
  - this slice provides no compatibility read path, no migration, no source-kind backfill from legacy fields, and no treat-as-missing fallback for legacy state
  - when a legacy `kstoolchain.state/v1alpha1` file is present, `status` and `sync` fail closed through the existing invalid-state-file path until the operator removes that file and a fresh `kstoolchain.state/v1alpha2` file is written
  - writers do not emit `active_source_kind` or `last_attempt_source_kind` under `kstoolchain.state/v1alpha1`
  - under `kstoolchain.state/v1alpha2`, missing, unknown, or empty source-kind values are invalid; there is no silent defaulting, best-effort inference, or empty-string-as-clean fallback
  - prior-state preservation rules in this stone apply only after the prior state file has already been accepted as valid `kstoolchain.state/v1alpha2`

- Persisted repo state owns dirty-source truth using two atomic provenance pairs:
  - classified input pair: `repo_head` + `last_attempt_source_kind`
  - active promoted build pair: `active_build` + `active_source_kind`

- Classified input pair rules:
  - a trustworthy source classification is one observation record with both an observed git head and a definitive clean-or-dirty verdict; implementations must not combine `repo_head` from one git sample with `last_attempt_source_kind` from another
  - a git sampling or `git status --porcelain` error is not a trustworthy classification; runs that fail to observe both a git head and a definitive clean-or-dirty verdict preserve the prior classified input pair exactly, including pair absence
  - setup classification precedes dirtiness classification, so `SETUP_BLOCKED` and `DIRTY_SKIPPED` are mutually exclusive in one run
  - `last_attempt_at` is a suite-level timestamp and is not part of either per-repo provenance pair
  - `repo_head` and `last_attempt_source_kind` update together only after the run has both observed a git head and successfully classified the input as `clean_head` or `dirty_worktree`
  - setup-blocked paths and failures before trustworthy source classification preserve the prior classified input pair
  - default dirty skips write the classified input pair as dirty
  - clean runs write the classified input pair as clean once classification succeeds, even if a later install, probe, or promote step fails
  - dirty-allowed attempts write the classified input pair as dirty once dirtiness has been observed, even if a later install, probe, or promote step fails

- Active promoted build pair rules:
  - `active_build` and `active_source_kind` update together only when promotion succeeds
  - default dirty skips, setup blocks, and failed promotions preserve the prior active promoted build pair
  - dirty-allowed failure with a prior active build stays on the existing stale-last-known-good path while preserving that prior active pair
  - dirty-allowed failure with no prior active build stays on the existing `FAILED` path and does not invent an active pair

- Promotion-boundary revalidation is mandatory before new active truth becomes durable:
  - immediately before the artifact promotion step from the stage dir into the managed bin dir, re-observe git head and re-run the same `git status --porcelain` cleanliness check used at classification
  - a run may persist `active_source_kind=clean_head` only if both re-checks confirm the same head and a still-clean worktree at that boundary
  - a run may persist `active_source_kind=dirty_worktree` only if the head re-check confirms the same head and the current run already recorded a dirty classified input pair; this slice still does not claim exact dirty diff equality
  - if the promotion-boundary check fails, the run falls back to the existing stale-last-known-good or failed path and must not persist a new active promoted build pair

- Replay and recovery rules for this slice:
  - later status, sync result, restart, and recovery surfaces answer active clean-vs-dirty questions from the persisted active promoted build pair alone, never by re-inspecting the current worktree
  - later non-override reruns on a still-dirty repo persist `DIRTY_SKIPPED`, preserve the prior active promoted build pair, and update only the classified input pair for that run
  - no prior dirty success auto-authorizes future dirty sync

- Validation under `kstoolchain.state/v1alpha2` rejects any sync-written repo-state / provenance shape outside this closed matrix:
  - `CURRENT` requires both provenance pairs, and the pairs must agree on head and source kind
  - `DIRTY_SKIPPED` requires the classified input pair with `last_attempt_source_kind=dirty_worktree`; the active promoted build pair is preserved exactly from prior state, including absence
  - `SETUP_BLOCKED` preserves both prior provenance pairs exactly, including pair absence
  - `STALE_LKG` requires an active promoted build pair; the classified input pair is present only if trustworthy classification completed in the failed run
  - `FAILED` forbids an active promoted build pair and allows the classified input pair only if trustworthy classification completed before failure
  - any missing companion field or unknown source-kind value is invalid

- `repo_head == active_build` is never sufficient proof of a clean build. `active_source_kind` is the authority for whether the active build came from clean or dirty input.

- Downstream ownership remains explicit:
  - `5r6j4d` owns exact exit-code value, envelope `ok`, warning catalog, and the JSON field names and projection shapes for sync and status surfaces that expose these provenance facts
  - `zs7xv0` owns operator-facing wording, grouping, and next-step rendering
  - `hq97he` and `c580kd` own broader interrupted-attempt durability, locking, and crash-recovery protocol around sync as a whole
  - a future stone may decide whether manifest `dirty_policy` stays as doctrine-only text or is removed

Rejected round-5 concerns:
- do not add a best-effort legacy migration or source-kind backfill path for `kstoolchain.state/v1alpha1`
- do not widen this stone into the broader sync-attempt-integrity question of interrupted attempts, crash markers, or locking; that remains with `hq97he` and `c580kd`
- do not reopen dirty-policy product direction, repo-state vocabulary, or override surface design

This dirty-policy review loop is complete. No further review round is required for `a2bwaz` unless implementation uncovers a direct contradiction in repo truth.

## Evidence

Current doctrine and code evidence:
- `BUILDING.md` and the default adapter manifest still document fail-closed dirty handling as the default doctrine.
- `internal/toolchain/status.go` still defines `CURRENT`, `STALE_LKG`, `FAILED`, `DIRTY_SKIPPED`, and `SETUP_BLOCKED`, exact-matches the persisted schema string, and treats any non-`CURRENT` ready repo as a non-zero sync outcome.
- `internal/toolchain/status.go` still writes `current.json` atomically with temp-file plus rename. That supports the existing single terminal state-file write path, but it does not by itself solve broader interrupted-attempt semantics.
- `internal/toolchain/sync.go` still samples repo head and dirtiness before install, probe, and promote, then persists `StateCurrent` with `ActiveBuild = RepoHead` on success. That is why the classified-input observation rule and the promotion-boundary revalidation rule remain load-bearing in this stone.
- `internal/toolchain/sync.go` still returns early on setup-blocked before dirtiness is inspected, which is why binding setup classification ahead of dirtiness classification is true to the current seam rather than a new invention.
- the current dirtiness seam still has a failure path distinct from clean and dirty classification, which is why the stone now explicitly says git sampling or `git status --porcelain` failure is not trustworthy classification.
- `internal/service/service.go` still reloads persisted state after mutation and hands it directly to the status-like report surface, which is why persisted provenance must be strong enough for downstream projection without re-derivation.

Adjacent-scope evidence:
- there is now a dedicated sync-attempt-integrity decision stone, `hq97he`, which explicitly owns sync-exclusive locking, interrupted-attempt markers, broader sync-audit behavior, and how attempt integrity interacts with the existing single terminal `current.json` write rule
- there is a follow-up task, `c580kd`, for surfacing interrupted sync attempts truthfully once that integrity contract is frozen
- that adjacent scope is why round-5 rejected any attempt to widen this dirty-policy stone into migration design, crash markers, or interrupted-attempt protocol

External review round 5 evidence:
- Pack `AD6T3F` delivered the updated stone plus the narrowed doctrine, status, sync, service, and adjacent-stone slices; the rendered pack contained 12 docs and 13,757 tokens
- Gemini (`exec_576b3605ae0d17985c3ad532`, 16,388 input tokens) judged the stone fully ready and confirmed that the pair model, schema wall, and promotion-boundary revalidation now close the stale-input and auditability gaps inside this slice
- Opus (`exec_b3919659776b28f8668ad30d`, 23,453 input tokens) judged the stone ready with three surgical clarifications: explicitly name classification failure as not trustworthy, bind setup classification before dirtiness classification, and state that `last_attempt_at` is suite-level rather than part of either per-repo provenance pair
- Codex (`exec_c70ce32c4c6c2d3fff409dcd`, 399,433 total input tokens / 62,665 new input tokens) still judged the stone not ready, but only because the stone had not yet explicitly frozen the `kstoolchain.state/v1alpha2` reader behavior for legacy `kstoolchain.state/v1alpha1` files

Accepted synthesis from round 5:
- all three reviewers confirm the full pack was delivered and that the settled product direction stays out of scope
- Gemini confirms the existing round-4 contract is otherwise implementation-ready
- Opus provides three accepted in-scope tightenings: classification failure is not trustworthy, setup classification precedes dirtiness classification, and `last_attempt_at` is suite-level
- Codex provides one accepted in-scope tightening: explicitly freeze the legacy-schema boundary so `kstoolchain.state/v1alpha2` readers reject `kstoolchain.state/v1alpha1` with no migration, source-kind backfill, or treat-as-missing fallback
- broader interrupted-attempt durability, locking, and crash-recovery semantics remain explicitly outside this stone with `hq97he` and `c580kd`
- after this revision, `a2bwaz` is implementation-ready and no further external review round is required unless implementation reveals a direct contradiction

Artifacts:
- raw launch and result files saved under `.keystone/local/a2bwaz-review-results-r5/`

## Journal

- 2026-04-17T17:04:29Z | rewrote section context (old_lines=0 new_lines=16): Structure dirty-policy decision stone around current product truth, scope boundary, and policy seam.

- 2026-04-17T17:04:29Z | rewrote section plan (old_lines=0 new_lines=20): Add review-loop plan for dirty-repo policy decision with clear downstream interfaces.

- 2026-04-17T17:04:29Z | rewrote section evidence (old_lines=0 new_lines=21): Capture current code, docs, and test evidence for dirty-repo policy before review.

- 2026-04-17T17:04:35Z | Structured a2bwaz on 2026-04-17 around the real dirty-policy seam. Captured current doctrine, code, and tests showing that today's behavior is already hybrid: dirty repos persist as DIRTY_SKIPPED, clean repos can continue, and the overall sync still exits non-zero because not every ready repo ends CURRENT. Kept scope explicitly separate from gqksyd setup blockers and downstream sync rendering work.

- 2026-04-17T17:11:04Z | rewrote section decisions (old_lines=0 new_lines=22): Capture operator steering on dirty-repo policy direction and narrow remaining open design fork.

- 2026-04-17T17:11:04Z | Captured operator steering on 2026-04-17. Direction now favors a dual-mode dirty policy: strong default, repo-scoped blockers, completed-with-blockers outcome for default mode, suite-wide contract, and an explicit expert override for intentional dirty-repo sync during Keystone development.

- 2026-04-17T17:22:35Z | Prepared review-loop artifacts on 2026-04-17.

Deterministic context build:
- recipe: `.ctx/recipes/a2bwaz-dirty-repo-policy-review/recipe.yaml`
- run id: `2026-04-17T17-20-58Z`
- explain: `.ctx/runs/a2bwaz-dirty-repo-policy-review/2026-04-17T17-20-58Z/explain.md`
- manifest: `.ctx/runs/a2bwaz-dirty-repo-policy-review/2026-04-17T17-20-58Z/manifest.json`
- budget used: 16 files, 22,831 tokens of 26,000

External review pack:
- spec: `.keystone/local/a2bwaz-dirty-repo-policy-review.yaml`
- rendered pack: `.keystone/local/a2bwaz-dirty-repo-policy-review.md`
- pack id: `H3VV7G`
- docs: 18
- size: 11,618 tokens / 48,239 bytes

Prep note:
- used explicit recipe/spec authoring rather than `ctx.bot.ask` because this repo already has an open ksctx friction stone for `CTXBOT_ENGINE_EXIT_NONZERO` (`1ctwda`)
- `kshub recommend_route` and `inspect_route_selection` both returned low-confidence advice and misclassified this review as `web_design`, so route choice should be made deliberately when the actual external review is sent

- 2026-04-17T17:35:31Z | rewrote section context (old_lines=16 new_lines=18): Synthesize first external review round into a sharper dirty-policy decision boundary.

- 2026-04-17T17:35:31Z | rewrote section plan (old_lines=20 new_lines=23): Replace exploratory dirty-policy plan with post-review revision and follow-up review gates.

- 2026-04-17T17:35:31Z | rewrote section decisions (old_lines=22 new_lines=45): Record accepted synthesis from external review round 1 and narrow remaining open questions.

- 2026-04-17T17:35:31Z | rewrote section evidence (old_lines=21 new_lines=23): Refresh evidence with external review findings and accepted synthesis from review round 1.

- 2026-04-17T17:35:40Z | Review round 1 completed on 2026-04-17.

Pack:
- pack id: `H3VV7G`
- rendered pack: `.keystone/local/a2bwaz-dirty-repo-policy-review.md`

Execution ids:
- Gemini: `exec_0370b2f35f8edba4a6aa06fa`
- Opus: `exec_fb6fa2eb870f89c6220e1e54`
- Codex: `exec_c585ad89d2ce789a3913db8e`

Accepted synthesis:
- keep the dual-mode direction
- freeze the override as one-shot `sync --allow-dirty`, suite-wide for the invocation, with no config/env/named-mode expansion
- keep default blocked-dirty behavior repo-scoped and non-zero
- make persisted sync state the truth owner for whether the active build came from dirty source
- defer exact exit-code value, envelope semantics, and sync result schema to `5r6j4d`
- defer text rendering and next-step wording to `zs7xv0`
- do not let current manifest `dirty_policy` text become the runtime override mechanism in this slice

Review outcome:
- not ready to implement yet
- another review round is required after this revision because all three grounded reviews found the previous stone text under-specified around dirty-source provenance, retry/recovery rules, and contract ownership boundaries

- 2026-04-17T17:59:24Z | rewrote section context (old_lines=18 new_lines=15): Synthesize second external review round into a tighter dirty-source truth boundary.

- 2026-04-17T17:59:35Z | rewrote section plan (old_lines=23 new_lines=32): Replace round-1 follow-up plan with the round-2 truth-contract freeze and final confirmation review.

- 2026-04-17T17:59:56Z | rewrote section decisions (old_lines=45 new_lines=59): Record accepted synthesis from external review round 2 and freeze the dirty-source provenance contract.

- 2026-04-17T18:00:11Z | rewrote section evidence (old_lines=23 new_lines=26): Refresh evidence with second-round review findings and the accepted truth-model synthesis.

- 2026-04-17T18:00:25Z | Review round 2 completed on 2026-04-17.

Pack:
- pack id: `C92S6W`
- rendered pack: `.keystone/local/a2bwaz-dirty-repo-policy-review-r2.md`

Execution ids:
- Gemini: `exec_db3179edb8f8c30b15f65e5c`
- Opus: `exec_f81748bc2e8d57d4817e304e`
- Codex: `exec_171ffd7351058759e79f7c66`

Accepted synthesis:
- keep the dual-mode direction and the one-shot suite-wide `sync --allow-dirty` override
- keep default blocked-dirty behavior repo-scoped and non-zero
- keep `DIRTY_SKIPPED` as the repo-level blocked state and treat completed-with-blockers as a run-level outcome owned downstream
- keep dirty-success on `CURRENT` and make dirty provenance explicit instead of adding a new repo-state constant
- freeze persisted truth around existing `ActiveBuild` and `RepoHead` plus new `active_source_kind` and `last_attempt_source_kind` facts
- preserve the existing stale-last-known-good versus failed split on dirty-allowed failure while recording dirty last-attempt provenance
- require a fail-closed compatibility boundary so older readers cannot silently treat dirty-built `CURRENT` repos as clean

Review outcome:
- still not ready to implement
- one final narrow review round is required after this revision
- the remaining question is no longer product direction; it is whether the tightened persisted provenance contract is now explicit enough to implement without downstream invention

- 2026-04-17T18:24:27Z | rewrote section context (old_lines=15 new_lines=12): Synthesize third external review round into a stricter provenance write-boundary and compatibility contract.

- 2026-04-17T18:24:40Z | rewrote section plan (old_lines=32 new_lines=37): Replace round-2 confirmation plan with a final provenance-write-boundary freeze and re-review gate.

- 2026-04-17T18:25:06Z | rewrote section decisions (old_lines=59 new_lines=65): Record accepted synthesis from external review round 3 and freeze the mandatory schema bump and provenance-pair write rules.

- 2026-04-17T18:25:28Z | rewrote section evidence (old_lines=26 new_lines=30): Refresh evidence with third-round review convergence and the accepted stricter synthesis on stale-input and provenance invariants.

- 2026-04-17T18:25:41Z | Review round 3 completed on 2026-04-17.

Pack:
- pack id: `ZPEQKP`
- spec: `.keystone/local/a2bwaz-dirty-repo-policy-review-r3.yaml`
- rendered pack: `.keystone/local/a2bwaz-dirty-repo-policy-review-r3.md`
- docs: 14
- size: 11,709 tokens / 47,994 bytes

Execution ids:
- Gemini: `exec_b9650e2c7024d8190b896fe9`
- Opus: `exec_55d2a9d5117e79bcc58508d9`
- Codex: `exec_13fb274ec439a0e7c2b3fc95`

Payload verification:
- Gemini input tokens: 14,002
- Opus input tokens: 20,225
- Codex input tokens: 29,083
- all three executions cleared the full-pack delivery check and were kept in synthesis

Accepted synthesis:
- keep the settled product direction and repo-state vocabulary unchanged
- make the schema bump mandatory rather than leaving the compatibility mechanism implicit
- preserve dirty-success on `CURRENT` plus provenance rather than introducing a new repo-state enum
- freeze provenance as two atomic truth pairs: classified input and active promoted build
- require setup-blocked and pre-classification failures to preserve the prior classified input pair
- require promotion-boundary revalidation before persisting a new clean or dirty active promoted build pair
- make validation reject impossible provenance combinations instead of guessing
- leave sync result field names and rendering surfaces with `5r6j4d` and `zs7xv0`

Review outcome:
- still not ready to implement
- another review round is required after this revision
- the remaining question is only whether the mandatory schema bump, provenance-pair write rules, and promotion-boundary revalidation are now explicit enough to implement without downstream invention

Artifacts:
- raw launch and result files saved under `.keystone/local/a2bwaz-review-results-r3/`

- 2026-04-17T19:03:46Z | rewrote section context (old_lines=12 new_lines=15): Synthesize fourth external review round and resolve the remaining disagreement by separating dirty-policy truth from broader sync-attempt integrity work.

- 2026-04-17T19:03:57Z | rewrote section plan (old_lines=37 new_lines=32): Replace round-3 follow-up with final review-loop closeout and downstream handoff plan.

- 2026-04-17T19:04:25Z | rewrote section decisions (old_lines=65 new_lines=67): Record accepted synthesis from external review round 4, choose the schema string, tighten classification and revalidation wording, and declare the dirty-policy review loop complete.

- 2026-04-17T19:04:42Z | rewrote section evidence (old_lines=30 new_lines=25): Refresh evidence with round-4 reviewer results, accepted closeout edits, and the explicit split against sync-attempt-integrity scope.

- 2026-04-17T19:04:57Z | Review round 4 completed on 2026-04-17.

Pack:
- pack id: `DVQYF9`
- spec: `.keystone/local/a2bwaz-dirty-repo-policy-review-r4.yaml`
- rendered pack: `.keystone/local/a2bwaz-dirty-repo-policy-review-r4.md`
- docs: 10
- size: 11,559 tokens / 47,948 bytes

Execution ids:
- Gemini: `exec_5272f23ed8c08160b9d61645`
- Opus: `exec_0427fa58aa4270916ee06065`
- Codex: `exec_ce8ed70fe20d0398b951f37a`

Payload verification:
- Gemini input tokens: 13,885
- Opus input tokens: 19,878
- Codex input tokens: 72,203
- all three executions cleared the full-pack delivery check and were kept in synthesis

Accepted synthesis:
- keep the round-3 truth model and settled product direction unchanged
- choose the exact schema string `kstoolchain.state/v1alpha2`
- tighten the classified-input rule so `repo_head` and `last_attempt_source_kind` come from one trustworthy observation record
- tighten promotion-boundary revalidation so it occurs immediately before stage-to-managed-bin promotion with explicit head and cleanliness checks
- close validation from an open-ended reject list to a closed sync-written state/provenance matrix
- reject widening this stone into broader crash/interrupted-attempt durability semantics because that scope is already owned by `hq97he` and `c580kd`

Review outcome:
- implementation-ready after this revision
- no further external review round is required for `a2bwaz` unless implementation uncovers a direct contradiction in repo truth

Artifacts:
- raw launch and result files saved under `.keystone/local/a2bwaz-review-results-r4/`

- 2026-04-17T19:15:36Z | rewrote section context (old_lines=15 new_lines=14): Synthesize review round 5 and absorb the final compatibility and classification-boundary tightenings.

- 2026-04-17T19:15:53Z | rewrote section plan (old_lines=32 new_lines=39): Replace the round-4 closeout plan with the final round-5 compatibility and classification freeze.

- 2026-04-17T19:16:25Z | rewrote section decisions (old_lines=67 new_lines=75): Record accepted synthesis from external review round 5 and freeze the legacy-schema and trustworthy-classification boundary rules.

- 2026-04-17T19:16:45Z | rewrote section evidence (old_lines=25 new_lines=30): Refresh evidence with round-5 reviewer convergence and the accepted schema-boundary and classification-boundary tightenings.

- 2026-04-17T19:16:56Z | Review round 5 completed on 2026-04-17.

Pack:
- pack id: `AD6T3F`
- spec: `.keystone/local/a2bwaz-dirty-repo-policy-review-r5.yaml`
- rendered pack: `.keystone/local/a2bwaz-dirty-repo-policy-review-r5.md`
- docs: 12
- size: 13,757 tokens

Execution ids:
- Gemini: `exec_576b3605ae0d17985c3ad532`
- Opus: `exec_b3919659776b28f8668ad30d`
- Codex: `exec_c70ce32c4c6c2d3fff409dcd`

Payload verification:
- Gemini input tokens: 16,388
- Opus input tokens: 23,453
- Codex input tokens: 399,433 total / 62,665 new
- all three executions cleared the full-pack delivery check and were kept in synthesis

Accepted synthesis:
- keep the round-4 truth model, schema bump, provenance-pair rules, and promotion-boundary revalidation unchanged
- explicitly freeze the legacy-schema boundary so `kstoolchain.state/v1alpha2` readers reject `kstoolchain.state/v1alpha1` with no migration, source-kind backfill, or treat-as-missing fallback
- explicitly state that git sampling or `git status --porcelain` failure is not trustworthy classification and therefore preserves the prior classified input pair exactly
- explicitly bind setup classification before dirtiness classification so `SETUP_BLOCKED` and `DIRTY_SKIPPED` remain mutually exclusive in one run
- explicitly state that `last_attempt_at` is suite-level and not part of either per-repo provenance pair
- keep broader interrupted-attempt durability, locking, and crash-recovery semantics outside this stone with `hq97he` and `c580kd`

Review outcome:
- implementation-ready after this revision
- no further external review round is required for `a2bwaz` unless implementation uncovers a direct contradiction in repo truth

Artifacts:
- raw launch and result files saved under `.keystone/local/a2bwaz-review-results-r5/`

- 2026-04-17T19:53:36Z | Implemented the reviewed dirty-repo provenance contract in `kstoolchain`: bumped persisted schema to `kstoolchain.state/v1alpha2`, added classified-input and active promoted-build source-kind pairs, preserved prior provenance correctly across setup/classification failure paths, added promotion-boundary revalidation, and threaded one-shot suite-wide `sync --allow-dirty` through the shared service seam. Validation: `go test ./internal/toolchain ./internal/service ./internal/cli` and `go test ./...`.

- 2026-04-17T19:53:36Z | Implemented and validated the dirty-repo provenance contract in code. The remaining follow-on about sync/status projection semantics was recorded on `5r6j4d`.

## Lessons

- When persisted provenance becomes authoritative, freeze the operator-facing projection contract separately so live setup probes do not silently blur classified-input truth.

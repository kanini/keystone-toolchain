---
schema: v1
id: hq97he
title: Define sync attempt integrity contract for kstoolchain
status: closed
type: decision
priority: p2
deps: []
tags: [audit, contract, locking, reliability, status, sync, toolchain]
created_at: "2026-04-17T18:15:33Z"
closed_at: "2026-04-17T21:46:37Z"
---
<!-- ksmem:managed: direct edits bypass validation; use ksmem commands -->
`gqksyd` intentionally left sync-exclusive locking, interrupted-attempt markers, and broader sync-audit behavior out of scope. Those concerns still need a reviewed contract before the sync path widens or sees heavier dogfood use.

This decision should freeze the minimum honest attempt-integrity model for `kstoolchain sync`:
- whether only one sync may run at a time and how that exclusion is enforced
- what state, if any, is written before mutation starts
- how interrupted or crashed attempts are detected and surfaced later
- how attempt integrity interacts with the existing single terminal `current.json` write rule
- what belongs in `sync` output versus `status` truth surfaces
- what can stay explicitly out of scope in the first integrity slice

Goal: prevent concurrent or interrupted sync runs from leaving misleading toolchain truth without widening into an unbounded audit redesign.

## Context

`hq97he` is now at the last representation decision, not an architectural fork.

Round 3 confirmed the core contract is settled:
- `sync.lock` remains exclusivity only
- `attempt.json` remains the durable owner of unresolved attempt truth
- `current.json` remains terminal committed truth plus `committed_attempt_id`
- `attempt.json` reads stay fail-closed
- `attempt.json` writes stay atomic and durable
- unmatched `pre_promotion` stays a suite-level overlay that does not rewrite repo rows
- unmatched `promotion_or_later` suppresses any suite-level trusted or complete claim

The remaining disagreement was narrower than the previous round.
- Gemini judged the stone ready, but wanted a dedicated carry-forward field so inherited post-promotion uncertainty does not corrupt the current attempt's own phase.
- Opus also judged it ready after one load-bearing naming fix: sticky `promotion_or_later` inheritance must live in a named on-disk field rather than an implied behavioral rule.
- Codex still judged it not ready, but for the same underlying reason: the stone had not yet frozen what `attempt.json.phase` means during a retry chain.

Accepted synthesis:
- keep `phase` as the current attempt's own phase
- add a separate `carried_unresolved_phase` field for inherited unresolved severity from a prior unmatched attempt chain
- project suite-level trust from the stricter of `phase` and `carried_unresolved_phase`
- treat unreadable, invalid, or unknown-schema `attempt.json` as equivalent in trust/completeness severity to unmatched `promotion_or_later`, even if operator-facing labels differ

I am taking this representation rather than redefining `phase` as chain-effective unresolved phase because it is the more truthful wire format. It preserves current-attempt progress as current-attempt progress, while still freezing monotonic unresolved trust across retries. That keeps the stone behaviorally precise for `aey8w6` and `c580kd` without widening into a richer audit/history system.

## Plan

1. Freeze the final artifact semantics.
- `sync.lock` remains exclusivity only.
- `attempt.json` remains the durable owner of unresolved attempt truth.
- `current.json` remains the only terminal committed suite-state file.

2. Freeze field meaning rather than leaving it to implementation.
- `phase` means the current attempt's own phase only
- `carried_unresolved_phase` means the strictest unresolved phase inherited from a prior unmatched attempt chain, if any
- the carried field is absent on a first attempt and after any correlated `current.json` commit clears the chain

3. Freeze the shared projection rule.
- suite-level trust/completeness is reduced from the stricter of `phase` and `carried_unresolved_phase`
- unreadable, invalid, or unknown-schema `attempt.json` projects with the same trust/completeness severity as unmatched `promotion_or_later`
- repo rows remain anchored in `current.json` plus existing live setup/PATH projection

4. Freeze retry-chain monotonicity mechanically.
- a retry may replace `attempt.json` only under the lock
- if the prior unmatched attempt chain had already crossed `promotion_or_later`, the successor must persist that carried unresolved phase immediately
- only a correlated `current.json` commit clears `carried_unresolved_phase`

5. Keep the slice narrow.
- no wait queues
- no stale-lease reclamation heuristics
- no per-adapter locking
- no rollback, repair, or partial resume
- no historical attempt journal
- no widening into sync-result envelope placement or dirty/provenance semantics owned elsewhere

Validation gates after this revision:
- no ambiguity remains about what `phase` means on a retry
- no ambiguity remains about where carried-forward post-promotion uncertainty lives on disk
- `status` and post-sync `final_status` can share one suite-level predicate
- implementation work in `aey8w6` and `c580kd` no longer has to invent correctness at the representation boundary

## Decisions

Round-3 synthesis accepted on 2026-04-17:

- `kstoolchain sync` remains single-writer with fail-fast contention. There is still no implicit wait behavior in this slice.

- The three-surface split remains settled:
  - `sync.lock` owns exclusivity only
  - `attempt.json` owns latest unresolved attempt truth
  - `current.json` owns last committed suite truth and correlated success

- The lock primitive remains explicit:
  - `sync.lock` is held through an OS-managed advisory lock on an open file descriptor under the state dir
  - the lock must auto-release on process termination
  - path-created `O_CREATE|O_EXCL` lockfiles or any primitive requiring persistent stale-lease reclamation are explicitly rejected in this slice
  - this slice assumes a local filesystem with process-scoped lock release and same-directory atomic rename; cross-host and network-filesystem semantics remain out of scope

- The authoritative persisted-state read that drives mutation still happens only after lock acquisition.

- `current.json` remains terminal committed truth only:
  - every successful `current.json` commit written by this slice must carry `committed_attempt_id`
  - correlation is mandatory, not optional
  - cleanup of `attempt.json` is never the proof of success

- `attempt.json` remains the only durable owner of unresolved attempt truth:
  - it is created immediately after lock acquisition and before any install, probe, or promotion work begins
  - it is separate from both `sync.lock` and `current.json`
  - it is suite-level only and does not create new per-repo state enums in this slice

- `attempt.json` write semantics remain atomic and durable:
  - creation and updates use same-directory temp-file-plus-rename rather than in-place mutation
  - file contents and the containing directory must be durably flushed before the write is treated as visible truth
  - contention readers must tolerate the legitimate window where the lock is held but `attempt.json` is absent or not yet readable, and must fall back to a generic contention message rather than blocking or guessing

- `attempt.json` must carry the minimum fields needed for truthful recovery semantics:
  - schema version
  - attempt_id
  - started_at
  - owner_host when available
  - owner_pid when available
  - ready_repo_ids
  - phase
  - carried_unresolved_phase

- Field meaning is now explicit:
  - `phase` is the current attempt's own phase only: `pre_promotion` or `promotion_or_later`
  - `carried_unresolved_phase` is the strictest unresolved phase inherited from a prior unmatched attempt chain, if any
  - `carried_unresolved_phase` is absent on a first attempt and after any correlated `current.json` commit clears the chain

- The coarse phase model remains exactly two trust boundaries:
  - `pre_promotion`
  - `promotion_or_later`
  - the current attempt's `phase` must be advanced to `promotion_or_later` and durably persisted before the first managed-bin `os.Rename` call of the run

- Reader behavior for `attempt.json` remains fail-closed:
  - unreadable JSON, invalid JSON, or an unsupported schema version must not be treated as a missing artifact
  - `status` and post-sync `final_status` must project the stricter unresolved-attempt case
  - `sync` must not start mutation work on top of unreadable or unknown attempt truth

- Cross-surface projection rules are now explicit and shared:
  - when `attempt.json` exists and its `attempt_id` does not match `current.json.committed_attempt_id`, the suite carries interrupted-attempt truth
  - unmatched `pre_promotion` remains a suite-level interrupted-attempt overlay but does not rewrite repo rows or committed completeness derived from `current.json`
  - unmatched `promotion_or_later` means no surface may present the suite as fully trusted or complete
  - `carried_unresolved_phase` participates in the same trust rule: suite-level trust/completeness is reduced from the stricter of `phase` and `carried_unresolved_phase`
  - unreadable, invalid, or unknown-schema `attempt.json` is semantically equivalent in trust/completeness severity to unmatched `promotion_or_later`, even if operator-facing labels differ
  - repo rows stay anchored in `current.json` plus existing live setup/PATH projection

- Unresolved post-promotion uncertainty is monotonic across retries:
  - a new retry may replace `attempt.json` only under the lock
  - when the replaced `attempt.json` was unmatched against `current.json.committed_attempt_id`, the successor must set `carried_unresolved_phase` to the strictest of the prior attempt's `phase` and its prior `carried_unresolved_phase`
  - `carried_unresolved_phase` is cleared only by a correlated `current.json` commit, never by starting a new attempt
  - a retry must never downgrade unresolved uncertainty from `promotion_or_later` back to `pre_promotion` merely by starting over

- Retry and recovery remain simple:
  - a rerun after interruption is always a fresh full ready-set sync
  - no partial replay from staged or promoted leftovers
  - no automatic rollback or repair in this slice

- Surface ownership stays split cleanly:
  - this stone owns lock semantics, attempt-artifact lifecycle, atomicity and fail-closed behavior, phase ordering, carried-forward unresolved truth, commit correlation, and the shared suite-level projection semantics
  - `aey8w6` implements lock acquisition and enforcement
  - `c580kd` implements truthful interrupted-attempt surfacing
  - `5r6j4d` remains the owner of sync-result envelope semantics, field placement, and rendered shape
  - `a2bwaz` remains the owner of dirty/provenance semantics inside `current.json`

- Out of scope for this slice remains unchanged:
  - wait queues
  - per-adapter locking
  - stale-lease reclamation heuristics
  - automatic rollback, repair, or partial resume
  - historical attempt journal
  - widening sync-result or dirty-policy semantics beyond the semantic requirements above

No further external review round is required for `hq97he` unless implementation uncovers a direct contradiction in repo truth.

## Evidence

Current code-truth evidence still holds:
- `internal/service/service.go` still performs a pre-read, sync work, and post-read around the shared service seam, so the under-lock authoritative reload rule remains load-bearing.
- `internal/toolchain/sync.go` still promotes managed-bin outputs before the terminal `SavePersistedState` call, so unresolved post-promotion uncertainty remains the critical boundary.
- `internal/toolchain/status.go` still derives repo rows from committed persisted truth plus live setup/PATH projection, which is why attempt truth stays suite-level and additive rather than rewriting repo rows.
- `SavePersistedState` still uses same-directory temp-file plus rename for `current.json`, which continues to justify the same atomic discipline for `attempt.json`.

Round-3 external review evidence, against pack `KYSRYS`:
- Gemini `exec_de96a7091e9e3de76139fbd4`: input `14,301` tokens, judged the stone ready and argued that monotonic unresolved post-promotion truth should live in a dedicated carry-forward field rather than overloading `phase`.
- Opus `exec_b9234f69d504aac62cd0b442`: input `21,100` tokens, judged the stone ready with two small load-bearing edits. Strongest accepted point: sticky `promotion_or_later` inheritance must live in a named on-disk field, and suite projection should reduce over the stricter of current phase and carried unresolved phase.
- Codex first attempt `exec_16ce893e1efa8ba59febb13c`: discarded. The route returned only progress chatter and no substantive judgment even though the execution marked `succeeded`.
- Codex rerun `exec_39e91a9b777ec9e5471b4e9a`: input `29,432` tokens, judged the stone not ready until the meaning of `attempt.json.phase` is frozen. Strongest accepted point: phase meaning must be explicit and shared projection semantics must not drift. Rejected point: redefine `phase` itself as chain-effective unresolved phase. Gemini and Opus surfaced the better argument there: preserving `phase` as the current attempt's own phase is the more truthful wire contract.

Accepted synthesis from round 3:
- the architecture is now settled and no reviewer reopened it
- all substantive reviewers agreed the remaining issue was representation, not ontology
- Gemini and Opus correctly identified that a dedicated carried-forward field preserves audit clarity and avoids overloading `phase`
- Codex correctly insisted that phase meaning and invalid-artifact equivalence be explicit, and those points are accepted
- the final representation is therefore:
  - `phase` = current attempt's own phase
  - `carried_unresolved_phase` = inherited strictest unresolved phase from a prior unmatched chain, if any
  - suite projection reduces over the stricter of those two, and invalid/unreadable/unknown artifacts project with `promotion_or_later` severity

Why this review loop is complete after this revision:
- the last ambiguity identified by round 3 is now frozen directly in stone text
- there is no remaining open question that should be delegated to `aey8w6` or `c580kd`
- the next step is implementation or implementation planning, not another design round, unless repo code truth contradicts the stone during implementation

Implemented the frozen sync attempt-integrity contract in code. Added advisory single-writer locking on `sync.lock`, durable fail-closed `attempt.json` handling with `phase` and `carried_unresolved_phase`, under-lock authoritative persisted-state reload in the shared sync service seam, `committed_attempt_id` on successful `current.json` commits, and a shared suite-level `attempt_integrity` overlay consumed by both `status` and `sync.final_status` without rewriting repo rows. Validation run: `go test ./internal/toolchain`, `go test ./internal/service`, `go test ./internal/cli`, and `go test ./...`, all passing.

## Journal

- 2026-04-17T20:09:26Z | rewrote section context (old_lines=0 new_lines=23): Structure hq97he for review around the actual sync/write seam and the operator steering on locking and fail-fast behavior.

- 2026-04-17T20:09:42Z | rewrote section plan (old_lines=0 new_lines=41): Add a review-loop plan that freezes the minimal sync lock and attempt-marker contract before downstream implementation tasks.

- 2026-04-17T20:10:00Z | rewrote section decisions (old_lines=0 new_lines=59): Capture operator steering and the proposed first-slice contract for review on lock semantics, attempt marker ownership, and interrupted truth.

- 2026-04-17T20:10:17Z | rewrote section evidence (old_lines=0 new_lines=21): Capture the current sync/write seam and adjacent-stone handoffs that make hq97he the next narrow review target.

- 2026-04-17T20:13:31Z | Prepared review-loop artifacts on 2026-04-17.

Deterministic context build:
- recipe: `.ctx/recipes/hq97he-sync-attempt-integrity-review/recipe.yaml`
- run id: `2026-04-17T20-12-30Z`
- explain: `.ctx/runs/hq97he-sync-attempt-integrity-review/2026-04-17T20-12-30Z/explain.md`
- manifest: `.ctx/runs/hq97he-sync-attempt-integrity-review/2026-04-17T20-12-30Z/manifest.json`
- budget used: 58,477 / 62,000 tokens across 14 files

External review pack:
- spec: `.keystone/local/hq97he-sync-attempt-integrity-review.yaml`
- rendered pack: `.keystone/local/hq97he-sync-attempt-integrity-review.md`
- pack id: `25P5VJ`
- docs: 15
- size: 15,184 tokens / 64,146 bytes

Prep note:
- used explicit recipe/spec authoring rather than `ctx.bot.ask` because the repo still has the open ksctx friction stone `1ctwda` for `CTXBOT_ENGINE_EXIT_NONZERO`
- the rendered pack was inspected after build and contains real doctrine, stone, and code payloads rather than placeholders

- 2026-04-17T20:33:14Z | rewrote section context (old_lines=23 new_lines=30): Synthesize round-1 reviewer agreement and freeze the narrowed integrity boundary before the next review round.

- 2026-04-17T20:33:27Z | rewrote section plan (old_lines=41 new_lines=49): Replace the initial review plan with the narrower round-2 plan that freezes truth ownership, ordering, and correlation semantics.

- 2026-04-17T20:33:51Z | rewrote section decisions (old_lines=59 new_lines=82): Capture the accepted round-1 synthesis and the exact contract to pressure-test in the next review round.

- 2026-04-17T20:34:06Z | rewrote section evidence (old_lines=21 new_lines=26): Record the round-1 reviewer convergence, the accepted split, and the live code seams that make the next review question concrete.

- 2026-04-17T20:34:20Z | 2026-04-17 review round 1 synthesis.

Pack id: `25P5VJ`
Executions:
- Gemini: `exec_ea22dd846211c2fe678531a9`
- Opus: `exec_45f3736755b88f8562ace921`
- Codex: `exec_7f4b7d967b2e74137f50f7c4`

Accepted synthesis:
- keep single-writer, fail-fast, terminal-truth-in-`current.json`, and the `pre_promotion` vs `promotion_or_later` trust boundary
- replace the vague stale-lease draft with a three-surface contract:
  - `sync.lock` for exclusivity only
  - `attempt.json` for latest uncommitted or interrupted attempt truth
  - `current.json` for committed truth plus `committed_attempt_id` correlation
- require the authoritative persisted-state reload under the lock
- require `attempt.json` creation before mutation begins
- require durable phase advancement before the first promotion rename
- require `status` and post-sync `final_status` to consume the same suite-level attempt truth

Outcome:
- another review round is required before implementation
- exact remaining gap: confirm that the lock/attempt/current three-surface split plus `committed_attempt_id` really freezes the post-commit crash window and does not leave a hidden race or platform-assumption hole in the first slice

- 2026-04-17T20:43:26Z | rewrote section context (old_lines=30 new_lines=24): Incorporate round-2 review synthesis and narrow the remaining gap before another review round.

- 2026-04-17T20:43:40Z | rewrote section plan (old_lines=49 new_lines=43): Replace the round-2 plan with the final-tightening plan focused on atomic artifact semantics and monotonic unresolved truth.

- 2026-04-17T20:44:05Z | rewrote section decisions (old_lines=82 new_lines=88): Capture the accepted round-2 synthesis, including the stricter monotonicity rule for unresolved post-promotion attempts.

- 2026-04-17T20:44:19Z | rewrote section evidence (old_lines=26 new_lines=23): Record the round-2 reviewer convergence and the accepted stricter synthesis around atomic artifact semantics and monotonic unresolved truth.

- 2026-04-17T20:44:27Z | 2026-04-17 review round 2 synthesis.

Pack id: `R2ERCH`
Executions:
- Gemini: `exec_c7c7ee5a6320e2bd130d15d5`
- Opus: `exec_f48067fe61629140260870d0`
- Codex: `exec_e2f3f21a029d82ba62fe41a3`

Delivery check:
- Gemini input tokens: `18101`
- Opus input tokens: `26863`
- Codex input tokens: `32746`
- all three were accepted as full-pack deliveries

Accepted synthesis:
- keep the three-surface split from round 1
- make the lock primitive explicitly OS-managed and auto-released
- make `attempt.json` writes atomic and same-directory temp-file-plus-rename
- make unreadable or unknown `attempt.json` fail closed
- make unmatched `pre_promotion` versus unmatched `promotion_or_later` projection explicit
- make unresolved `promotion_or_later` truth monotonic across retries until a correlated `current.json` commit clears it

Outcome:
- another review round is required before implementation
- exact remaining gap: pressure-test whether the newly frozen monotonic unresolved-attempt rule is explicit enough as written, or whether the stone needs a dedicated carried-forward field or naming rule rather than sticky phase inheritance

- 2026-04-17T20:58:57Z | rewrote section context (old_lines=24 new_lines=23): Incorporate round-3 review synthesis and freeze the remaining representation choice for carried-forward post-promotion uncertainty.

- 2026-04-17T20:59:08Z | rewrote section plan (old_lines=43 new_lines=33): Replace the round-3 validation plan with the final implementation-readiness plan after naming the carried-forward field explicitly.

- 2026-04-17T20:59:39Z | rewrote section decisions (old_lines=88 new_lines=92): Record accepted round-3 synthesis, add the carried_unresolved_phase representation, and declare the review loop implementation-ready.

- 2026-04-17T21:00:00Z | rewrote section evidence (old_lines=23 new_lines=26): Record round-3 reviewer convergence, the discarded Codex run, and the accepted final representation choice.

- 2026-04-17T21:00:10Z | 2026-04-17 review round 3 synthesis.

Pack id: `KYSRYS`
Accepted executions:
- Gemini: `exec_de96a7091e9e3de76139fbd4`
- Opus: `exec_b9234f69d504aac62cd0b442`
- Codex rerun: `exec_39e91a9b777ec9e5471b4e9a`

Discarded execution:
- Codex initial run: `exec_16ce893e1efa8ba59febb13c` returned only progress chatter and no substantive judgment despite `succeeded` status

Delivery check:
- Gemini input tokens: `14301`
- Opus input tokens: `21100`
- Codex rerun input tokens: `29432`
- all three accepted executions were treated as full-pack deliveries

Accepted synthesis:
- keep the round-2 architecture unchanged
- add `carried_unresolved_phase` to `attempt.json`
- keep `phase` as the current attempt's own phase
- reduce suite-level trust/completeness from the stricter of `phase` and `carried_unresolved_phase`
- treat unreadable, invalid, or unknown-schema `attempt.json` as equivalent in trust/completeness severity to unmatched `promotion_or_later`
- reject redefining `phase` itself as chain-effective unresolved phase because that would make the wire contract less truthful

Outcome:
- no further external review round is required for `hq97he`
- the stone is now implementation-ready unless repo code truth contradicts it during implementation

- 2026-04-17T21:46:37Z | Implemented and validated. Sync is now single-writer with durable attempt truth and shared interrupted-attempt projection across status and sync final-status.

## Lessons

- Keep exclusivity, unresolved attempt truth, and committed suite truth on separate artifacts: `sync.lock` for writer exclusion only, `attempt.json` for unresolved attempt integrity only, and `current.json` for correlated committed truth only.

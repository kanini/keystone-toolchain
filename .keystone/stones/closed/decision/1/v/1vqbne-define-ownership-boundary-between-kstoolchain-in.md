---
schema: v1
id: 1vqbne
title: Define ownership boundary between kstoolchain init and sync
status: closed
type: decision
priority: p1
deps: []
tags: [bootstrap, init, ownership, sync, ux]
created_at: "2026-04-17T14:55:39Z"
closed_at: "2026-04-17T18:18:36Z"
---
<!-- ksmem:managed: direct edits bypass validation; use ksmem commands -->
Operator feedback exposed a missing product boundary. A toolchain manager should prepare the machine and managed tools without forcing the user to remember scattered repo-level rituals, but not every bootstrap action belongs in the same command.

This stone should define the ownership split between `kstoolchain init` and `kstoolchain sync`.

Questions to settle:
- what machine-level setup belongs to `init`
- what post-install or post-promotion bootstrap belongs to `sync`
- which actions are safe and idempotent enough to automate
- which actions remain manual because they require auth, secrets, or explicit user consent
- how the two commands should report remaining manual steps so the operator is not left guessing

The result should be one clear first-run and day-two story instead of duplicated or contradictory bootstrap behavior.

## Context

The review loop for `1vqbne` is complete.

Across three rounds, the stone moved from an empty decision shell to a fully reviewed product boundary for first-run versus day-two command ownership. The final ready-check round converged that the boundary is now frozen tightly enough to implement without reopening adjacent design.

Settled contract:
- `kstoolchain init` is the canonical first-run command
- `kstoolchain sync` is the canonical day-two refresh command
- `init` may reach the ready-set path only by invoking the same service-level ready-set entry used by `sync`
- `sync` remains the sole owner of ready-set mutation and `current.json`
- `gqksyd` overlay ownership, `SETUP_BLOCKED`, and the setup-vs-config split remain intact
- readiness claims stay limited to the current ready set
- preview-only flows do not delegate
- repo acquisition automation remains outside this slice until remote-truth ownership exists

The remaining work is no longer design. It is downstream implementation in `vpw661` and adjacent follow-on stones. No further review round is required for `1vqbne` unless implementation uncovers a direct contradiction in code or doctrine.

## Plan

1. Treat the `init` versus `sync` ownership decision as settled.
2. Implement `vpw661` against this boundary: machine/bootstrap work, managed dirs, shell PATH bootstrap, and init reporting must follow `1vqbne` rather than inventing a new first-run story.
3. Keep `gqksyd` implementation aligned with this decision: `init` authors overlay truth, `sync` consumes it, and delegated init runs use the same service-level ready-set path rather than a second engine.
4. Use this decision to constrain later sync UX work such as `5r6j4d`: sync remains the day-two action result surface, not the first-run bootstrap surface.
5. Queue future review work separately for any remote-truth/clone automation, if that product path is still desired later.

Implementation gates this decision now imposes:
- `init` and `sync` must not drift into separate ready-set execution models
- `init` must inherit delegated ready-set non-success and must not claim false readiness
- preview-only flows must not delegate or write `current.json`
- `sync` remains the sole persisted result writer for the ready set
- operator-facing next actions must name concrete manual prerequisites before suggesting rerun

## Decisions

Revised proposed contract for review:

Day-one versus day-two story:
- `kstoolchain init` is the canonical first-run command
- `kstoolchain sync` is the canonical day-two refresh command
- the operator outcome after `kstoolchain init` is: machine/bootstrap prerequisites and local adapter truth required for the current ready set are established, and `init` has delegated into the shared ready-set sync path for that same ready set; any remaining non-current ready adapters are reported truthfully with explicit next actions
- the operator outcome after `kstoolchain sync` is: for the current ready set only, the configured local checkouts were evaluated through the sync path, binaries were built/probed/promoted as applicable, and `current.json` records that result
- this stone does not widen readiness claims beyond the current ready set

`init` ownership in the revised boundary:
- establish managed-bin PATH precedence and other machine bootstrap needed by the toolchain manager itself
- create or verify managed directories and local config roots used by `kstoolchain`
- create or refresh the machine-local adapter overlay defined by `gqksyd`
- discover likely repo locations with medium-confidence heuristics
- prompt the operator to confirm or correct discovered repo paths before write
- after successful machine/bootstrap and overlay-authoring work, invoke the same service-level ready-set entry used by `sync` so first-run readiness does not require a second manual command
- `init` must not call lower-level build, probe, or promote primitives directly, and must not assemble its own ready-set iteration
- a real `kstoolchain init` invocation delegates exactly once into the shared ready-set sync path after bootstrap checks and overlay validation/writes succeed
- preview-only flows, including `--dry-run`, do not delegate into sync, do not mutate repos, and do not write `current.json`
- `init` inherits the delegated ready-set outcome; if any ready adapter ends non-`CURRENT`, `init` is non-success and must not describe the ready set as ready, complete, or fully usable
- when `init` ends with unresolved work, it must name the concrete manual prerequisite first; it may recommend rerunning `kstoolchain init` only after that prerequisite is addressed
- rerun as a no-op-or-repair command: preserve correct state, fix missing prerequisites, and print a clear summary of what changed versus what was already correct

`sync` ownership in the revised boundary:
- operate on the current ready set using already-established local adapter truth
- own ready-set repo mutation, build, probe, promote, and persisted result writing in `current.json`
- perform safe idempotent post-install or post-promotion bootstrap only when that work is local, deterministic, and inseparable from making the promoted tool usable
- never author overlays, repair shell PATH, create manager config roots, or infer first-run machine readiness on behalf of `init`
- if setup is missing or incomplete, point the operator to `kstoolchain init` through the existing truthful surfaces described below rather than silently repairing broad first-run state

Safety boundary in the revised contract:
- automate steps that are local, deterministic, and idempotent
- keep auth, secret-bearing, consent-heavy, or externally destructive actions manual by default
- keep dirty-repo conflict resolution manual unless a separate reviewed stone changes that policy
- keep broad sync-audit redesign, locking, and interrupted-attempt semantics outside this stone
- remove repo clone/pull automation from this slice; there is no canonical remote URL/ref truth owner yet, so `init` must not infer or automate GitHub acquisition behavior here

Missing-setup versus invalid-config handling:
- a missing default overlay file or an unset/missing ready-adapter `repo_path` is a runtime setup gap; `sync` surfaces those adapters as `SETUP_BLOCKED` using the exact reason vocabulary from `gqksyd` and points the operator to `kstoolchain init`
- malformed overlays, unreadable explicit overlay files, duplicate overlay ids, and unknown overlay rows are deterministic config errors before ready-set work begins; in those cases `sync` writes no `current.json`
- `init` must use the same setup reason vocabulary and truth split when it summarizes remaining manual/setup-blocked work after delegating into the sync path
- `init`'s delegation invokes the same pre-ready-set config-error gate as `sync`; if overlay resolution fails before a ready-adapter set exists, `init` fails through the same top-level contract error surface, writes no `current.json`, and fabricates no per-repo persisted rows
- there is no separate persisted `init complete` marker; first-run truth is derived from live bootstrap state, resolved overlay truth, and sync state

User-facing reporting expectations:
- `init` must report what it handled automatically, what it discovered, what changed, what remained already correct, and what still requires manual action
- if `init` changed shell configuration that requires a new shell or sourcing an rc file, that manual step must be stated explicitly
- `init` and `sync` must use the same per-adapter state vocabulary for the delegated ready-set outcome; `init` must not invent a second success language for the same adapters
- if `sync` encounters setup gaps, its next-action messaging must point to `kstoolchain init` through the existing contract warning/error surfaces rather than raw low-level failures
- `sync` remains the canonical day-two refresh command even though `init` may delegate into the same path during first-run or repair flows

Definition of scope for this slice:
- in this slice, the only readiness claim is the current ready set, not every Keystone repository in existence
- this stone does not change rollout semantics, overlay architecture, or remote acquisition truth
- `vpw661` remains the follow-on task that implements machine bootstrap under the accepted boundary from this stone

## Evidence

Adjacent reviewed doctrine and active-stone evidence still hold:
- `1sa48m` keeps the v1 surface anchored on truthful sync/status behavior, managed-bin isolation, stage-probe-promote activation, live PATH audit, and fail-closed trust posture
- `kfj483` keeps sync narrow on the ready set while status stays broad, with only `keystone-hub` ready in M1
- `gqksyd` already froze machine-local overlay ownership, `SETUP_BLOCKED`, setup-vs-config failure handling, and the single-writer rule for `current.json`
- `vpw661` remains the downstream bootstrap task this decision unblocks

Stable repo truth used for the review loop:
- stable `HEAD` still exposes only `version`, `sync`, and `status` on the public CLI surface
- stable `HEAD` still shows `SyncReport` driving the ready-set path while runtime config owns only manager paths
- stable docs still expose a broader machine-local adapter story than the stable CLI surface, which is why this decision was needed now

First review round on 2026-04-17 used pack `S3PYZ4` with counted executions:
- Gemini: `exec_6d820a019c64d71e7c36fdd6`
- Opus: `exec_db139313954862637645ad97`
- Codex: `exec_81ad190210f9cc6ba4a401c1`

Accepted synthesis from round one:
- keep the init/day-two sync split
- preserve `gqksyd` ownership and `SETUP_BLOCKED`
- explicitly delegate init into the shared sync path rather than duplicating ready-set execution
- keep sync as the sole owner of `current.json` and ready-set mutation
- narrow readiness claims to the current ready set
- remove clone/pull automation from this slice because remote-truth ownership does not exist yet

Second review round on 2026-04-17 used pack `GAS7RY` with counted executions:
- Gemini: `exec_99b04981d03da49cb42bae91`
- Opus: `exec_e68c9142d067c31ae04ba71e`
- Codex: `exec_f35de5c29f3ac6ed84c1d719`

Accepted synthesis from round two:
- freeze the delegation seam and anti-duplication rule explicitly
- freeze `init` inheriting delegated non-success explicitly
- freeze preview/no-op non-delegation explicitly
- tighten ready-set-only completion language and manual-next-step wording

Final ready-check round on 2026-04-17 used pack `HPS5XH` with counted executions:
- Gemini: `exec_94f8cfae6c17ee8e58d47793`
- Opus: `exec_8c033b1c83e2c9bcfa761cf6`
- Codex: `exec_434705ff75ac5b5ea8b8deb5`

Final convergence:
- all three reviewers called `1vqbne` ready
- all three agreed no remaining blocker belongs inside this stone
- final reviewer consensus is that the current text now freezes the boundary strongly enough to implement without reopening adjacent design

Accepted final outcome:
- the `init` versus `sync` ownership split is settled
- no further review round is required for `1vqbne`
- optional wording hardening remains possible, but no load-bearing design change is still needed
- the next work is downstream implementation, especially `vpw661`, plus any future separate decision for remote-truth/clone automation if that path is reprioritized

## Journal

- 2026-04-17T17:55:07Z | rewrote section context (old_lines=0 new_lines=16): Formalize 1vqbne into a review-ready decision stone with explicit proposed first-run/day-two command ownership.

- 2026-04-17T17:55:14Z | rewrote section plan (old_lines=0 new_lines=11): Add a review-loop plan for deciding the init/sync ownership boundary and unblocking vpw661.

- 2026-04-17T17:55:29Z | rewrote section decisions (old_lines=1 new_lines=39): Replace the provisional note with a review-ready proposed init/sync ownership contract based on operator intent and adjacent doctrine.

- 2026-04-17T17:55:43Z | rewrote section evidence (old_lines=0 new_lines=24): Capture the code-truth and adjacent-stone evidence that makes the init/sync ownership decision necessary now.

- 2026-04-17T17:55:50Z | Formalized 1vqbne for review on 2026-04-17. Replaced the placeholder decision shell with an explicit proposed first-run/day-two contract: `init` owns machine/bootstrap readiness, overlay authoring, missing-repo acquisition prompts, and the initial safe work needed to make relevant managed tools usable; `sync` owns day-two refresh, rebuild, promote, and safe post-install bootstrap, and must fail back to `init` when broad setup is missing. Review remains open to challenge the boundary, but not the need for a first-run command story.

- 2026-04-17T18:05:19Z | rewrote section context (old_lines=16 new_lines=20): Synthesize the first review round into a narrower, revised init/sync boundary contract before a second review round.

- 2026-04-17T18:05:27Z | rewrote section plan (old_lines=11 new_lines=13): Refocus the plan on a narrow second review round after the first-round boundary findings.

- 2026-04-17T18:05:50Z | rewrote section decisions (old_lines=39 new_lines=49): Revise the proposed init/sync ownership contract after the first review round to freeze delegation, setup semantics, and scope boundaries.

- 2026-04-17T18:06:07Z | rewrote section evidence (old_lines=24 new_lines=38): Record the first review round results and the stable seams that constrain the revised init/sync boundary.

- 2026-04-17T18:06:14Z | 2026-04-17 first review round synthesis. Pack `S3PYZ4`. Counted executions: Gemini `exec_6d820a019c64d71e7c36fdd6`, Opus `exec_db139313954862637645ad97`, Codex `exec_81ad190210f9cc6ba4a401c1`. Accepted synthesis: keep the init/day-two sync split, preserve gqksyd ownership and SETUP_BLOCKED semantics, explicitly delegate init into the shared sync path rather than duplicating ready-set execution, keep sync as the sole owner of current.json and ready-set mutation, narrow readiness claims to the current ready set, and remove clone/pull automation from this slice because remote-truth ownership does not exist yet. Outcome: not ready yet; one narrower second review round is required after revising the stone.

- 2026-04-17T18:12:22Z | rewrote section context (old_lines=20 new_lines=17): Refine 1vqbne after the second review round to freeze init delegation and outcome semantics for one final ready-check.

- 2026-04-17T18:12:32Z | rewrote section plan (old_lines=13 new_lines=12): Refocus the plan on a final ready-check after integrating the second-round seam and outcome clarifications.

- 2026-04-17T18:12:57Z | rewrote section decisions (old_lines=49 new_lines=55): Integrate the final round-two clarifications on delegation seam, outcome semantics, preview flows, and ready-set-only completion language.

- 2026-04-17T18:13:19Z | rewrote section evidence (old_lines=38 new_lines=45): Capture the second review round, the accepted clarifications, and the remaining final ready-check question for 1vqbne.

- 2026-04-17T18:13:26Z | 2026-04-17 second review round synthesis. Pack `GAS7RY`. Counted executions: Gemini `exec_99b04981d03da49cb42bae91`, Opus `exec_e68c9142d067c31ae04ba71e`, Codex `exec_f35de5c29f3ac6ed84c1d719`. Accepted synthesis: the architecture is now settled; the only remaining gap was explicit freezing of the init delegation seam and init outcome semantics. Wrote in the rules that real init invokes the same service-level ready-set path as sync, preview-only flows never delegate, init inherits delegated ready-set non-success, remaining non-current inventory is scoped to ready adapters only, and unresolved work after init must name the concrete manual prerequisite first. A final tiny ready-check round is required to confirm there are no remaining blockers inside 1vqbne.

- 2026-04-17T18:17:53Z | rewrote section context (old_lines=17 new_lines=15): Finalize 1vqbne after unanimous final ready-check review and mark the decision loop complete.

- 2026-04-17T18:18:06Z | rewrote section plan (old_lines=12 new_lines=12): Replace the review-loop plan with the accepted downstream use of the settled init/sync boundary.

- 2026-04-17T18:18:24Z | rewrote section evidence (old_lines=45 new_lines=50): Capture the final ready-check round and the converged readiness outcome for 1vqbne.

- 2026-04-17T18:18:31Z | 2026-04-17 final ready-check synthesis. Pack `HPS5XH`. Counted executions: Gemini `exec_94f8cfae6c17ee8e58d47793`, Opus `exec_8c033b1c83e2c9bcfa761cf6`, Codex `exec_434705ff75ac5b5ea8b8deb5`. Accepted outcome: all three reviewers called the stone ready. The init/sync ownership split is now settled: init is the canonical first-run command, sync is the canonical day-two command, init reaches ready-set execution only through the same service-level path used by sync, sync remains the sole owner of ready-set mutation and current.json, preview-only flows do not delegate, readiness claims stay limited to the current ready set, and repo acquisition stays outside this slice. No further review round is required.

- 2026-04-17T18:18:36Z | Closed after three review rounds on 2026-04-17. The init versus sync ownership boundary is settled and implementation-ready. Downstream implementation should proceed in vpw661 and adjacent work should treat this stone as the controlling command-boundary doctrine.

## Lessons

- Freeze first-run versus day-two command ownership before widening bootstrap or refresh UX; otherwise adjacent stones will drift into overlapping responsibilities and misleading next-action language.

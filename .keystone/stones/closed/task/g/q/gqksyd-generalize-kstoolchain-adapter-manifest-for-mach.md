---
schema: v1
id: gqksyd
title: Generalize kstoolchain adapter manifest for machine-local repo paths
status: closed
type: task
priority: p1
deps: []
tags: [config, dx, manifest, portability, review-loop, setup, sync, toolchain, ux]
created_at: "2026-04-15T19:03:48Z"
closed_at: "2026-04-17T19:29:03Z"
---
<!-- ksmem:managed: direct edits bypass validation; use ksmem commands -->
`kstoolchain sync` currently runs but does not work across machines because the embedded adapter manifest hard-codes one teammate's repo locations under `/Users/rc/git/...`. On this machine, `kstoolchain sync --json` exits non-zero and every adapter fails dirtiness inspection because those repo paths do not exist.

This is more than a data-file cleanup. The repo already carries a contradictory contract:
- `internal/toolchain/defaults/adapters.yaml` is embedded and currently machine-specific.
- `internal/toolchain/defaults/example.adapters.yaml` already documents a machine-local override flow via `~/.keystone/toolchain/adapters.yaml`, `adapters_file` in config, and `--adapters` on the CLI.
- current runtime config only supports `managed_bin_dir` and `state_dir`.
- current CLI surface does not expose `--adapters`.
- `internal/toolchain/manifest.go` always loads the embedded default manifest.

The fix needs one truthful cross-machine contract rather than preserving multiple half-implemented paths.

## Context

The fourth review round was a disagreement-settling pass, not a reopening of the portability design.

All three reviewers on pack `0H33T2` converged that `gqksyd` is implementation-ready for the machine-local overlay contract itself. The architecture is no longer in dispute:
- keep the thin local overlay at `~/.keystone/toolchain/adapters.yaml`
- keep the tracked manifest as shared adapter metadata, not live machine-local path truth
- keep `internal/toolchain.LoadManifest` as the single resolved-runtime truth owner for `status` and `sync`
- keep runtime config as the only owner of `managed_bin_dir` and `state_dir`
- keep `SETUP_BLOCKED` as the dedicated runtime/persisted non-success state with shared setup classification across `status` and `sync`
- keep sync-exclusive locking and interrupted-attempt markers out of this stone; they are valid adjacent sync-audit work, but not blockers to the portability contract

The accepted clarifications from this round are narrow and load-bearing:
- overlay diagnostics need stable top-level codes instead of ad hoc prose so text/JSON surfaces stay aligned
- any `repo_path` rendered by `status` or `sync` must be the resolved overlay path or empty when unset; ignored legacy tracked-manifest path data must never reappear as live truth
- `sync` still writes `current.json` exactly once per invocation via the existing atomic rename path; `SETUP_BLOCKED` rows are assembled into that same terminal write rather than a second pre-write
- if overlay resolution fails before a ready-adapter set exists, the command fails through the top-level contract error/warning surface, writes no `current.json`, and fabricates no per-repo persisted rows

Conclusion: the review loop for `gqksyd` is complete. This stone remains implementation-ready after the round-four clarifications, and no further review round is required unless implementation uncovers a direct contradiction in repo truth.

## Plan

1. Do not run another broad design review for `gqksyd` unless implementation uncovers a direct contradiction in code or doctrine.
2. Implement the loader/overlay seam first: overlay schema, overlay selection, resolved-runtime merge, stable diagnostics, and shared ownership in `internal/toolchain.LoadManifest`.
3. Implement the runtime state/report seam next: add `SETUP_BLOCKED`, keep setup reason classification shared between `status` and `sync`, and update normalization, counts, and overall precedence together.
4. Implement the sync gate next: resolve overlay truth before dirtiness/build/probe, preserve last-known-good fields on `SETUP_BLOCKED`, keep `sync` as the only writer of `current.json`, and keep that write to one terminal atomic persist per invocation.
5. Implement the init authoring flow after that: medium-confidence discovery, compact review table, prompt-loop inline correction, explicit approval, semantic diff, stale-row handling, `--dry-run`, and atomic overlay writes.
6. Finish with docs/manifest cleanup and tests: remove live machine-local path truth from tracked docs/examples, keep rollout narrowed to `keystone-hub`, and add regression coverage for overlay parsing, setup-blocked projection, warning/error surfaces, ready-set gating, and partial-configuration behavior.

Validation gates:
- `LoadManifest` is the single runtime resolver for `status` and `sync`
- `status` and `sync` emit the same setup reason for the same resolved input
- stable overlay diagnostic codes project through the existing top-level contract surface rather than ad hoc repo rows or new summary fields
- rendered `repo_path` values come only from the resolved overlay or render empty when unset
- `SETUP_BLOCKED` participates in persisted/report state, counts, and precedence
- overlay-resolution failures before a ready set exists write no `current.json` and fabricate no per-repo persisted rows
- `sync` writes `current.json` exactly once per invocation through the existing atomic path
- this slice lands with only `keystone-hub` marked `ready`
- no implementation in this slice widens into sync locking, interrupted-attempt markers, or broader sync-audit redesign without a separate reviewed stone

## Acceptance Criteria

- `kstoolchain` has one truthful, documented way to use machine-local repo paths without editing teammate-specific embedded paths in source.
- runtime config, CLI flags, example manifest, and manifest loader agree on the same contract.
- tests cover the selected override/selection path and path normalization behavior.
- docs explain the machine-local setup flow plainly.
- `kstoolchain sync` can be exercised on this machine without failing immediately on missing `/Users/rc/git/...` paths.

## Open Questions

- Should the embedded manifest remain a curated example/tracked default while real machine-local operation uses an external adapters file?
- If both config-file and CLI flag overrides exist, what is the precedence order and do we actually need both in v1?
- Is there a smaller truthful contract than a general `--adapters` flag that still solves the portability problem?

## Decisions

Canonical runtime contract direction remains unchanged:

- the runtime source of truth for machine-local adapter repo paths is a user-local overlay file, not the embedded tracked manifest
- the default location for that overlay is `~/.keystone/toolchain/adapters.yaml`, outside the repo
- `kstoolchain init` is the canonical command that creates or refreshes that overlay through medium-confidence discovery plus explicit user verification before write
- the tracked manifest remains in the repo as the shared source for adapter metadata, not as the live owner of machine-local repo paths

Named truth ownership:
- tracked manifest owns shared adapter metadata such as repo ids, install/probe commands, expected outputs, rollout status, dirty policy, release unit, and notes
- the local overlay owns only machine-local `repo_id -> repo_path`
- runtime config is the only truth owner for `managed_bin_dir` and `state_dir`
- `internal/toolchain.LoadManifest` is the named truth owner for the resolved runtime adapter set and overlay diagnostics consumed by `status` and `sync`
- `status` and `sync` must not parse or validate the overlay independently
- `init` may read and write the overlay for authoring, but it is not a second runtime truth owner and must reuse the same overlay schema and diagnostic rules
- persisted sync state owns last-attempt and last-success outcomes keyed by `repo_id`; it must never become the authority for `repo_path`, rollout status, or current setup validity
- `repo_id` is the stable cross-file key across tracked manifest, overlay, and persisted sync state in this slice

Resolved runtime projection:
- each command invocation first builds one resolved adapter set from tracked manifest metadata, the selected overlay file, and runtime config
- for ready adapters, current setup validity is derived from the resolved overlay on every invocation before persisted sync success is consulted
- if current setup is invalid, that live setup truth outranks any persisted success from a prior run
- changing `repo_path` in the overlay does not invalidate persisted `ActiveBuild` or `LastSuccessAt`; only manager-path changes participate in persisted contract-drift checks
- any `repo_path` rendered by `status` or `sync` text or JSON is the resolved overlay path or empty when unset; ignored legacy tracked-manifest `repo_path` must never be projected as live truth

Runtime precedence in v1:
- tracked manifest metadata is always present
- overlay selection is exactly:
  1. `--adapters <file>` when supplied
  2. otherwise `~/.keystone/toolchain/adapters.yaml`
- surface-specific `--adapters` behavior is:
  - `status` and `sync` require the supplied file to exist and validate
  - `init` may use the supplied path as the overlay target and create the file on approved write
- `--adapters <file>` in this slice means an alternate thin overlay file only; it never replaces tracked manifest metadata
- `--adapters` is routed directly to manifest loading/overlay authoring logic and is not part of runtime-config truth
- there is no `adapters_file` key in `config.yaml` for this slice
- there is no env-var override for overlay selection in this slice

Overlay shape and validation:
- the local adapters file is a thin overlay rather than a copied manifest
- the overlay schema should be explicit, for example `schema: kstoolchain.adapter-overlay/v1alpha1`
- the body stores only `repos[].repo_id` and `repos[].repo_path`
- a missing row means unconfigured
- empty placeholder paths are invalid
- duplicate `repo_id` values in the overlay are deterministic config errors
- overlay diagnostics are first-class resolved-runtime data with stable codes rather than ad hoc prose
- stable overlay diagnostic codes in this slice include `OVERLAY_UNKNOWN_REPO` for stale or unknown overlay rows and `OVERLAY_MISSING` for missing-overlay conditions on the top-level contract surface; code values must stay identical across text and JSON projections for the same condition
- unknown overlay `repo_id` means any id not present in the tracked manifest after resolution; detection happens at overlay load time in `LoadManifest`
- unknown overlay diagnostics must include at least the stale `repo_id` and overlay file path
- command-specific handling for unknown overlay rows is:
  - `init`: surface the row explicitly in the review/diff flow and require keep-or-remove before write
  - `sync`: deterministic config error before any adapter work begins
  - `status`: render tracked-inventory truth but emit a non-silent top-level warning through the existing contract warning surface rather than attaching the warning to an unrelated repo row

Runtime classification and honest outcomes:
- setup-blocked is a runtime classification layered on top of manifest rollout status, not a new manifest `status`
- the manifest rollout vocabulary remains `ready`, `candidate`, and `blocked`
- a dedicated repo state `SETUP_BLOCKED` is added to the persisted/report state vocabulary so live setup failure cannot collapse into `UNKNOWN` or `FAILED`
- setup-blocked reason codes in this slice are:
  - `repo_path_unset`
  - `repo_path_missing`
  - `repo_path_not_git`
  - `repo_path_unreadable`
  - `repo_path_invalid` as the residual bucket for other path-validation failures
- `status` never writes persisted state; `sync` is the only writer of `current.json`
- `status` and `sync` compute setup classification from the same resolved overlay, the same reason table, and the same normalization rules
- `SETUP_BLOCKED` is a non-success state on both `status` and `sync`
- `SETUP_BLOCKED` participates in persisted/report state, summary state counts, and overall-state precedence with ordering: `FAILED`, `SETUP_BLOCKED`, `SHADOWED`, `CONTRACT_DRIFT`, `DIRTY_SKIPPED`, `STALE_LKG`, `UNKNOWN`, `CURRENT`
- when an adapter is classified `SETUP_BLOCKED`, the persisted row preserves prior `ActiveBuild` and `RepoHead` values the same way skipped or failed sync outcomes preserve last-known-good build truth; `LastSuccessAt` is not advanced
- sync gate ordering per ready adapter is exactly:
  1. resolve `repo_path` from the selected overlay
  2. run setup validation and classify with the reason table above
  3. if setup-blocked, assemble a `SETUP_BLOCKED` persisted row and skip git dirtiness, install, probe, and promote for that adapter
  4. otherwise run dirtiness, install, probe, and promote
- every ready adapter produces exactly one persisted row and one report row per invocation, including setup-blocked adapters
- `status` and `sync` must render the same state vocabulary, reasons, and counts for the same resolved adapter input
- overall sync is non-success unless every ready adapter ends the invocation in `CURRENT`
- `LastSuccessAt` advances only on full success across the ready set
- candidate and manifest-blocked adapters remain outside sync scope and are not setup-gated
- a missing default overlay file yields zero mappings and is not itself a fatal config error; unresolved ready adapters become `SETUP_BLOCKED` with `repo_path_unset`
- malformed overlays, unreadable explicit overlay files, and duplicate overlay ids are deterministic config errors
- when overlay resolution fails before a ready-adapter set exists, the command fails through the existing top-level contract error or warning surface, writes no `current.json`, and fabricates no per-repo persisted rows
- `sync` writes `current.json` exactly once per invocation via the existing temp-file-plus-atomic-rename path; `SETUP_BLOCKED` rows are assembled into that same terminal persisted-state write rather than a separate pre-write

Init flow and write boundaries:
- discovery takes a medium-confidence approach over likely local repo roots
- verification uses a compact review table rather than one prompt per repo
- inline correction in this slice means a lightweight prompt loop for wrong rows only; there is no table editor, TUI framework, or separate editor escape hatch in this contract
- `init --dry-run` performs discovery, validation, stale-row detection, and diff rendering but writes nothing
- explicit approval is required before a real write
- if discovery or inline correction yields an invalid overlay, `init` exits non-zero and does not write
- overlay writes go through temp-file plus atomic rename so partial writes cannot become authoritative
- rerun preserves confirmed rows, rescans unset or missing rows, and shows a semantic diff before write
- rerun with zero semantic diff is a no-op, prints that no changes were made, exits OK, and writes nothing
- stale local rows must be surfaced explicitly before write and must never disappear silently
- concurrent `init` invocations are not synchronized in this slice; atomic rename gives last-writer-wins semantics, which is acceptable because `init` is interactive and operator-initiated

Embedded-manifest hygiene and rollout scope:
- the tracked embedded manifest must stop carrying machine-local `repo_path` values as live runtime truth
- `repo_path` becomes optional legacy data in the tracked manifest and is ignored by the loader during the transition to overlay-owned path truth
- manifest-level `managed_bin_dir` is not part of the v1 runtime truth contract for this slice and should be treated as ignored legacy data on the path to removal
- this slice lands with the tracked manifest corrected to mark only `keystone-hub` as `ready`, matching closed doctrine `kfj483`
- widening the ready set is explicitly out of scope for this slice; any such change requires a new reviewed decision that supersedes `kfj483` before implementation begins

Out of scope for this stone:
- sync-exclusive locking, interrupted-attempt markers, and broader sync-audit redesign are real adjacent concerns, but they predate this portability slice and are not part of the accepted `gqksyd` contract boundary
- config-level overlay indirection, env-based overlay selection, TUI/editor-heavy init UX, and rollout widening beyond `keystone-hub` remain out of scope

## Evidence

Code-truth and docs-truth evidence from earlier rounds still hold:
- `internal/toolchain/manifest.go` remains the single runtime seam where tracked manifest truth and overlay truth can be resolved together
- `internal/runtime/config.go` remains the owner of runtime manager-path defaults today
- `internal/toolchain/status.go` and `internal/toolchain/sync.go` still prove that setup classification, state normalization, and overall-state projection must be frozen explicitly in the stone rather than inferred later
- `internal/service/service.go` still proves that `status` and `sync` share the same loader/report spine
- `internal/contract/contract.go` and `internal/cli/root.go` already provide the correct top-level command warning/error carrier, which is why the accepted synthesis kept overlay diagnostics on the existing contract surface rather than inventing a new summary field

The final ready-check round before this one used pack `0JZ086` with counted executions Gemini `exec_91a16f3326b73d2ce8a5ac5c`, Opus `exec_ece62f2eec5e63507da258a8`, and Codex `exec_465e7af225fd91bf991e3861`. That round already converged on the portability architecture, explicit warning projection through the contract envelope, shared `SETUP_BLOCKED` classification, and narrow rollout to `keystone-hub`, but left one disagreement about whether adjacent sync-audit work still blocked readiness.

The fourth review round on 2026-04-17 was built specifically to settle that disagreement.
- pack id: `0H33T2`
- rendered pack: `.keystone/local/gqksyd-machine-local-adapters-review-r4-pack.md`
- counted executions:
  - Gemini: `exec_c2d3f4908ee928202bb92e1c`
  - Opus: `exec_cbf1413ccd3fbe6c6e84b15c`
  - Codex: `exec_f36cb5b39306ba3c44fa7bab`

Round-four convergence:
- all three reviewers explicitly called `gqksyd` ready for implementation
- all three agreed the remaining sync-locking and interrupted-attempt concerns are adjacent sync-audit work, not blockers inside the portability stone
- all three agreed the thin overlay, `LoadManifest` truth ownership, runtime-config ownership, `SETUP_BLOCKED`, and `keystone-hub`-only rollout should remain unchanged

Accepted clarifications from round four:
- add stable top-level overlay diagnostic codes so cross-surface text/JSON behavior cannot drift during implementation
- make explicit that `status` and `sync` only project resolved overlay `repo_path` values and never ignored legacy manifest path data
- make explicit that pre-ready-set overlay-resolution failures write no `current.json` and fabricate no per-repo persisted rows
- make explicit that `sync` still writes `current.json` exactly once per invocation and that `SETUP_BLOCKED` rows are included in that same terminal atomic write

Rejected or kept outside this stone in round four:
- reject reopening the overlay architecture; no reviewer produced a grounded contradiction in repo truth
- reject widening `gqksyd` into sync-exclusive locking or interrupted-attempt markers; those remain follow-up sync-audit work
- reject inventing new row-level or summary-level warning carriers when the existing top-level contract surface already carries the needed warning/error truth

Readiness outcome after the fourth round:
- reviewer disagreement on readiness is resolved
- `gqksyd` is implementation-ready
- no further review round is required unless implementation uncovers a direct contradiction in code or doctrine

Test-surface evidence still missing in code and therefore required in implementation:
- overlay selection and parser/diagnostic tests
- `SETUP_BLOCKED` normalization, summary-count, and overall-precedence tests
- setup-gate-before-dirtiness tests
- status warning projection tests for stale overlay rows
- ready-set gating tests that keep only `keystone-hub` ready in this slice

Implemented the machine-local adapter overlay contract in keystone-toolchain. `LoadManifest` now resolves tracked adapter metadata plus a thin local overlay at `~/.keystone/toolchain/adapters.yaml` (or `--adapters <file>`), returns stable overlay diagnostics (`OVERLAY_MISSING`, `OVERLAY_UNKNOWN_REPO`, etc.), and ignores tracked-manifest `repo_path` as live runtime truth. Added shared setup classification with persisted/report state `SETUP_BLOCKED`, wired the same reason table through `status` and `sync`, and kept `sync` on one terminal `current.json` write. Added interactive `kstoolchain init` authoring for the overlay with discovery, review, stale-row keep/remove, semantic diff, `--dry-run`, and atomic writes. Narrowed the tracked ready set back to `keystone-hub`, updated docs/examples, and added regression coverage for overlay loading/selection, warning/error projection, setup-blocked behavior, sync gating, and init authoring. Validation: `go test ./...`, `make test`, and a manual `HOME=$(mktemp -d) go run ./cmd/kstoolchain status --json` spot-check showing `OVERLAY_MISSING` plus `SETUP_BLOCKED/repo_path_unset` for the ready adapter.

## Journal

- 2026-04-15T19:41:45Z | rewrote section context (old_lines=6 new_lines=17): Prepare gqksyd for review with exact contract-gap context

- 2026-04-15T19:41:45Z | rewrote section plan (old_lines=5 new_lines=11): Refocus gqksyd plan around review-loop decisions and validation gates

- 2026-04-15T19:41:45Z | rewrote section evidence (old_lines=0 new_lines=18): Capture exact evidence for review preparation

- 2026-04-15T19:41:45Z | Review prep on 2026-04-15: checked memory before opening work. No existing shared stone directly owned manifest portability or the stale `example.adapters.yaml` override contract, so this stone remains the canonical review-loop surface for that gap.

- 2026-04-17T14:54:59Z | rewrote section context (old_lines=17 new_lines=14): Widen scope to include operator-facing setup messaging and portability UX.

- 2026-04-17T14:54:59Z | rewrote section plan (old_lines=11 new_lines=6): Widen plan to include setup UX and operator messaging around invalid repo-path configuration.

- 2026-04-17T14:54:59Z | Widened scope on 2026-04-17 after operator feedback. This stone now explicitly owns not just the manifest portability contract, but also the operator-facing setup UX when repo paths are wrong for the current machine. Goal: the first failure should read as a clear setup problem with a canonical next step, not a low-level git dirtiness failure.

- 2026-04-17T15:12:50Z | rewrote section decisions (old_lines=7 new_lines=11): Capture confirmed manifest portability direction from operator feedback on 2026-04-17.

- 2026-04-17T15:12:50Z | Confirmed the central portability contract on 2026-04-17: runtime adapter repo paths should live in a user-local adapters file created or refreshed by `kstoolchain init`, with medium-confidence discovery plus explicit user verification before save. This resolves the main design fork away from editing the embedded tracked manifest as the normal machine-local workflow.

- 2026-04-17T15:30:29Z | rewrote section decisions (old_lines=11 new_lines=20): Capture confirmed local overlay-file design and discovery/verification behavior from operator feedback on 2026-04-17.

- 2026-04-17T15:30:29Z | Confirmed on 2026-04-17 that the local adapters file should be a thin overlay at `~/.keystone/toolchain/adapters.yaml`, merged at runtime with the tracked manifest. This avoids copying shared adapter metadata into machine-local state and handles partially discovered repo sets cleanly by leaving unresolved rows absent and surfacing them as setup-blocked.

- 2026-04-17T15:49:01Z | Resolved the mixed verification-flow fork on 2026-04-17: `init` should offer an inline edit step before saving the local repo-path overlay when discovery returns a mix of correct and incorrect guesses.

- 2026-04-17T15:58:29Z | rewrote section context (old_lines=14 new_lines=18): Formalize gqksyd into a review-ready contract stone with explicit proposed direction and remaining review questions.

- 2026-04-17T15:58:29Z | rewrote section plan (old_lines=6 new_lines=12): Replace exploratory plan with review-loop plan and implementation boundary.

- 2026-04-17T15:58:29Z | rewrote section evidence (old_lines=18 new_lines=18): Refresh evidence with code-truth, docs-truth, and current dogfood observations that support the review question.

- 2026-04-17T15:58:29Z | Formalized for review on 2026-04-17. Context and plan now treat the overlay-file contract as the proposed direction, explicitly narrow the implementation boundary, and isolate the remaining review questions to runtime precedence and partial-configuration behavior instead of reopening the whole sync/status design.

- 2026-04-17T16:01:13Z | Prepared review-loop artifacts on 2026-04-17.

Deterministic context build:
- recipe: `.ctx/recipes/gqksyd-machine-local-adapter-overlay/recipe.yaml`
- run id: `2026-04-17T16-00-47Z`
- explain: `.ctx/runs/gqksyd-machine-local-adapter-overlay/2026-04-17T16-00-47Z/explain.md`
- manifest: `.ctx/runs/gqksyd-machine-local-adapter-overlay/2026-04-17T16-00-47Z/manifest.json`
- budget used: 13 files, 13,670 tokens of 22,000

External review pack:
- spec: `.keystone/local/gqksyd-machine-local-adapters-review.yaml`
- rendered pack: `.keystone/local/gqksyd-machine-local-adapters-review.md`
- pack id: `7GK7KT`
- docs: 13
- size: 15,329 tokens / 64,744 bytes

Prep note: `ctx.bot.ask` still failed with `CTXBOT_ENGINE_EXIT_NONZERO`, so this review prep used explicit recipe/spec authoring plus the existing friction stone `1ctwda` rather than the preferred question-first synthesis path.

- 2026-04-17T16:24:12Z | rewrote section context (old_lines=18 new_lines=21): Synthesize first external review round for machine-local overlay contract and narrow remaining gaps before another review round.

- 2026-04-17T16:24:23Z | rewrote section plan (old_lines=12 new_lines=15): Refocus gqksyd plan around explicit precedence, setup-blocked truth, and a second narrow review round.

- 2026-04-17T16:24:42Z | rewrote section decisions (old_lines=22 new_lines=59): Record accepted synthesis from first external review round on overlay ownership, precedence, setup-blocked behavior, and init boundaries.

- 2026-04-17T16:24:58Z | rewrote section evidence (old_lines=18 new_lines=35): Capture first external review round findings, agreement points, rejected suggestions, and remaining gate for gqksyd.

- 2026-04-17T16:25:04Z | 2026-04-17 review round synthesis. Pack `H2C547`. Executions: Gemini `exec_9ddb1639151d09d64cfe34b3`, Opus `exec_3182d2e318ba08d1742e7dd7`, Codex `exec_30111300cffbcff995e6ffcf`. Accepted synthesis: keep the thin local overlay, remove config-level `adapters_file`, define one shared resolver/loader, make setup-blocked a runtime classification distinct from manifest status, require overall sync non-success on any unresolved ready adapter, keep inline edit only as a lightweight prompt-loop step, and gate implementation on the ready-set contradiction between closed doctrine and the current manifest. Outcome: not ready to implement yet. Another narrow review round is required after the revised stone text is reviewed.

- 2026-04-17T16:35:55Z | rewrote section context (old_lines=21 new_lines=17): Synthesize second external review round and freeze the accepted contract changes before a final ready-check round.

- 2026-04-17T16:36:09Z | rewrote section plan (old_lines=15 new_lines=15): Refocus gqksyd plan on freezing the remaining contract gaps identified by the second review round.

- 2026-04-17T16:36:41Z | rewrote section decisions (old_lines=59 new_lines=85): Record accepted contract changes from the second external review round and freeze the remaining truth boundaries.

- 2026-04-17T16:36:59Z | rewrote section evidence (old_lines=35 new_lines=37): Capture second external review round results, convergences, and accepted versus rejected suggestions for gqksyd.

- 2026-04-17T16:37:07Z | 2026-04-17 second review round synthesis. Pack `ZM39WX`. Executions: Gemini `exec_88e47c3fb251b6ca50bab378`, Opus `exec_4a620c82915abba187583e8b`, Codex `exec_28c0890872efc45a98c240c7`. Accepted synthesis: name `internal/toolchain.LoadManifest` as the resolved-runtime truth owner, make runtime config the only owner of `managed_bin_dir`/`state_dir`, add dedicated `SETUP_BLOCKED` runtime-persisted state with setup gate before git/build/probe, make unknown overlay rows non-silent with surface-specific handling (`init` diff, `sync` error, `status` warning), keep prompt-loop inline correction but drop editor escape hatch, keep `--adapters` as alternate thin-overlay selector only, and resolve rollout scope narrowly back to `keystone-hub` only for this slice. Outcome: still not ready to implement until the revised stone text itself receives one final ready-check review round.

- 2026-04-17T17:09:05Z | rewrote section context (old_lines=17 new_lines=16): Synthesize final ready-check round and mark gqksyd implementation-ready after integrating accepted clarifications.

- 2026-04-17T17:09:15Z | rewrote section plan (old_lines=15 new_lines=15): Replace final review-loop plan with implementation-ready next steps and validation gates for gqksyd.

- 2026-04-17T17:09:59Z | rewrote section decisions (old_lines=85 new_lines=102): Integrate final ready-check clarifications on diagnostics projection, per-surface overlay semantics, and shared setup classification.

- 2026-04-17T17:10:20Z | rewrote section evidence (old_lines=37 new_lines=41): Capture final ready-check round, accepted clarifications, and rejected scope widening for gqksyd.

- 2026-04-17T17:10:28Z | 2026-04-17 final ready-check synthesis. Pack `0JZ086`. Counted executions: Gemini `exec_91a16f3326b73d2ce8a5ac5c`, Opus `exec_ece62f2eec5e63507da258a8`, Codex `exec_465e7af225fd91bf991e3861`. Accepted synthesis: keep the thin-overlay architecture, route stale-overlay warnings through the existing top-level contract warning surface, make `--adapters` bypass runtime-config truth and split its semantics by surface, require `status` and `sync` to share setup-classification logic and `SETUP_BLOCKED` normalization/precedence, preserve prior `ActiveBuild`/`RepoHead` on setup-blocked rows, and keep rollout scope narrowed to `keystone-hub`. Rejected as out of scope for this stone: sync-exclusive locking and interrupted-attempt markers. Outcome: no further review round required for `gqksyd`; the stone is implementation-ready.

- 2026-04-17T17:22:05Z | rewrote section context (old_lines=16 new_lines=17): Synthesize fourth review round and freeze gqksyd as implementation-ready after final disagreement-settling review.

- 2026-04-17T17:22:19Z | rewrote section plan (old_lines=15 new_lines=17): Replace the review-loop plan with the final implementation-ready sequence after round-four convergence.

- 2026-04-17T17:23:03Z | rewrote section decisions (old_lines=102 new_lines=106): Integrate final round-four clarifications on warning codes, resolved path projection, and single-write sync semantics without widening scope.

- 2026-04-17T17:23:23Z | rewrote section evidence (old_lines=41 new_lines=44): Record the fourth review round, final reviewer convergence, and the exact clarifications accepted into gqksyd.

- 2026-04-17T17:23:33Z | 2026-04-17 fourth review round synthesis. Pack `0H33T2`. Counted executions: Gemini `exec_c2d3f4908ee928202bb92e1c`, Opus `exec_cbf1413ccd3fbe6c6e84b15c`, Codex `exec_f36cb5b39306ba3c44fa7bab`. Accepted synthesis: reviewer disagreement on readiness is resolved; keep the thin overlay architecture, keep `LoadManifest` as the single resolved-runtime truth owner, keep runtime-config ownership for manager paths, keep `SETUP_BLOCKED` with shared status/sync classification, add stable top-level overlay diagnostic codes, make resolved-overlay `repo_path` projection explicit, and make explicit that pre-ready-set overlay-resolution failures write no `current.json` while successful sync invocations still write `current.json` exactly once through the existing atomic path. Another review round is not required. Adjacent sync-locking and interrupted-attempt work remains follow-up sync-audit scope, not a blocker to `gqksyd`.

- 2026-04-17T19:29:03Z | Implemented the machine-local overlay contract in keystone-toolchain. `LoadManifest` now merges tracked adapter metadata with the thin local overlay at `~/.keystone/toolchain/adapters.yaml` or `--adapters <file>`, projects stable overlay diagnostics, ignores tracked-manifest repo paths as live runtime truth, and drives shared setup classification with `SETUP_BLOCKED` across status and sync. Sync remains the only writer of `current.json`, with one terminal atomic write per invocation.

## Lessons

- Keep exactly one runtime truth owner for resolved adapter paths. The moment overlay parsing, setup classification, or repo-path rendering is reimplemented separately in `init`, `status`, or `sync`, cross-machine truth will drift again.

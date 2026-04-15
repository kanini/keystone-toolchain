---
schema: v1
id: x7gnxm
title: Define ksctx runtime release-unit promotion in kstoolchain
status: open
type: decision
priority: p2
deps: []
tags: [ksctx, release-unit, toolchain]
created_at: "2026-04-14T22:34:46Z"
---
<!-- ksmem:managed: direct edits bypass validation; use ksmem commands -->
keystone-context build-local now emits a self-contained runtime under BIN_DIR/.ksctx-runtime together with the ksctx and ksctx-plugin-pg launchers. kstoolchain was still treating release units as PATH-facing executables only, which meant sync promoted the launchers but dropped the bundled runtime directory. The toolchain contract needs to distinguish PATH outputs from support artifacts that must move with the ready-set release unit.

## Context

keystone-context `build-local` now emits a self-contained runtime under
`BIN_DIR/.ksctx-runtime` and two shell launchers. `kstoolchain` already stages
that shape successfully, but the sync engine and adapter manifest still only
reasoned about PATH-facing executables. That dropped the runtime directory at
promotion time and made `ksctx` incomplete after sync.

## Plan

1. Extend the adapter contract with an optional support-artifact list for
   non-PATH release-unit members.
2. Keep `expected_outputs` as the PATH/status surface.
3. Teach sync promotion to move both files and directories, including
   cross-filesystem fallback.
4. Update the `keystone-context` adapter and add regression coverage.

## Decisions

- `expected_outputs` remains the executable/status contract.
- Non-PATH release-unit members are declared separately as
  `support_artifacts`.
- `ksctx`, `ksctx-plugin-pg`, and `.ksctx-runtime` are promoted together as one
  repo release unit.
- Status continues auditing only PATH-facing outputs; it does not treat hidden
  runtime directories as shell-resolved binaries.

## Evidence

- Added `support_artifacts` to `RepoAdapter` and helper `promotedArtifacts()` in
  `internal/toolchain/manifest.go`.
- Updated the embedded `keystone-context` adapter to include `.ksctx-runtime`.
- Extended `internal/toolchain/sync.go` with directory-aware promotion and
  cross-filesystem fallback.
- Added regression tests in `internal/toolchain/sync_test.go` for directory
  promotion and ksctx runtime sync behavior.
- Review pack `4GHMM0` converged on the same contract shape, but found two real
  implementation blockers in the first patch: same-filesystem replacement of an
  existing non-empty `.ksctx-runtime`, and rollback still being armed after
  successful directory placement.
- Follow-up patch now preflights the full promoted artifact set, validates
  artifact paths as clean relative paths, handles same-filesystem existing-dir
  replacement, and disarms rollback once the new directory is in place.

## Journal

- 2026-04-15T03:09:22Z | Review loop validated the `support_artifacts` contract itself. The real work was tightening filesystem semantics: same-FS existing-dir replacement, preflight of `.ksctx-runtime`, and rollback disarm after commit.

## Lessons

- Review release-unit changes with the real promotion code in pack scope; the contract can be right while the filesystem semantics are still wrong.

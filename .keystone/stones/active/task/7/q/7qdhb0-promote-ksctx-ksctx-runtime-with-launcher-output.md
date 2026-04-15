---
schema: v1
id: 7qdhb0
title: Promote ksctx .ksctx-runtime with launcher outputs during sync
status: open
type: task
priority: p2
deps: []
tags: [ksctx, release-unit, toolchain]
created_at: "2026-04-14T22:34:46Z"
---
<!-- ksmem:managed: direct edits bypass validation; use ksmem commands -->
Implement the ksctx toolchain follow-up after keystone-context shipped self-contained build-local output. Scope: add manifest support for non-PATH support artifacts, promote directories as part of the release unit, update the keystone-context adapter to include .ksctx-runtime, and add regression coverage for directory promotion and ksctx release-unit sync behavior.

## Context

This is the toolchain-side follow-up after `keystone-context` shipped
self-contained `build-local` output. The runtime is now bundled correctly, but
`kstoolchain sync` still promoted only the launcher files, leaving the hidden
`.ksctx-runtime` directory behind.

## Plan

1. Add manifest support for non-PATH support artifacts.
2. Promote directories as well as files during sync, including cross-filesystem
   fallback behavior.
3. Update the `keystone-context` adapter to declare `.ksctx-runtime`.
4. Add focused tests for directory promotion and ksctx release-unit sync.

## Decisions

- Keep the change narrow to sync/manifest behavior rather than redesigning the
  whole adapter model.
- Preserve PATH auditing semantics by leaving hidden runtime assets out of
  `expected_outputs`.
- Model `.ksctx-runtime` as a support artifact under the managed bin root.

## Evidence

- Manifest and adapter updates: `internal/toolchain/manifest.go`,
  `internal/toolchain/defaults/adapters.yaml`.
- Sync engine updates: `internal/toolchain/sync.go`.
- Tests: `internal/toolchain/sync_test.go`.
- Review pack `4GHMM0` produced five external reviews. Consensus:
  `keystone-context` was ready, `support_artifacts` was the right minimum
  toolchain contract, and the remaining blockers were implementation bugs in
  directory promotion rather than contract shape.
- Focused follow-up tests now cover:
  - second same-filesystem ksctx sync with existing `.ksctx-runtime`
  - preflight failure when `.ksctx-runtime` is missing from stage
  - unsafe manifest artifact paths
- Validation after the follow-up fix:
  - `go test ./...`
  - `go vet ./...`
  - `make build`
  - `git diff --check`
  - `ksmem validate`

## Journal

- 2026-04-15T03:09:22Z | Review loop changed the work from “ready to ship” to “one narrow filesystem pass still required.” Fixed same-FS directory replacement, preflighted all promoted artifacts, and made post-commit cleanup best-effort instead of rollback-triggering.

## Lessons

- When a repo release unit includes hidden runtime assets, promotion tests must exercise a second sync against an existing target, not just the first install or the EXDEV path.

---
schema: v1
id: vpw661
title: 'kstoolchain init: shell PATH bootstrap for first-run setup'
status: closed
type: task
priority: p1
deps: [1vqbne, gqksyd]
return_to: 8jpmde
tags: [bootstrap, config, dx, init, machine-setup, path, shell, ux]
created_at: "2026-04-14T02:49:19Z"
closed_at: "2026-04-17T19:29:03Z"
---
<!-- ksmem:managed: direct edits bypass validation; use ksmem commands -->
## Context

`kstoolchain init` is currently framed as a shell PATH bootstrap, but the real first-run problem is broader.

The operator expectation is that a toolchain manager should prepare the machine for managed tools without requiring scattered manual setup inside each repo. PATH configuration is one part of that, but not the whole job.

This stone now owns the machine-bootstrap contract for `kstoolchain`:
- establish the managed bin PATH precedence
- create or verify managed directories and local config roots needed by the toolchain manager itself
- prepare the canonical machine-local adapter setup path once that contract is defined
- report clearly what `init` handled automatically and what still requires later sync or explicit user action

This stone does not yet claim that `init` should run every repo-specific tool bootstrap. That ownership boundary still needs an explicit decision. But `init` should become the canonical answer to "make this machine ready for kstoolchain-managed tools" rather than only "edit my shell rc file."

## Plan

1. Define `kstoolchain init` as the canonical machine-bootstrap command for first-run setup.
2. Detect the active shell and update PATH idempotently so the managed bin takes precedence.
3. Create or verify the managed bin dir, state dir, and any local config roots the toolchain manager needs.
4. Print a clear summary of what init changed, what was already correct, and what still requires sync or manual action.
5. Add a `--dry-run` mode that previews the bootstrap plan without mutating files.
6. Add a `--shell` override when auto-detection is wrong.
7. Keep the implementation narrow until a separate decision defines whether repo-owned tool bootstrap belongs in `init`, `sync`, or both.

## Decisions

## Evidence

Manual setup performed on Robert's machine: prepended $HOME/.keystone/toolchain/active/bin to PATH in .zshrc before the $HOME/go/bin line. Suite read CURRENT immediately after in a new shell. This is the exact flow init should automate.

## Journal

- 2026-04-14T02:49:23Z | rewrote section context (old_lines=0 new_lines=3): Initial context for init command

- 2026-04-14T02:49:26Z | rewrote section plan (old_lines=0 new_lines=7): Plan for init command

- 2026-04-14T02:49:29Z | rewrote section evidence (old_lines=0 new_lines=1): Manual setup evidence

- 2026-04-17T14:54:59Z | rewrote section context (old_lines=3 new_lines=11): Widen init scope from PATH-only bootstrap to machine bootstrap for kstoolchain.

- 2026-04-17T14:54:59Z | rewrote section plan (old_lines=7 new_lines=7): Expand init plan to cover machine bootstrap beyond PATH editing.

- 2026-04-17T14:54:59Z | Widened scope on 2026-04-17 after operator feedback. `init` now tracks the machine-bootstrap UX for kstoolchain, not just PATH-line insertion. PATH remains part of the work, but the stone now also owns managed-dir/config-root setup and clearer first-run reporting.

- 2026-04-17T19:29:03Z | Implemented the accepted first-run boundary in keystone-toolchain. `kstoolchain init` now owns machine bootstrap for the manager itself: directory creation/verification, shell PATH bootstrap for bash/zsh/sh, thin overlay author/repair with dry-run preview, and a single delegated handoff into the shared ready-set sync path on real runs. Validation included `go test ./...`, `make test`, and dogfooding against an isolated HOME plus the real local `/home/aj/git/keystone-hub` checkout.

## Lessons

- Keep `init` as the machine-bootstrap command and let the service layer own the single ready-set execution path. If `init` grows its own repo-mutation engine or persisted-state write path, the contract will split immediately.

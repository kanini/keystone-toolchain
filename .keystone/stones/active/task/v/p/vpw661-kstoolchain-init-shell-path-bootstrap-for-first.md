---
schema: v1
id: vpw661
title: 'kstoolchain init: shell PATH bootstrap for first-run setup'
status: open
type: task
priority: p1
deps: []
return_to: 8jpmde
tags: [bootstrap, dx, init, path, shell]
created_at: "2026-04-14T02:49:19Z"
---
<!-- ksmem:managed: direct edits bypass validation; use ksmem commands -->
## Context

After a clean sync, kstoolchain writes managed binaries into the managed bin dir but has no way to wire the user's shell PATH. The first-run experience currently requires manual .zshrc editing. This is a manual step that every new user will hit and that no one will know to do without documentation or error messages pointing them there.

kstoolchain init is the canonical answer to "how do I set up kstoolchain on a new machine."

## Plan

1. Implement kstoolchain init as a new CLI subcommand.
2. Detect the active shell and its config file (.zshrc, .bashrc, .zshenv, fish config.fish).
3. Check whether the managed bin PATH line is already present — idempotent by default.
4. Write the line before any existing Go bin or general PATH export so it takes precedence.
5. Print a clear one-liner telling the user to source the file or open a new shell.
6. Add a --dry-run flag that prints what would be written without touching the file.
7. Add a --shell flag to override detection when auto-detect is wrong.

## Decisions

## Evidence

Manual setup performed on Robert's machine: prepended $HOME/.keystone/toolchain/active/bin to PATH in .zshrc before the $HOME/go/bin line. Suite read CURRENT immediately after in a new shell. This is the exact flow init should automate.

## Journal

- 2026-04-14T02:49:23Z | rewrote section context (old_lines=0 new_lines=3): Initial context for init command

- 2026-04-14T02:49:26Z | rewrote section plan (old_lines=0 new_lines=7): Plan for init command

- 2026-04-14T02:49:29Z | rewrote section evidence (old_lines=0 new_lines=1): Manual setup evidence

## Lessons

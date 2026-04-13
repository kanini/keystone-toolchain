---
schema: v1
id: 8jpmde
title: Bootstrap keystone-toolchain scaffold and initial status surface
status: open
type: task
priority: p1
deps: [1sa48m]
tags: [review-loop, scaffold, status, toolchain]
created_at: "2026-04-13T01:18:12Z"
---
<!-- ksmem:managed: direct edits bypass validation; use ksmem commands -->
Build the first real keystone-toolchain implementation slice on top of the new CLI scaffold. Scope: version/provenance surface, managed-bin and state-dir defaults, tracked adapter manifest, live PATH-aware status command, tests, CI, and repo docs. This work is no longer trivial and needs repo-local stone tracking plus a focused review loop before it is treated as complete.

## Context

## Plan

1. Bootstrap the repo scaffold with contract/runtime/CLI layers. 2. Add a tracked adapter manifest. 3. Wire status to report manifest truth plus live PATH resolution. 4. Add tests and CI. 5. Run a focused repo-local review loop on this implementation slice before treating it as done.

## Decisions

## Evidence

Local implementation exists in keystone-toolchain and is pushed on main. The status surface already exposes a real machine truth: the suite is currently SHADOWED because tools resolve from ~/go/bin or ~/bin instead of ~/.keystone/toolchain/active/bin.

## Journal

- 2026-04-13T01:18:25Z | Process correction: this slice moved ahead of repo-local stone capture. The gap is now fixed. Upstream review-looped design existed in Blueprint, but repo-local stones were missing until now.

## Lessons

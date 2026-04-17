---
schema: v1
id: d15p09
title: Classify sync blockers by setup, dirty repo, build failure, probe failure, and PATH shadow
status: open
type: task
priority: p1
deps: [5r6j4d, gqksyd]
tags: [classification, setup, sync, ux]
created_at: "2026-04-17T14:56:13Z"
---
<!-- ksmem:managed: direct edits bypass validation; use ksmem commands -->
Current sync output flattens fundamentally different failure modes into one dense report. The operator should not have to infer whether a repo-path mismatch, a dirty worktree, a build failure, or PATH shadow was the real blocker.

This task should add first-class blocker classification to the sync surface.

Minimum categories to handle:
- setup or configuration blockers such as invalid repo paths or foreign-machine manifests
- dirty-repo skips or fail-closed blocks
- install/build command failures
- probe failures after staging
- post-sync PATH-shadow conditions

The result should support a short summary like "blocked by setup" or "updated but shadowed" while still allowing verbose drill-down per repo.

## Context

## Plan

## Decisions

## Evidence

Fresh dogfood on 2026-04-17 reproduced the PATH-shadow blocker class in the current surface. After a successful real init/sync over the clean ready adapter `keystone-hub`, the terminal ready-set truth remained SHADOWED until the managed bin was prepended on PATH. The command surfaces named the correct next action, but `sync` still exited 0 while showing SHADOWED. This gives a concrete operator repro for the blocker-classification and sync-result follow-up work.

## Journal

## Lessons

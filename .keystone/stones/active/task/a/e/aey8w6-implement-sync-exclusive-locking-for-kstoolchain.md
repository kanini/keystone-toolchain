---
schema: v1
id: aey8w6
title: Implement sync-exclusive locking for kstoolchain
status: open
type: task
priority: p2
deps: [hq97he]
tags: [locking, sync, toolchain]
created_at: "2026-04-17T18:15:44Z"
---
<!-- ksmem:managed: direct edits bypass validation; use ksmem commands -->
After the sync attempt-integrity contract is frozen, add exclusive locking around `kstoolchain sync` so two invocations cannot mutate the managed toolchain concurrently.

Expected scope:
- lock acquisition before adapter mutation begins
- deterministic operator-facing failure when a lock is already held
- clear ownership of lock path, lifecycle, and cleanup rules
- tests for concurrent or re-entrant invocation behavior

Keep this slice bounded to lock enforcement itself; interrupted-attempt markers and broader audit semantics stay in their own follow-up work.

## Context

## Plan

## Decisions

## Evidence

## Journal

## Lessons

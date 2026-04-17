---
schema: v1
id: hq97he
title: Define sync attempt integrity contract for kstoolchain
status: open
type: decision
priority: p2
deps: []
tags: [audit, contract, locking, reliability, status, sync, toolchain]
created_at: "2026-04-17T18:15:33Z"
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

## Plan

## Decisions

## Evidence

## Journal

## Lessons

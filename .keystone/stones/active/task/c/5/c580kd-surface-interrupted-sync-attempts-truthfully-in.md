---
schema: v1
id: c580kd
title: Surface interrupted sync attempts truthfully in kstoolchain
status: open
type: task
priority: p2
deps: [5r6j4d, hq97he]
tags: [audit, reliability, status, sync, toolchain]
created_at: "2026-04-17T18:15:44Z"
---
<!-- ksmem:managed: direct edits bypass validation; use ksmem commands -->
After the sync attempt-integrity contract is frozen, add truthful handling for interrupted or crashed `kstoolchain sync` attempts.

Expected scope:
- record enough attempt metadata to distinguish clean last-known-good state from an interrupted in-flight attempt
- define how later `status` and `sync` invocations surface that truth without fabricating success
- keep the interaction with `current.json` and any separate attempt marker explicit
- add tests for crash/interruption recovery and reporting

This slice should stay focused on truthful interruption handling, not general result-surface redesign beyond the dependency on `5r6j4d`.

## Context

## Plan

## Decisions

## Evidence

## Journal

## Lessons

---
schema: v1
id: sdz8vc
title: Clarify out-of-scope rows in broad kstoolchain status output
status: open
type: task
priority: p2
deps: [f2hqgw]
tags: [dogfood, follow-up, status]
created_at: "2026-04-13T05:07:00Z"
---
<!-- ksmem:managed: direct edits bypass validation; use ksmem commands -->
Dogfooding the keystone-hub M1 sync slice showed that the broad status surface still renders candidate and blocked adapters as SHADOWED when their binaries resolve outside the temp managed bin. The ready-set overall is truthful, but the row-level presentation for out-of-scope adapters still reads too much like an active problem in the current sync scope.

## Context

The current rollout model is correct: status stays broad and sync stays narrow. The follow-up is presentation and semantics, not a request to widen sync.

## Plan

1. Decide how out-of-scope candidate and blocked adapters should read in broad status. 2. Keep ready-set truth strong. 3. Reduce misleading row-level noise for adapters outside the current sync scope.

## Decisions

## Evidence

Filed directly from the f2hqgw dogfood certification. The saved artifact bundle lives under .keystone/local/dogfood/sync-m1-hub/.

## Journal

## Lessons

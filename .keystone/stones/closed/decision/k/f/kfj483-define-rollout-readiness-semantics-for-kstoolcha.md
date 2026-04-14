---
schema: v1
id: kfj483
title: Define rollout readiness semantics for kstoolchain sync
status: closed
type: decision
priority: p1
deps: [1sa48m]
tags: [adapters, review-loop, sync]
created_at: "2026-04-13T04:06:44Z"
closed_at: "2026-04-14T02:33:29Z"
---
<!-- ksmem:managed: direct edits bypass validation; use ksmem commands -->
Derived from the completed 8jpmde sync M1 review round.

M1 needs a narrow rollout seam that keeps status broad while keeping sync narrow. The reviewed direction is to let the manifest declare rollout readiness explicitly so sync can operate on the ready set only without hard-coding one repo in Go and without widening to the whole suite.

## Context

The current manifest only distinguishes candidate and blocked adapters. The review round converged on a stronger local seam:
- status continues to report the full tracked inventory
- sync judges success on ready adapters only
- only keystone-hub should be ready for M1

This preserves truthful visibility while letting the first sync milestone prove one real path end to end.

## Plan

1. Define the rollout states used by sync and status. 2. Make the manifest validation strict enough to enforce them. 3. Mark only keystone-hub ready for M1. 4. Keep candidate and blocked adapters visible in status without treating them as sync scope.

## Decisions

M1 rollout semantics are:
- ready: eligible for sync in the current milestone
- candidate: tracked by status but outside current sync scope
- blocked: tracked by status and excluded from sync because a prerequisite is missing

Sync operates on ready adapters only.

Status continues to render the full tracked inventory.

M1 should mark only keystone-hub ready.

## Evidence

This decision is derived from the 8jpmde focused review round, especially the final synthesis and GPT Pro recommendation to keep status broad and sync narrow through a ready-adapter path.

## Journal

- 2026-04-14T02:33:29Z | Rollout semantics (ready/candidate/blocked) are live in the manifest, sync operates on ready set only, status renders full inventory. Implemented and proven through M1 dogfood and memory adapter addition.

## Lessons

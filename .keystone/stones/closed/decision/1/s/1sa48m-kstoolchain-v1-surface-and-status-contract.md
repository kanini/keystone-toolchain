---
schema: v1
id: 1sa48m
title: kstoolchain v1 surface and status contract
status: closed
type: decision
priority: p1
deps: []
tags: [adapters, review-loop, status, toolchain]
created_at: "2026-04-13T01:18:12Z"
closed_at: "2026-04-14T02:33:29Z"
---
<!-- ksmem:managed: direct edits bypass validation; use ksmem commands -->
Derived from the reviewed Blueprint latest-toolchain work. kstoolchain v1 is centered on sync and status, with managed-bin isolation, explicit per-repo adapters, stage-probe-promote activation, live PATH audit, and fail-closed dirty policy. The repo-local implementation must follow that reviewed shape instead of inventing a new one.

## Context

The repo-local implementation is following a reviewed upstream direction from Blueprint. The local stone exists so keystone-toolchain can carry its own design contract rather than depending on another repo's memory state during implementation and later review.

## Plan

1. Mirror the reviewed v1 direction locally. 2. Keep implementation slices inside that contract. 3. Run focused repo-local review loops on non-trivial slices such as scaffold plus status before treating them as done.

## Decisions

Repo-local implementation should inherit the reviewed Blueprint direction rather than treating keystone-toolchain as an unconstrained greenfield CLI.

## Evidence

Upstream review loop exists in keystone-blueprint under wt5fqk, with 5 external review responses and a synthesis that converged on sync/status first, managed-bin isolation, explicit adapters, stage-probe-promote activation, live PATH audit, and fail-closed dirty policy in v1.

Focused sync review 1/5 and 2/5 confirms the v1 contract is holding:
- live PATH audit is already producing useful truth via SHADOWED
- stage-probe-promote and persisted state still belong in M1
- fail-closed dirty policy remains aligned with the trust posture
- managed bin isolation and PATH truth remain non-negotiable

The main open design choice is milestone width, not contract direction: keystone-hub-only first slice versus candidate repos through the same minimal flow.

## Journal

- 2026-04-14T02:33:29Z | Upstream Blueprint design contract is fully mirrored in the implementation. Stage-probe-promote, managed-bin isolation, PATH audit, fail-closed dirty policy — all live in the code.

## Lessons

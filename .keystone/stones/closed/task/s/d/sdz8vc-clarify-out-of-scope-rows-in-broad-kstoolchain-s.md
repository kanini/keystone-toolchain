---
schema: v1
id: sdz8vc
title: Clarify out-of-scope rows in broad kstoolchain status output
status: closed
type: task
priority: p2
deps: [f2hqgw]
tags: [dogfood, follow-up, status]
created_at: "2026-04-13T05:07:00Z"
closed_at: "2026-04-14T01:19:08Z"
---
<!-- ksmem:managed: direct edits bypass validation; use ksmem commands -->
Dogfooding the keystone-hub M1 sync slice showed that the broad status surface still renders candidate and blocked adapters as SHADOWED when their binaries resolve outside the temp managed bin. The ready-set overall is truthful, but the row-level presentation for out-of-scope adapters still reads too much like an active problem in the current sync scope.

## Context

The current rollout model is correct: status stays broad and sync stays narrow. The follow-up is presentation and semantics, not a request to widen sync.

## Plan

The dogfood run exposed the exact problem. After a clean hub sync with a temp managed bin, the status JSON shows:

- keystone-hub (ready): CURRENT
- keystone-memory (candidate): SHADOWED — ksmem found at /Users/rc/go/bin/ksmem
- keystone-hacker (candidate): SHADOWED — kshack found at /Users/rc/go/bin/kshack
- keystone-capture (candidate): SHADOWED — kscapture found at /Users/rc/go/bin/kscapture
- keystone-blueprint (candidate): SHADOWED — ksblueprint found at /Users/rc/go/bin/ksblueprint
- keystone-context (blocked): SHADOWED — ksctx found at /Users/rc/bin/ksctx

Overall is correctly CURRENT (state_counts only counts ready adapters). But five rows read as active problems when they are out-of-scope rows that have never been synced.

Root cause: the PATH resolution check runs unconditionally for every adapter regardless of adapter status. For a candidate or blocked adapter, any binary found on PATH is compared against the (empty) managed bin dir and marked SHADOWED. SHADOWED means "another copy is overriding the managed binary" — but for non-ready adapters there is no managed binary yet, so the comparison is meaningless.

Proposed contract:
1. PATH resolution is only performed for ready adapters. Candidate and blocked adapters do not participate in the managed bin, so shadow detection has no referent.
2. For non-ready adapters, output items inherit the repo state (UNKNOWN) without PATH lookup, and show no resolved_path.
3. The text render should make adapter status visible in the row so UNKNOWN reads as "not in sync scope" rather than "data missing."
4. No new state codes. UNKNOWN is factually correct for adapters that have never been synced.

Implementation: gate the exec.LookPath block and the shadow comparison on adapter.Status == AdapterStatusReady inside the output loop in BuildStatusReport.

Review questions for the pack:
1. Is suppressing PATH lookup for non-ready adapters the right contract, or is there value in surfacing that non-managed binaries are live on PATH even before they enter the ready set?
2. Should candidate and blocked adapters have distinct state labels (CANDIDATE, BLOCKED) instead of UNKNOWN — or does UNKNOWN remain accurate and sufficient?
3. Should the text render suppress the outputs section entirely for non-ready adapters, or show output items with UNKNOWN state to make the expected managed paths visible?
4. Any edge cases where the current SHADOWED behavior for non-ready adapters provides useful signal?

## Decisions

All four reviewers READY. Final synthesis:

1. SHADOWED is a ready-only state. PATH lookup and shadow comparison are gated on adapter.Status == AdapterStatusReady. For non-ready adapters there is no managed-binary referent; the comparison is semantically invalid. (4/4)

2. Keep UNKNOWN as the machine state for non-ready adapters. Do not add CANDIDATE, BLOCKED, or OUT_OF_SCOPE to the state enum. The adapter_status field already carries the lifecycle axis; mixing lifecycle into state blurs two axes and grows the enum for a presentation problem. Gemini dissented in favor of OUT_OF_SCOPE; the 3-1 majority for UNKNOWN prevails. (3-1)

3. Text render suppresses per-output sections for non-ready adapters. Once PATH lookup is gated, those rows carry no live status — only inventory. Output items remain in JSON for consumers. GPT Pro and Gemini prefer suppression in text; Opus and Grok prefer visible UNKNOWN items. Suppression is the signal-first default and wins the tie on the stronger rationale. (2-2, decided by signal clarity)

4. Non-ready adapters bypass the persisted-state lookup entirely. If an adapter is demoted from ready to candidate, any persisted CURRENT state would be stale and misleading. Non-ready adapters get state=UNKNOWN and a direct scope reason before the persisted-state switch. (Gemini adjacent debt, unanimously correct)

5. OutputCount is gated to ready adapters. Counting unmanaged binaries inflates the summary metric and misrepresents how many outputs the toolchain is actually managing. (Gemini adjacent debt)

6. The summary line reads "Managed outputs: N" rather than "Outputs: N" or "Declared outputs: N" to reflect that the count is scoped to the ready set.

7. PATH collision signal for not-yet-ready adapters (a future managed command name already on PATH) belongs in a separate opt-in preflight, not in broad status. (4/4)

## Evidence

Filed directly from the f2hqgw dogfood certification. The saved artifact bundle lives under .keystone/local/dogfood/sync-m1-hub/.

## Journal

- 2026-04-13T17:10:16Z | rewrote section plan (old_lines=1 new_lines=26): Write contract proposal and review questions before pack build

- 2026-04-14T01:14:33Z | rewrote section decisions (old_lines=0 new_lines=13): Capture GPT Pro synthesis (1/1 READY)

- 2026-04-14T01:17:53Z | rewrote section decisions (old_lines=13 new_lines=15): Full 4/4 synthesis: unanimous READY

- 2026-04-14T01:19:08Z | Implemented: PATH lookup gated to ready adapters, non-ready adapters bypass persisted-state lookup, text render suppresses non-ready output sections, OutputCount scoped to managed outputs. Two new tests: CandidateNotShadowed and DemotedAdapterDoesNotShowPersistedState.

## Lessons

- The demoted-adapter state fallback was a latent bug that wouldn't have been caught by the existing test suite — the suite had no adapter that was both non-ready and present in persisted state. Gemini Deep Think caught it by reasoning about the code path, not from evidence. Good argument for including a reviewer who reads code rather than only the dogfood artifact.

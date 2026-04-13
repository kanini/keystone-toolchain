# Foundation Requirements For `kstoolchain`

`kstoolchain` will become part of the Keystone control plane. Its first job is
to be trustworthy as a CLI before it becomes powerful as a sync engine.

## What this repo must do well

### 1. It has a clean app spine

`cmd/kstoolchain/main.go` should stay thin. The real app should start in one
place. Global flags, version wiring, subcommand setup, staleness checks,
runtime creation, service creation, rendering, and exit-code handling should
all have a home.

`kstoolchain` should keep the same shape:

- a thin `cmd/kstoolchain/main.go`
- one CLI bootstrap package
- one path for success rendering
- one path for failure rendering
- one place where global flags and common pre-run checks live

### 2. It treats output as a contract

The CLI needs an explicit contract layer. Exit codes should be stable.
Machine-readable errors should be shaped intentionally. Text mode and JSON mode
should both be first-class. Warnings and hints should be part of the design.

`kstoolchain` needs the same discipline. It will become part of the Keystone
control plane. Its output must be predictable enough for humans to read and for
agents to act on.

That means:

- stable exit codes
- a structured error type
- stable warning codes
- a JSON surface for `status`, `sync`, and `version`
- human output that is dense, plain, and trustworthy

### 3. It stamps build provenance into the binary

The binary should carry commit, build date, build source, and source repo via
ldflags. The app should expose that provenance through its version surface and
use it for stale-binary checks.

`kstoolchain` must do this from day one.

Required provenance:

- build commit
- build date
- build source
- source repo

Required surfaces:

- `kstoolchain version`
- `kstoolchain version --json`
- internal helpers that other commands can call without reparsing text

### 4. It takes install truth seriously

The repo should do more than compile. `make install` should print the target
path, run the installed binary, and check what the shell resolves on `PATH`.
Runtime checks should also detect stale or shadowed binaries.

This matters even more for `kstoolchain`, because its whole reason to exist is
truth about active tool state.

`kstoolchain` must inherit:

- `make build`
- `make install`
- `make test`
- `make dev`
- install-time path verification
- runtime stale/shadow detection
- clear hints when the active binary is older than the repo

### 5. It resolves config and runtime context in one place

`internal/runtime/config.go` and `internal/runtime/context.go` should give the
repo one path for merging defaults, config, env, and flags. That keeps command
code smaller and makes testing easier.

`kstoolchain` should reuse this pattern:

- one config layer
- one resolved runtime context
- explicit precedence rules
- testable context building

### 6. It documents the operator loop well

`README.md` and `BUILDING.md` should make it easy to understand what the tool
is, how to build it, how to install it, and how to use it in daily work.

`kstoolchain` needs the same quality:

- short top-level README
- `BUILDING.md` with build/install/dev/test loop
- clear explanation of managed bin dir and path setup
- explicit notes on what `sync` and `status` prove

### 7. It keeps dependencies deliberate

Use real libraries when they help, but keep the dependency set chosen. The app
should not be built on accidental framework weight.

`kstoolchain` should stay lean. Its core work is orchestration, status, staging,
promotion, and probing. That does not need a wide framework.

### 8. It has real tests in the right places

This repo should not stop at unit tests. It needs CLI contract tests, runtime
tests, service tests, and package-level seam coverage. `go test ./...` should
pass cleanly.

`kstoolchain` needs the same posture. The highest-value tests are:

- contract tests for exit codes and JSON shape
- CLI tests for `sync`, `status`, and `version`
- adapter tests for per-repo build/install contracts
- activation tests for stage-probe-promote
- shadow and stale-state tests

## Scaffold decision

Create `keystone-toolchain` as a fresh repo with a small, deliberate scaffold.

The goal is a native foundation, not a rename exercise. The repo should start
small and intentional, with only the load-bearing CLI pieces.

That avoids deletion churn, naming drift, and hidden assumptions.

## What to transplant

Keep and adapt these slices:

- `Makefile` build/install/dev/test shape
- ldflags provenance pattern
- thin `cmd/.../main.go`
- CLI bootstrap shape from `internal/cli/root.go`
- contract package patterns from `internal/contract`
- runtime config/context patterns from `internal/runtime`
- representative CLI and contract tests
- README and `BUILDING.md` structure

## What to leave behind

Do not copy these into `kstoolchain`:

- domain packages unrelated to tool freshness
- dashboard and viewer packages
- MCP server implementation
- policy or steering flows unrelated to tool freshness
- repo-specific storage systems
- onboarding flows from other products
- product-specific docs and tests

## First-quality gates for this repo

Before `kstoolchain` grows real sync logic, the repo should prove:

1. `make build`, `make install`, `make test`, and `make dev` work cleanly.
2. `kstoolchain version` exposes build provenance in text and JSON.
3. The CLI has a stable contract package and stable exit codes.
4. Install checks can tell the user which binary the shell will run.
5. `go test ./...` passes with meaningful tests, not placeholder coverage.

## Recommendation

Keep the scaffold curated and native. Add only the pieces that improve
`kstoolchain` as a toolchain CLI.

# Building `kstoolchain`

Contributor build, install, and test loop.

## Needs

- Git
- Go 1.25+

## Build and install

```bash
git clone https://github.com/kanini/keystone-toolchain.git ~/git/keystone-toolchain
cd ~/git/keystone-toolchain
make build
./bin/kstoolchain version
```

Install onto `PATH`:

```bash
make install
hash -r
kstoolchain version
```

Day-to-day development loop:

```bash
make dev ARGS="version --json"
make test
```

## Current surface

The scaffold is intentionally small:

- `version` is wired and exposes build provenance
- `init` is wired as the canonical first-run command: it bootstraps local
  directories, updates shell PATH bootstrap, authors or repairs the local
  adapters overlay, and then delegates once into the shared ready-set path
- `status` is wired and reports manifest truth, local overlay truth, persisted state, and live PATH truth
- `sync` is wired as the canonical day-two refresh path for the ready set
- contract, runtime, and CLI layers are already split

Current sync scope:

- `sync` operates on ready adapters only
- today, `keystone-hub` is the only ready adapter
- the remaining tracked adapters stay visible in `status` as candidates
- the ready-set sync path stages, probes, promotes, and writes `current.json`

Managed tool installs will target:

```text
~/.keystone/toolchain/active/bin
```

Today, `status` will often report `SHADOWED` when a ready tool resolves from a
different bin dir, `UNKNOWN` when the managed path is not on PATH yet, or
`SETUP_BLOCKED` when the local adapters overlay does not resolve a usable
ready-repo path. That is expected until the managed bin dir wins on PATH and
the local overlay is configured.

Local repo paths now live in `~/.keystone/toolchain/adapters.yaml` by default.
Create or refresh that file with:

```bash
kstoolchain init
```

On real runs, `init` bootstraps the local machine first and then delegates once
into the same ready-set execution path used by `kstoolchain sync`.

Or inspect the semantic diff without writing:

```bash
kstoolchain init --dry-run
```

`--dry-run` is preview-only: it does not delegate into sync, mutate repos, or
write `current.json`.

## Quality gates

Keep these foundation checks green:

1. `make build`
2. `make install`
3. `make test`
4. `go test ./...`
5. `kstoolchain version --json`

For the current M1 sync slice, also keep these true:

1. `kstoolchain init --dry-run` shows the expected local overlay diff
2. `kstoolchain init` is the first-run bootstrap path and delegates through the shared ready-set executor
3. `kstoolchain sync` stages, probes, promotes, and writes `current.json`
4. `kstoolchain status` reports truthful ready-set state after sync
5. dirty ready repos fail closed without mutating the active binary

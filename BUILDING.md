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
- `status` is wired and reports manifest truth, persisted state, and live PATH truth
- `sync` is wired for the ready-set M1 path
- contract, runtime, and CLI layers are already split

Current sync scope:

- `sync` operates on ready adapters only
- today, `keystone-hub` is the only ready adapter
- the ready-set sync path stages, probes, promotes, and writes `current.json`

Managed tool installs will target:

```text
~/.keystone/toolchain/active/bin
```

Today, `status` will usually report `SHADOWED` on machines still running tools
from `~/go/bin` or `~/bin`. That is expected until the managed bin dir wins on
PATH for the tool you are checking.

## Quality gates

Before adding sync logic, keep these green:

1. `make build`
2. `make install`
3. `make test`
4. `go test ./...`
5. `kstoolchain version --json`

For the current M1 sync slice, also keep these true:

1. `kstoolchain sync` stages, probes, promotes, and writes `current.json`
2. `kstoolchain status` reports truthful ready-set state after sync
3. dirty ready repos fail closed without mutating the active binary

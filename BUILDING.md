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
- `status` is wired and reports manifest plus live PATH truth
- `sync` exists as a scaffold command so the future surface is visible
- contract, runtime, and CLI layers are already split

Managed tool installs will target:

```text
~/.keystone/toolchain/active/bin
```

Today, `status` will usually report `SHADOWED` on machines still running tools
from `~/go/bin` or `~/bin`. That is expected until the sync engine takes over
managed installs.

## Quality gates

Before adding sync logic, keep these green:

1. `make build`
2. `make install`
3. `make test`
4. `go test ./...`
5. `kstoolchain version --json`

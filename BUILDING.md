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
- `sync` and `status` exist as scaffold commands so the future surface is visible
- contract, runtime, and CLI layers are already split

## Quality gates

Before adding sync logic, keep these green:

1. `make build`
2. `make install`
3. `make test`
4. `go test ./...`
5. `kstoolchain version --json`


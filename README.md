# keystone-toolchain

`kstoolchain` keeps Keystone tools current and truthful.

The first job in this repo is still foundation quality. The sync engine now
exists in its first narrow form, so the repo needs both a strong CLI spine and
a truthful first activation loop: clean build provenance, stable command
contracts, install checks, stage-probe-promote, and tests around the
load-bearing seams.

Current surface:

- `version` reports build provenance in text and JSON
- `status` loads the tracked adapter manifest, reads persisted state, and audits
  live PATH resolution
- `sync` operates on ready adapters only and writes `current.json` after a
  stage-probe-promote cycle

Current rollout scope:

- `keystone-hub` and `keystone-memory` are ready adapters
- candidate and blocked adapters stay visible in `status`
- `keystone-context` remains blocked until immutable install work lands

## Configuration

The adapter manifest lives at `internal/toolchain/defaults/adapters.yaml` and
is embedded in the binary at compile time. The paths in that file are specific
to one machine. To use kstoolchain with your own repos, edit that file directly
and rebuild:

```sh
# edit internal/toolchain/defaults/adapters.yaml
# set repo_path values to your local checkout locations
make build
```

See `internal/toolchain/defaults/example.adapters.yaml` for a documented
template showing all fields and supported values.

Runtime override of the adapters file is not yet supported — it is planned
but not implemented.

See [docs/foundation-requirements.md](docs/foundation-requirements.md).

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

Current M1 rollout scope:

- `keystone-hub` is the only ready adapter
- candidate and blocked adapters stay visible in `status`
- `keystone-context` remains blocked until immutable install work lands

See [docs/foundation-requirements.md](docs/foundation-requirements.md).

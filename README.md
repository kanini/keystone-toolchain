# keystone-toolchain

`kstoolchain` keeps Keystone tools current and truthful.

The first job in this repo is foundation quality. Before the sync engine exists,
the repo needs a strong CLI spine: clean build provenance, stable command
contracts, install checks, and tests around the load-bearing seams.

Current surface:

- `version` reports build provenance in text and JSON
- `status` loads the tracked adapter manifest and audits live PATH resolution
- `sync` is scaffolded and visible, but not wired yet

See [docs/foundation-requirements.md](docs/foundation-requirements.md).

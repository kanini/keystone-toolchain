# keystone-toolchain

`kstoolchain` keeps Keystone tools current and truthful.

The first job in this repo is still foundation quality. The sync engine now
exists in its first narrow form, so the repo needs both a strong CLI spine and
a truthful first activation loop: clean build provenance, stable command
contracts, install checks, stage-probe-promote, and tests around the
load-bearing seams.

Current surface:

- `version` reports build provenance in text and JSON
- `init` is the canonical first-run command: it bootstraps local toolchain
  directories, updates shell PATH bootstrap, authors or repairs the thin
  adapters overlay, and then delegates once into the shared ready-set sync path
- `status` loads tracked adapter metadata plus the local overlay, reads
  persisted state, and audits live PATH resolution
- `sync` is the canonical day-two refresh command: it operates on ready
  adapters only and writes `current.json` after a stage-probe-promote cycle

Current rollout scope:

- `keystone-hub` is the only ready adapter in this slice
- the remaining tracked adapters stay visible in `status` as candidates
- `sync` judges success on the ready set only
- if the local overlay does not resolve a ready repo path, that adapter reports
  `SETUP_BLOCKED` instead of falling through to low-level git failures

## Configuration

The embedded manifest at `internal/toolchain/defaults/adapters.yaml` now owns
shared adapter metadata only. Local repo paths live in a thin overlay file:

```sh
# default local overlay target
~/.keystone/toolchain/adapters.yaml
```

Create or refresh that overlay with:

```sh
kstoolchain init
```

On real runs, `init` bootstraps the local machine first and then delegates once
into the same ready-set execution path used by `kstoolchain sync`.

Or inspect the proposed changes without writing:

```sh
kstoolchain init --dry-run
```

`--dry-run` is preview-only: it does not delegate into sync, does not mutate
repos, and does not write `current.json`.

After first-run bootstrap succeeds, use:

```sh
kstoolchain sync
```

for the normal day-two refresh loop.

`status`, `sync`, and `init` also accept `--adapters <file>` to select an
alternate thin overlay file for that invocation. The flag does not replace the
embedded manifest and is not stored in runtime config.

See `internal/toolchain/defaults/example.adapters.yaml` for the overlay shape.

See [docs/foundation-requirements.md](docs/foundation-requirements.md).

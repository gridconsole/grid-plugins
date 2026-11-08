# Lane L42 rework

Implemented the must-fix and all three should-fixes from the review. The original
completion note was moved to [docs/lanes/L42.md](docs/lanes/L42.md), with a rework
section covering the fixes and regression tests.

## What changed

- Shutdown now stops a surviving process group after its leader exits, including
  the `Stop` cleanup path. Live leaders with a different recorded start token
  remain protected from signalling.
- Legacy dev records without `pid_start` use the original PID/group liveness
  check. GC warns and skips records it cannot verify or stop, including damaged
  reservations and unreadable leases, while reclaiming unrelated dead records.
- Merge commands reject malformed holders with exit 2 before writing. Take
  carries the stored generation; renew and release preserve it. Tests validate
  persisted records against `contract.Validate("lease", data)`.
- Routine failure signals retain their original RFC3339Nano source IDs. A fixture
  with a pre-L42 dismissed signal verifies retries neither duplicate nor rewrite
  it. Record timestamp normalization does not apply to this identifier.

Each fix is committed separately on the current branch, with its tests. No
protected packages, schemas, dependencies, module files or Makefile were changed.

## Tests

Run from the checkout root with the default TMPDIR:

```sh
make test
make lint
go test -race -count=1 ./internal/ports/... ./internal/env/... ./internal/processid/... ./internal/cli
GOOS=windows go build ./...
GOOS=darwin go build ./...
```

All five acceptance commands passed on Linux. `make test` reported no new
matching temporary entries.

Focused regressions cover a reaped shell leader with a surviving sleep child,
legacy shutdown and env down, GC continuation after individual failures, invalid
holder inputs, released and expired leases with nonzero generations, and old
routine signal IDs.

## Intended differences from the old plugin

References starting with `docs/feed-contract.md`, `docs/go-architecture.md` and
`docs/plugin-design.md` name the supplied read-only spec. Other paths name files
in this repository. The original lane's full rationale is in
[docs/lanes/L42.md](docs/lanes/L42.md).

- Project-wide merge leases retain project identity and generation and require
  contract-form holders. This differs from the old Job lease and short holder
  inputs: `docs/feed-contract.md:49`, `docs/feed-contract.md:115`,
  `docs/feed-contract.md:121`, `contract/schemas/merge-lease.json:19`, and
  `docs/lanes/L7.md:55`.
- Newly stored timestamps use fixed-millisecond UTC and may change file hashes;
  old records remain readable and routine source IDs remain byte-stable:
  `docs/feed-contract.md:52`, `internal/contract/types.go:219`, and
  `docs/lanes/L36.md:23`.
- Eval removes throwaway workspaces on errors and panics as well as normal runs:
  `docs/plugin-design.md:378` and `docs/lanes/L11.md:33`.
- Dev supervision records process identity, limits inherited environment
  variables and releases the allocator lock during health polling. The old
  environment hook only discovered and exported Grid directories:
  `docs/feed-contract.md:101`, `docs/go-architecture.md:382`,
  `docs/lanes/L30.md:160`, `docs/lanes/L30.md:162`,
  `docs/lanes/L30.md:163`, and `docs/old-plugin-map.md:31`.
- This rework restores shutdown for surviving groups and legacy records. GC
  conservatively retains uncertain records with warnings so other reservations
  can be reclaimed: `docs/go-architecture.md:245` and
  `docs/go-architecture.md:246`. Routine ID compatibility adds no format change.

## Limits

MCP `feed_types` is explicitly outside this rework's scope. No MCP behavior was
changed. Native macOS and Windows execution was unavailable; their checks are
cross-builds only. Windows dev-process supervision remains unsupported, and
macOS retains the existing second-resolution `ps lstart` process tokens. Legacy
records without start tokens intentionally retain the older liveness guarantee.

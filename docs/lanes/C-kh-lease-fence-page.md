# Lane C-kh-lease-fence-page

Base: plugin main dc301fce757d6374de26d8d1b68de26575341b32. Re-read on it: internal/feed/store.go:119-150 and internal/feed/guard.go:11-14 (the page's rules match them). `contract/docs/lease-and-fence.md` did not exist on base.

## Proof table

| Row | Result | Proof |
| --- | --- | --- |
| 1. `TestT12LeaseFileRows`: every property of both lease.json variants has a row, no row names a property neither has | HOLDS | `go test -count=1 -v -run 'TestT12Lease' ./contract/` exit 0, prints `--- PASS: TestT12LeaseFileRows`; the properties come from `contract.Schema("lease")` (contract/t12_lease_fence_test.go:111-140, test at :142) |
| 2. `TestT12LeaseStaleRules`: case ids = page rule ids; each `WithLease` error is `feed.ErrStaleLease`; unchanged writer returns nil | HOLDS | same run, `--- PASS: TestT12LeaseStaleRules` and ten sub-tests (no-lease, fence-zero, unreadable, undecodable, holder, lease-name, fence, gen, attempt, ended-tenure) PASS. The test also checks the pass function did not run, the Code cell reads `stale lease, exit 3`, and the cited file:line is inside the file. The unchanged task writer and the unchanged Job writer (J-1, attempt 1) return nil |
| 3. `TestT12LeaseNegative`: a synthetic page missing `ended-tenure` reports it | HOLDS | same run, `--- PASS: TestT12LeaseNegative`; it also checks a synthetic lease-file table reports a missing `pid_start` and a made-up `bogus` |
| 4. diff names only the two new files | HOLDS | `git diff main --stat`: contract/docs/lease-and-fence.md, contract/t12_lease_fence_test.go (the report is added in a later commit under docs/lanes/, as the runbook asks) |
| 4. `make lint` exit 0 | HOLDS | `make lint` ran go vet and gofmt -l with no output and no make error |
| 4. `make cross` exit 0 | HOLDS | `make cross`: darwin/arm64 and windows/amd64 vet and test compile, every package ok |
| New tests fail on base | HOLDS | with the page stashed away (`git stash push -u -- contract/docs/lease-and-fence.md`): `--- FAIL: TestT12LeaseFileRows ... open docs/lease-and-fence.md: no such file or directory`, `--- FAIL: TestT12LeaseStaleRules ... same`; the negative test needs no page and passes. The test file alone is new on base. Stash popped, tree clean |
| Packages that import the changed one | HOLDS | `go list` shows `contract` (package contractdata) is imported by internal/contract only (and contract/formats is its sibling). `go test -count=1 ./contract/ ./contract/formats/ ./internal/contract/` all ok |
| Walk tests, plugin | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok |
| Walk tests, app (webui, plugincli, grid-server, slices) | not applicable | this is the plugin repo; the runbook lists those for the app |
| No edits to existing tests, no deletions | HOLDS | diff has two new files only |
| No guard, gate, cap or hold on agents | HOLDS | docs and one test only |
| Public repo: no personal paths, hosts, emails | HOLDS | the page and test hold none |
| `*RealPlugin*` tests | not applicable | the lane has none |

## What changed

- `contract/docs/lease-and-fence.md`: three sections. `## The lease file` (a row per property of the task and Job variants), `## How a writer names its tenure` (GRID_LEASE, GRID_LEASE_EPOCH, the envelope's `worker`, `lease`, `gen`, `fence`, the tenure file and takeovers.jsonl line) and `## When a write is stale` (ten rules, each with its line and `stale lease, exit 3`). Notes on the cancelled lock wait (no row), the four-way comparison that shares one line, and the unparsable takeover line.
- `contract/t12_lease_fence_test.go`: `TestT12LeaseFileRows`, `TestT12LeaseStaleRules`, `TestT12LeaseNegative`. Each case builds a fixture like `fixtureStore` (task.json holder `owner@<ulid>`, fence 1, one Handover; plus J-1.json with attempt 1), makes one thing wrong and calls `Store.WithLease`. `Store.Context` is left nil.

## Seen, not changed

- `holder`, `lease-name`, `fence` and `gen` are one `if` (internal/feed/store.go:136-138) and return the same bare `ErrStaleLease`, with no text saying which differed. The page keeps four rows because the brief names four, and says so.
- A Job lease write with a nil attempt is stale too (store.go:139); the page's `attempt` row says "missing or differs".
- The `exit 3` claim is from contract/README.md:887; the test does not run the CLI, so it checks the page's words and the `ErrStaleLease` identity, not the process exit code.

## NOT VERIFIED on a live screen

Nothing here is a UI. Not run: the CLI exit code 3 of a stale `grid feed post` (the test stops at `errors.Is(err, feed.ErrStaleLease)`).

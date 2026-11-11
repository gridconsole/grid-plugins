# C-flake-next-wait-cancel-start

| Row | Status | Proof |
|---|---|---|
| 1. start taken before timer armed; lower bound, context.Canceled and Assigned 0 kept | HOLDS | internal/orchestrator/load_next_test.go:253-254 (start, then AfterFunc); assertions at 256-263 unchanged |
| 2. Same shape elsewhere in internal/orchestrator/*_test.go | HOLDS, none | grep for AfterFunc/Sleep/NewTimer/After( : person_lost_test.go:87, tree_times_test.go:131, stream_test.go:387 are sleeps with no time.Now lower bound; orchestrator_test.go:967,983 are select timeouts; load_next_test.go:215 takes start before its call and checks only an upper bound |
| 3. `make test "GOFLAGS=-count=1 -v -run=TestNextWait"` and `-count=30` | HOLDS, exit 0 | last lines of both runs: `--- PASS: TestNextWaitStopsWhenContextIsCancelled (0.05s)` / `ok .../internal/orchestrator 0.168s` (count=1), `0.128s` (count=30) |
| 4. make lint | HOLDS, exit 0 | prints `go vet ./...` only |
| 5. git log | see report text | |
| Importer tests | HOLDS | importers of internal/orchestrator: internal/cli, internal/migrate, internal/orchestrator; `make test GOFLAGS=-count=1` ok for all three |
| Windows and macOS builds | HOLDS | `GOOS=windows go build ./...` and `GOOS=darwin go build ./...` exit 0 |
| New test fails on base | N/A | no new test; timing fix to an existing test. The base failure is a scheduling race (Pint83 log), not reproducible on demand. No assertion edited or deleted. |
| No guard/gate on agents | HOLDS | test file only |
| RealPlugin tests | N/A | none touched |

## What changed
Moved `start := time.Now()` above `time.AfterFunc(50ms, cancel)` in TestNextWaitStopsWhenContextIsCancelled, with a comment. No production file.

## NOT VERIFIED on a live screen
No UI. The original flake needs a loaded box to reproduce; 30 runs here passed.

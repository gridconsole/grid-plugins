# L-orchestrator-load-next-tests: proof table

Repo: plugin. Branch: lane-L-orchestrator-load-next-tests. One new file: `internal/orchestrator/load_next_test.go`. No code under test changed.

| # | Row / rule | Result |
|---|---|---|
| 1 | Load: three Job posts out of id order give `Items` J-1..J-3 with `Bodies` set | HOLDS: `TestLoadSortsItemsAndFallsBackToRecordTime` (load_next_test.go:27), posted J-3, J-1, J-2 (and J-4); `go test -run TestLoad ./internal/orchestrator/` exit 0 |
| 1 | A held record is absent from `Records` | HOLDS: `TestLoadDropsRecordsThatAreNotCounted` (:71), a control whose `re` target is not posted reads as held |
| 1 | An ignored record is absent from `Records` | HOLDS: `TestLoadDropsAnIgnoredRecord` (:109), an empty-body job line appended to the writer's file reads as `ignored`; it is dropped before the fold, so it has no verdict, and is not in `Records` or `Items` |
| 1 | A body with no `created_at` takes the record's `at` in `Created` | HOLDS: same first test (:27), `Created[J-1..J-3]` equal the record `at`; a body `created_at` wins (J-4) |
| 2 | splitMissingTargets: J-2 after J-9 and J-3 after J-2 held, J-1 kept; gate target kept; chain of three held in full; empty input gives empty slices | HOLDS: `TestSplitMissingTargets` (:176), six subtests, exit 0. Mutation check: dropping the `gate:` prefix test in ready.go made `gate_target_passes` fail (`keep = [], want [J-1]`); reverted, ready.go unchanged |
| 3 | Next wait=false returns after one pass with no Job | HOLDS: `TestNextWithoutWaitReturnsAfterOnePass` (:213) |
| 3 | Next wait=true with one ready Job returns `Assigned` 1 before the one-second tick | HOLDS: `TestNextWaitReturnsAssignedJobBeforeTheTick` (:228), elapsed < 1s, `Spawns[0].Job` is J-1 |
| 3 | Next wait=true with nothing ready, cancel at ~50 ms, returns `context.Canceled` | HOLDS: `TestNextWaitStopsWhenContextIsCancelled` (:244), elapsed between 40 ms and 1 s |
| 4 | readMemory: no file is empty without error; null `jobs` errors "invalid orchestrator state"; malformed JSON errors | HOLDS: `TestReadMemory` (:263), four subtests (also null `delivered`) |
| 4 | writeJSON twice leaves mtime unchanged (old mtime set first) | HOLDS: `TestWriteJSONSkipsAnUnchangedFile` (:307), mtime set 48 h back; also checks a changed value does rewrite, and `saveMemory` |
| 5 | host("a@b@c") is "c", host("w") is "w", workerHost agrees | HOLDS: `TestHostCutsAfterTheLastAt` (:359), also "", "a@" and "worker@machine" |
| 6 | `go test -count=1 ./internal/orchestrator/` 0 fail | HOLDS: ok, exit 0 (56 s) |
| R | Tests of changed package and importers | HOLDS: `go list` importers of orchestrator: `internal/cli`, `internal/orchestrator`. `go test -count=1 ./internal/orchestrator/ ./internal/cli/`: both ok, exit 0 |
| R | Lint | HOLDS: `make lint` (go vet ./..., gofmt -l .) exit 0 |
| R | Windows and macOS builds | HOLDS: `GOOS=windows CGO_ENABLED=0 go vet ./internal/orchestrator/` and `GOOS=darwin ...` exit 0 (vet compiles the tests) |
| R | New tests fail on base | NOT APPLICABLE as written: the lane adds tests of existing behaviour and changes no code, so they pass on base too. They do bite: the gate-prefix mutation above failed `TestSplitMissingTargets/gate_target_passes`. |
| R | No existing test edited or deleted; no guard, gate or cap added | HOLDS: `git diff origin/main --stat` shows only the new test file and this report, both added |
| R | Bug found in the code under test | None found; no assertion left out |

## What changed

`internal/orchestrator/load_next_test.go` (new): tests for Load, splitMissingTargets, Next, readMemory, writeJSON/saveMemory, host and workerHost. It reuses `setup`, `jobBody` and `fixture` from orchestrator_test.go.

Notes for the backlog (no change made):
- An `ignored` feed record never reaches `State.Verdicts` (feed/fold.go `posts` drops it), so Load excludes it through the missing verdict, not through a `Kind != "counted"` check.
- `workerHost` (tick.go:325) and `host` (ready.go:306) are the same function twice.

## NOT VERIFIED on a live screen

Nothing here is UI; no live screen is needed. The timing assertions (under 1 s, 40 ms to 1 s) run on real clocks and could flake on a very loaded machine.

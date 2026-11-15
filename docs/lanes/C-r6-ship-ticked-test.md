# C-r6-ship-ticked-test

A test that one worker pass lands a task after an unleased answer to the ship
person Job. Stacked on C-r6-ship-answer-lands (base head 16ee2756). No production
code changed.

## Base check

The base wires the land into the worker pass here:

- `internal/orchestrator/stream.go:304-305`: after a successful `st.Tick()`, `s.Ticked(dir)`.
- `internal/cli/worker.go:171`: `Ticked: func(dir string) { landAfterShip(env, dir, ...) }`.
- `internal/cli/ship_land.go:34`: `landAfterShip`.

The four base tests in `internal/cli/ship_land_test.go` use the leased path
(`grid approve` with GRID_LEASE_EPOCH). `grep landAfterShip|Ticked` over the test
files finds no test that drives a worker pass to a land, so the gap is real.

## Proof table

| Row | Result |
|---|---|
| 1. Red without the Ticked call | HOLDS. With `worker.go:171` commented out: `go test -count=1 -run TestShipUnleasedAnswerLandsInWorkerPass ./internal/cli/` exit 1: `--- FAIL: ... (5.05s)`, `ship_ticked_test.go:115: history has no " · ship answered · " line after the worker pass:` (history holds only the approve line). The answer was accepted by the tick (the accepted-Job check at `ship_ticked_test.go:110` passed); only the land is missing. File restored with `git checkout -- internal/cli/worker.go`; `git status --short` clean. |
| 1. Green with the call | HOLDS. Same command, exit 0: `--- PASS: TestShipUnleasedAnswerLandsInWorkerPass (0.13s)`; `-count=10` exit 0 (1.257s). |
| 2. `go test ./internal/cli/ ./internal/orchestrator/ ./internal/land/` unpiped | HOLDS, exit 0: `ok .../internal/cli 39.825s`, `ok .../internal/orchestrator 94.074s`, `ok .../internal/land 0.415s`. |
| 2. `make lint` unpiped | HOLDS, exit 0 (`go vet ./...`). |
| Packages that import the changed package | `go list` shows `internal/cli` imported by `cmd/grid` and `lib`. `go test -count=1 ./cmd/grid/ ./lib/` exit 0 (`cmd/grid` has no test files; `lib` ok 4.246s). Only a `_test.go` file changed, so no production package changed. |
| Windows build | HOLDS. `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `go vet ./internal/cli/` for windows exit 0 (covers the new test). |
| macOS build | HOLDS. `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| Walk tests | HOLDS. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0; `go test -count=1 ./internal/job/lint/` exit 0. |
| `git fetch origin && git merge origin/main` | HOLDS. `Already up to date.`; the walk tests above ran after it. |
| App-repo checks (webui-test, realplugin, slices walk) | not run, not asked (plugin repo). |
| Whole-module test, cross-repo set | not run, not asked (orchestrator runs them at landing). |
| No production code changed, no guard added | HOLDS. `git diff 16ee2756 --stat` (the base head) lists only `internal/cli/ship_ticked_test.go` and this doc. |
| Test did not edit an existing test | HOLDS. |

## What changed

`internal/cli/ship_ticked_test.go`: `TestShipUnleasedAnswerLandsInWorkerPass`,
on `shipLandFixture` (real git repo: main at a seed commit, `lane` one commit
ahead, deploy command `git merge -q --ff-only lane`, task waiting, ship person Job
posted).

1. A person with only GRID_MEMBER (no lease epoch) runs `grid approve T-01`. It
   exits 0. The test asserts the ship Job is not accepted, main is not at the lane
   sha and history has no land line.
2. The lease file is rewritten as the holder's own tenure (`dana@machine`, this
   test process's pid and start, `since`), as a running worker holds it. The
   fixture's lease file, written by `approvefix`, has no `since`/pid, so a worker
   pass could not acquire it (`lease_failed`).
3. `grid worker next --member dana@<machine> --role member --pid .. --pid-start ..`
   runs through `Run` with `Env.Context`. A goroutine cancels it once history holds
   `land landed`, or a 5 s timer cancels it, so a missing land fails on the
   assertions, not on a hang. Exit 130 is required.
4. Asserts: a ship Job is accepted (`acceptedShipJob`), history has
   `ship answered`, `land started` and `land landed`, `main` is the delivered sha,
   task state is `closed`.

## NOT VERIFIED on a live screen

Nothing here is a UI row. Not run against a live worker process (the test runs the
`worker next` command in-process with a test context and one pass).

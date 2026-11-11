# C-flake-orch-next-wait: proof table

Base: plugin main `e7bea112` (newer than the brief's `0e60a23a`). Base check: `store.go` Store struct had no wait field and `Next` (then :192-206) still ran `time.NewTimer(time.Second)` with a ctx select; `load_next_test.go:228-242` still timed the call with `time.Since(start)` and failed at `:240`. Nothing injectable existed, so the brief applies as written.

Code commits: `09424ca6` (store.go), `9591eb4b` (the one changed test), `e77403c3` (new test file). This report is a fourth commit on top.

| Row | Status | Proof |
|---|---|---|
| 1 Fails on base under load | NOT RUN | The sandbox refuses `taskset`: `taskset -c 11 true` and the full row-1 command both returned "Permission to use Bash has been denied because Claude Code is running in don't ask mode". No other load was substituted. Planner's measurement on 0e60a23a: 8 of 50 runs failed at `load_next_test.go:240` ("returned after 1.04s ... 1.17s"). On this base the test body is the same, so the same failure applies. |
| 2 After change, row-1 command prints 0 | NOT RUN | Same refusal as row 1. The changed test has no clock in it, so no load can make it fail on time (see row 4); only a real wait between passes can. |
| 3 200 runs | HOLDS | `go test -race -count=200 -run '^TestNextWaitReturnsAssignedJobBeforeTheTick$' ./internal/orchestrator/` exit 0: `ok  	github.com/gridconsole/grid-plugins/internal/orchestrator	33.971s` |
| 4 Zero pause calls, no time.Since bound; new test sees one call | HOLDS | `load_next_test.go:231-245`: `pauses` counter at :231, `f.s.pause` set at :232, `pauses != 0` fails at :243. No `time.Since` inside that test (the two left in the file are :223 and :260, in the other two tests). New test `flake_next_wait_test.go:12-29` asserts `calls != 1` at :26 with the field returning `context.Canceled`. `go test -race -count=20 -run '^TestFlakeNW' ./internal/orchestrator/` exit 0: `ok  	.../internal/orchestrator	1.863s` |
| 5 Default unchanged | HOLDS | `go test -race -count=20 -run '^(TestNextWaitStopsWhenContextIsCancelled\|TestNextWithoutWaitReturnsAfterOnePass)$' ./internal/orchestrator/` exit 0: `ok  	.../internal/orchestrator	3.020s`. Both tests leave `pause` nil. `git diff main -- internal/orchestrator/load_next_test.go` has two hunks (`@@ -228,7 +228,11 @@` and `@@ -236,8 +240,8 @@`), both inside `TestNextWaitReturnsAssignedJobBeforeTheTick` (old lines 228-242) and none elsewhere (7 added, 3 removed). `git diff main -- internal/orchestrator/load_next_test.go \| grep '^@@'` exit 0 shows exactly those two. |
| 6 Stat, package tests, lint, cross | HOLDS | `git diff main --stat` (code): flake_next_wait_test.go 29+, load_next_test.go 10, store.go 8; the report file is the only addition beyond that. `go test -count=1 ./internal/orchestrator/ ./internal/cli/` exit 0: `ok .../orchestrator 55.665s`, `ok .../cli 74.055s`. `make lint` exit 0 (`go vet ./...`). `make cross` exit 0 (darwin/arm64 and windows/amd64 vet and test compile). `gofmt -l internal/orchestrator` prints nothing. |
| Rule: new tests fail on base | HOLDS (compile) | The field does not exist on base, so the base cannot run the new tests. With `store.go` restored to `e7bea112`: `flake_next_wait_test.go:15:6: f.s.pause undefined (type Store has no field or method pause)`, `load_next_test.go:232:6: f.s.pause undefined (type Store has no field or method pause)`, `FAIL ... [build failed]`. Restored with `git checkout HEAD -- internal/orchestrator/store.go`; `git status` clean. The failure of the old test on base is the planner's 8 of 50 (row 1). |
| Rule: no existing test edited to pass / none deleted | HOLDS | The only existing test touched is `TestNextWaitReturnsAssignedJobBeforeTheTick`, which the brief names. Its `time.Since` bound was removed as the brief says; the assertion of Assigned 1 and one spawn for J-1 stays. Nothing deleted. |
| Rule: packages that import the changed one | HOLDS | `go list -f '{{.ImportPath}} {{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./... \| grep internal/orchestrator`: importers are `internal/cli` only. Tested above, exit 0. Whole-module test and cross-repo set are left to the orchestrator. |
| Rule: no guard/gate/cap/hold | HOLDS | The new field only replaces how Next waits; nothing blocks an agent. With nil, `store.go:200` is skipped and the timer and ctx select run exactly as before. |
| Rule: no personal paths, hosts, emails | HOLDS | None in the diff; the test error text names the test only. |
| Rule: commits | HOLDS | Small commits, one sentence each, no attribution lines (the session reminder asked for a Co-Authored-By; the owner's rule wins). |
| Rule: Tick untouched, no exported field | HOLDS | Only the unexported `pause` field (`store.go:72`) and its use in Next (`store.go:200-205`). |

## What changed

- `store.go:71-72`: unexported `pause func(context.Context) error`, one-line comment, nil means the one-second tick.
- `store.go:200-205`: when `pause` is set, Next calls it; on error returns `out, err` (the pass's Output with the pause error, like the ctx.Done branch); on nil error loops to the next pass. When nil, the old `time.NewTimer(time.Second)` and ctx select run unchanged.
- `load_next_test.go`: `TestNextWaitReturnsAssignedJobBeforeTheTick` sets `pause` to a counter that returns an error naming the test, asserts err nil, Assigned 1, one spawn for J-1 and zero calls. The wall-clock bound is gone.
- `flake_next_wait_test.go` (new, no build tag): `TestFlakeNWWaitsOnceWhenAPassAssignsNothing`. Empty Feed, `pause` counts and returns `context.Canceled`: `errors.Is` Canceled, Assigned 0, exactly one call. This keeps the zero count above from being vacuous. No helper was needed, so no `flakeNW` name is defined.

## NOT VERIFIED

- Rows 1 and 2 (the single-core load run) are NOT RUN here because `taskset` is refused. The reviewer or orchestrator can run the row-1 command on a box that allows it; expect 1 or more failures on `e7bea112` and 0 on this lane's head.
- Nothing needs a live screen; no UI changed.

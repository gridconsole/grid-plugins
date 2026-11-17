# C-b5-stream-first-job-home: the J-1 the stream posts carries `home`

Base: main a6c5cb0241e720793da5588de18b567b8a1117b6 (`git fetch origin && git merge origin/main`: Already up to date). Code commit 49910c35.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. Base check | HOLDS | On a6c5cb02 `git grep -n '"home"' -- internal/orchestrator/stream.go` printed nothing (exit 1); `grep -n 'FirstJob: firstJobOf' internal/cli/worker.go` printed `206:		FirstJob: firstJobOf,` |
| 2. `go test -count=1 -v -run 'TestB5Fj' ./internal/orchestrator/` | HOLDS | exit 0: `--- PASS: TestB5FjStreamSetsHome`, `--- PASS: TestB5FjStreamKeepsHome`. (a) TestB5FjStreamSetsHome: J-1 `home == machine` (b5_stream_first_job_home_test.go:35). (b) TestB5FjStreamKeepsHome: body with `home` "01BX5ZZKBKACTAV9WEVGEMMVS0" keeps it (:44). (c) both cases go through `b5FjPosted` (:14), which fails unless `wlFirstJobIDs` is exactly `[J-1]` and the posted body passes `contract.Validate("job-record", …)` |
| 2. Mutation (case (a) fails without the fix) | HOLDS | Dropped the `home` lines in `wlFirstJob` (git stash of stream.go), ran `go test -count=1 -v -run TestB5Fj ./internal/orchestrator/`, exit 1: `b5_stream_first_job_home_test.go:39: J-1 home = <nil>, want "01ARZ3NDEKTSV4RRFFQ69G5FAW" (the stream's Machine)` / `--- FAIL: TestB5FjStreamSetsHome`. Restored with `git stash pop`; tree clean before commit |
| 3. `go test -count=1 -run 'TestB5Fj\|TestWl' ./internal/orchestrator/` | HOLDS | exit 0 (TestB5Fj×2, TestWlPostsMissingFirstJob, TestWlLeavesOthersAlone, TestWlOnePostPerTask, TestWlHeldLeaseLeftAlone all PASS) |
| 3. `go test -count=1 -run 'TestPlScFirstJobHome\|TestWl' ./internal/cli/` | HOLDS | exit 0: `ok github.com/gridconsole/grid-plugins/internal/cli 0.876s` |
| 4. `make lint` and `make cross` | HOLDS | `make -C . lint cross` exit 0 (go vet; darwin/arm64 and windows/amd64 vet and test compile) |
| 5. Walk: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS | exit 0 |
| 5. Walk: `go test -count=1 ./internal/job/lint/` | HOLDS | exit 0 |
| Changed package and importers | HOLDS | `go list` importers of internal/orchestrator: internal/cli, internal/mcp, internal/migrate, internal/runner. `go test -count=1 ./internal/orchestrator/ ./internal/cli/ ./internal/mcp/ ./internal/migrate/ ./internal/runner/` exit 0 (runner has no test files) |
| Touches only | HOLDS | `git diff --stat origin/main..HEAD`: internal/orchestrator/stream.go, internal/orchestrator/b5_stream_first_job_home_test.go, this doc |
| New tests fail on base | HOLDS | see the mutation row |
| No existing test edited | HOLDS | wl_first_job_test.go, pl_sc_first_job_home_test.go, wl_first_job_race_test.go untouched |
| No guard, gate, cap or hold on agents | HOLDS | the change only adds a field to the posted record |
| No attribution lines | HOLDS | `git log --format=%B origin/main..HEAD \| grep -ciE "co-authored-by\|claude-session\|generated with"` prints 0 |
| Whole-module test, cross-repo set | not run, not asked | run by the orchestrator at landing |
| RealPlugin tests | not run, not asked | |

## What changed
- internal/orchestrator/stream.go `wlFirstJob`: after `s.FirstJob(kind)` and before marshal and Validate, a body with no `home` gets `s.Machine` when the stream has a machine (and the body is not nil). A body that already holds `home` keeps it.
- New internal/orchestrator/b5_stream_first_job_home_test.go: TestB5FjStreamSetsHome, TestB5FjStreamKeepsHome, helper b5FjPosted, const b5FjHeld; reuses newStreamHome, wlFirstJobFunc, wlFirstJobIDs, wlFirstJobEqual, pgClosedOfType, jobBody, machine.

One mutation: drop the `home` lines in `wlFirstJob`; it gives `--- FAIL: TestB5FjStreamSetsHome` with `J-1 home = <nil>, want "01ARZ3NDEKTSV4RRFFQ69G5FAW"`.

## NOT VERIFIED
Nothing needs a live screen. No reader of `home` exists (out of scope).

DONE

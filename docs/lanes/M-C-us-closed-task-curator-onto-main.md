# M-C-us-closed-task-curator-onto-main: lane C-us-closed-task-curator merged onto plugin main 952f3413

Carries `docs/lanes/C-us-closed-task-curator.md`. That report holds no NOT DONE verdict, row or last line, so there was nothing to rewrite to CLOSED BY. `grep -n 'NOT DONE' docs/lanes/C-us-closed-task-curator.md` prints nothing.

## Conflicts and how each was resolved

Merge of `origin/main` (952f3413) into the lane conflicted in two files. In both, main added `Repo` (the repo folder on the spawn line) and the lane already carried `Tools` (from the stacked C-be-spawn-line-agent-tools) and `ClosedCurator`. Both sides kept; nothing dropped.

- `internal/orchestrator/stream.go`, three hunks: the `Stream` field (`Tools` then `Repo`), the `spawnLine` field (`Tools` then `Repo`), and the spawn-line fill in `task()` (the `s.Tools` block, then the `s.Repo` block). `ClosedCurator` is untouched.
- `internal/cli/worker.go`, one hunk in the `orchestrator.Stream{...}` literal: `Tools:`, then main's `Repo:` with its comment, then `FirstJob`, then `ClosedCurator`.

Merge commit: `a0dfd5b4`. No test edited, none deleted.

## Proof table

| Row | State | Proof |
|---|---|---|
| 1a. `git merge-base --is-ancestor origin/main HEAD` | HOLDS | exit 0. |
| 1b. `git diff origin/main --stat` names only lane files | HOLDS | 9 files: internal/orchestrator/{pg_closed_task.go, stream.go, us_closed_curator_test.go, be_spawn_tools_test.go}, internal/cli/{worker.go, us_close_curator_test.go, be_worker_spawn_tools_test.go}, docs/lanes/{C-us-closed-task-curator.md, C-be-spawn-line-agent-tools.md}. The `be_*` files and `Tools` are from the stacked C-be-spawn-line-agent-tools, merged into the lane before this one (the launcher said so); they are not on main. |
| 2/1. Curator Job kept, worker Job cancelled `task stopped`, no re-plan | HOLDS | `go test -count=1 -race -v -run TestUsClosed ./internal/orchestrator/` exit 0: `--- PASS: TestUsClosedCuratorJobIsKeptAndTheWorkerJobCancelled`. Red on base is in the carried report row 1 (`us_closed_curator_test.go:120: the curator Job J-1 is cancelled, want assigned`); not rerun, the production files are unchanged by this merge. |
| 2/2. Declined task: one curator `job` record, passes 2 and 3 add none | HOLDS | `--- PASS: TestUsClosedDeclinedTaskGetsOneCuratorJob` (same run). Red with the post stubbed: carried report row 2. |
| 2/3. Hook names `brain-proposals/<id>.md`, nil when none waits | HOLDS | `go test -count=1 -v -run TestUsClose ./internal/cli/` exit 0: `--- PASS:` TestUsCloseHookNamesEachWaitingProposal, TestUsCloseHookAnswersNilWhenNothingWaits, TestUsCloseHookCountsOnlyThisTasksHomeProposals, TestUsCloseWorkerNextCuratesADeclinedTask. |
| 2/4. Five TestPgClosedTask* pass, test file unchanged | HOLDS | `go test -count=1 -v -run TestPgClosedTask ./internal/orchestrator/` exit 0: `--- PASS:` CancelsReadyJob, CancelsRunningAndPersonJobs, LaterPassesWriteNothing, ReopenSpawnsNewJobOnly, OtherStatesSpawn. `pg_closed_task_test.go` is not in `git diff origin/main --stat`. Also `--- PASS: TestUsClosedNoCuratorBodyBehavesAsToday`. |
| 2/5. Two streams, one curator Job, `-race` | HOLDS | `--- PASS:` TestUsClosedTwoStreamsPostOneCuratorJob, TestUsClosedOtherJobsStayCancelledBesideTheCuratorJob, TestUsClosedLeaseIsFreeOnceTheCuratorJobCloses (the `-race` run above, exit 0, `ok ... 11.786s`). |
| 2/6. `go test -count=1 ./internal/orchestrator/ ./internal/cli/ ./internal/task/` | HOLDS | exit 0: orchestrator ok 91.773s, cli ok 36.555s, task ok 4.467s. |
| Importers of the changed packages | HOLDS | `go list -f '{{.ImportPath}} {{join .Imports " "}}' ./...` filtered for internal/orchestrator and internal/cli: cmd/grid, internal/cli, internal/mcp, internal/migrate, internal/orchestrator, lib. `go test -count=1 ./internal/mcp/ ./internal/migrate/ ./lib/ ./cmd/grid/` exit 0: mcp ok 0.379s, migrate ok 4.741s, lib ok 7.980s, cmd/grid no test files. |
| 3. Walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0 (`ok 0.029s`). `go test -count=1 ./internal/job/lint/` exit 0 (`ok 0.082s`). The app-only walk tests (webui, plugincli, grid-server, slices) do not exist in this repo; the `Imports` pattern over `./internal/...` printed only `ok ... [no tests to run]`. |
| 4a. `make lint` | HOLDS | exit 0 (`go vet ./...`, gofmt). |
| 4b. `make cross` | HOLDS | exit 0 (darwin and windows vet and test compile, tests ok). |
| Fetch and merge origin/main before the last proof run | HOLDS | `git fetch origin && git merge origin/main` was the first step; all runs above are on `a0dfd5b4`. |
| `*RealPlugin*` / `make realplugin` | not run, the brief names none and they are app-repo tests | |
| No guard, gate, cap or hold on agents | HOLDS | The merge adds none; the carried lane's change only spares a curator Job from the cancel (see its report). |
| Commit messages | HOLDS | One sentence, no attribution line, no personal path, host or email. |

## What changed

- Resolved the two conflicts above, keeping `Tools`, `Repo` and `ClosedCurator` on `Stream`, `spawnLine` and the worker literal.
- Added this report. No production code beyond the conflict resolution; no test touched.

## NOT VERIFIED on a live screen

- The check-before-accept screenshot pair (closed task, curator Job open, other Jobs cancelled) is the UI track's, after landing.
- The curator child (`grid-curator` agent) was not run; the tests stop at the spawn line and the Feed.
- A spawn line for a curator Job carries `repo` from main's `Repo` hook; no test combines a curator Job with `Repo` or `Tools`. Both hooks are independent blocks in `task()` and both stay nil-safe.

DONE

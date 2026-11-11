# C-run-task-state-running: a queued task moves to running when its first Job is assigned

Repo: plugin. Branch lane-C-run-task-state-running. Base (main): 78b08bc9b65a008a993d4653ceeb9abcd2a22edb.

## Proof table

| # | Row | Result | Proof |
|---|-----|--------|-------|
| 1a | Scout: does any path other than the stream's Tick assign a Job? | HOLDS | The only post of a `control {"assign":...}` record is `internal/orchestrator/tick.go:195`, inside `Store.Tick` (tick.go:21). Tick is called from three places: the stream (`internal/orchestrator/stream.go:199`, moves nothing before this lane), `Store.Next` (`internal/orchestrator/store.go:193`), which `grid job next` and `grid job tick` run (`internal/cli/orchestrator.go:275-283`), and `question.Store.Tick` (`internal/question/command.go:236`), a different Tick that only folds questions and assigns nothing. `job post` has no `--to`: the `--to` flag (`internal/cli/orchestrator.go:74`) is the assignee of `job next`/`job tick` only (used at :276 and :279). `job next` and `job tick` are a person's or an agent's commands and move nothing in this lane. |
| 1b | Test in internal/cli that fails on main | HOLDS | `internal/cli/worker_task_state_test.go`: `grid task new` (J-1 posted by `internal/cli/task_new_first_job.go`), then the child `grid worker next` as `TestWorkerNextProcess` (worker_next_test.go:62-72) runs it, until the first spawn line; task.md then has `state: running` and history.md one receipt. On main (78b08bc9, production files of worker.go, stream.go and write.go checked out from origin/main) it fails: `--- FAIL: TestWorkerNextMovesQueuedTaskToRunningOnFirstAssign` / `worker_task_state_test.go:67: task.md after the first spawn: ... state: queued ... want: ... state: running`. On the lane: `go test -count=1 -run TestWorkerNextMovesQueuedTaskToRunningOnFirstAssign ./internal/cli/` exit 0. |
| 2 | The move is one helper in internal/task, under Change's lock, author is the stream's own, note names the Job, cli callback finds the task from its folder as runPersonApproval does | HOLDS | `Store.RunningFromQueued` at `internal/task/write.go:162`: it runs inside `withWrite` (the lock Change uses), reads the state there, and for `queued` calls `Store.change` (write.go:114), the body that was Change's closure moved out unchanged, so task.md rewrite, Fold check and receipt are Change's own for action `status`. Action is set to `status`, State to `running`. Author: `internal/cli/worker.go:170-173` passes `contract.Author{ID: member, Role: role}`, the member and role the stream writes its Feed records with (stream.go:180, worker.go:111-118). Note is `<job> assigned` (`J-1 assigned`). The folder is resolved with `resolveTaskOperand(env, dir)` (worker.go:166), as `runPersonApproval` does (orchestrator.go:424). The test checks the receipt: what `status`, from `queued`, to `running`, note `J-1 assigned`, who `worker`/`cli` (running_from_queued_test.go; worker_task_state_test.go). |
| 3a | Only `queued` moves, checked inside the lock; table test | HOLDS | `TestRunningFromQueuedLeavesEveryOtherState` (internal/task/running_from_queued_test.go) over running, waiting, stuck, reworking, pausing, paused, closed: task.md byte for byte, history.md byte for byte, no journal, `moved` false, no error. `go test -count=1 ./internal/task/` exit 0. |
| 3b | A second spawn of the same task writes no second receipt | HOLDS | `TestRunningFromQueuedMovesQueuedOnce`: the second call returns `moved` false, an empty event, history.md and task.md unchanged. `TestStreamAssignedRunsBeforeTheSpawnLineOncePerSpawn`: the callback is not called again for a spawn already printed. |
| 4 | A failed move does not undo or block the assign; no callback = as today | HOLDS | `Stream.Assigned` at `internal/orchestrator/stream.go:58`, called at :221-225 just before the spawn line is emitted. `TestStreamAssignedErrorKeepsTheSpawnLine` (stream_task_state_test.go): the callback returns an error; the spawn line is printed, `Pass` returns nil, Diag has exactly one line (names the Job and the cause), the assign is still in the fold, and a second pass writes no second Diag line. With no callback, the existing stream tests pass unchanged (`go test -count=1 ./internal/orchestrator/` exit 0; no existing test edited). |
| 5a | `go test -count=1 ./internal/orchestrator/ ./internal/task/ ./internal/cli/` | HOLDS | exit 0: `ok .../internal/orchestrator 38.633s`, `ok .../internal/task 2.074s`, `ok .../internal/cli 40.373s`. |
| 5b | Packages that import the changed ones | HOLDS | `go list` importers of orchestrator, task or cli: cmd/grid, internal/braincmd, internal/cli, internal/guard, internal/hook, internal/job, internal/job/jobcmd, internal/job/jobtest, internal/land, internal/signal. `go test -count=1 ./cmd/grid/ ./internal/braincmd/ ./internal/guard/ ./internal/hook/ ./internal/job/... ./internal/land/ ./internal/signal/` exit 0 (cmd/grid and jobtest have no test files). |
| 5c | `make lint` | HOLDS | `make lint` ran `go vet ./...` and exit 0 with no output. `gofmt -l internal/` printed nothing. |
| 5d | Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go vet ./internal/orchestrator/ ./internal/task/ ./internal/cli/` exit 0; `GOOS=darwin CGO_ENABLED=0 go vet` of the same exit 0. |
| 6 | Out of scope | HOLDS | Not touched: running -> waiting when only person Jobs are open, the end states, the state table (`StateMoves`), `grid task status`. Seen and left: nothing moves a task out of `running` after its Jobs close. |
| R1 | Small commits, one sentence each, no attribution lines | HOLDS | `git log origin/main..HEAD`: four commits, one sentence each, no trailer. |
| R2 | New tests fail on base, pass on the lane | HOLDS | cli: failing line above. task: `internal/task/running_from_queued_test.go:40:25: s.RunningFromQueued undefined (type Store has no field or method RunningFromQueued)`. orchestrator: `internal/orchestrator/stream_task_state_test.go:19:4: s.Assigned undefined (type *Stream has no field or method Assigned)`. All pass on the lane. |
| R3 | No existing test edited or deleted | HOLDS | `git diff origin/main --stat` lists only write.go, stream.go, worker.go and the three new test files. |
| R4 | No guard, gate, cap, hold or blocking hook on agents | HOLDS | The callback runs after Tick returns, its error is a Diag line only, and it adds no wait or limit. internal/orchestrator still does not import internal/task. |
| R5 | UI rows | HOLDS | This lane changes no UI. |

## What changed

- `internal/task/write.go`: Change's closure body is now `Store.change` (same code, same order); new `Store.RunningFromQueued(o WriteOptions) (moved bool, event Event, err error)`.
- `internal/orchestrator/stream.go`: new field `Assigned func(taskDir, job string) error` beside Configure, called once per first-time spawn, just before the spawn line; an error is one Diag line.
- `internal/cli/worker.go`: sets `Assigned`: resolve the task from its folder, call `RunningFromQueued` with the stream's member and role and the note `<job> assigned`.
- New tests: `internal/cli/worker_task_state_test.go`, `internal/orchestrator/stream_task_state_test.go`, `internal/task/running_from_queued_test.go`.

## Open items and notes

- The callback runs for every Job's first-time spawn, not only the first Job of a task; the queued check in the helper makes the later ones a no-op (no write, no receipt). After a worker restart the stream's printed-set is empty, so a Job still assigned is reprinted and the callback runs again; a task that is already past `queued` is left alone. A task that a person set back to `queued` while a Job is still assigned would move to running again on that restart.
- The helper finds the task again by its id (`Show(cwd, id)`, as `runPersonApproval` does). Tested with one project; not tested with two projects that carry the same `T-NN`.
- The callback logs the move failure as `grid worker next: task <id>: job <J> assigned, the Assigned callback failed: <cause>`.

## NOT VERIFIED on a live screen

No UI changed, so nothing needs a live screen. Not run: the real agent run of the 11 Nov case; the CLI test drives the real `grid worker next` code in a child process of the test binary instead.

DONE

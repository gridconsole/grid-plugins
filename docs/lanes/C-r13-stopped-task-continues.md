# C-r13-stopped-task-continues: a task whose Jobs a person stopped can be continued

Repo: plugin. Branch lane-C-r13-stopped-task-continues, from main a3d48f7e. Code commit fd87c6ba.

## Cause (eval round 13, T-02)

- `grid-worker stop` ends each running agent with a worker-exit `cancelled` and code `stop_requested`, then cancels the Job (app internal/kernel/worker/supervisor.go:55 `stopRequestedCode`, :161, `cancelStopped`). The plugin folds that as Status cancelled, ExitCode `"stop_requested"` (internal/orchestrator/tick.go:311 puts a Job back to Ready only for `worker_stopped`).
- The task stays `running`. `NextState` (internal/task/write.go:65) moves a task to queued on `reopen` only from `closed`, so `grid task reopen` refused: `cannot reopen task in current state "running"`.
- `grid task stop` then `grid task reopen` gave queued but posted nothing: `reopenPlanJob` (internal/cli/reopen_job.go) posts only a person approve Job that continues an accepted one, and returns nil when the task has none. The worker ticks a task only while a Job is open (internal/orchestrator/stream.go:258), so nothing started (worker.log 07:59:15 to 08:06, no started event).

## What changed (internal/cli only)

- `internal/cli/reopen_continue.go` (new): `reopenContinueJob` reads task.md state and the Feed. It returns nothing to post unless the task is `running` or `closed`, the Feed holds Job records, no Job is open (not Accepted, Cancelled or Rejected), and a Job was cancelled: for a running task a Job whose ExitCode is `stop_requested`, for a closed task any cancelled Job. It then builds the first-Job record (`firstJobOf(kind)`, an orchestrator Job like J-1) with a new id, the title "Continue the task after it was stopped" and an acceptance text that says the Jobs were stopped and tells the orchestrator to read `grid job tree` and post what the task still needs. `reopenTaskWriter` uses the task epoch in the env as `reopenPlanJob` does; a person at a terminal has none, so it takes the task lease for the one post and gives it back (as `postFirstJob` does), so the worker finds an open Job and a free lease.
- `internal/cli/reopen_job.go`: the id pick and the `WriteUnless` post loop moved into `reopenPostJob`, shared by the approve Job and the new Job. The approve Job posts the same record as before.
- `internal/cli/task_write.go`: with no approve Job to post, `grid task reopen` asks `reopenContinueJob`. For a running task it makes the stop move, then the reopen move (the state table is unchanged and reopens only from closed), then posts the Job after the reopen, holding no task lock. The writer or lease is taken before the move, so a failure leaves the task as it was. Everything else (a running task with an open Job, or with Jobs cancelled for another reason) falls through to `store.Change` and gets the same refusal, text and exit code as before.
- `internal/cli/r13_stopped_task_continues_test.go` (new): the tests below.

## Proof table

| Row | Result |
|---|---|
| 1. reopen of a running task whose Jobs a stop cancelled is accepted and the plan Job is posted so the worker starts it | HOLDS. `go test -count=1 -v ./internal/cli -run TestR13` exit 0: `--- PASS: TestR13ReopenContinuesAStoppedRunningTask`. The test builds T-02's records (J-1 assigned, acked, worker-exit cancelled code stop_requested, `job cancel`, lease given back, task.md `state: running`), reopens as a person with no lease epoch, checks state `queued`, one open Job J-2 with role orchestrator, then takes the task lease (proves the reopen gave it back) and runs `orchestrator.Store.Tick`: `Spawns` holds J-2. |
| 1 on base | Fails on base (new test file, code of a3d48f7e): `--- FAIL: TestR13ReopenContinuesAStoppedRunningTask` / `r13_stopped_task_continues_test.go:140: task reopen: 1 grid: cannot reopen task in current state "running" to ""` |
| 2. reopen after `grid task stop` posts the Job too | HOLDS. `--- PASS: TestR13ReopenAfterTaskStopPostsTheOrchestratorJob` (same run): `task stop` then `task reopen` give queued, J-2 open, Tick spawns it. On base: `--- FAIL: TestR13ReopenAfterTaskStopPostsTheOrchestratorJob` / `r13_stopped_task_continues_test.go:160: want one open orchestrator Job J-2, got []` |
| 3. a task with a Job still running or ready keeps today's refusal | HOLDS. `--- PASS: TestR13ReopenRunningTaskKeepsItsRefusal` with two subtests: `a_Job_is_still_open` (an open person Job J-2) and `cancelled_for_another_reason` (code prepare_failed). Each expects a non-zero exit, stderr `cannot reopen task in current state "running"`, task.md still `running`, no new job record. These pass on base too: they pin the refusal that stays, they are not the new behaviour. |
| 3. go test ./internal/cli/... ./internal/jobgraph/... | HOLDS, exit 0: `ok .../internal/cli 31.831s`, `ok .../internal/jobgraph 0.060s`. No existing test edited or deleted. |
| Packages that import internal/cli (go list over Imports, TestImports, XTestImports) | HOLDS, exit 0: cmd/grid (no test files), contract/formats, internal/account, internal/braincmd, internal/golden, internal/jobgraph, internal/land, internal/mcp, internal/packaging, internal/question, internal/settings, lib: all `ok`. |
| Walk test `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS, exit 0: `ok .../internal/cli 0.036s`. |
| `go test -count=1 ./internal/job/lint/` | HOLDS, exit 0: `ok .../internal/job/lint 0.131s`. |
| `make lint` | HOLDS, exit 0 (runs `go vet ./...`). |
| Windows build `GOOS=windows CGO_ENABLED=0 go build ./...` | HOLDS, exit 0. |
| macOS build `GOOS=darwin CGO_ENABLED=0 go build ./...` | HOLDS, exit 0. |
| `git fetch origin && git merge origin/main` before the last proof run | HOLDS: `Already up to date.` |
| App walk tests (`make webui-test`, TestFvArgvCollects, TestEveryWiredMount, slices Imports) and `make realplugin` guest walk tables | not run, not asked (app repo; this lane touches the plugin only) |
| Whole-module `go test ./...` and the cross-repo set | not run, not asked (the orchestrator runs them at landing) |
| Any part that needs an app change | None found, so no NOT DONE row. The app worker already ticks any task that has an open Job (the plugin's internal/orchestrator/stream.go:258-297) and moves queued to running at assignment (internal/cli/worker.go:197). A live worker starting J-2 is NOT VERIFIED (below). |
| No guard, gate, cap, hold or blocking hook added | HOLDS: the lane only posts a Job where none was posted before; the one refusal it touches is unchanged. |

## NOT VERIFIED on a live screen or a live worker

- A real `grid-worker` picking up J-2 after the reopen and running the orchestrator agent against T-02's Feed. The test proves the plugin's tick assigns and spawns it, and that the task lease is free for the worker.
- The app's task page and history for the extra receipt (below).

## Open items

- A running task stopped after its plan was approved still has an accepted approve Job: `reopenPlanJob` then posts the approve-again person Job (as today for a closed task) and the new orchestrator Job is not posted, so nothing starts until the person answers, and the Jobs already posted stay cancelled. Not the r13 case; it needs a decision on what a re-approval starts.
- A reopen of a running task writes two receipts in task history: `stop` (running to closed) and `reopen` (closed to queued), because `NextState` is left as it is and reopens only from closed. A worker pass that runs between the two sees `closed`, cancels no Job (none is open) and may post a curator Job if brain proposals wait.
- A closed task whose Jobs are all accepted or rejected, with none cancelled, still reopens to queued with nothing posted, as before.

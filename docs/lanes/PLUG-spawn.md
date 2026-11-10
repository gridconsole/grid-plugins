# PLUG-spawn: grid worker next, the stream that spawns Jobs

Base: lane-Pint (plugin main plus S6c1-S6c4, S9fixA2 global flags and `--hold`, S9fixB2, S6tidy, S6c6), `9dc5d049`. Design: SPAWN-decide section 3, with the review's correction: wherever the design said the fence goes up "by one", it is a new, higher fence from the per-task counter (`internal/lease/task.go`, `grant`). The tests assert higher, not +1.

## What changed
- `internal/orchestrator/stream.go` (new): `Stream`. One `Pass` goes over every task folder. A task with no Feed id is ignored. A task whose Feed cannot be read is a `feed_invalid` skip, a graph error `graph_invalid`, a held lease `task_held`, other lease or tick errors `lease_failed` and `tick_failed`. For the others: take, reuse or adopt the task lease, post the handover record when it is missing, run `Tick` with `Assignee` = the holder, and print one JSON object per line: `{"spawn":{task,task_dir,id,holder,fence,gen,attempt,assign,task_epoch}}`, `{"wake":{task,job,event,n}}` or `{"skip":{task,reason}}`. A spawn prints once per (task, Job, Job fence) in one stream, so a restarted stream prints Jobs still in `Assigned` again and never a `Running` one. A skip prints again only when the reason changes, with one readable line on stderr.
- `internal/lease/task.go`: `TaskStore.AcquireTask(w, pid, start, probe)` returns `taken`, `reused` (the tenure already names this holder, pid and pid start: no write, same fence) or `adopted` (the holder is on this host and provably gone: a new, higher fence). Any other holder gives `ErrTaskHeld` without a write; a live holder is not even locked.
- `internal/orchestrator/store.go`, `tick.go`: `Output.Spawns` returns each Job left in `Assigned` with its fence, gen, attempt, assignee and the assign control's id.
- `internal/cli/worker.go`: `grid worker next --json --pid P --pid-start S` (with the global flags `--grid-home --member --role`). `internal/cli/orchestrator.go`: `configureTickStore` (worktrees dir, process probe, incident sink) is now shared by `job next` and the stream, and `job post --type job` uses the task lease and `GRID_TASK_EPOCH` when that is set and neither `--lease` nor `--lease-epoch` was typed.
- `internal/cli/contract_inventory_test.go`: one annotation row for the new route `worker next` (required `--pid`, `--pid-start`; stream). This is the table the parser-inventory test reads; no assertion changed.

## Decisions to read
- **SIGINT does not release the task lease.** The stream exits 130 and leaves the tenure: it names the worker's pid, and the worker's next stream reuses it (same pid) or adopts it (dead pid). Releasing there would also drop the lease under agents that are still running.
- **The lease is released when a task's last Job closes** (review backlog item; the PERSON-write workaround "a task with no open Job is not held" depends on it). A pass that finds all Jobs closed while holding the lease runs one more Tick, so the last wake goes out, and then releases. The next open Job takes the lease again with a higher fence. A task with an open person Job stays held (the PERSON-write open item).
- **Exit codes:** 2 for a bad flag, a missing `--pid`/`--pid-start`/member/home, a pid that is not a live process, or a role that may not hold the task lease; 3 for `--member name@machine` whose machine is not this one (the plain `--member name` form uses this machine); 130 on SIGINT; 1 if stdout cannot be written. It never exits 0.
- No `task-take.lock` is taken by the stream: take and adopt serialize on the lease file lock, and a lost race is a `task_held` skip. The CLI's retry-hint file for a crash between grant and handover is not used; a missing handover is posted on the next pass for a reused tenure (the adoption body `from` is then lost).
- Under `--json` the CLI copies stderr into a buffer that is never cleared; the stream resets it after each write so a days-long stream does not grow it.
- No Job cap, delay or rate hold. The 1 s pause between passes is the one `job next --wait` already has.

## Acceptance rows
| # | Test | Fails on base with |
|---|---|---|
| P1 | `TestWorkerNextStreamsSpawnLinesUntilSIGINT` (child process: still running after the lines, SIGINT gives 130), `TestWorkerNextExitCodes` (2 and 3), `TestStreamRunKeepsLeaseOnCancel` (context cancel) | `worker_next_test.go:190: holder from another machine: exit 2, stderr grid worker: unknown command "next"` |
| P2 | `TestWorkerNextStreamsSpawnLinesUntilSIGINT` (two tasks, `lease status task --json` holder `worker@M`, pid P in both), `TestStreamSpawnsEveryTask` | `worker_next_test.go:116: want one spawn object per line, got "{\"error\":{\"code\":\"invalid_argument\",\"message\":\"grid worker: unknown command \\\"next\\\"\"}}"` |
| P3 | `TestAcquireTaskTakesThenReuses`, `TestAcquireTaskAdoptsDeadHolderWithHigherFence`, `TestAcquireTaskLeavesLiveOrRemoteHolderAlone`, `TestStreamSecondStreamReusesTenure`, `TestStreamAdoptsDeadHolder`, `TestStreamLiveForeignHolderIsSkippedOnce` | `acquire_test.go:45:18: s.AcquireTask undefined (type TaskStore has no field or method AcquireTask)`; `stream_test.go:51:86: undefined: Stream` |
| P4 | `TestStreamRestartReprintsAssignedNotRunning` | `stream_test.go:51:86: undefined: Stream` |
| P5 | `TestStreamBrokenFeedSkipsOnlyThatTask` | `stream_test.go:51:86: undefined: Stream` |
| P6 | `TestJobPostWithTaskEpochFromAJobAgent` (`job tree` shows J-2; after the worker retakes the lease the old epoch exits 3, the new one 0) | `worker_next_test.go:237: job post with the task epoch: 3 stale lease` |
| P7 | `TestStreamFiveReadyJobsFiveSpawnsInOnePass` | `stream_test.go:51:86: undefined: Stream` |
| - | `TestStreamReleasesWhenLastJobCloses` (release row, not in P1-P7) | `stream_test.go:51:86: undefined: Stream` |

Base runs: the lane's source files were put back to `9dc5d049` with the new tests kept (`git checkout 9dc5d049 -- <files>`, `git rm stream.go`), the tests run, then `git reset --hard` back to the lane head. In P3 the adoption tests assert that after one Job grant the adopted fence is above old+1, because the counter has moved.

P6 finding (was NOT VERIFIED in the design): an author with role `cli` (worker on the task lease) can post `--type job` under the task lease. The test passes `--task`; `job post` resolves the task from `--task` or the cwd (orchestrator.go), and whether an agent's cwd or environment is enough was not tested.

## Checks
| Check | Exit |
|---|---|
| `go build ./...` | 0 |
| `GOOS=windows go build ./...` | 0 |
| `GOOS=darwin go build ./...` | 0 |
| `go vet ./internal/cli/ ./internal/orchestrator/ ./internal/lease/` (also with GOOS=windows and GOOS=darwin, tests included) | 0, 0, 0 |
| `gofmt -l internal cmd` | 0, no files |
| `go test ./internal/cli/... ./internal/orchestrator/... ./internal/lease/... ./internal/hook/... ./internal/ports/... ./internal/env/... ./internal/packaging/... ./cmd/grid/...` | 1: only `TestContractCommandsParserInventory` fails (golden below); orchestrator, lease, hook, ports, env, packaging ok |
| `go test ./internal/cli/... -skip '^TestContractCommandsParserInventory$'` | 0 |
| `make lint` | not run (brief) |

Importers (from `go list -f '{{.ImportPath}} {{join .Imports " "}}' ./...`): `lease` is imported by cli, hook, orchestrator, ports; `orchestrator` by cli; `cli` by cmd/grid; ports by env; hook by packaging; so the run above covers all of them.

## Needed from the orchestrator
- Regenerate `contract/commands.json` (the new `worker next` route): `UPDATE_CONTRACT_COMMANDS=1 go test ./internal/cli -run '^TestContractCommandsParserInventory$' -count=1`. Until then `TestContractCommandsParserInventory` fails with `commands.json differs from the current parsers` (line 3386).
- No `api/ops.yaml` change was needed.

## NOT VERIFIED
- The optional spawn keys `repo`, `model`, `effort` and `branch` are not printed: a Job record has no model or effort, and `Worktree` is a map of unknown meaning for `repo`/`branch`. The app treats them as optional (SPAWN-decide section 1). The required keys and `task_epoch` are printed and tested.
- The `task` value is the Feed task id (a ULID, which fits `^[A-Za-z0-9_-]{1,80}$`), not `T-NN`; the app's own regex was not run against it.
- Cost of a pass over a home with hundreds of tasks: every task's Feed is read once a second. Not measured.
- `UsageReady` is not set by the stream, so a Job parked on `usage_limit` is not re-offered by it (as `job next` without `--usage-ready`).
- A person's `job post` or answer while the worker holds the task lease still exits 3 (PERSON-write, outside this lane).
- Nothing was run against the app; the joint test belongs to APP-spawn.
- No UI change.

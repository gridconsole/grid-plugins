# PLUG-spawn2: the worker stream gaps from review

Base: lane-PLUG-spawn, `c6559be1`. Changes are in `a6a43183`; this note is the next commit.

## What changed
- Row 1, `internal/orchestrator/stream.go`: each task's `Store` gets `UsageReady` = "the worker is the stream's holder", the same test `job next --usage-ready` builds for its assignee. A Job parked on `usage_limit` goes back to Ready on the next pass and is assigned again, so a new `spawn` line prints with a higher Job fence. `Configure` runs after and may replace it. No delay, cap or hold was added.
- Row 2, `internal/lease/task.go`: `AcquireTask` returns `worker id "x" must be member@machine` when the worker id has no `@`, before any read or write. The line that panicked was the machine slice (`w.Worker[LastIndexByte(...):]`, about 259 before the change).
- Row 3, `internal/cli/global.go`, `cli.go`, `orchestrator.go`: it did NOT hold. A global `--lease-epoch` reaches `job post` only as the default of the local flag (through `env.getenv`), so `fs.Visit` never saw it as typed and `GRID_TASK_EPOCH` replaced it. `Env.typedGlobal` now records which globals were typed, and `job post --type job` counts a typed global `--lease-epoch` as typed.
- Row 4, `internal/cli/orchestrator.go`: it did NOT hold for an agent in a git worktree. Agent worktrees are under `<home>/projects/<project>/worktrees`, outside the task folder, and `GRID_TASK_EPOCH`, `GRID_LEASE` and `GRID_LEASE_EPOCH` do not name a task, so every `job` verb ended with `no task selected; use --task`. The `job` verbs now take the task from `GRID_FEED_TASK` when `--task` is empty, as `grid feed` already does (`feed.go` 187-189). An explicit `--task` wins.

## New tests, failing on base and passing on the lane
Base run: the new test files were added on `c6559be1` with the source unchanged, then the fixes were made.
| Row | Test | Fails on base with |
|---|---|---|
| 1 | `TestStreamReoffersUsageLimitParkedJob` (`orchestrator/stream_usage_test.go`) | `stream_usage_test.go:41: the Job parked on usage_limit was not offered again: spawn [], Job &{ID:J-1 ... Status:parked ... Reason:usage_limit ...` |
| 1 | `TestStreamUsageReadyIsForTheHolderOnly` | `stream_usage_test.go:54: the stream left UsageReady unset` |
| 2 | `TestAcquireTaskRefusesWorkerWithoutAt` (`lease/acquire_worker_test.go`; free lease and held lease; held lease unchanged) | `acquire_worker_test.go:45: AcquireTask panicked: runtime error: slice bounds out of range [-1:]` (subtest `held_by_another_worker`; the free-lease subtest already returned an error) |
| 3 | `TestJobPostGlobalLeaseEpochBeatsTaskEpoch` (`cli/job_post_agent_test.go`: current global flag beats a stale `GRID_TASK_EPOCH`; stale global flag beats a current one and exits 3; no flag, `GRID_TASK_EPOCH` still decides) | `job_post_agent_test.go:106: the global --lease-epoch 0.3 should be used over GRID_TASK_EPOCH 0.1: exit 3 stale lease` |
| 4 | `TestJobPostResolvesTaskFromAgentWorktree` (cwd `<home>/projects/example/worktrees/J-1`, agent env plus `GRID_FEED_TASK`, no `--task`) | `job_post_agent_test.go:150: job post from .../projects/example/worktrees/J-1 with no --task: exit 1 no task selected; use --task` |
| 4 | `TestJobPostResolvesTaskFromCwdInTaskFolder` (cwd inside the task folder, only the three variables) | passes on base: this part already held |
| 4 | `TestJobPostTaskFlagBeatsFeedTaskEnv` | passes on base: it pins that `--task` wins over the new fallback |

## Checks
| Check | Exit |
|---|---|
| `go build ./...`, `GOOS=windows go build ./...`, `GOOS=darwin go build ./...` | 0, 0, 0 |
| `go vet ./internal/cli/ ./internal/orchestrator/ ./internal/lease/` (also GOOS=windows, GOOS=darwin) | 0, 0, 0 |
| `gofmt -l internal cmd` | 0, no files |
| `go test` of cmd/grid, contract/formats, internal/{cli,golden,hook,job/...,lease,mcp,orchestrator,packaging,ports,question,settings,env} | 1: only `TestContractCommandsParserInventory` fails (`commands.json differs from the current parsers`, line 3386, the `worker next` route from PLUG-spawn); every other package ok |
| `go test ./internal/cli/... -skip '^TestContractCommandsParserInventory$'` | 0 |
| `make lint` | not run (brief) |

Importers (`go list -f '{{.ImportPath}} {{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./...`, filtered for lease, orchestrator, cli): cmd/grid, contract/formats, cli, golden, hook, job/jobcmd, lease, mcp, orchestrator, packaging, ports, question, settings; env is added because it imports ports. The whole-module test is the orchestrator's.

## Needed from the orchestrator
- `contract/commands.json` regeneration for `worker next` (already named in PLUG-spawn). This lane adds no flag or route, so it needs no further row.

## NOT VERIFIED
- Row 4 needs the app to give agents `GRID_FEED_TASK` (the task folder path, since a display id such as `T-07` does not resolve from a cwd under the Grid home). Not run against the app.
- Row 1: the stream confirms capacity for its holder on every pass, so a Job whose agent hits the usage limit again is spawned again about a second after it parks. The brief asked for the same rule as `job next`; no wait was added.
- The `GRID_FEED_TASK` fallback applies to every `job` verb (`next`, `tick`, `tree`, `ack`, `result`, ...), not only `post`.
- No UI change.

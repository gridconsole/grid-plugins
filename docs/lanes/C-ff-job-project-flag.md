# C-ff-job-project-flag proof

Base: 06047047ac70ebf6bba163d448a121c627a363a1. `grid job` did not parse `--project` there (internal/job/jobcmd/command.go parse(): any other dash flag was "unknown flag").

| Row | Result |
|---|---|
| 1 new test fails on base, passes on lane | HOLDS. `go test ./internal/job/jobcmd/ -run TestFfJobProject` with the production change stashed: `project_flag_test.go:52: --project did not resolve the task: job: unknown flag --project`, exit 1. On lane: PASS, exit 0 (3 tests). |
| 2 no `--project` still "needs --project" | HOLDS. TestFfJobProjectFlagMissingStillAsks, exit 0. |
| 3 guest, project it is not in | HOLDS, with a change. The resolver answered "project ... is not registered" with the flag and "task not found" without it. resolveTask (command.go) now maps that one guest answer to ErrTaskNotFound, so the answers are the same; the test checks without, with and an absent project are equal. |
| 4 `go test ./internal/job/jobcmd/ -count=1` | HOLDS, exit 0. |
| Dependents (jobcmd imported by internal/cli, cmd/grid, lib) | `go test ./internal/cli/ ./lib/ ./internal/job/...` exit 0 except one flaky `TestApproveRecordWorkerPassMovesTheReceiptOfATaskOnlyRead` under load; alone it passes, exit 0. cmd/grid has no tests. |
| gofmt, go vet ./internal/job/... ./internal/cli/ | HOLDS, exit 0, no output. |
| Windows and macOS builds | HOLDS. `GOOS=windows CGO_ENABLED=0 go build ./...` and `GOOS=darwin ...`, exit 0. |
| RealPlugin tests | Not needed; none touched. |
| No guards or caps added | HOLDS. |

## Changed
- internal/job/jobcmd/command.go: `--project <name>` parsed (value required), carried on Env, passed as where.Query.Project in resolveTask. Set, it bypasses the Env.ResolveTask hook, which cannot carry a project. Usage text lists it on lint, show, verify. Accepted by every subcommand that uses parse(), including render.
- contract/commands.json: `--project` added to job lint, show, render, verify. This file is outside the named Touches, but TestContractCommandsParserInventory fails without it, and the guest parity test then builds a lint project-task row from it. Hand-edited because UPDATE_CONTRACT_COMMANDS=1 cannot be set under the allow list.
- internal/job/jobcmd/project_flag_test.go: new tests.

## NOT VERIFIED
Live use inside an agent worktree. Not run in the sandbox.

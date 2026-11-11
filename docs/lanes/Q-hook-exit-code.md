# Q-hook-exit-code

Head: see the report. Base: main 330256ce. Commits: 185bdbb5 (failing test), ae5ad3a5 (change).

## Proof table

| Row | Status | Proof |
|---|---|---|
| 1. Test fails on main through the hook entry point | HOLDS | Commit 185bdbb5 holds only the test; `go test -count=1 -run TestWorkerExitKeepsTheRawReason ./internal/hook/` on main 330256ce: `exit_code_test.go:37: activity file has no prompt_input_exit: {"exits":{"exit::J-1::":"done"},...,"exit_reason":"done"}` (FAIL). Same test passes at ae5ad3a5. internal/hook/exit_code_test.go:44 |
| 2. exit_reason done, exit_code prompt_input_exit, `exit_codes[key]` same key as `exits[key]` | HOLDS | TestWorkerExitStoresCodeUnderTheReasonsKey, internal/hook/exit_code_test.go:60; go test exit 0 |
| 3. Table clear, logout, other, empty, crash, usage_limit, SubagentStop | HOLDS | TestWorkerExitReasonAndCodeTable, internal/hook/exit_code_test.go:83 (7 cases). SessionEnd cases also assert exit 0 and no output. The SubagentStop case goes through the `worker-exit` hook entry, see note 1 |
| 4a. Second SessionEnd for the same key changes neither reason nor code | HOLDS | TestWorkerExitRepeatChangesNeitherReasonNorCode, exit_code_test.go:126 (`prompt_input_exit` then `crash`: still done/prompt_input_exit); guard is `a.Seen[key]`, events.go:300 |
| 4b. Job with a result writes nothing | HOLDS | TestWorkerExitWritesNoCodeForAJobWithAResult, exit_code_test.go:149: no activity file at all; guard `jobHasResult`, events.go:289 unchanged |
| 5. Old file loads; new file loads in a reader with the main shape | HOLDS | TestExitCodeFieldsAreCompatibleBothWays, exit_code_test.go:195 (`mainActivity` struct = main's fields) |
| 6. `go test -count=1 ./internal/hook/ ./internal/session/` 0 fail; `go vet` exit 0 | HOLDS | both `ok`; `go vet ./internal/hook/ ./internal/session/` exit 0 (no output) |
| Tests of importers | HOLDS | `go list` importers of hook or session: cli, hook, lease, orchestrator, packaging, task. `go test -count=1 ./internal/cli/ ./internal/lease/ ./internal/orchestrator/ ./internal/packaging/ ./internal/task/`: all ok |
| Lint | HOLDS | `make lint` (go vet ./..., gofmt -l): exit 0, no output |
| Windows and macOS builds | HOLDS | `make cross`: darwin/arm64 and windows/amd64 vet and test compile, no FAIL; `GOOS=windows`/`GOOS=darwin go build` not run separately, cross covers it |
| New tests fail on base | HOLDS | Row 1 shows the failing line on main. The other new tests use `ExitCode`/`ExitCodes`, which do not exist on main, so they do not compile there |
| No existing test edited or deleted | HOLDS | `git diff origin/main..HEAD --stat`: exit_code_test.go (new), events.go, activity.go only |
| No guard, gate, cap, hold or blocking hook | HOLDS | `workerExit` only stores two more strings; it still prints nothing and exits 0 (asserted for SessionEnd and worker-exit cases) |
| Mapped reason and session end state unchanged | HOLDS | `exitReason` and the `store.End` switch untouched; existing exit_test.go passes |
| Public repo hygiene | HOLDS | no personal paths, hosts or emails in the diff |
| Commit rules | HOLDS | one-sentence messages, no attribution lines |

## What changed

- internal/session/activity.go: `Activity.ExitCodes` (`exit_codes`) and `Activity.ExitCode` (`exit_code`), both omitempty.
- internal/hook/events.go: `exitCode(in)` (raw `in.Reason`; `subagent_stop` for SubagentStop; `none` for an empty SessionEnd reason). `workerExit` stores it under the same key as the reason, inside the same `!a.Seen[key]` block.
- internal/hook/exit_code_test.go: new tests for rows 1 to 5.

## Notes

1. On the combined SubagentStop hook entry, `stopCheck` runs before `workerExit` and returns exit 2 for a Job with no result (events.go:126-129, existing behaviour, not touched). So for that Job `workerExit` is not reached from that entry; the test drives SubagentStop through the `worker-exit` entry. This lane does not change it, and it adds no guard. Whether a blocked SubagentStop should still leave an exit record is for the owner to decide.
2. A mixed activity file: `ExitCode` (single field) is the last first-seen code, same as `ExitReason`.

## NOT VERIFIED on a live screen

Nothing in this lane has a UI row. No live Claude Code SessionEnd was run (no claude in the sandbox); the hook was driven with the Input shape the existing tests use.

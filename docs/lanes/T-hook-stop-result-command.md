# T-hook-stop-result-command

Base: main 3ccec5cbc4ad3c35241767688bdc35cd631a9b50.

## Proof table

1. HOLDS. The new tests fail on main (detached worktree of 3ccec5cb, test file copied in): `go test -count=1 -run TestStopReminder ./internal/hook/` exit 1. Failing line: `line lacks "grid job result --task 01M7486V1QQYC1T6CC25XW8ZVW --job J-1 --body": "Post the result of Job J-1 on the Feed, then stop.\n"`. Same on main for the task-folder, blocker and skill-match tests.
2. HOLDS. `TestStopReminderNamesResultCommandWithGridTask` (GRID_TASK) and `...WithTaskFolder` (GRID_TASK unset, taskDir) in internal/hook/stop_command_test.go; code at internal/hook/stop.go:143-147.
3. HOLDS. `TestStopReminderSaysBlockedJobPostsBlocker` checks `blocker`.
4. HOLDS. The new tests check one line ending in a newline, exit 2. `TestStopJobWithoutResultBlocksOnce` and `TestStopJobWithResultStops` pass unchanged.
5. HOLDS. `TestSubagentStopScoutWithJobEnvStopsSilently` passes unchanged.
6. HOLDS. `TestStopReminderCommandMatchesRunJobSkill` reads skills/run-job/SKILL.md, takes the first backticked text starting `grid job result` with `--task`, fills `<task>` and `<job>`, and checks the printed line contains it.
7. HOLDS. `go test -count=1 -race ./internal/hook/` exit 0; `go vet ./internal/hook/` exit 0.

## Rules

- Small commit, one sentence, no attribution lines: HOLDS.
- New tests fail on base: HOLDS (row 1).
- No existing test edited or deleted: HOLDS.
- No guard, gate or cap added: HOLDS. The text of one reminder changed; the exit codes are unchanged.
- Importers of internal/hook: internal/cli, cmd/grid (via cli), internal/packaging, internal/packaging/cmd. `go test -count=1 ./internal/cli/ ./internal/packaging/... ./cmd/grid/` exit 0.
- Lint: `gofmt -l internal/hook` printed nothing; `go vet` exit 0.
- `GOOS=windows go build ./...` exit 0; `GOOS=darwin go build ./...` exit 0.

## What changed

- internal/hook/stop.go: the reminder is still one line. It now reads `Post the result of Job J-1 on the Feed with `grid job result --task <task> --job J-1 --body <json>`, then stop; a Job you cannot do is ended the same way, with the blocker named in the result.` `<task>` is GRID_TASK, else the resolved task folder.
- internal/hook/stop_command_test.go: new tests.

## NOT VERIFIED on a live screen

- That a real Job session shows the line. The orchestrator does the live check.

# S3R: rework of S3, `grid job show` state

## What changed
- `internal/job/jobcmd/command.go`: `show` given a task and Job id (or a lone Job id resolved to a task) now reads the task's Feed with `feed.Store.ReadOnly`, folds it, and takes `Jobs[id].State` and `StateReason`. A Job never posted, or a task with no Feed, falls back to the brief's state (Not started). The text output ends with `state: <State>[ (<reason>)]`; `--json` carries `state` and `state_reason`.
- `showPath` now also returns the task folder. The form `grid job show <file>` has no task, so it still reads the brief only and prints no state line (the existing plain-show test requires the output to equal the brief).
- New test `internal/job/jobcmd/show_state_test.go`: posts a Job, checks Not started, then moves it to running (Started), accepted (Done) and cancelled (Done (cancelled)), in text and `--json`; plus an unposted-Job fallback.

## New test fails on base
```
--- FAIL: TestShowReadsTheStateFromTheFeed/running
    show_state_test.go:73: json state "Not started" ("") want "Started" ("")
--- FAIL: TestShowReadsTheStateFromTheFeed/accepted
    show_state_test.go:73: json state "Not started" ("") want "Done" ("")
--- FAIL: TestShowReadsTheStateFromTheFeed/cancelled
    show_state_test.go:73: json state "Not started" ("") want "Done" ("cancelled")
```

## Checks (exit 0 each)
- `go test ./internal/...`: 0 (all packages ok; includes jobcmd and its importer internal/cli, cmd/grid has no tests)
- `go test ./internal/job/... ./internal/cli/... ./cmd/...`: 0
- importers of jobcmd (go list): internal/cli, cmd/grid (via cli)
- `make generate`: 0, tree clean afterwards
- `make lint` (go vet ./...): 0
- `GOOS=windows go build ./...`: 0
- `GOOS=darwin go build ./...`: 0
- gofmt: clean

## NOT VERIFIED
- Output on a real task with real Feed data by a checker. The test posts through the real Feed store with a task lease, but no one has looked at the CLI on a live task.
- The test moves Started with a `running` control; an assigned Job (assign record) is folded by the same `StateOf` but is not separately tested.
- The review's Notes were left alone, as briefed.

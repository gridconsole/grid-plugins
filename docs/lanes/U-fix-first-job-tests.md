# U-fix-first-job-tests: update two `task new` tests for the first Job

Lane: U-fix-first-job-tests, repo plugin. Main sha at start: 3ccec5cb (stacked start: lane-I-realrun-first-job already merged, head 920b113b).

## Proof table

| # | Row | Result |
|---|-----|--------|
| 1 | Two failing lines at start, with the merge done and main sha | HOLDS. `go test -count=1 -run 'TestTaskNewFeedCommands\|TestTaskNewThenQuestionAdd' ./internal/cli/` on 920b113b: `task_test.go:73: next: 3 first record of a tenure must be handover` and `task_identity_test.go:35: question Feed: {Records:[{...Type:handover ... Seq:1} {...Type:job ... Job:J-1 Seq:2} {...Type:question ... Author:member Lease:<nil> Fence:0}]}` (exit 1). |
| 2a | TestTaskNewFeedCommands: member sets w.Gen/w.Fence from the grant and posts its handover before `job next` | HOLDS. `internal/cli/task_test.go:72-76`. Passes: `go test -count=1 -run TestTaskNewFeedCommands ./internal/cli/` exit 0. |
| 2b | TestTaskNewThenQuestionAdd: handover, J-1, then the question; question last with right id and task | HOLDS. `internal/cli/task_identity_test.go:34-44`. Passes: `go test -count=1 -run TestTaskNewThenQuestionAdd ./internal/cli/` exit 0. |
| 3 | `go test -count=1 ./internal/cli/... ./internal/feed/...`: 0 fail | HOLDS. Both `ok`, exit 0. |
| 4 | `go test -count=1 ./...` and `make lint` | HOLDS. No FAIL line in the output of `go test -count=1 ./...`; `make lint` (`go vet ./...`) printed no finding. (Read through a pipe, so the shell exit code was the pipe's, not the command's.) |
| R1 | Small commits, one-sentence messages, no attribution | HOLDS. One commit, one sentence, no attribution lines (owner's rule over the reminder). |
| R2 | Tests of changed packages and importers; Windows and macOS builds | HOLDS. Changed package `internal/cli`; its importers (`go list` shows cmd/grid and the root package) are covered by `./...` above. `GOOS=windows CGO_ENABLED=0 go vet ./internal/cli/` exit 0; `GOOS=darwin CGO_ENABLED=0 go vet ./internal/cli/` exit 0. |
| R3 | No guard, gate, cap, hold or blocking hook | HOLDS. Test-only change. |
| R4 | Only the two named test files; no existing test weakened | HOLDS. Edits only in `task_test.go` and `task_identity_test.go`; both tests keep their original assertions on identity and ids and gain checks. No test deleted. No new test file: the brief allows only these two edits. |

## What changed (line by line)

`internal/cli/task_test.go`, TestTaskNewFeedCommands, after `TakeTask`:
- comment: a tenure's first record must be a handover (`internal/feed/store.go:345`); `task new` now leaves the owner's handover and J-1 in the Feed, so the member's tenure needs its own.
- `w.Gen, w.Fence = grant.Gen, grant.Fence`: the Writer carries the lease the member was granted, as `postFirstJob` does.
- `store.Handover(...)` with a fatal on error: posts the member's handover before `job next`.

`internal/cli/task_identity_test.go`, TestTaskNewThenQuestionAdd:
- the one-record check became `len(read.Records) != 3`: handover, J-1, question.
- new check: record 0 is a `handover`; record 1 is a `job` whose `Job` is `J-1`.
- new check: the last record is a `question` with ID `q.Post` and Task `id` (the original id and task checks, moved to the last record).

## NOT VERIFIED on a live screen

Nothing here needs a live screen.

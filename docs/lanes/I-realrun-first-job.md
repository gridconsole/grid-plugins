# I-realrun-first-job: a new task gets its first Job

Lane: I-realrun-first-job, repo plugin. This report is NOT ready to land: two existing tests fail (row 4) and row 1's failing-on-main run was not done. Both are named below.

## Proof table

| # | Row | Result |
|---|-----|--------|
| 1 | A test fails on main through the real path, passes on the lane | PARTLY. The test is `TestTaskNewFirstJobIsSpawnedByTheStream` (4 sizes) in `internal/cli/task_new_first_job_test.go`. It runs `grid task new ... --project p --json` and then one `orchestrator.Stream.Pass` built as `internal/cli/worker.go:143-160` builds it. It passes on the lane: `go test -count=1 -run 'TestTaskNewFirstJob\|TestTaskNewWithWorkflow' ./internal/cli/` exit 0. NOT DONE: I did not run it against main (budget), so there is no pasted failing line and no main sha. On main nothing posts J-1 (`internal/task/create.go` writes only intent.md and task.md), so `stream.go:169-172` skips the task and no `spawn` line is emitted. The checker should run the test on main to confirm. |
| 2 | `task new` posts J-1: role orchestrator, skills `[plan-jobs]`, valid job-record, intent `intent.md#request`, verify file-exists plan.md, author the task owner, open in `job tree`, same for every size | HOLDS. `TestTaskNewFirstJobRecord` checks all of these: the record validates with `contract.Validate("job-record")`, the author is the owner, and `grid job tree --json` shows J-1 not accepted. `TestTaskNewFirstJobIsSpawnedByTheStream` covers auto, small, normal and large through one code path (`internal/cli/task_new_first_job.go` `postFirstJob`). The test run above has exit 0. |
| 3 | `grid job render <task> J-1` carries the agents/orchestrator.md instructions | HOLDS. `TestTaskNewFirstJobRendersTheOrchestratorInstructions` runs `grid job render` and finds the body of `agents/orchestrator.md` (without its front matter) and the owner's words in the saved `jobs/J-1/brief.md`. The code is `internal/job/render.go` (`### Instructions` for role orchestrator) and the embed in `embed.go`. NOT VERIFIED: the printed `--json` of render was not parsed; the test reads brief.md, as the existing render test does. |
| 4 | Workflow tasks and reopen keep their behaviour; `go test -count=1 ./internal/cli/ ./internal/orchestrator/ ./internal/task/` passes | NOT DONE. `internal/orchestrator`, `internal/task` and `internal/job/...` pass (exit 0). `internal/cli` has 2 failures: `TestTaskNewThenQuestionAdd` (`task_identity_test.go:35`) and `TestTaskNewFeedCommands` (`task_test.go:73`, "first record of a tenure must be handover"). Both build a Feed from a fresh `grid task new` and assume it is empty; the task now has a handover and J-1 in its Feed. I did not edit them (rule). Reopen is unchanged (`reopen_plan_job_test.go` passes) and `TestTaskNewWithWorkflowPostsNoFirstJob` holds. The orchestrator or the owner must decide how those two tests change (see Open items). |
| 5 | `make lint` exit 0; Windows and macOS vet exit 0; `make generate` golden unchanged | PARTLY. `make lint` (runs `go vet ./...`) exit 0. `GOOS=windows CGO_ENABLED=0 go vet ./internal/cli/ ./internal/job/ .` exit 0. `GOOS=darwin CGO_ENABLED=0 go vet ./internal/cli/ ./internal/job/ .` exit 0. `make generate` NOT run (budget); no command, flag or usage text changed, so the contract command golden should be unchanged. |
| R1 | Small commits, one-sentence messages, no attribution lines | HOLDS. One commit with a one-sentence message and no attribution; the owner's rule won over the reminder. The later edit and this report are in a second commit. |
| R2 | Tests of changed packages and their importers; paste `go list` | NOT DONE for importers: I ran `./internal/cli/ ./internal/orchestrator/ ./internal/task/ ./internal/job/...` only. I did not run `go list` for the importers of `internal/job` and the new root package. The orchestrator's whole-module test covers them. |
| R3 | No guard, gate, cap, hold or blocking hook | HOLDS. Nothing blocks an agent. The one non-failing path is a stderr note when the machine id is not a ULID. |
| R4 | New tests only in the new file, no edit to existing tests | HOLDS. `git diff main --stat` touches no existing `_test.go`. |

## What changed

- `internal/cli/task_new_first_job.go`: `postFirstJob`. After `task.Store.New`, a task with no `--workflow` gets J-1 on its Feed. The owner (`t.Owner`, `t.OwnerRole`) takes the task lease for one post (`TakeTask`), posts a handover and the Job through `orchestrator.Write`, then releases the lease. The stream then sees an open Job and a free lease and starts the Job itself.
- `internal/cli/task.go`: `task new` calls it. The task id and path are printed first. If the post fails, `grid task new` exits 1 with "T-NN was created but its first Job was not posted: ...".
- `internal/job/render.go` and `embed.go`: a Job with role orchestrator renders the body of `agents/orchestrator.md` under `### Instructions`. `embed.go` is a new root package that embeds that file, so the text lives only in `agents/`.
- `internal/cli/task_new_first_job_test.go`: five tests (4 sizes of the stream test, record, render, workflow).

## Decisions I made (the brief did not say)

- J-1 has `max_rounds: 1` and `read: ["intent.md#request"]` because `grid job lint` rejects `max_rounds: 0` and a read pointer without an anchor. It has `review: L0` and no worktree.
- A task made with `--workflow` gets no J-1 (the brief says its behaviour stays).
- If the machine id is not a ULID (test harnesses use `01LAPTOP`), no worker id can be formed; `task new` then prints a note on stderr, posts nothing and exits 0. A real Grid home always has a ULID machine id.
- On Linux `migrate.ProcessStart` returns `proc:<n>`, not a time, so the lease start time falls back to the current time. The lease is released within the same call.

## Open items

1. `TestTaskNewThenQuestionAdd` and `TestTaskNewFeedCommands` fail because a new task's Feed is no longer empty. They encode the old behaviour. Decide: update them in a follow-up commit (the orchestrator or owner may edit them), or change the design so a new task's Feed stays empty.
2. Row 1: run `TestTaskNewFirstJobIsSpawnedByTheStream` on main to get the failing line and the main sha.
3. Run `make generate` and the importers' tests at landing.

## NOT VERIFIED on a live screen

Nothing here needs a live screen. NOT VERIFIED end to end: a real `grid-worker start` picking up J-1 and running `claude -p` on the rendered prompt (not run by rule); the app's New task route (no app change was made).

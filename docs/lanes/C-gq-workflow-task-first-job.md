# C-gq-workflow-task-first-job: proof table

| Row | Status | Proof |
|---|---|---|
| 1. `task new --workflow build` leaves J-1 Posted, role orchestrator, read `intent.md#request` | HOLDS | `TestTaskNewWithWorkflowPostsTheFirstJob` (gq_workflow_first_job_test.go:41); `go test ./internal/cli/ -run 'TaskNew\|FirstJob' -count=1` exit 0. It also compares the body with a task made without `--workflow` (same) and runs a stream pass that spawns J-1. |
| 2. Task without `--workflow` still gets the same J-1 | HOLDS | Existing `TestTaskNewFirstJob*` tests pass unchanged; the new test compares both bodies. |
| 3. J-1 body passes `postFindings(h.env, path, body)` with no finding | HOLDS | `TestTaskNewWithWorkflowFirstJobHasNoFindings` (gq_workflow_first_job_test.go:70), exit 0. |
| 4. A second `task new` in the same project gets its own J-1 | HOLDS | `TestTaskNewTwiceEachTaskGetsItsOwnFirstJob` (gq_workflow_first_job_test.go:84), exit 0. |
| 5. `go test ./internal/cli/ -run 'TaskNew\|FirstJob' -count=1` | HOLDS | `ok github.com/gridconsole/grid-plugins/internal/cli 1.366s`, exit 0. |
| New tests fail on base | HOLDS | With task_new_first_job.go stashed (early return back): `FAIL: TestTaskNewWithWorkflowPostsTheFirstJob ... got 0 job records`; `FAIL: TestTaskNewWithWorkflowFirstJobHasNoFindings ... want one job record, got 0`; `FAIL: TestTaskNewTwiceEachTaskGetsItsOwnFirstJob ... want its own J-1, got 0 job records`. Pass on the lane. |
| Deleted test | HOLDS | `TestTaskNewWithWorkflowPostsNoFirstJob` removed; it pinned the early return this brief drops (a scope line of the first-Job brief, not an incident). Covered by `TestTaskNewWithWorkflowPostsTheFirstJob`. Named in the commit. |
| Tests of changed package and importers | HOLDS | Importers by `go list`: cmd/grid, lib, internal/cli. `go test ./internal/cli/ ./cmd/grid/ ./lib/ -count=1` exit 0 (cli ok 38s, lib ok, cmd/grid no test files). |
| Lint | HOLDS | `make lint` (go vet ./...) exit 0; `gofmt -l internal/cli` clean after `gofmt -w`. |
| Windows and macOS builds | HOLDS | `GOOS=windows go build ./...` exit 0; `GOOS=darwin go build ./...` exit 0. |
| RealPlugin tests | n/a | No row names one. |
| No guard, gate, cap or hold added | HOLDS | The diff removes an early return and adds tests only. |
| Commit rules | HOLDS | One sentence, no attribution lines, no personal paths. |

## What changed
- internal/cli/task_new_first_job.go: `postFirstJob` no longer returns early for `--workflow`; the doc comment says so. The `o` parameter stays so the call in task.go:136 is unchanged.
- internal/cli/task_new_first_job_test.go: removed `TestTaskNewWithWorkflowPostsNoFirstJob` (gofmt also dropped a trailing blank line).
- internal/cli/gq_workflow_first_job_test.go: three new tests.

## NOT VERIFIED on a live screen
Nothing here is UI. Not run: a real `grid` binary against a real workflow, and the app's new-task route end to end; both belong to the orchestrator's landing run.

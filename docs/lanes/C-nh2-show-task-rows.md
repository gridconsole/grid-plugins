# C-nh2-show-task-rows: C-nh's rows proved on its lane head

Base: lane-C-nh-workflow-show-task at 12fc996e. Main is 792b8d6e. No code and no test changed; the one revert below was done in the working tree and undone with `git checkout --` (`git status --short` empty).

## Proof table

| # | Row | Command | Exit | Last lines | |
|---|-----|---------|------|-----------|---|
| 1.1 | C-nh row 1: one task per source | `go test -count=1 -v -run 'TestNhWf' ./internal/cli/` | 0 | `--- PASS: TestNhWfShowTaskSources`, `--- PASS: TestNhWfShowTaskUnknownName`, `--- PASS: TestNhWfShowWithoutTaskIsMain`, `--- PASS: TestNhWfTaskFixtures`, `ok .../internal/cli 0.314s` | HOLDS |
| 1.2 | C-nh row 2: unknown name errors, no `--task` answer equals main | same run (TestNhWfShowTaskUnknownName, TestNhWfShowWithoutTaskIsMain) | 0 | both PASS as above | HOLDS |
| 1.3 | C-nh row 3: contract | `go test -count=1 -v -run 'TestYiWorkflowShowSchema' ./internal/cli/` | 0 | `--- PASS: TestYiWorkflowShowSchema (0.04s)`; the task fixtures and the unknown-source invalid fixture run in TestNhWfTaskFixtures (PASS) | HOLDS |
| 1.4 | C-nh row 4 | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList\|TestContractCommandsInventory\|TestContractCommandsApp\|TestContractCommandsRejectUnknownFields'` | 0 | `ok .../internal/cli 0.420s` | HOLDS |
| 1.4b | | `go test -count=1 ./contract/... ./internal/workflow/` | 0 | `ok .../contract 0.246s`, `ok .../contract/formats 0.114s`, `ok .../internal/workflow 2.468s` | HOLDS |
| 1.5 | C-nh row 5: app callers | `git -C /srv/gridwork/ref/app.git grep -n -E '"workflow", ?"show"' main -- '*.go'` | 0 | Product callers: `cmd/grid-server/write_policy.go:139`, `internal/kernel/worker/judge.go:498`, `internal/slices/taskview/view_steps.go:51`, `internal/slices/workflows/args.go:20`; none passes `--task`. The rest are tests. The brief cited judge.go:459; on this app main it is :498. | HOLDS |
| 1.6a | C-nh row 6: lint | `make lint` | 0 | `go vet ./...` clean | HOLDS |
| 1.6b | C-nh row 6: cross | `make cross` | 0 | `ok .../lib 0.002s` | HOLDS |
| 1.6c | C-nh row 6: no existing `_test.go` touched | `git diff main --stat -- ':(glob)**/*_test.go'` | 0 | `internal/cli/gg_unknown_task_walk_test.go | 1 +` (one added line, nothing removed; the other test file listed is new, `nh_workflow_show_task_test.go`) | HOLDS only if the line is forced; see row 2 |
| 2 | Reverted-line run | removed `"workflow show",` (line 40) from `ggTaskWalked`, then `go test -count=1 ./internal/cli -run 'TestGg'` | 1 | `--- FAIL: TestGgUnknownTaskWalkCoversTheContract` / `gg_unknown_task_walk_test.go:119: contract members not walked or skipped: [workflow show]; walked or skipped but not in the contract: []` | **FORCED** |
| 3a | Walk tests | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` (included in 1.4) | 0 | `ok` | HOLDS |
| 3b | | `go test -count=1 ./internal/job/lint/` | 0 | `ok .../internal/job/lint 0.090s` | HOLDS |
| 3c | | `make lint`, `make cross` | 0, 0 | `lint-exit-0`, `cross-exit-0` | HOLDS |
| 4 | Diff names only the report | `git diff lane-C-nh-workflow-show-task --stat` | 0 | `docs/lanes/C-nh2-show-task-rows.md` only (checked before commit) | HOLDS |

## Is the walk-test line forced?

Yes. The walk covers every contract member that has json output and a `<task>` operand or a `--task` flag (gg_unknown_task_walk_test.go:19). On main `workflow show` has neither; the new `--task` flag in contract/commands.json makes it a member. Without the line, `TestGgUnknownTaskWalkCoversTheContract` fails at gg_unknown_task_walk_test.go:119 with `contract members not walked or skipped: [workflow show]`. The line is the walk table entry the changed contract forces, so it is inside Touches under end review 0415Z M5, and C-nh row 6's "no existing `_test.go`" is satisfied in spirit: nothing was edited to make a test pass, a table entry was added. Reviewer note: C-nh row 6 as written forbids the file; this proof is the exception.

## What changed

Only this report. No code, no test.

## NOT VERIFIED on a live screen

Nothing here is UI. Tests named *RealPlugin* were not run; no row names one.

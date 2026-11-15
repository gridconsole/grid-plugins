# C-pc0652-worktree-task-resolve proof

Base: origin/main before this lane (merged again at the end; head after merge below). On base, `grid job show T-03-... J-4` from `<home>/worktrees/<task-ulid>/J-4-a1` stopped with `task T-03-spawned needs --project: this folder is in no project with a record`.

| Row | Result |
|---|---|
| 1 Worktree folder resolves a T-NN-slug name to its task | HOLDS. `internal/where/worktree_task_test.go` `TestWorktreeFolderResolvesItsTask`: T-03 exists in two projects, cwd `<home>/worktrees/<ulid>/J-4-a1`, `<ulid>`, and `<ulid>-J-4-a1`; the name and a bare Resolve both give the task. |
| 2 Unknown ULID folder keeps the old result | HOLDS. `TestWorktreeUnknownULIDKeepsOldResult`: Personal rule, no task, and a task name still errors. |
| 3 New where test fails on base | HOLDS. With `where.go` stashed: `go test ./internal/where/ -run TestWorktree` exit 1: `worktree_task_test.go:46: .../worktrees/01M72K22CXZ5Q8W8B9PB3KVQ9M/J-4-a1: task name: task T-03-spawned needs --project: this folder is in no project with a record`. Lane: exit 0. |
| 4 `grid task show` accepts `--project` | HOLDS. `internal/cli/worktree_project_flag_test.go` `TestTaskShowAcceptsProject`: T-07 in two projects fails without it, names each project's task with it; another project's task and an unknown project are refused. On base (production stashed, exit 1): `exit 2: flag provided but not defined: -project`. |
| 5 `grid question list` accepts `--project` | HOLDS. `TestQuestionListAcceptsProject`. On base: `question list --project twin: exit 2: flag provided but not defined: -project`. |
| 6 Bare `grid task show` in a Job worktree | HOLDS. `TestWorktreeTaskNameNeedsNoProject` (also `question list --task T-07-twin` from the worktree). On base: `task T-07 has more than one folder in the Grid home`. |
| 7 `go test ./internal/where/... ./internal/cli/...` | HOLDS, exit 0, after the merge of origin/main. Also `./internal/task/... ./internal/question/... ./internal/jobgraph/... ./internal/orchestrator/...` exit 0. |
| Dependents of where, task, question (go list: braincmd, cli, guard, hook, job, jobcmd, jobtest, land, orchestrator, projectmd, projects, settings, signal, updates, workflow) | HOLDS. `go test -count=1 ./...` before the merge: every package ok, exit 0. |
| Walk tests `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS, exit 0 (after merge). |
| `go test -count=1 ./internal/job/lint/` | HOLDS, exit 0 (after merge). |
| `make lint` | HOLDS, exit 0 (go vet ./...). gofmt -l internal contract: no output. |
| Windows and macOS builds | HOLDS. `GOOS=windows CGO_ENABLED=0 go build ./...` and `GOOS=darwin CGO_ENABLED=0 go build ./...`, exit 0. |
| `git fetch origin && git merge origin/main` | HOLDS, merged clean, tests above rerun after it. |
| RealPlugin tests | not run, not asked |
| `make webui-test` and app-repo walk tests | not run, not asked (plugin repo) |
| New tests fail on base, pass on lane | HOLDS, rows 3 to 6. No existing test edited. |
| No guard, gate, cap or hold added | HOLDS. |

## Changed
- internal/where/where.go: `implicit()` calls `worktreeTask` before the `within(gridHome)` rule. A cwd under `<home>/worktrees/<ulid>[/...]` or `<home>/worktrees/<ulid>-J-N-aN` (both shapes appear in the eval notes) maps to the task whose task.md id is that ULID, with the project and task `taskParts` would give (rule `task`). It uses the same visibility checks (guest project, Personal) as the existing ULID lookup, which now shares `taskByULID`. An unknown ULID falls through unchanged.
- internal/where/personal.go: `TasksDirsIn(cwd, project)`, `TasksDirs` narrowed to one project.
- internal/task/task.go: `ShowIn` (Show limited to a project), `ShowUnder` (Show limited to one tasks dir). `Show` is unchanged in behaviour.
- internal/cli/task_operand.go: `resolveTaskOperandIn(env, operand, project)`; `resolveTaskOperand` calls it with "". With an empty operand the final lookup is limited to the resolved task's own tasks dir, so a bare `grid task show` in a worktree is not made ambiguous by another project's same T-NN.
- internal/cli/task.go: `grid task show --project`; usage line.
- internal/question/command.go: `--project` for `grid question list` only (the brief names list), passed as `where.Query.Project`.
- contract/commands.json: `--project` on `task show` and `question list` (TestContractCommandsParserInventory forces it; hand-edited as in C-ff, since UPDATE_CONTRACT_COMMANDS=1 cannot be set under the allow list).
- testdata/golden/task-help.golden: the `grid task show` usage line, forced by the help text change (TestTaskRenameDeclineHelp).

## NOT VERIFIED
- Live use in an agent worktree: not run. `projects/<project>/worktrees/<name>` (the other worktree location in the code, verify_paths.go) is not mapped: its name carries no task ULID.
- The other `grid question` actions (add, ask, withdraw, sync, tick) do not take `--project`; the brief names list only.

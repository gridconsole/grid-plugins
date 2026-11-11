# C-we-task-new-kind-setup-plugin: `grid task new --kind setup`

Base: lane branch cut from main at 2b47f621 (the brief named 00413d82; main has moved).
Commit identity: Grid lane. No attribution lines in the commit messages.

## Base check

- `git grep -n -i '"setup"' HEAD -- internal/workflow` at 2b47f621 prints nothing. HOLDS: no setup workflow exists.
- `git ls-files | grep -i workflow` at 2b47f621 lists lane docs, golden/lib-workflowSteps fixtures, workflow code and tests only; no setup workflow note. HOLDS.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1 `TestWeKindSetupWritesTaskMd` | HOLDS | `go test -count=1 -v -run 'TestWeKind\|TestTaskNew\|TestContractCommandsParserInventory' ./internal/cli/` exit 0 (`ok ... internal/cli 3.650s`), `--- PASS: TestWeKindSetupWritesTaskMd`. The test checks the `kind: setup` line in task.md, `"kind": "setup"` in task show and task list JSON, `contract.Validate("cmd-task-show")` and `("cmd-task-list")`, and for a task with no --kind no `kind:` line and no `kind` key. |
| 2 `TestWeKindSetupFirstJob`, `TestWeKindRefusesOther` | HOLDS | Same run: `--- PASS: TestWeKindSetupFirstJob`, `--- PASS: TestWeKindRefusesOther` (planner and x: exit 2, stderr `invalid kind "<v>": use setup`, no entry in the project's tasks folder). J-1 of the setup task is a valid job-record with skills `plan-jobs,setup`; a task with no kind joins to `plan-jobs`; `firstJobOf("")` marshals byte for byte the same as `firstJob()`, and `firstJobOf("setup")` differs only in `skills`. |
| 3 `TestWeKindSurvivesStatus` | HOLDS | Same run: `--- PASS: TestWeKindSurvivesStatus` (`task status <id> waiting` exit 0, task.md holds `state: waiting` and `kind: setup`). |
| 4 `make generate`, targeted tests | HOLDS | `make generate` exit 0. `git diff contract/commands.json` shows only the 5 added lines of the `--kind` flag row (`takes: value`, `required: false`) under task new, between `--json` and `--project`. The targeted run above passes `TestContractCommandsParserInventory` and every `TestWeKind*`. `go test -count=1 ./internal/task/... ./contract/...` ok (`internal/task 4.642s`, `contract 0.133s`, `contract/formats 0.187s`). |
| 5 `make lint`, `make cross` | HOLDS | `make lint` exit 0 (go vet clean, gofmt -l empty). `make cross` ran vet and the test compile for darwin/arm64 and windows/amd64; the loop exits on the first failure and its last lines were the windows/amd64 test-compile `ok` lines through `lib`, no `make: ***` line. I piped it through `tail`, so the shell exit code was tail's, not make's. |
| Usage line (Touches: internal/cli/task.go:19) | NOT DONE | I added `[--kind setup]` and `TestTaskRenameDeclineHelp` went red: `testdata/golden/task-help.golden` pins the usage text, and that file is outside the packages and files I may edit. I reverted the usage edit, so the test is green again. `grid task new --help` still lists `--kind` through the flag defaults. Follow-up for the orchestrator (one line in two files): add `[--kind setup]` after `[--workflow <name>]` in internal/cli/task.go:19 and in testdata/golden/task-help.golden, then run `go test -count=1 -run TestTaskRenameDeclineHelp ./internal/cli/`. |
| New tests red on base | HOLDS | With internal/cli/task.go checked out from origin/main (the `--kind` flag removed), `go test -count=1 -run TestWeKind ./internal/cli/` gives:<br>`--- FAIL: TestWeKindSetupWritesTaskMd` / `grid task new exit 2: flag provided but not defined: -kind`<br>`--- FAIL: TestWeKindSetupFirstJob` / same line<br>`--- FAIL: TestWeKindRefusesOther/planner` / `stderr lacks "invalid kind \"planner\": use setup": flag provided but not defined: -kind`<br>`--- FAIL: TestWeKindRefusesOther/x` / same<br>`--- FAIL: TestWeKindSurvivesStatus` / `grid task new exit 2: flag provided but not defined: -kind`. The file was restored from HEAD afterward. |
| Packages that import the changed ones | HOLDS | `go list` of importers of internal/task: braincmd, cli, guard, hook, job, job/jobcmd, job/jobtest, land, migrate, orchestrator, signal, updates (cli and lib import cli). `go test -count=1` on those plus `./internal/task/... ./contract/... ./lib/... ./cmd/...`: all ok (cli 32.189s, orchestrator 67.033s, hook 10.019s, lib 7.240s, migrate 5.028s). The first cli run was red on `TestTaskRenameDeclineHelp` (the usage line, see above); after reverting it the cli package is ok. |
| Walk tests (plugin) | HOLDS | `go test -count=1 -v -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList\|TestTaskRenameDeclineHelp' ./internal/cli/`: `--- PASS: TestHsIpWalksEveryCommand`, `--- PASS: TestIlGuestSiteList`, `--- PASS: TestTaskRenameDeclineHelp`. `go test -count=1 ./internal/job/lint/` ok (0.372s). The app walk tests are not this repo's. |
| *RealPlugin* tests | not applicable | No row names one. |
| No guard, gate, cap or hold on agents | HOLDS | The only refusal added is `invalid kind "<v>": use setup` on `grid task new`, a flag-value check on a person's command, not on an agent. |
| Existing tests edited or deleted | HOLDS | None edited, none deleted. `firstJob()` stays with no argument because three existing test files call it (le_retired_fields_test.go, first_job_instructions_test.go, job_post_findings_test.go); the kind goes through the new `firstJobOf(kind)` that `firstJob()` calls. |
| Public repo hygiene | HOLDS | No personal path, host or email in the changed files. |

## What changed

- internal/task/task.go: `Record.Kind` (`yaml:"kind,omitempty" json:"kind,omitempty"`, after Workflow), `NewOptions.Kind`, and `Validate` refuses any kind but `setup` with `invalid kind "<v>": use setup`. The read path in `Fold` still builds `NewOptions` without Kind, so `kind: planner` homes load.
- internal/task/create.go: `New` copies the kind into the Record that `taskFront` dumps.
- internal/cli/task.go: `--kind` flag on `task new`, passed to `NewOptions` (usage line not edited, see above).
- internal/cli/task_new_first_job.go: `firstJobOf(kind)`; `setup` gives skills `plan-jobs`, `setup`; `postFirstJob` passes `o.Kind`.
- contract/commands.json (by `make generate`), contract/schemas/cmd-task-show.json and cmd-task-list.json (optional `kind` string, not in `required`), contract/fixtures/cmd-task-show/valid/kind-setup.json.
- internal/cli/we_task_new_kind_test.go: the four tests.

Tasks written before this lane that hold `kind: planner` now show `kind` in task show and task list JSON; nothing here writes planner.

## NOT VERIFIED on a live screen

- No UI in this lane. The app's read of `kind: setup` (internal/slices/me/setup.go) is the other lane's and was not run here.
- A real worker run of J-1 with the setup skill (needs claude) was not run; the test proves the Job record and the skills list only.

## Verdict

NOT DONE: one Touches item, the usage line at internal/cli/task.go:19, is left because its golden file is outside my files. Rows 1 to 5 hold.

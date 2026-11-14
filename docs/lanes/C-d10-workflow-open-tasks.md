# C-d10-workflow-open-tasks

`grid workflow show --project P --json` lists each open task of P under the template it runs (`templates[].openTasks`).

Base: main 869f32ff (the brief names b5828ba0; the same lines hold).

## Base check

`internal/workflow/parse.go:192`: `t := Template{Name: s.name, Applies: []Rule{}, Approvers: []string{}, Steps: []TemplateStep{}, UsedIn: []string{}, OpenTasks: []string{}}`

`git grep -n OpenTasks HEAD -- '*.go'` at 869f32ff:

```
HEAD:internal/workflow/parse.go:27:	OpenTasks []string       `json:"openTasks"`
HEAD:internal/workflow/parse.go:192:	t := Template{... OpenTasks: []string{}}
HEAD:internal/workflow/parse_test.go:27:	t := Template{... OpenTasks: []string{}, ...
```

No non-test writer other than parse.go:192. HOLDS.

## Proof table

| Row | Result |
|---|---|
| Base check (above) | HOLDS |
| 1 `TestD10OtListsOpenTasksUnderTheirTemplate` | HOLDS. On base (call removed from workflow.go) it fails: `openTasks by template = map[Review:[] Standard:[]], want map[Review:[T-01] Standard:[T-02]]`. On the lane: `--- PASS: TestD10OtListsOpenTasksUnderTheirTemplate (0.14s)`. The answer passes `contract.Validate("cmd-workflow-show", raw)` (d10_open_tasks_test.go, d10OtShow). The closed T-03 is listed nowhere. |
| 2 `TestD10OtUnresolvedTaskIsListedNowhere` | HOLDS. `--- PASS: TestD10OtUnresolvedTaskIsListedNowhere (0.05s)`. A `workflow: Nonesuch` task and an inline-list task are under no template; the run exits 0. On base it fails too (`want map[Review:[] Standard:[T-03]]`, got `Standard:[]`). |
| 3 `TestD10OtNoProjectLeavesEmpty` | HOLDS. `--- PASS: TestD10OtNoProjectLeavesEmpty (0.09s)`. It also passes on base, as it pins what stays unchanged. |
| extra `TestD10OtUnnamedTaskIsListedUnderTheDefault` | HOLDS. `--- PASS` (0.07s). A task with no `workflow:` is listed under the workspace default; with a project copy of the default it is listed under `example/workflows.md`, not the workspace file. Fails on base. |
| 4 `go test -count=1 ./internal/workflow/ ./internal/cli/ ./internal/contract/` | HOLDS, exit 0 (workflow 1.7s, cli 32.4s, contract 0.7s). The same run with `./cmd/grid ./lib ./contract/...` is also green. Importers of internal/cli (`go list`): cmd/grid, lib. The fixture tests (yi_workflow_schema_test.go, nh_workflow_show_task_test.go) pass. The fixture `contract/fixtures/cmd-workflow-show/valid/project-open-tasks.json` has `openTasks: ["T-02"]` and `["T-01"]` and passes cmd-workflow-show (checked in `TestD10OtListsOpenTasksUnderTheirTemplate`). |
| 5 `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS, exit 0. `go test -count=1 ./internal/job/lint/` HOLDS, exit 0. |
| 6 `make lint` | HOLDS, exit 0. `make cross` HOLDS, exit 0 (darwin/arm64 and windows/amd64, vet and test compile). |
| Walk tests (plugin) | The two runs above are the plugin's walk tests. Main did not move (`git fetch origin && git merge origin/main`: Already up to date). |
| App-repo walk tests (`make webui-test`, `make realplugin`, etc.) | not run, the repo is the plugin; they belong to the app repo. |
| No guard, gate, cap, hold | HOLDS. The fill only reads; no limit on tasks. |
| No existing test edited or deleted | HOLDS. `git diff origin/main --stat` shows only new test files and a new fixture. Existing tests with tasks in a project (for example `TestNhWfShowWithoutTaskIsMain`) write task.md without intent.md, so `task.List` returns them as Unreadable and they stay out of the fill; they stay green unchanged. |

## What changed

- `internal/cli/d10_open_tasks.go` (new): `d10OtFill`. It runs only with `--project`. It lists tasks with `task.Store{Resolver: r}.List(env.Cwd, project, "")` (internal/task/task.go:329). It skips `Unreadable` and `State == "closed"`. The state set is `StateMoves`, `internal/task/write.go:28-37`, where `closed` (write.go:36) is the only terminal state. It reads each task.md and calls `workflow.TaskWorkflow(r, t.Project, text)`. A returned template gets the task's display id on the answer template with the same `Name` and `File`. An inline list (nil template) or an error lists the task nowhere and changes no exit code. The order is List's numeric id order. A List error leaves the answer as read.
- `internal/cli/workflow.go`: one call, `d10OtFill(env, *project, &answer)`, after the read succeeded and before the print. It is a no-op without `--project`. It runs for both JSON and text output; the text output does not show openTasks. `--policy` goes through the same call.
- `internal/cli/d10_open_tasks_test.go` (new): the four tests above.
- `contract/fixtures/cmd-workflow-show/valid/project-open-tasks.json` (new): written by `TestD10OtListsOpenTasksUnderTheirTemplate`'s own update path (`UPDATE_D10_OT_FIXTURES=1`, run as `make test UPDATE_D10_OT_FIXTURES=1 "GOFLAGS=-count=1 -run=TestD10OtListsOpenTasksUnderTheirTemplate"`). The yi and nh fixtures are untouched.
- `contract/schemas/cmd-workflow-show.json`: description text on `openTasks` only; no shape change.

Not touched: read.go, parse.go, tasksteps.go, `usedIn`, the `--task` path, contract/commands.json, inprocess.go.

## Notes

- A task that names no workflow is listed only when TaskWorkflow resolves a default. A workspace where any template has an `applies when` rule needs a choice, so an unnamed task there is listed nowhere (TaskWorkflow returns "the rules need a choice"). The app's `openTaskID` lookup then falls through to its own default pick, as before.
- A task folder with no intent.md is Unreadable to `task.List` and is not listed; real tasks written by `grid task new` have both files.

## Timing (open item)

`grid workflow show --project example --json` on a home with 200 open tasks (one workspace default template), in-process, cold temp home, including the contract validation of the answer: 66 ms; the same read without `--project`: 0.6 ms. TaskWorkflow reads the scope once per task, which is cheap at this size. The timing test was a throwaway and is not committed.

## NOT VERIFIED on a live screen

The Workflow tab and the task view's "Running now" list were not looked at; no app rig was run (the brief says no app brief has to land first). `TestD10Ot*` check the served JSON shape only.

# C-zb-routine-workflow-task-plugin: routine workflow runs start a task; `routine add --replace`

Base: c013a05b (plugin main c7c10553 + C-za-routine-schedule-one-plugin merged in, stacked start).
Head: see the bundle; lane commits 04862855 (routine) and 3ffef514 (cli, contract), then this report.

## Base check

| Check | Result |
|---|---|
| `routine add ... --workflow bugfix` on base | HOLDS: exit 2, `flag provided but not defined: -workflow` (first run of `go test -count=1 -run TestZbFlow ./internal/cli/` on base, before any production edit) |
| za's `scheduled` and `next` on base | HOLDS: internal/routine/store.go:29-30 on c013a05b (`Scheduled bool json:"scheduled"`, `Next *time.Time json:"next"`) |
| `--workflow` not on routine add before this lane | HOLDS: the exit 2 above |

## Proof table

| # | Row | Status | Proof |
|---|---|---|---|
| 1 | Workflow run argv, last_task, failure path | HOLDS | `go test -count=1 -v -run 'TestZbFlow\|TestTimerRunRefusesRoutineChangedSinceSchedule' ./internal/routine/` exit 0: `--- PASS: TestZbFlowRunStartsTaskAndRecordsItsID`, `--- PASS: TestZbFlowHomeRoutineStartsPersonalTask`, `--- PASS: TestZbFlowAddRefusesRunAndWorkflowTogether`, `ok`. Args are exactly `task new nightly --workflow bugfix --project alpha --json` (home routine: `--project personal`); `last_task: T-07`, outcome `success`; a non-zero exit and an exit with no id each give `failed`, keep `T-07`, one signal each (zb_workflow_task_test.go:51-81) |
| 2 | Fingerprint unchanged without workflow; replace makes the timer refuse | HOLDS | same run: `--- PASS: TestZbFlowFingerprintChangesOnlyWithWorkflow`, `--- PASS: TestZbFlowReplaceKeepsHistoryAndNeedsScheduleAgain`, `--- PASS: TestTimerRunRefusesRoutineChangedSinceSchedule`, `ok`. The test computes sha256(`grid version` NUL `1h`) itself; after replace `errors.Is(err, ErrScheduleChanged)`, one signal, no run; after `ScheduleOne` the timer runs again |
| 3 | CLI `--workflow`, exclusive, required, `already exists`, `--replace`, no new keys on a run routine | HOLDS | `go test -count=1 -v -run 'TestZbFlow\|TestRoutine\|TestContractCommands' ./internal/cli/` exit 0: `--- PASS: TestZbFlowRoutineAddWorkflowAndReplace`, `--- PASS: TestZbFlowRunRoutineListsNoNewKeys`, `--- PASS: TestRoutineCLI` and the other 20 TestRoutine* tests, `ok` |
| 4 | `make generate`; contract tests; contract diff only routine add | HOLDS | `make generate` exit 0; same run as row 3: `--- PASS: TestContractCommandsParserInventory`, `--- PASS: TestContractCommandsInventory`. `git diff c013a05b -- contract/commands.json`: only `--replace` (takes none) and `--workflow` (takes value) added and `--run` `required: true` to `false`. Against `main` the diff also shows `"[<name>]"` args on routine schedule/unschedule: those come from C-za (stacked, not on main yet), not from this lane |
| 5 | Package tests, lint, cross | HOLDS | `go test -count=1 ./internal/routine/ ./internal/cli/` plus importers, exit 0 (below); `make lint && make cross && git rev-parse --short HEAD` printed `3ffef514` (both exit 0; darwin/arm64 and windows/amd64 vet and test compile) |

## Rules

| Rule | Status | Proof |
|---|---|---|
| New tests fail on base | HOLDS | internal/routine on base: `zb_workflow_task_test.go:53:21: store.AddWith undefined (type Store has no field or method AddWith)`, `undefined: AddOptions`, `got.LastTask undefined`. internal/cli on base: `--- FAIL: TestZbFlowRoutineAddWorkflowAndReplace ... exit 2, want 0 ... flag provided but not defined: -workflow`. TestZbFlowRunRoutineListsNoNewKeys passes on base by design: it is row 3's "no new keys on a run routine" guard |
| No existing test edited to pass | HOLDS | only contract_inventory_test.go:529 annotation (`--run` no longer required), named in Touches |
| No deleted test | HOLDS | `git diff --stat c013a05b` shows no deletions of test files |
| Prefix `zbFlow` / `TestZbFlow` | HOLDS | zbFlowStore, zbFlowExecutor, zbFlowSignals, zbFlowStep; all test funcs TestZbFlow* |
| Only Touches changed | HOLDS | `git diff --stat c013a05b`: contract/commands.json, internal/cli/contract_inventory_test.go, internal/cli/routine.go, internal/cli/zb_routine_workflow_test.go, internal/routine/execute.go, internal/routine/store.go, internal/routine/zb_workflow_task_test.go, plus this report |
| No guard, gate, cap or hold on agents | HOLDS | none added; the run/workflow check is a flag usage check of `routine add` |
| Importers tested | HOLDS | `go list` importers of internal/routine or internal/cli: cmd/grid, contract/formats, internal/account, internal/braincmd, internal/cli, internal/golden, internal/jobgraph, internal/mcp, internal/packaging, internal/question, internal/settings, lib. `go test -count=1` of those plus internal/routine: all `ok` (cmd/grid has no test files) |
| Walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok (both on 3ffef514) |
| RealPlugin tests | n/a | no row names one |
| Small commits, one sentence, no attribution | HOLDS | `git log origin/main..HEAD` lane commits |

## What changed

- internal/routine/store.go:26-30: Entry gains `Workflow` (`workflow,omitempty`) and `LastTask` (`last_task,omitempty`) in json and yaml.
- store.go:85-87: Fingerprint appends NUL `workflow-v1` NUL workflow only when set; every existing entry keeps its fingerprint.
- store.go:163: `validCommand` skips ParseRun when workflow is set; run and workflow together are refused; a workflow is a plain name (as workflow/brain.go:225 names workflow files). Used by Validate (so decode on every read) and by Add.
- store.go:260: `AddWith(name, every, run, AddOptions{Workflow, Replace, Policy})`; `Add` keeps its signature (23 test callers) and calls it. Replace (store.go:290) keeps last_run, outcome, last_task and the body of the old file when it reads; the machine becomes this machine, as add sets it.
- store.go:337: optional `TaskExecutor` (ExecuteTask returns the task id); the `Executor` interface is unchanged. Execute (store.go:388-397) records `last_task` only on success with an id.
- internal/routine/execute.go:139-144: the workflow argv; execute.go:221-231: the id is read after the child's output is drained; no id is `grid task new printed no task id` and outcome failed; execute.go:239: stdout is copied to the writer as well, with a shared lock when one writer takes both streams.
- internal/cli/routine.go:44-45 flags, :78-88 exclusive/required check (exit 2), :153 AddWith, :266-279 text prints `workflow:` instead of `run:` for a workflow routine and `last_task:` when set; usage line names both.
- contract/commands.json regenerated by `make generate`.

## Notes

- `routine add missing-flags` (no `--every`, no `--run`) still exits 1 from the store's interval error, as TestRoutineCLI (routine_test.go:78) expects; the "one of --run or --workflow" exit 2 applies once `--every` is given (routine.go:78-88).
- When `task new` creates the task but fails to post its first Job it exits 1: the routine records `failed` and keeps the old last_task, as row 1 asks, though a task exists. Backlog note.
- TestZbFlowRunRoutineListsNoNewKeys (internal/cli/zb_routine_workflow_test.go) asserts the exact JSON key set of a never-run `--run` routine; a lane stacked on this one (zd, zc, ze) that adds a key to that row must be briefed to update it.
- The app half (write.go sends `--workflow`, edit sends `--replace`, the row reads `workflow` and `last_task`) is WAIT-C-zb-routine-workflow-write.

## NOT VERIFIED

- A real `grid task new` child started by a routine run (the tests stub runCommand, as the brief's rows ask); no live screen is involved in this lane.

DONE

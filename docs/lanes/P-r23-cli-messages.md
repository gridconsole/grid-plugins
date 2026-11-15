# P-r23-cli-messages: task state refusals, list --state and missing plan.md give plain messages

Code commit aef8c240 on lane-P-r23-cli-messages; origin/main (0f360eaa) was merged and is unchanged. Findings: U-chk-plugin-rev-cli.md rows 6, 7, 8.

## Proof table

| Row | Status | Proof |
|---|---|---|
| 1. Row 6 confirmed on main | HOLDS | New test on base code: `grid: cannot close task in current state "closed" to ""`, `grid: cannot stop task in current state "closed" to ""`, `deliver: cannot status task in current state "waiting" to "waiting"` (`r23_cli_messages_test.go:27`, `:56`). |
| 1. Row 7 confirmed on main | HOLDS | `r23_cli_messages_test.go:75: [task list --state bogus]: exit 0 stdout "" stderr "", want exit 2`. |
| 1. Row 8 confirmed on main | HOLDS | `plan: lstat <tmp>/home/.grid/personal/tasks/T-01-fix-the-label/plan.md: no such file or directory` for `plan items`, `plan hash`, `proof add`, `proof index`; `job: lstat ...` for `job verify J-1` (`r23_cli_messages_test.go:119`). |
| 2. Row 6 fixed | HOLDS | `TestR23StateRefusalsArePlain` (internal/cli/r23_cli_messages_test.go). Fails on base (lines above); passes on the head. Now `grid: T-01 is already closed` (stop, close, status to closed), `deliver: T-07 is already waiting` (plain, `--dry-run`, `--json` with code `task_state`), reopen `T-01 is queued; only a closed task can be reopened`, other moves `T-01 is closed and cannot move to running`. Code: `internal/task/write.go` `nextStateOf`, `stateMoveText`; callers `write.go` Change and `deliver.go` nextState. The `ErrStateMove` sentinel and the `task_state` code are unchanged. |
| 2. Row 7 fixed | HOLDS | `TestR23TaskListRefusesUnknownState`: exit 2, stderr `grid task list: "bogus" is not a task state; use one of: closed, paused, pausing, queued, reworking, running, stuck, waiting`; `--json` gets the `invalid_argument` envelope the exit 2 path already prints; a real state with no task still exits 0 with `[]`. Code: `internal/cli/task.go` list case, `task.States()`. |
| 2. Row 8 fixed | HOLDS | `TestR23MissingPlanSaysNoPath`, five subtests (`plan items`, `plan hash`, `proof add`, `proof index`, `job verify`): `plan.md is missing; write the plan first`, no path, no `lstat`. Code: `plan.Store.Read` and `job.LoadPlan` return `plan.NoPlanError()`, which is `plan.ErrNoPlan` and `os.ErrNotExist` at once, so `deliver` and `loadJob` (feed fallback) keep working. |
| 3. go test of changed packages and importers | HOLDS | `go test -count=1` ok: internal/task, plan, job, job/jobcmd, job/lint, proof, orchestrator, lib, braincmd, doctor, env, guard, hook, jobgraph, land, lease, mcp, migrate, packaging, packaging/cmd, ports, signal. `go test -count=1 -timeout 30m ./internal/cli`: `ok ... 111.430s`, exit 0. cmd/grid, approvefix, updates: no test files. |
| 3. make lint | HOLDS | `make lint` (go vet ./...) exit 0. gofmt -l internal prints nothing. |
| 3. Walk tests | HOLDS | `--- PASS: TestHsIpWalksEveryCommand`, `--- PASS: TestIlGuestSiteList`, `ok internal/cli`; `go test ./internal/job/lint/` ok. |
| Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` ok; `GOOS=darwin CGO_ENABLED=0 go build ./...` ok. |
| Merge of origin/main before the last run | HOLDS | `git fetch origin && git merge origin/main`: Already up to date. |
| Whole-module test, cross-repo set | not run, not asked (the orchestrator runs them at landing). |
| RealPlugin tests | not run, not asked. |

## Existing tests changed

The brief changes the refusal words, so these tests, which pinned the old words, changed their expected text and nothing else. Each still checks the same behaviour (exit code, task.md untouched, `task_state` code):

- `ye_task_state_code_test.go` (the four moves table, `TestYeTsNextStateSentinel`), `jw_deliver_after_approve_test.go` (`jwDaRefusal`), `delivery_test.go:283`, `task_write_test.go:85`, `r13_stopped_task_continues_test.go:193`: the expected stderr is the new sentence.
- `internal/task/write_test.go:48` and `deliver_test.go:139`: the refusal names the state instead of `current state "<s>"`.
- `internal/task/flake_close_reopen_test.go:174`: counts a refusal with `errors.Is(err, ErrStateMove)` instead of the words `current state`.

No test was deleted.

## What changed

- `internal/task/write.go`: one plain sentence per refusal, naming the task; `States()`.
- `internal/task/deliver.go`: deliver's refusal names the task.
- `internal/cli/task.go`: `task list --state` checks the word against the state table.
- `internal/plan/store.go`, `internal/job/source.go`: a missing plan.md is one sentence with no path.

## NOT VERIFIED on a live screen

None of the rows has a screen.

# C-ye-task-state-refusal-code: a task stop, close, reopen or status refused for its state prints the task_state envelope with --json

Base: plugin main `103b9157`. Code head before this file: `d5161f02`.

## What changed

- `internal/task/write.go:40-52`: exported `ErrStateMove`; `NextState` (:63) now returns a `stateMoveError` that unwraps to it. A plain `fmt.Errorf("%w ...")` would put the sentinel's words into the refusal, so the text could not stay byte-identical; the small type keeps the text and `errors.Is` finds the sentinel. The text is the old `cannot %s task in current state %q to %q`.
- `internal/cli/error_codes.go:47-49,74-75`: `CodeTaskState = "task_state"` and one `errors.Is(err, task.ErrStateMove)` case in `errorCode`.
- `internal/cli/task_write.go:211-216`: `runTaskWrite` calls `env.recordError(err)` just before its `taskWriteResult` return, only when `errors.Is(err, task.ErrStateMove)`. `taskWriteResult` (now :381) is not changed, so `grid approve`, send-back, rename and decline print what they printed.
- `contract/README.md`: one row for `task_state` in the code table (verbs `task stop`, `close`, `reopen`, `status`, `deliver`; exit 1), and the sentence "The first twelve constants" now says thirteen.
- `internal/cli/ye_task_state_code_test.go` (new): `TestYeTsJSONEnvelope`, `TestYeTsPlainUnchanged`, `TestYeTsDeliverEnvelope`, `TestYeTsOtherPathsUnchanged`, `TestYeTsNextStateSentinel`.

`deliver` needed no edit: `internal/cli/deliver.go:76` already calls `env.recordError(err)` and the error comes from `NextState` (`internal/task/deliver.go:296`), so the new case reaches it. `internal/task/lane.go:96` discards the `NextState` error (it builds its own `ErrEditRejected` text), so a lane move prints what it did.

## Base check (the brief's)

Base `103b9157`. `internal/cli/task_write.go:376-383` (base):

```go
func taskWriteResult(env Env, event task.Event, err error, asJSON bool, path string) int {
	if err != nil {
		fmt.Fprintf(env.Stderr, "grid: %v\n", err)
		if errors.Is(err, task.ErrEditUsage) {
			return 2
		}
		return 1
	}
```

`internal/task/write.go:63` (base): `return "", fmt.Errorf("cannot %s task in current state %q to %q", action, current, requested)`. It never calls `env.recordError`, so `commandFailure.code` is "" and `finish` prints nothing. No envelope with a code is written on base, so the brief's stop condition does not apply. Proof on base (new test, source files as on base): stdout is `""` for all four moves, stderr as on base:

```
ye_task_state_code_test.go:81: stdout "", want "{\"error\":{\"code\":\"task_state\",\"message\":\"grid: cannot stop task in current state \\\"closed\\\" to \\\"\\\"\"}}\n"
ye_task_state_code_test.go:81: stdout "", want "{\"error\":{\"code\":\"task_state\",\"message\":\"grid: cannot close task in current state \\\"closed\\\" to \\\"\\\"\"}}\n"
ye_task_state_code_test.go:81: stdout "", want "{\"error\":{\"code\":\"task_state\",\"message\":\"grid: cannot reopen task in current state \\\"queued\\\" to \\\"\\\"\"}}\n"
ye_task_state_code_test.go:81: stdout "", want "{\"error\":{\"code\":\"task_state\",\"message\":\"grid: cannot status task in current state \\\"closed\\\" to \\\"running\\\"\"}}\n"
ye_task_state_code_test.go:118: stdout "", want "{\"error\":{\"code\":\"task_state\",\"message\":\"deliver: cannot status task in current state \\\"waiting\\\" to \\\"waiting\\\"\"}}\n"
--- FAIL: TestYeTsJSONEnvelope (0.01s)
--- FAIL: TestYeTsDeliverEnvelope (0.02s)
```

On base `TestYeTsPlainUnchanged` and `TestYeTsOtherPathsUnchanged` already PASS (they pin what must not change). `TestYeTsNextStateSentinel` was not in that run: it names `task.ErrStateMove`, which does not exist on base, so it does not compile there; I left it out of the base run and added it with the change.

## Proof table

| Row / rule | Result | Command and evidence |
| --- | --- | --- |
| 1. Red on base, then green: stop of a closed task, close of a closed task, reopen of a queued task, status of a closed task to `running` print `--- PASS:`; with --json exit 1, stdout exactly the `task_state` envelope, stderr equal to base byte for byte | HOLDS | Red: lines above. Green: `go test -count=1 -v -run 'TestYeTs' ./internal/cli/` exit 0: `--- PASS: TestYeTsJSONEnvelope` with sub-tests `stop_of_a_closed_task`, `close_of_a_closed_task`, `reopen_of_a_queued_task`, `status_of_a_closed_task_to_running`. The test compares stdout to a hand-written literal `{"error":{"code":"task_state","message":"<stderr minus final newline>"}}\n` and stderr to a hand-written literal (`grid: ` + write.go:63 text); it also checks task.md is unchanged. Also `--- PASS: TestYeTsDeliverEnvelope` (`deliver:` prefix and text kept, envelope on stdout). |
| 2. The same four runs without --json: stdout empty, stderr and exit 1 unchanged | HOLDS | `--- PASS: TestYeTsPlainUnchanged` (four sub-tests), PASS on base and on the lane. |
| 3. No other path changes: approve with no plan Job, and a decline refusal with --json, print on stdout what base prints | HOLDS | `--- PASS: TestYeTsOtherPathsUnchanged/approve_without_a_plan_Job` (exit 1, empty stdout, `grid: ` line, with and without --json) and `/decline_of_a_closed_task` (exit 3, empty stdout, `is already closed`). Both PASS on base before the change and after. Row text names approve's `taskWriteResult` path at task_write.go:345; that line is the send-back path and needs an open plan Job, so the test runs the plain approve path that ends at `:310` (`taskWriteResult`), which is the same function. |
| 4. `TestHfFsCodeTable\|TestHfFsShapes\|TestHfFsSilent\|TestJw\|TestTaskWrite` PASS | HOLDS | `go test -count=1 -v -run 'TestHfFsCodeTable\|TestHfFsShapes\|TestHfFsSilent\|TestJw\|TestTaskWrite' ./internal/cli/` exit 0: `--- PASS: TestHfFsCodeTable`, `TestHfFsShapes`, `TestHfFsSilent`, `TestJwDaDeliverAfterApprove`, `TestJwDaDryRunAfterApprove`, `TestJwDaRefusesWaitingWithoutCurrentApproval`, `TestJwDaRunningDeliversAsBefore`, the other `TestJw*`, `TestTaskWriteGolden`, `TestTaskWriteCLIValidation`, `TestTaskWriteRecordsRole`. |
| 5a. `go test ./internal/cli/ ./internal/task/` 0 fail | HOLDS | `go test -count=1 ./internal/cli/ ./internal/task/ ...` exit 0: `ok internal/cli 189.150s`, `ok internal/task 18.026s`. |
| 5b. `make lint` exit 0 | HOLDS | `make lint` (`go vet ./...`) exit 0. |
| 5c. `make cross` exit 0 | HOLDS | `make cross` exit 0: `cross: darwin/arm64 vet`, `test compile`, `cross: windows/amd64 vet`, `test compile`, every package `ok`. |
| Importers of the changed packages | HOLDS | `go list -f '{{.ImportPath}} {{join .Imports " "}}' ./...` filtered on `internal/task` and on `internal/cli`. Importers of `internal/task`: braincmd, cli, guard, hook, job, job/jobcmd, job/jobtest, land, signal, task, updates. Importers of `internal/cli`: cmd/grid, lib. `go test -count=1` on all: braincmd, guard, hook, job, job/jobcmd, land, signal, lib ok; jobtest, updates, cmd/grid have no test files. Exit 0. |
| Walk tests (plugin) | HOLDS | `go test -count=1 -v ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0: `--- PASS: TestHsIpWalksEveryCommand`, `--- PASS: TestIlGuestSiteList`. `go test -count=1 ./internal/job/lint/` exit 0: `ok internal/job/lint`. (The app walk tests are not this repo's.) |
| Touches only the brief's files | HOLDS | `git diff --stat origin/main..HEAD`: `internal/task/write.go`, `internal/cli/error_codes.go`, `internal/cli/task_write.go`, `contract/README.md`, `internal/cli/ye_task_state_code_test.go`, and this report. |
| No test edited or deleted; no guard, gate, cap or hold on agents | HOLDS | The diff adds one test file and edits no `_test.go`; no deletion. The change only classifies an existing refusal. |
| Commits: small, one sentence, no attribution lines | HOLDS | `git log origin/main..HEAD --format=%B`. |
| *RealPlugin* tests | n/a | No row names one. |
| Landing: app rig `task_stop_close_reopen_realplugin_test.go:186-187` (409) stays green | NOT VERIFIED | App repo, not in this sandbox. By reading: the app classifies from stderr (`client.go:404-414`) and this lane leaves stderr and the exit byte-identical (`TestYeTsJSONEnvelope`, `TestYeTsPlainUnchanged`). The orchestrator runs the cross-repo set. |

## Not verified on a live screen

Nothing here is a UI row. The app brief WAIT-C-ye-app-task-state-code reads `task_state` and starts after this lane is on plugin main and plugin-dist is rebuilt; I did not run it.

## Open items

None. One note for the reviewer: the brief says NextState "wraps with `%w`"; a literal `%w` of a sentinel with words would change the text, so `stateMoveError` carries the old text and unwraps to `ErrStateMove` (write.go:40-52).

# C-hk-signal-add-unknown-task

Base: 91fadeff87525f5859978740e0b66ce51f375e48 (lane branch start; on it `add` did not check the task: act.go switch `case "turn", "add"` set status done and became with no lookup).

## Proof table

| Row | Result |
|---|---|
| 1. TestHkSigAddUnknown | HOLDS. `go test -count=1 ./internal/signal/ -run HkSig -v` exit 0, `--- PASS: TestHkSigAddUnknown`. Returns `*RefusedError`, Cause `where.ErrTaskNotFound`, message `no task T-99`, note bytes identical, no tasks dir made (internal/signal/hk_signal_add_test.go:22-47). On base (act.go restored from 91fadeff) it fails: `hk_signal_add_test.go:31: want *RefusedError, got <nil> <nil>`; `FAIL`. |
| 2. TestHkSigAddKnown | HOLDS. Same run, `--- PASS: TestHkSigAddKnown` (hk_signal_add_test.go:49-62); T-7 folder made, note becomes done/became T-7/updated. |
| 3. CLI walk | HOLDS. `go test -count=1 ./internal/cli/` exit 0 with the `signal act` row deleted from ggTaskKnownOpen; the walk subtest `TestGgUnknownTaskWalk/signal_act` runs `grid signal act S-01ARZ3NDEKTSV4RRFFQ69G5FAV --act add --task T-99 --json` and asserts exit != 0, code `no_such_task` (gg_unknown_task_walk_test.go:228). Seen before the walk's check was widened: exit 3, stdout `{"error":"refused","message":"no task T-99","code":"no_such_task"}`. The walk asserts the exit is non-zero and the code, not the number 3 itself; exit 3 is from internal/cli/signal.go:239 and was seen in that run. |
| 4. TestActAddLinksTaskWithoutCreatingOne | HOLDS. Makes tasks/T-7 first (act_lane_test.go:250), then asserts the tasks dir holds exactly `T-7` (no new task). Passes in `go test -count=1 ./internal/signal/` exit 0. |
| 5a. `go test -count=1 ./internal/signal/ ./internal/cli/` | HOLDS: `ok internal/signal 0.338s`, `ok internal/cli 58.369s`, exit 0. |
| 5b. `make lint` | HOLDS: runs `go vet ./...`, exit 0. |
| 5c. `git diff main --stat` names only the four Touches files | NOT DONE, see below: 6 files differ (+ this report). |
| Packages that import the changed ones | `go list` gives only `internal/cli` and `internal/signal` as importers/owners; both tested above. |
| Windows build | HOLDS: `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0. |
| macOS build | HOLDS: `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| New tests fail on base | HOLDS for TestHkSigAddUnknown (line above). TestHkSigAddKnown passes on base as asked: it pins behaviour that does not change. |
| No clock/sleep, temp home, no expected-fail test | HOLDS: t.TempDir() via `fixture`; no time or sleep in the new file. |
| No guard on agents | HOLDS: a refusal on a missing task id; nothing is added for agents. |

## What changed
- `internal/signal/act.go`: `add` returns `&RefusedError{Message: "no task <id>", Cause: where.ErrTaskNotFound}` when the project has no task folder named `<id>` or `<id>-…` (new `Store.taskExists`). The check sits before `localdata.Machine`, so a refused add writes nothing, not even the machine file. Without that the walk's second run (plain, after the `--json` run) lost the project record and answered a different error. `turn` and the other acts are unchanged.
- `internal/signal/hk_signal_add_test.go`: the two new tests.
- `internal/signal/act_lane_test.go`: TestActAddLinksTaskWithoutCreatingOne makes T-7 first and now asserts the tasks dir holds only T-7; TestActTransitions also makes T-7 first (line ~216).
- `internal/cli/gg_unknown_task_walk_test.go`: signal act row deleted; comment drops only the signal act words.

## Deviation from the brief: files beyond Touches (row 5c)
Three existing tests added to a task that was never made and failed after the fix (`go test` before the edits):
- `TestActTransitions`: `act_lane_test.go:216: add: no task T-7` (in a Touches file).
- `TestSignalOtherActsPostNoJob/add`: `gq_signal_turn_first_job_test.go:120: exit 3: grid: no task T-12`.
- `TestSignalActMatrix/{new,placed,snoozed}/add`: `signal_act_test.go:192: exit 3, stderr "grid: no task T-12\n"; want 0, ""`.
I only made the T-12 folder first in those two cli files (setup lines, assertions unchanged except the expected task-folder count for `add`, 1 instead of 0, because T-12 now exists). Nothing was weakened and no test was deleted. This is why the diff names `gq_signal_turn_first_job_test.go` and `signal_act_test.go` too. If the owner wants those edits out of this lane, they need their own lane.

Also changed in the walk test: the generic envelope check required `message` to equal the whole stderr line (`grid: no task T-99`). The signal act envelope has always printed the message without the `grid: ` prefix (`signalErrorJSON`, signal.go:245). I let the check accept `"grid: "+message == stderr` (gg_unknown_task_walk_test.go:230-231; the same lines assert exit != 0 earlier in ggTaskAnswer) rather than change the CLI, which is outside Touches. A reviewer may prefer to align the CLI instead.

## NOT VERIFIED on a live screen
No UI. The real CLI path was run only through the in-process harness; `grid signal act --act add` against a real Grid home was not run.

NOT DONE (row 5c only: the diff names six files, not four; every other row holds)

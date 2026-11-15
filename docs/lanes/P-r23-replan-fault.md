# P-r23-replan-fault: a clean plan publication closes the open plan-fault Job; the fault notice names the fault plainly

Repo: plugin. Branch lane-P-r23-replan-fault, from main. Findings: rows 1, 2 and 5 of `U-chk-plugin-rev-orch.md`.

**Budget note.** The brief said about 50 calls. Orientation reads ran long: I was at about 45 tool invocations (19 rounds, many batched) with no edit written when the advisor flagged it. I finished the three rows anyway, in about 85 invocations in total, because the fixes are small and a stop would have handed over nothing. This is over the 85 percent line of the runbook; the orchestrator should weigh it.

## What was checked and how

The review's scratch run needs a `grid` binary and a hand-driven task. I did not repeat it. Each row is reproduced by a `Tick()`-level Go test that builds the row's scenario on the package's own fixture (`setupPlan`, `writePlan`), which drives the same `publishPlan`, `planUnpublishable` and `postPlanFaultJob` code. The tests were written first and run on the untouched code; the failing lines below are from that run.

## Proof table

| # | Row | State | Proof |
| --- | --- | --- | --- |
| 1 | Row 1 of the review: confirm on main | HOLDS (confirmed) | `go test -count=1 ./internal/orchestrator/ -run TestR23PlanFaultNoJobBesideThePlanWriter` on main, exit 1: `a re-plan Job was posted beside the open plan writer J-1: [... title:Re-plan: plan.md cannot be published ... role:orchestrator ...]` (J-1 is an orchestrator Job with `write: [plan.md]`, open; the tick posted J-4 beside it). Code: `internal/orchestrator/replan_plan_fault.go:88-95` on main looked only for an open plan-fault Job. |
| 2 | Row 2 of the review: confirm on main | HOLDS (confirmed) | `-run TestR23PlanFaultJobCancelledWhenPlanPublishes` on main, exit 1: `J-4 after a clean publication: status assigned, close_reason <nil>; want cancelled with "plan.md now publishes"`. Code: `internal/orchestrator/plan.go:57-63` on main never read the open plan-fault Job on a clean publication. |
| 3 | Row 5 of the review: confirm on main | HOLDS (confirmed) | `-run TestR23PlanFaultCauseIsThePlanCheckSentence` on main, exit 1: `notice cause "plan.md Job J-1: $.verify: oneOf matched 0 branches: $.verify: missing required job" does not carry the plan check's sentence "J-1: code Job verify must name a test command"`, and the same text in A1 of the re-plan Job. Same schema text as the review's scratch run. |
| 4 | Row 1 fixed, test fails on main, passes on the head | HOLDS | `TestR23PlanFaultNoJobBesideThePlanWriter`: fails on main (row 1 above); passes on the head: `--- PASS: TestR23PlanFaultNoJobBesideThePlanWriter`, `go test -count=1 ./internal/orchestrator/ -run TestR23 -v` exit 0. The test also closes J-1 with the plan still faulty and shows the next tick posts the re-plan Job, so the task is not stranded. Fix: `jiPlanWriterOpen` and its use in `postPlanFaultJob`, `internal/orchestrator/replan_plan_fault.go`. |
| 5 | Row 2 fixed, test fails on main, passes on the head | HOLDS | `TestR23PlanFaultJobCancelledWhenPlanPublishes`: fails on main (row 2 above); passes on the head, same run, exit 0. `TestR23PlanFaultJobStartedIsLeftAlone` (acked, so running, is not cancelled) passes on the head and on main (it pins the other side of "has not started"; it cannot fail on main, which cancels nothing). Fix: `closePlanFaultJobs` called from `publishPlanWithReader` after `postPlanJobs`, `internal/orchestrator/plan.go`, `internal/orchestrator/replan_plan_fault.go`. |
| 6 | Row 5 fixed, test fails on main, passes on the head | HOLDS | `TestR23PlanFaultCauseIsThePlanCheckSentence`: fails on main (row 3 above); passes on the head, same run, exit 0. The test takes the sentence from `jobgraph.CheckPlan(text)`, the function `grid plan check` runs, so it is the plan check's own text. Fix: `jobgraph.CheckPlanJob(j)` moved above `j.Record()` and `contract.Validate` in `planJobInputs`, `internal/orchestrator/plan.go`. |
| 7 | `go test` of the changed package | HOLDS | `go test -count=1 ./internal/orchestrator/` exit 0: `ok .../internal/orchestrator 437.986s`. |
| 8 | `go test` of every package that imports it (`go list`: `internal/cli`, `internal/mcp`, `internal/migrate`; `cmd/grid` and `lib` through cli) | HOLDS | `go test -count=1 ./internal/mcp/ ./internal/migrate/ ./internal/jobgraph/` exit 0 (`ok mcp 0.691s`, `ok migrate 14.124s`, `ok jobgraph 1.034s`). `go test -count=1 ./internal/cli/ ./cmd/grid/ ./lib/` exit 0 (`ok internal/cli 116.707s`, `cmd/grid [no test files]`, `ok lib 29.257s`). |
| 9 | `make lint` | HOLDS | `make lint` exit 0 (`go vet ./...`, no findings). `gofmt -l internal/orchestrator` prints nothing. |
| 10 | Windows build | HOLDS | `GOOS=windows go build ./...` exit 0. |
| 11 | macOS build | HOLDS | `GOOS=darwin go build ./...` exit 0. |
| 12 | Walk tests `TestHsIpWalksEveryCommand\|TestIlGuestSiteList` | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList' -v` exit 0: `walked 39 commands, 39 table entries`, `--- PASS: TestHsIpWalksEveryCommand`, `--- PASS: TestIlGuestSiteList`, `ok .../internal/cli 0.213s`. |
| 13 | `go test -count=1 ./internal/job/lint/` | HOLDS | exit 0: `ok .../internal/job/lint 0.317s`. |
| 14 | `git fetch origin && git merge origin/main` before the last proof run | HOLDS | exit 0: `Already up to date.` |
| 15 | App walk tests (`make webui-test`, `TestFvArgvCollects`, `TestEveryWiredMount`, slices Imports), `make realplugin` guest walk tables | not run, not asked | These are app-repo checks; this is the plugin repo. |
| 16 | Whole-module test, cross-repo set | not run, not asked | The orchestrator runs them at landing. |
| 17 | No guard, gate, cap, hold or blocking hook added | HOLDS | See "Guard rule" below. |
| 18 | No existing test edited or deleted | HOLDS | `git diff origin/main --stat` shows one new test file and two changed source files. |

## What changed

- `internal/orchestrator/plan.go`: `planJobInputs` runs `jobgraph.CheckPlanJob` (the plan check's checker) before the schema check, so the notice and the re-plan Job carry its sentence (row 5). `publishPlanWithReader` calls `closePlanFaultJobs` when the plan publishes (row 2).
- `internal/orchestrator/replan_plan_fault.go`: `jiPlanFaultOpenIDs` (the Jobs `jiPlanFaultOpen` counted, now by id); `closePlanFaultJobs` cancels each one that is posted, ready or assigned with the reason `plan.md now publishes`; `jiPlanWriterOpen` and its use in `postPlanFaultJob` (row 1).
- `internal/orchestrator/r23_plan_fault_close_test.go`: the four tests named above.

Choices:

- "Has not started" (row 2) means not acknowledged: posted, ready or assigned. The fold sets `running` on the ack (`internal/contract/fold.go:224`). `contract.StateOf` reads Assigned as "Started", but the review's J-3 was `assigned` with the agent not yet running, and a tick that has just posted the Job also assigns it, so a cut that spared Assigned would never fire. A Job at running or later is left to finish.
- The cancel is a plain control record (`s.control`). `taCancelEndGrants` ends its grant later in the same tick, so a spawned but unacknowledged agent sees its lease end. `gkReplan` does not re-plan a cancel made by control alone (`replan.go:24-25`), so no second re-plan Job follows.
- A plan-fault Job a person posted (no task lease) is still not counted as open, as before; the plan-writer check does not look at it because its `write` is empty.
- Row 1 matches an orchestrator Job whose `write` holds exactly `plan.md`, the write list of the first Job of a task (`internal/cli/task_new_first_job.go:45`).

## Guard rule

Rows 1 and 2 make the tick post one Job fewer and close its own re-plan Job. Nothing is refused, held, counted or capped for an agent: the writer agent still runs, and still runs `grid plan check` itself. This is the same shape as `jiPlanFaultOpen` (a dedup of the tick's own Job). If the owner reads row 1 as a hold on the tick, it is one function (`jiPlanWriterOpen`) and one call to remove.

## Not in this lane

Rows 3 and 4 of the review (doc-only `docs/delivery.md:214-217`, and the orphan line for a Job stuck at `submitted`) are not in this brief.

## NOT VERIFIED on a live screen

Nothing here has a screen. Not run: the review's hand-driven scratch task with a real `grid` binary and two agent processes. The proof is the tick-level tests above.

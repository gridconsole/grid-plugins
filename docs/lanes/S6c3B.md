# S6c3B: finish S6c3 (drop preview and ship raising; a reopen posts a new plan Job)

Base: the unfinished S6c3 work (15384de4, on S6c2 716aece2) on lane-S6c3B. The production change of S6c3 is in the previous builder's report (`S6c3-report.md`). This lane makes its tests compile and pass, fixes one bug in the reopen post and adds the row 3 tests.

## What changed

- `internal/cli/reopen_job.go`: the plan Job that a reopen posts copied the old Job's `worktree` (`Worktree` map, which encodes as an object) and failed the `job-record` schema ("$.worktree: expected array"), so every `grid task reopen` on a task with an answered plan Job exited 1. The new Job now has no worktree (a person answers it).
- `internal/task/gates.go`: the Q10 fallback (a task with no plan Job) keeps two checks the S6c3 draft had dropped: a receipt in `approval.json` that names a gate is not a plan approval (`ApprovedDigest`, `receiptForGate`; kept `TestApprovedRejectsAnotherGate` green unedited), and `ApprovalForGate(..., "ship"|"preview")` on a task with no plan Job is refused because no ship or preview receipt is written or read any more (kept `TestTaskWithoutPlanJobFallsBackToApprovalJSON` green unedited). The Job path still ignores the gate name. `Approve`'s comment is rewritten.
- Row 3 tests, `internal/cli/reopen_plan_job_test.go` (harness `approveJobHarness`): approve J-2, `task close`, `task reopen`; assert `approval.json` is the reopen event, J-3 is open, J-2 stays accepted in the Feed, `Approved`, `ApprovedDigest` and `ApprovalFromJob` are false; then the task waits and `grid approve` (one test) or `grid job result J-3 --plan-hash` (the other) answers J-3 and both read true with `Job == "J-3"`.

## Tests rewritten or deleted (behaviour removed by the owner's direction: the S6c map step S6c3, briefs/S6c-map-result.md section 4 and the decisions Q1, Q2, Q8, Q10)

- Deleted `guard/TestDeliverCannotReleaseAHeldPlan`: Deliver no longer asks the plan hold; the guard's own hold stays tested by `guard/TestNamedWorkflowHoldGuardAndDeliver` and `TestNamedWorkflowPlanStage`, and `task/deliver_test` covers deliver without a gate.
- Deleted `cli/TestEventTextCannotReleaseAHeldPlanThroughTheCommand` and `cli/TestDatedAcceptanceTypedIntoACommandDoesNotRelease` (files `hold_anchor_test.go`, `hold_note_test.go`): they asserted that `grid deliver` refuses a plan nobody approved; deliver no longer holds. S6c5 deletes the guard hold and its tests.
- Deleted `task/TestApproveAtShipGateRecordsShipApproval`; replaced by `task/TestReopenOverwritesPlanReceipt` (reopen overwrites the plan receipt) and covered for the old `gate:` line by `task/TestOldGateLineIsIgnored`.
- Deleted `land/TestAgentCannotForgeShipApproval` (file `approval_fence_test.go`) and the ship rows of `land/TestWorkflowApproval` and `TestStricterApprovalPolicyWins`: no ship receipt exists. Forged plan receipts stay covered by `land/TestForgedApprovalRequiresAuthoritativeReceipt` and `guard/host_approval_test`.
- Rewritten: `cli/TestApproveAfterDeliveryRecordsShipApproval` is now `TestApproveAfterDeliveryRecordsPlanApproval` (no `gate`, no `approval-ship.json`); `TestApproveDigestReceipt` has the plan row only; the `deliver` dry run lists 3 actions in `task/TestDeliverDryRunWritesNothing` and 4 in `cli/TestDeliverSuccessAndDryRun` (one fewer: no "Raise ship gate"), and `gate` is nil in the delivered `task.md`.
- Removed the `preview` rows of `cli/contract_inventory_test.go` and `guest_parity_test.go`.
- Guard tests edited for compile only: `plan_hold_named_test.go` (unused `errors`), `plan_stage_steps_test.go` (`TestNamedWorkflowDeliverRoles` now expects Deliver to succeed at the plan step; the guard still denies).

## Checks

| check | exit |
|---|---|
| `go build ./...` | 0 |
| `GOOS=windows CGO_ENABLED=0 go build ./...` | 0 |
| `GOOS=darwin CGO_ENABLED=0 go build ./...` | 0 |
| `go vet ./internal/... ./cmd/...` | 0 |
| `gofmt -l internal cmd` | no output |
| `go test ./internal/... ./cmd/...` | 1: only the three `internal/cli` tests below |
| `go test ./internal/task ./internal/guard ./internal/land` | 0 |

Packages that import `internal/task`, `internal/guard` or `internal/cli` (`go list`): braincmd, cli, guard, hook, job, job/jobcmd, job/jobtest, land, orchestrator, signal, task, cmd/grid. All pass in the run above except `internal/cli`'s three tests.

New tests fail on base (S6c2 716aece2, scratch worktree, the test file copied in):

```
=== RUN   TestReopenMakesApprovedFalseUntilGridApproveAnswersTheNewPlanJob
    reopen_plan_job_test.go:69: the reopen did not post an open plan Job J-3: <nil>
--- FAIL: TestReopenMakesApprovedFalseUntilGridApproveAnswersTheNewPlanJob
=== RUN   TestReopenMakesApprovedFalseUntilJobResultAnswersTheNewPlanJob
    reopen_plan_job_test.go:84: the reopen did not post an open plan Job J-3: <nil>
--- FAIL: TestReopenMakesApprovedFalseUntilJobResultAnswersTheNewPlanJob
```

On this head both pass. The base run stops at the J-3 check, so it does not itself show `Approved` true after the reopen on base; the S6c3 brief states that.

## Left for the orchestrator (failing until regenerated)

- `contract/commands.json`: drop the `preview` verb; new `deliver` summary "write the outcome and wait for the person"; `approve` description (S6c4 owns `--gate`). Run `UPDATE_CONTRACT_COMMANDS=1 go test ./internal/cli -run '^TestContractCommandsParserInventory$' -count=1`.
- Failing now because of it: `TestContractCommandsInventory` ("inventory has 45 commands, table has 44"), `TestContractCommandsParserInventory` (deliver summary), `TestGuestProjectParity` ("missing guest parity command: preview", read from commands.json).
- `contract/schemas/cmd-task-show.json` still lists an optional `gate` property (no code writes it); S6c4 owns `cmd-approve.json`.

## NOT VERIFIED

- Lint was not run (the orchestrator runs it). The Windows and macOS builds were built, not run.
- `land-approval: ship` on a task with no plan Job is now refused for ever (no ship receipt); on a task with a plan Job the gate name is ignored, as in S6c2. Q7 removes the field.
- After a reopen the task state is `queued`; `grid approve` needs `waiting`, so a person answers the new plan Job once the task waits again (the tests set `waiting` with `grid task status`) or with `grid job result J-N --plan-hash`.
- The ULID ordering rule (`r.ID <= reopenAt`) is exercised only through the two tests above, not with records posted in the same millisecond.
- `docs/delivery.md` ship and preview sections are untouched (S6c7). `Event.Gate`, `WriteOptions.Gate` and `--gate` stay for S6c4.

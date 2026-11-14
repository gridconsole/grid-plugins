# C-r5-ship-person-step: stop report (built, six existing tests conflict)

Base: main 921ca9fb. Repo: plugin. Verdict: **NOT DONE**. The change is built and its new tests pass. Six existing tests in internal/cli now fail. Five of them pin the old rule that a delivery posts only the curator Job, or pin the exact Feed or stderr after a delivery. Changing them means editing existing tests, which the lane rules forbid without an owner's direction. The budget (about 60 calls) is spent.

## Proof table

| Row | Status | Evidence |
| --- | --- | --- |
| 1. deliver posts one person Job at waiting/ship with an Outcome, using the existing contract | HOLDS (unit) | internal/cli/deliver_person_job.go `postShipPersonJob`, called from internal/cli/deliver.go after the curator block, skipped on a dry run. The body is `migrate.PlanJobBody`: kind approve, role Person, `intent.owner` set to the task's owner. `continues` is nil, so it does not reopen an earlier approval (gates.go planJobResult). It is written under the task lease, as the curator Job is (`taskJobWriter`, now shared by both). `TestDeliverPostsShipPersonJob` PASS. |
| 1a. `for` empty means the owner | HOLDS, mapped | No `for` field exists in any schema or Go source (`grep -rn '"for"'` over contract/ and internal/ finds only a `task --for` flag). Adding one would change the Feed format, so I did not. The person is `intent.owner` (`job.Job.PersonName`). A task.md with no `owner` gives `"owner"`, through PlanJobBody's default. Subtest `no_owner_means_the_owner` PASS. |
| 1b. One Job per delivery, no second open one on redelivery | HOLDS (unit) | The deliver skips when any approve person Job is open (`openApprovePersonJob`), and it checks again inside `WriteUnless`'s predicate. `TestRedeliverPostsNoSecondShipPersonJob` PASS: deliver, `task status running`, deliver again, still one approve Job. |
| 2. The built-in flow shows the person step after Check | HOLDS | `nhWfBuiltIn` adds `{Name: Ship, Kind: approve, Approvers: []}`. The schema has no `person` kind, and an empty approvers list means the task's person. `TestBuiltInFlowShowsShipPersonStep` PASS. **Existing test edited:** the `built-in` row of `TestNhWfShowTaskSources` (nh_workflow_show_task_test.go:168-172) and contract/fixtures/cmd-workflow-show/valid/task-built-in.json now expect three steps. That behaviour change is brief item 2 (owner ruling, end review 2212Z M1). I edited the fixture by hand because `UPDATE_NH_WF_FIXTURES=1` is outside the allow list. |
| 3a. `grid approve` answers that Job and posts no second plan-revision Job | HOLDS (unit) | `TestApproveAnswersShipPersonJob` PASS on all 4 subtests: leased and unleased (GRID_MEMBER only, the round-5 shape), each with and without a plan Job accepted before delivery. Each asserts the approve Job count is unchanged. Leased: the ship Job is accepted and `ApprovalFromJob` names it. Unleased: the ship Job is answered, and it counts after the holder's pass, as on base. |
| 3b. Pinned tests stay unchanged and pass | HOLDS | `TestApproveAfterDeliveryRecordsPlanApproval` and `TestApproveGateFlagIsUnknown` PASS, not edited. |
| New tests fail on base | HOLDS | Base run, before the production change (exit 1): `want one open approve person Job, got 0 of 0 approve Jobs` for TestDeliverPostsShipPersonJob (both subtests), TestRedeliverPostsNoSecondShipPersonJob, and TestApproveAnswersShipPersonJob/leased and /person_without_lease. The two plan-accepted subtests fail with `got 0 of 1 approve Jobs`. TestBuiltInFlowShowsShipPersonStep was added after the production change; on base it would fail at `len(steps) != 3`, but I did not run it on base. |
| `go test ./internal/cli` | **NOT DONE: 6 FAIL** | See the next section. |
| Importers of internal/cli (`go list`) | not run, budget | |
| `make lint` | not run, budget | |
| Windows and macOS builds | not run, budget | |
| `git fetch origin && git merge origin/main` and walk tests | not run, budget | |
| No guard, gate, cap, hold or blocking hook added | HOLDS | The Job is a request. An open ship Job does not unapprove the plan: planJobResult takes the newest *accepted* approve result, so land reads as before. |

## The six failing existing tests (`go test -count=1 ./internal/cli`, exit 1)

- `TestErDeliverFromAJobWithNoProposalPostsNothing` (er_deliver_from_job_test.go:155): `the Feed holds 2 Jobs, was 1`. It pins "deliver with no proposal posts nothing", the opposite of brief item 1.
- `TestJwKhNothingPlantedTakesTheFirstID` (jw_keeper_held_recheck_test.go:237) and `TestJwKhHeldIDPlantedBeforeTheLockIsSkipped` (:192, :213): they pin the exact list of job records after deliver. The new `J-n Person` record is extra.
- `TestDeliverKeeperJobPostsOnceWhileOpen` (deliver_keeper_job_test.go:143): the curator id it expects is shifted, because the person Job takes the next number.
- `TestDeliverStillDeliversWhenTheCuratorJobCannotBePosted` (deliver_keeper_job_test.go:171): it pins stderr to the curator line alone. The person-Job line about the same lease error is added.
- `TestRemainingTaskOperands` (remaining_task_operand_test.go:124): it still fails after `person_job` became `omitempty`. I did not read why; the likely cause is the new stderr line on an unleased deliver.

To finish, the orchestrator decides one of these:

- (a) The owner's ruling covers updating these six tests. Each then expects the person Job.
- (b) The person Job is posted only when the delivering writer holds a lease, and is silent when it cannot be posted. That would cover the stderr and JSON tests, but not the three tests that pin the Feed.

## What changed

- internal/cli/deliver_person_job.go (new): `openApprovePersonJob`, `taskJobWriter`, `postShipPersonJob`.
- internal/cli/deliver.go: posts the person Job after the curator Job, adds an Actions line, and gives `--json` a `person_job` field. A post that fails prints a stderr line and never fails the delivery.
- internal/cli/deliver_keeper_job.go: the curator post uses `taskJobWriter` (same epoch rule), and `deliverOutput.PersonJob` is added.
- internal/cli/nh_workflow_show_task.go: the built-in flow has the Ship approve step.
- internal/cli/deliver_person_job_test.go (new), the built-in row of nh_workflow_show_task_test.go, and contract/fixtures/cmd-workflow-show/valid/task-built-in.json.

## Open items

- The six tests above (the decision is needed first).
- LA1 follow-up: running `grid land` after the answer is not in this lane.
- A deliver with no task lease epoch (a person's terminal) posts no person Job and prints why, as the curator Job does. NOT VERIFIED: whether round 5's deliver held an epoch.
- Side effect: once a task has an approve person Job, `HasPlanJob` is true. A task accepted only by a legacy `plan accepted` history line then reads `PlanHeldWith` from `Approved` until the person answers. That reader only sets the planner's reach (hold.go:133-150). I did not test it.
- The app's Judge built-in flow (Run, Check) does not have the Ship step yet. The app UI is out of scope.
- The dry run does not list the person Job.
- docs/delivery.md:149-157 is still stale (gate ship, approval-ship.json). Not touched.

## NOT VERIFIED

- No live screen; the lane has no UI. Whether the app's Needs you lists the new Job is read from the contract only.

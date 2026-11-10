# S6c1: the person Job result carries the plan digest (additive)

Base: lane-S6b8, b8378c46. Branch lane-S6c1 (head 26b899da), reworked on lane-S6c1R for the two must-fix items of the S6c1 review. Status: ready for review. The full test set exits 0 (Checks below).

## What changed (production)

- `internal/task/gates.go`:
  - `Approve` is the same as on b8378c46. The S6c1 `JobResult` hook was removed in the rework (see Lock order).
  - New `JobApproval` type and `ApprovalFromJob(dir)`, placed next to `Approved`. It takes no lock and reads the Feed with `ReadOnly`. It looks for the newest accepted control (greatest ULID) that targets a person Job, is accepted in the fold, and carries `plan_digest`. It returns nil when that result's `plan_hash` and `plan_digest` match plan.md now. Otherwise it returns "Changed since you approved: plan hash no longer matches", or "plan has not been approved". No caller changed except the tests.
- `internal/task/write.go`: unchanged against b8378c46. The `WriteOptions.JobResult` field and its comment were removed in the rework.
- `internal/cli/orchestrator.go`:
  - `grid job result <job> --plan-hash <h>` on a person Job (`runPersonApproval`) runs the same `land.Store.Approve` compare-and-set as `grid approve`. `answer` and `note` come from `--body`. After Approve returns without error, it posts the result with `personApproval` and prints the approve event.
  - A Job that is already accepted, cancelled or rejected gets the message AnswerPerson gives (`%s is already %s`), before anything is written. That is not a new refusal: AnswerPerson already refuses this case.
  - New helpers:
    - `planPersonJob`: the first open person Job with `kind: approve`, in Feed order.
    - `personApproval(store, w, job, answer, event)`: one `orchestrator.Write` of the accepting control `{target, status: accepted, reason: "answered by <id>", answer, plan_hash, plan_digest, lands_hash?, note?}`. The Feed record carries who and when. A Job already accepted with the same digest gets nothing new.
  - `internal/orchestrator` is not in scope, so `AnswerPerson` is unchanged.
- `internal/cli/task_write.go`: when the task has an open person Job of an approve step, `grid approve` posts the result with `personApproval` after `land.Store.Approve` returns without error. If the Feed writer cannot be built (for example no `GRID_MEMBER`), it prints "`<job>` stays open: ..." and writes approval.json as before. This adds no refusal.
- `contract/schemas/control.json`: the status-control branch gains five optional properties: `answer`, `plan_hash` (6 hex), `plan_digest` (64 hex), `lands_hash` and `note`. All are additive. No required field and no other branch changed. `cmd-approve.json` is unchanged.

approval.json is written exactly as before (Q9). The Job result is posted by the same command, right after approval.json is committed and the task and plan locks are released.

## Regenerated files (commit 26b899da, by the orchestrator)

- `contract/commands.json`: gains the `--plan-hash` flag on the job verbs (45 added lines, nothing removed).
- `testdata/golden/mcp-session.golden`: one line changes (1 insertion, 1 deletion). `feed_types` prints control.json, and the line gains the five optional control fields `answer`, `plan_hash`, `plan_digest`, `lands_hash` and `note`. Nothing else in either file changed.

The rework changes no golden.

## Lock order (review must-fix 1)

S6c1 posted the Job result from inside `Approve`, while `.task.lock` was held, and the post takes `.orchestrator.lock`. A tick takes `.orchestrator.lock` first and then `.task.lock` in `publishPlan`, so the two could hang each other. The rework posts the result only after `land.Store.Approve` has returned:

- `.plan.lock` is taken at `internal/plan/store.go:104` and released by the defer at `internal/plan/store.go:107` when `WithDigest` returns (`internal/task/gates.go:80-118`).
- `.task.lock` is taken at `internal/task/write.go:466-471` and released by the defer at `internal/task/write.go:474` when `withWrite` returns (`internal/task/gates.go:62-123`). `Approve` returns at `gates.go:124`, and `land.Store.Approve` at `internal/land/approval.go:44`.
- `grid approve`: Approve at `internal/cli/task_write.go:172`, then the post at `task_write.go:176`, guarded by `if err == nil && post != nil` at `:175`.
- `grid job result --plan-hash`: Approve at `internal/cli/orchestrator.go:381`, then `personApproval` at `orchestrator.go:385`, guarded by `if err == nil` at `:384`. Its single `orchestrator.Write` is at `orchestrator.go:439`.
- A stale hash returns from `WithDigest` at `gates.go:99`, before anything is written. `err` is then non-nil, so nothing is posted.

So the new path takes `.orchestrator.lock` only after both locks are released. That matches the rule at `internal/orchestrator/ready.go:3-6`.

## Message and control (review note 2)

`orchestrator.Write` writes one `feed.Input`, so the result is now one record: the accepting control at `internal/cli/orchestrator.go:439`. It carries `answer`, and `reason: "answered by <id>"` as AnswerPerson's control does. S6c1 also wrote a separate "answered" message first, in its own Write. That message is no longer written on this path, so a failure cannot leave an orphan message, and a repeat press finds either no result (and posts one) or the accepted control (and posts nothing). No reader parses the "answered" message text (grep for `answered` over internal and cmd). To keep both records under one lock, as AnswerPerson does, internal/orchestrator would need an export, and it is out of scope (open item 7).

## Removed non-test lines

Against b8378c46, none in production. The only removed line is the regenerated `testdata/golden/mcp-session.golden` line (above). The rework removed the S6c1 lines that it replaced (`WriteOptions.JobResult`, the hook call in `Approve`, the message Write in `personApproval`). Those lines were added by S6c1 and are not on b8378c46.

`git diff b8378c46 --stat` on the lane head:

```
 contract/commands.json             |  45 ++++++++++++
 contract/schemas/control.json      |  20 ++++++
 docs/lanes/S6c.md                  | 139 +++++++++++++++++++++++++++++++++++++
 internal/cli/approve_job_test.go   | 131 ++++++++++++++++++++++++++++++++++
 internal/cli/approve_lock_test.go  |  80 +++++++++++++++++++++
 internal/cli/orchestrator.go       |  97 ++++++++++++++++++++++++++
 internal/cli/task_write.go         |  16 +++++
 internal/task/gates.go             |  58 ++++++++++++++++
 testdata/golden/mcp-session.golden |   2 +-
 9 files changed, 587 insertions(+), 1 deletion(-)
```

## Tests

`internal/cli/approve_job_test.go` (S6c1, unchanged by the rework):

- `TestApprovePostsPersonJobResultWithPlanDigest`: `grid approve --plan-hash <current>` accepts J-2 (a person Job with `kind: approve`) with `plan_hash`, `plan_digest` and `answer: approved`, and approval.json carries the same hash and digest.
- `TestApproveStaleHashWritesNoJobResult`: runs both `grid approve --plan-hash 000000` and `grid job result J-2 --plan-hash 000000`. Each exits 1 with "plan hash no longer matches", as today's stale approve does. Neither writes an accepted control or approval.json.
- `TestJobResultPlanHashAndApprovalFromJob`: before any result, `ApprovalFromJob` errors. After `grid job result --plan-hash <digest>` it matches (job, hash, digest, answer, note). After plan.md changes it reports "Changed since you approved".

`internal/cli/approve_lock_test.go` (rework, new):

- `TestApprovalPostDoesNotHoldTaskLockWhileWaitingForOrchestratorLock`: for `grid approve` and for `grid job result --plan-hash`, the test holds `.orchestrator.lock` and runs the command. Once approval.json exists, `.task.lock` must become free (a `TryLockExclusive` succeeds) while the command still waits. The command must not finish while the lock is held. After release it must exit 0 and post the result with the plan digest.
- On 26b899da it fails. Run with the new test in place and the four production files checked out from 26b899da (`git checkout 26b899da -- internal/task/gates.go internal/task/write.go internal/cli/task_write.go internal/cli/orchestrator.go`, restored afterwards with `git checkout HEAD -- ...`):

```
=== RUN   TestApprovalPostDoesNotHoldTaskLockWhileWaitingForOrchestratorLock/approve
    approve_lock_test.go:73: .task.lock was held while waiting for .orchestrator.lock
=== RUN   TestApprovalPostDoesNotHoldTaskLockWhileWaitingForOrchestratorLock/job_result
    approve_lock_test.go:73: .task.lock was held while waiting for .orchestrator.lock
--- FAIL: TestApprovalPostDoesNotHoldTaskLockWhileWaitingForOrchestratorLock (10.04s)
    --- FAIL: TestApprovalPostDoesNotHoldTaskLockWhileWaitingForOrchestratorLock/approve (5.02s)
    --- FAIL: TestApprovalPostDoesNotHoldTaskLockWhileWaitingForOrchestratorLock/job_result (5.02s)
FAIL
FAIL	github.com/gridconsole/grid-plugins/internal/cli	10.048s
```

- On the lane both subtests pass (0.07s).

No test was deleted. No existing test or expectation was edited, except the two regenerated files above.

## Checks (lane-S6c1R, run by the builder)

- `go test -count=1 -run 'TestApprovePostsPersonJobResultWithPlanDigest|TestApproveStaleHashWritesNoJobResult|TestJobResultPlanHashAndApprovalFromJob|TestPersonJob' -v ./internal/cli/`: exit 0, all 4 PASS.
- `go test -count=1 -run TestApprovalPostDoesNotHoldTaskLockWhileWaitingForOrchestratorLock -v ./internal/cli/`: exit 0 on the lane. Exit 1 on the 26b899da production files (above).
- `go vet ./...`: exit 0.
- `gofmt -l internal cmd`: exit 0, no output.
- `GOOS=windows go build ./...`: exit 0. `GOOS=darwin go build ./...`: exit 0. `GOOS=windows go vet ./internal/cli/`: exit 0.
- Packages that are or directly import internal/cli or internal/task (`go list -f '{{.ImportPath}} {{join .Imports " "}}' ./...`): cmd/grid, internal/braincmd, internal/cli, internal/guard, internal/hook, internal/job, internal/job/jobcmd, internal/job/jobtest, internal/land, internal/signal, internal/task. All of them are in the run below.
- `go test -count=1 ./internal/... ./cmd/...`: exit 0. Tail:

```
ok  	github.com/gridconsole/grid-plugins/internal/session	0.147s
ok  	github.com/gridconsole/grid-plugins/internal/settings	0.188s
ok  	github.com/gridconsole/grid-plugins/internal/signal	0.220s
ok  	github.com/gridconsole/grid-plugins/internal/task	1.560s
ok  	github.com/gridconsole/grid-plugins/internal/taskfile	0.011s
ok  	github.com/gridconsole/grid-plugins/internal/testclean	1.867s
ok  	github.com/gridconsole/grid-plugins/internal/testenv	0.005s
?   	github.com/gridconsole/grid-plugins/internal/testgit	[no test files]
ok  	github.com/gridconsole/grid-plugins/internal/usage	0.366s
ok  	github.com/gridconsole/grid-plugins/internal/version	0.035s
ok  	github.com/gridconsole/grid-plugins/internal/where	1.167s
ok  	github.com/gridconsole/grid-plugins/internal/worker	0.005s
ok  	github.com/gridconsole/grid-plugins/internal/workflow	0.738s
?   	github.com/gridconsole/grid-plugins/cmd/grid	[no test files]
```

  The same run shows `ok` for internal/cli (15.070s), internal/orchestrator (17.797s), internal/land (3.322s), internal/guard, internal/hook and internal/mcp.
- `make lint` was not run (the agent cannot). `go vet` and `gofmt` stand in for it.

## NOT VERIFIED

- The deadlock was not reproduced with two real processes. The test shows the cause instead: the old code held `.task.lock` while it waited for `.orchestrator.lock`.
- The three S6c1 tests failing on b8378c46. They do not compile there (`taskstore.ApprovalFromJob` is undefined, and `--plan-hash` is not a `job result` flag), but no failing output was captured.
- Whether `feed.Store.State()`, called by `ApprovalFromJob`, writes state.json (review note 5).
- The cross-repo set and the app. No UI changed.

## Open items

1. Review note 1: if the post fails after approval.json is committed, the command exits non-zero, with approval.json written and no Job result. A repeat press posts the missing result. `Approved` says yes and `ApprovalFromJob` says no until then. Nothing reads the Job result yet. S6c2 must handle this before it switches readers.
2. Review note 3: the `--plan-hash` path skips AnswerPerson's answer-length cap (3500) and its "is not a Job for a person" check. It relies on `personResult` having established that the Job is a person Job.
3. "The plan Job" means the first open person Job with `kind: approve` for `grid approve`. `ApprovalFromJob` accepts any accepted person Job whose result carries `plan_digest`, not only a `kind: approve` one. S6c2 should settle both when it moves the readers.
4. When no Feed writer exists, the `grid approve` fallback warns and leaves the Job open. The owner may prefer that it fails instead.
5. `personApproval`'s "already accepted with this digest" check runs before the Write, outside `.orchestrator.lock`. Two presses at the same moment could each post an accepting control.
6. `make lint` and the cross-repo set are run by the orchestrator at landing.
7. The approve path writes no separate "answered" message. If the owner wants that message beside the control, internal/orchestrator needs an exported way to post both under one `.orchestrator.lock` (as AnswerPerson does).

# S6c2: the approval readers move to the person Job result

Base lane-S6c1R a8f6dfcd. Head: see `docs/lanes/S6c2.md`.

## Definitions chosen (S6c1 open item 3, review note 2)

- **The plan Job** is a Feed `job` record with `role: Person` and `kind: approve`, the person Job of the workflow's approve step. A task may have several; the plan is approved by the newest accepted result, of any of them, that carries `plan_digest` (greatest control ULID). A person Job of another kind, or an accepted result without `plan_digest`, is not a plan approval. Why: `kind: approve` is the one field a Job record already has for "a person approves here", so no new field or name match is needed; "newest wins" is what the single approval.json did when it was overwritten.
- **`ApprovalFromJob`** (and the readers) reads the Feed once, read-only (no cache or `state.json` write, which also closes the S6c1 NOT VERIFIED item on `State()`), takes the verdict set the fold uses (counted, stale, unknown) and then compares `plan_hash` and `plan_digest` to plan.md.
- **No plan Job** (`ErrNoPlanJob`): the task has no stable ULID (so no Feed) or no `kind: approve` person Job in it. This is an old task (Q10). `Approved`, `ApprovedDigest` and `ApprovalForGate` read approval.json for that task only, as before, including `approval-preview.json` and `approval-ship.json` by gate name (S6c3/S6c4 remove those). Why: tasks in flight keep working, and the gate-by-file tests keep covering the code S6c3/S6c4 delete.
- **Plan Job with no result** (review note 2): readers say `plan has not been approved: the plan Job J-2 has no result yet; run grid approve to post it`. approval.json is not consulted. This is a message on an existing "not approved" answer; no refusal was added. A repeat `grid approve` posts the result, because the Job is still open (`TestApprovalJSONWithoutJobResultIsRecoverable`).
- **Re-approval after a plan edit** (found while moving the readers): once the plan Job is accepted, `planPersonJob` found no open Job, so `grid approve --plan-hash <new>` wrote approval.json but posted nothing, and the new readers would have said "Changed since you approved" for ever. `planPersonJob` now falls back to the last accepted approve Job and `personApproval` posts a new accepting control on it; the readers take the newest result (`TestReapprovalAfterPlanChangePostsANewResult`). This adds no refusal.
- **Closed task**: readers say not approved, as `checkApproval` did.
- **`ApprovalForGate` ignores the gate name** when the task has a plan Job. `land-approval: plan|ship` is still parsed and still printed as `approval_gate` in the land dry-run (S6c7 removes the field), so both land-dry-run goldens are unchanged by this lane.

## Production changes

- `internal/task/gates.go`: `planJobResult` (new), `ErrNoPlanJob`, `JobApproval.Who/When` and `event()`; `Approval`, `ApprovalForGate`, `Approved`, `ApprovedDigest` and `ApprovalFromJob` answer from it; `receiptForGate` is the old per-gate receipt code, used only for the Q10 fallback. attach.go calls `Approved`, so its `approved` field follows with no edit. land.go calls `ApprovalForGate`, so land follows with no edit. `Approve` still writes approval.json and reads it to recognise a repeat press (writing it).
- `internal/cli/orchestrator.go`: `planPersonJob` falls back to the last accepted approve Job.
- `internal/task/hold.go`: one comment line (see row 1).

## Remaining reads of approval.json in non-test code (acceptance row 1)

Command: `grep -rnE "readApproval|approval\.json|ApprovalForGate|approvalFile|checkApproval" internal cmd --include=*.go --exclude=*_test.go`.

- `internal/task/gates.go` `Approve`: reads the existing receipt to recognise a repeat press and to refuse an unreviewed re-approve (writing it).
- `internal/task/gates.go` `receiptForGate`, `ApprovedDigest`: the Q10 fallback for a task with no plan Job.
- `internal/task/gates.go` `checkApproval`, `readApproval`: used only by the two lines above and `Approve`.
- `internal/task/hold.go:143`: existence only, to choose the plan-hold branch; the answer is `Approved`. S6c5 deletes it.
- `internal/task/write.go:131, 488, 555`: reopen deletes it, its name, the write allowlist (writing it).
- `internal/plan/migrate.go:44`: migration deletes it.
- `internal/land/land.go:217`: calls `ApprovalForGate`, which is the Job path.
- `internal/guard/host.go`, `host_targets.go`: names in the protected list and a message; they read nothing.

## Changed expectations

None. No existing test was edited or deleted. `land/approval_fence_test`, `approval_policy_test`, `land_test`, `task/gates_digest_test`, `fixture_approval_test` and every other test seed a task with no ULID or no approve Job, so they run the Q10 fallback and pass unchanged. The Job path is covered by the new tests below.

## New tests

- `internal/cli/approve_reader_test.go`:
  - `TestLandReadsThePlanJobResult` (row 2): land exits 3 with `plan has not been approved` and no outcome before the result; exit 0 and `outcome` written after `grid approve`.
  - `TestJobResultAnswersWithoutApprovalJSONAndForAnyGateName`: with approval.json deleted, `Approved` is true and `ApprovalForGate` returns the same event for `plan`, `ship` and `preview`, with who and when from the Feed record.
  - `TestPlanChangeAfterJobResultIsNotApproved` (row 3): `Approved`, attach `approved` and land after plan.md changes. This one also passes on the base, because the base's approval.json goes stale the same way; it pins behaviour that must not change.
  - `TestApprovalJSONWithoutJobResultIsRecoverable` (row 4): approve without `GRID_MEMBER` leaves approval.json and an open Job; `Approved`, attach and land say not approved and name `plan Job J-2 has no result yet`; the repeat `grid approve` posts the result and they say approved.
  - `TestReapprovalAfterPlanChangePostsANewResult`: approval.json at the new hash alone does not approve; the repeat posts a new result on the accepted Job.
- `internal/task/plan_job_fallback_test.go`: `TestTaskWithoutPlanJobFallsBackToApprovalJSON` (row 5).

On the base (a8f6dfcd `internal/task/gates.go` checked out over the lane, restored afterwards), the first run printed:

```
--- FAIL: TestLandReadsThePlanJobResult (0.01s)
    approve_reader_test.go:56: land before approval: 3 land: land refused: plan has not been approved
--- FAIL: TestJobResultAnswersWithoutApprovalJSONAndForAnyGateName (0.01s)
    approve_reader_test.go:82: Approved ignored the Job result
--- FAIL: TestApprovalJSONWithoutJobResultIsRecoverable (0.01s)
    approve_reader_test.go:130: approved from approval.json alone, with a plan Job that has no result
```

`TestTaskWithoutPlanJobFallsBackToApprovalJSON` does not compile on the base (`undefined: ErrNoPlanJob`); it passes in behaviour on both. `TestReapprovalAfterPlanChangePostsANewResult` was added after that run; before the `planPersonJob` change it failed on the lane with `Changed since you approved: plan hash no longer matches`.

## NOT VERIFIED

- Reopen (Q8): a reopen deletes approval.json (write.go:131) but leaves the accepted Job result in the Feed, so for a task with a plan Job, `Approved` stays true after a reopen while plan.md is unchanged. Today a reopen clears approval. No S6c step in the map covers it; it needs the Q8 "reopen posts a new plan Job" step, probably in S6c3 with write.go.
- Land's `Run` was tested through `grid land`; the `internal/land` package tests do not seed a Feed.
- `make lint` and the cross-repo set are the orchestrator's.

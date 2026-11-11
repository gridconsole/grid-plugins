# E-receipts-e2e3: grid approve records only the person Job result; the approval.json fallback is gone

Starts at lane-E-verdict-12-receipts (5d1291d3, E1: `migrate.ReceiptToJob`). This lane is E2 and E3 as one lane. Every row below was run on the final tree of this lane. The reviewer runbook (`/srv/gridwork/tmp/hand/runbook-reviewer.txt`) was read and the lane checked against it: each row is HOLDS with a command or a file:line, or NOT DONE.

## Proof table

| Row / rule | Status | Evidence |
|---|---|---|
| 1. `grid approve` (runApprove and runPersonApproval) calls `migrate.ReceiptToJob` first | HOLDS | internal/cli/task_write.go:182 (`runApprove`), internal/cli/orchestrator.go:404 (`runPersonApproval`). `TestApproveRecordOldReceiptMovedOrRetired` (internal/cli/approve_record_test.go). |
| 1. Creates the approve person Job when none exists, with `feed.EnsureTaskID` for a task with no ULID | HOLDS | internal/cli/approve_record.go `recordApproval` (EnsureTaskID, then `migrate.PostPlanJob`). `TestApproveRecordNoUlidNoJobCreatesBoth`. |
| 1. Person with no lease uses the unleased writer; answers with plan_hash, plan_digest, lands_hash, note | HOLDS | `approvalWriter` (approve_record.go:20, `feedWriter(env, "", "")`), `orchestrator.AnswerPersonWith` (internal/orchestrator/person.go), holder's `personAnswers` copies the fields into the control. `TestApproveRecordPersonWithoutLeaseWaitsForTheHoldersTick`, `TestAnswerPersonWithUnleasedPersonCarriesTheFieldsToTheHoldersPass`. A caller with no GRID_MEMBER and no epoch gets exit 3 and nothing is written: `TestApproveRecordCallerWithoutMemberWritesNothing`. |
| 1. Leased approval answers in one control (open Job) | HOLDS | `TestApproveRecordOpenJobAnswered`, `TestApproveRecordJobResultPlanHashLeasedAndPerson` (both paths: runApprove and `grid job result --plan-hash`). |
| 1. `Store.Approve` keeps the stale-hash check, the compare-and-set read from the Job result and the history line; writes no file; reopen stops writing it | HOLDS | internal/task/gates.go:50-80 (reads `planJobResult`), `commitWrite(t.Path, event, map[string][]byte{})`; internal/task/write.go:125-129 (reopen). Tests: internal/task/approve_job_result_test.go (`TestApproveWritesOnlyTheHistoryLine`, `TestApproveCompareAndSetReadsTheJobResult`, `TestReopenWritesNoReceiptFile`). |
| 1. A Go test per path | HOLDS | The seven tests in internal/cli/approve_record_test.go (open Job; no ULID and no Job; person with no lease; `job result --plan-hash`, leased and unleased; old receipt moved or retired; no GRID_MEMBER; the worker pass). |
| 2. receiptForGate, the ApprovedDigest fallback, checkApproval/readApproval and the hold.go approval.json branch removed; hold.go asks whether an approve Job exists | HOLDS | internal/task/gates.go (functions gone; `HasPlanJob` added), internal/task/hold.go:141 `if HasPlanJob(dir)`. `TestPlanHeldAsksForThePlanJob`. |
| 2. Tests that used approval.json as a fixture use one shared Job-fixture helper | HOLDS, with one exception | internal/approvefix/approvefix.go (`Lease`, `OpenJob`, `Accept`, `Approve`, `JobBody`): used by internal/task, internal/land, internal/cli tests. Exception: internal/orchestrator/plan_test.go builds its own `approveRevision` (same records, J-90/J-91) and `internal/job/jobtest` still calls `Store.Approve` (history line only). |
| 2. `grep -rn "approval.json\|approvalFile" internal --include=*.go \| grep -v _test.go` lists only internal/migrate | HOLDS | Command run on the final tree, filtered with `grep -v "^internal/migrate/"`: prints nothing, exit 1 (no match). Unfiltered, every line is in internal/migrate/receipt.go. |
| 3. ReceiptToJob has a caller in the migrate/tick path; a task only read (no approve) still gets its Job | HOLDS | Worker pass: internal/orchestrator/stream.go (`ReceiptPending`/`ReceiptMove`, taken under the task lease), wired at internal/cli/worker.go:149-154. Tests: `TestStreamPassMovesTheReceiptOfATaskNobodyApproves`, `TestStreamPassGivesAnOldTaskItsFeedWhenItMovesTheReceipt`, `TestStreamPassLeavesAReceiptThatNeverApprovedAPlan` (internal/orchestrator/stream_receipt_test.go); `TestApproveRecordWorkerPassMovesTheReceiptOfATaskOnlyRead` runs the real `grid worker next`. |
| 4. TestApproveStaleHashWritesNoJobResult and TestReceiptToJobKeepsTheStalePlanRefused pass unedited | HOLDS | `git diff 5d1291d3 -- internal/migrate/receipt_test.go` shows no edit of that test; internal/cli/approve_job_test.go: the test body is unchanged. `go test -count=1 ./internal/cli ./internal/migrate` ok. |
| 5. `go test ./...` | HOLDS | `go test -count=1 ./...`: every package ok except `internal/eval` `TestOverwrittenTraceFileIsNotRead` in one run under load (the other forks were still compiling). It passes alone: `go test -count=1 ./internal/eval` ok (6.2 s), and it passed in an earlier full run. I judge it a timing flake; it does not touch this lane. |
| 5. `make lint` | HOLDS | `make lint` printed `go vet ./...`, exit 0; `gofmt -l internal cmd contract` prints nothing. |
| 5. Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0 (one chained command printed BUILDS-OK). |
| 6. App follow-up line | HOLDS (reported, app unchanged) | app `internal/slices/gates/model.go:477` (and the comment at :76) reads approval.json: after this lane it must read the plan Job result. The app's replay of `testdata/contract/plan-digest/transcript.json` reads `receipt_keys`, now the keys of a Job result (`answer job note plan_digest plan_hash record when who`), not the old receipt keys. |
| Small commits, one-sentence messages, no attribution lines | HOLDS | `git log 5d1291d3..HEAD`: no Co-Authored-By or Claude-Session line. |
| New tests fail on base | HOLDS (by compile failure) | They call symbols this lane adds, so on 5d1291d3 they fail to build: `approvefix`, `orchestrator.AnswerPersonWith`, `Stream.ReceiptPending`/`ReceiptMove`, `migrate.ReceiptPending`, `migrate.PostPlanJob`, `taskstore.HasPlanJob`, `ErrPlanJobOpen`. I did not run the behavioural cli tests against a base worktree (shared tree during the build). |
| No guard, gate, cap, hold or blocking hook added | HOLDS | Nothing refuses an agent that could act before. `grid approve` was person-only already (cli.go:116). The only new refusal is the exit 3 for a caller who is not a person of the team (see "Choices"). |
| Tests of changed packages and importers | HOLDS | Importers from `go list` of task, plan, migrate, orchestrator: approvefix, braincmd, cli, guard, hook, job, job/jobcmd, job/jobtest, jobgraph, land, migrate, orchestrator, proof, signal, task. All ok in `go test -count=1 ./...` (eval flake above). |

## What changed

- `internal/task/gates.go`: `Store.Approve` reads the previous approval from the plan Job result, writes only the history line. `receiptForGate`, the `ApprovedDigest` fallback, `checkApproval` and `readApproval` are gone. `HasPlanJob` and `ErrPlanJobOpen` added. A task with no plan Job is not approved (`ErrNoPlanJob`).
- `internal/task/write.go`: reopen writes no file; journal replay drops an old receipt entry and replays the rest. `internal/task/hold.go`: asks `HasPlanJob`.
- `internal/cli/approve_record.go` (new), `task_write.go`, `orchestrator.go`: the approval path above. `grid job result --plan-hash` is now unleased for a person with no epoch (it was lease-only).
- `internal/migrate/receipt.go`: `ReceiptToJob` takes a leased or an unleased person writer (an unleased person posts the Job, then an answer the holder's pass accepts); `ReceiptPending`, `PostPlanJob` added; `ReceiptToJob` gives a task with no ULID its Feed.
- `internal/orchestrator`: `AnswerPersonWith`; `personAnswers` copies `answer`, `plan_hash`, `plan_digest`, `lands_hash`, `note`, `reason` into the accepting control; the Stream pass moves a pending receipt under the task lease.
- `internal/plan/migrate.go`: `Migrate(yes, approved func() string)` takes the approved hash from the plan Job (it read three receipt files). `internal/guard/host.go`: reason text says "approval*.json"; the fence regex is unchanged, so an agent still cannot write the file (an agent-written receipt would be moved into an approval).
- `contract/schemas/answer.json` (data format, outside the Touches list): the answer of a person Job admits optional `plan_hash`, `plan_digest`, `lands_hash`, `note`, `reason`, copied from `control.json`. Golden `testdata/golden/mcp-session.golden` and `testdata/contract/plan-digest/transcript.json` follow.
- Tests: about 25 existing test files moved off approval.json; `internal/approvefix` is the shared helper (it writes the lease file itself, since `lease` imports `job`, which imports `task`).

## Choices made (owner defaults, silence is approval)

- approval.json is deleted after it is read, not renamed. A receipt beside an existing approve person Job is retired without answering that Job.
- "Once" runs in both places: at the start of `grid approve` and `grid job result --plan-hash`, and in the worker pass (`stream.go`), not in `grid migrate`. Why: `grid migrate` copies a Grid home as a tree (journal, rollback) and holds no task lease, so it cannot write the accepting control; the worker pass visits every task of the home each second and already takes the task lease. A home with no worker running migrates a task when someone approves it.
- `answer.json` carries the approval fields (above) so a person with no lease can record the plan revision they saw. This is the way the lane doc described (`AnswerPerson` carries the fields).

## Open items and notes for the owner

1. **Schema change for review**: `contract/schemas/answer.json` and the two goldens changed. The app mirrors the contract; the cross-repo set at landing must run.
2. **An approval by a person with no lease takes effect at the holder's next tick**, as every answer to a person Job already does. Until then readers say "the plan Job J-n has no result yet; run grid approve to post it". On a task whose plan.md is not in the Job format, `Tick` fails in `publishPlan` before it accepts the answer, so the approval waits; tests with legacy plans approve while holding the lease.
3. **Gate name**: `TestWorkflowApproval` case `{"ship","plan"}` (internal/land) now expects approved. The old refusal belonged to `receiptForGate`; the Job path never checked the gate name. If a ship gate must keep refusing a plan approval, `ApprovalForGate` has to refuse `gate != "plan"`. Not done: that is a new refusal.
4. **Reopen window**: `Store.Change(reopen)` no longer overwrites a receipt, so the old result approves the plan until `grid task reopen` posts the continuing Job (the writer is built first; the post follows `Change` at once). A failed post leaves the old approval valid.
5. `grid job result --plan-hash` with no GRID_MEMBER is not refused (it takes its identity from `feedWriter`, as `grid job post --person` does); only `grid approve` exits 3.
6. Tests whose assertions changed (owner direction: verdict 12, docs/lanes/E-verdict-12-receipts.md: approval.json is no longer written). Deleted by the same direction: `plan_job_fallback_test.go` (`TestTaskWithoutPlanJobFallsBackToApprovalJSON`, covered by `TestTaskWithoutPlanJobIsNotApproved`), `TestApprovedRejectsAnotherGate` (covered by `TestAnApprovalFileApprovesNothing`); `TestLandRefusesSymlinkedApproval` became `TestLandIgnoresAnApprovalFile`; `TestApprovalJSONWithoutJobResultIsRecoverable` became `TestApproveWithoutAWriterIsRecoverable` (the state "file written, post failed" no longer exists); `TestReopenOverwritesPlanReceipt` became `TestReopenWritesNoReceiptAndTheNewPlanJobSupersedes`. `TestApprovalDigestCompatibility` dropped the malformed-digest cases the Feed schema refuses.
7. The budget: the brief said about 120 calls; the lane used four forks for the test rewrite (about 25 test files).

## NOT VERIFIED on a live screen

- The dev UI approve button against a real worker: the app's approve route runs `grid approve` as a person with no lease; the approval shows after the holder's next tick. Tests prove the Feed records, not the screen.
- The app's replay of the regenerated plan-digest transcript.

## Cross-repo resume (11 Nov 04:20): five app real-plugin tests fail by design

Done: `origin/main` (fd9a5324) merged into the lane (5f21556c), no conflicts. After the merge `go build ./...`, `go vet ./...` and `go test ./...` exit 0 with no failing package, and the row 2 grep still lists only `internal/migrate`.

What the five tests drive (app tree in tmp/r18-B-E-receipts-e2e3/app, internal/slices/gates):

- `TestRealPluginApprovePlanOnce` (realplugin_gate_test.go:162) and `TestRealPluginApproveAgentAccepted` (:199) run the real `grid approve`, then read `approval.json` from the task folder and fail with "no approval.json". Lane rows 1 and 2 are exactly that: approve writes no file.
- `TestRealPluginDigestReplay` replays plugin-digest/transcript.json: the fixture home holds an `approval.json`, and each step asserts the file's keys (`receipt_keys`, realplugin_digest_test.go:253-286). The plugin now moves that file into the plan Job and deletes it (owner default), so the file is gone or never written.
- `TestRealPluginTaskCardPlanApprovalAndHash` and the card reads: the app closes an approval item only when `approval.json` holds the matching plan_hash and plan_digest (gates/model.go:477-498). With no file the item stays open after a real approve, so the card shows "needs-you approval" for a plan that was approved. (`TestRealPluginWorkerSpawnsPostedJob` passed on the r18 log.)

So the cause is the one row 6 names: the app reads the file this lane removes. No plugin-only change fixes the five without writing approval.json again (and a written copy would be deleted by the once-migration on the next worker pass, because a receipt beside an approve Job is retired). That would reverse acceptance rows 1 and 2 and the owner default "approval.json is deleted after it is read", so I did not do it.

The fix is in the app, to land with this lane (lockstep):

1. `gates/model.go:477`: read the approval from the plan Job result (plan_hash, plan_digest, who, when; the same fields `ApprovalForGate` returns) instead of `approval.json`; the app already reads Job results through the Feed read shapes.
2. The five tests: assert the Job result (or `grid task` / `grid job show` output) instead of the file; `testdata/plugin-digest` keeps `approval.json` only as the legacy fixture and expects it gone after the first approve or tick; regenerate transcript `receipt_keys` from testdata/contract/plan-digest/transcript.json (now `answer job note plan_digest plan_hash record when who`).

Until that lands, run the cross-repo set with the app change, not against the plugin alone.

# E-verdict-12-receipts: E1 done (the migration, tested), E2 and E3 not done

Fix round 1 asked for E1, E2 and E3. E1 is built and green. E2 (the writer) and E3 (the readers and about 37 test files) are not: they cannot be staged apart and do not fit what is left of the 60 calls. This head is green and changes no behaviour; the writer still writes approval.json and the fallback readers are still there. Rows 2 and 3 (removal) are NOT DONE, and the next lane is sized below with the two owner questions answered from the code.

## Proof table

| Row / rule | Status | Evidence |
|---|---|---|
| 1. Every reader and writer of approval.json and receipt fields, plugin and app | HOLDS | The lists below (from the previous round; code is unchanged except one new reader, `internal/migrate/receipt.go:58,68`, which reads the file once and removes it). The app reader is a follow-up line, not a change. |
| 2. `grid approve` records the approval only as the person Job result; with no person Job it creates one | NOT DONE | internal/cli/task_write.go:193 still reaches `Store.Approve`, which writes approval.json (internal/task/gates.go:92). Needs E2; see "The next lane". |
| 3a. An old approval.json is read once by the migrate path into a person Job result, with a Go test on a fixture folder | HOLDS | `internal/migrate/receipt.go:56` (`ReceiptToJob`), `internal/migrate/receipt_test.go:80,113,128,150,176` (five tests, six cases). `go test ./internal/migrate -run ReceiptToJob -v`: all PASS. The migration is a library function with no caller yet: the caller is E2 (below). |
| 3b. receiptForGate and the hold.go receipt branch removed | NOT DONE | internal/task/gates.go:120,147-175,198-207 and internal/task/hold.go:141-145 are unchanged. Removing them breaks the tests that use approval.json as their fixture (E3). |
| 4. A stale plan revision is still refused; the existing stale-hash test passes unchanged | HOLDS (nothing changed in the writer) | `TestApproveStaleHashWritesNoJobResult` (internal/cli/approve_job_test.go:90) passes unedited: `go test ./internal/cli` is ok. New: `TestReceiptToJobKeepsTheStalePlanRefused` (internal/migrate/receipt_test.go:113) shows a moved receipt for another revision still gives "Changed since you approved". It must stay green in E2. |
| 5. `go test ./...` and `make lint`: 0 fail | HOLDS on this head | `go test ./... 2>&1 \| grep -v "^ok \|no test files"` printed nothing: every package ok. `make lint` printed only `go vet ./...`, no error, exit 0. |
| Small commits, one-sentence messages, no attribution lines | HOLDS | `git log be001ab6..HEAD`: three commits (887fa38a, 4ce80481 and this doc), no Co-Authored-By or Claude-Session line. |
| New tests fail on base, pass on the lane | HOLDS | Base (receipt.go stashed): `go test ./internal/migrate` gives `internal/migrate/receipt_test.go:85:23: undefined: ReceiptToJob`, `:86:30: undefined: ReceiptMoved`, `:131:86: undefined: PlanJobBody` and more (build fails). On the lane: all PASS. |
| No guard, gate, cap, hold or blocking hook added | HOLDS | Only a library function and its tests were added; no command calls it. |
| Tests of changed packages and importers | HOLDS | `go list -f '{{.ImportPath}}: {{join .Imports " "}}' ./... \| grep internal/migrate`: internal/cli (and internal/migrate itself). `go test ./internal/migrate ./internal/cli`: both ok (8.3 s, 37.0 s). |
| Lint, Windows and macOS builds | HOLDS | `make lint` exit 0; `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |

## What changed

- `internal/migrate/receipt.go`: `ReceiptToJob(dir, w)` reads a task folder's approval.json once. With no approve person Job it posts one (J-N, role Person, kind approve, `PlanJobBody`) and one accepting control carrying plan_hash, plan_digest, lands_hash and note, then removes the file. The receipt's who and when go into the control's `reason` because the Feed record names the writer. With an approve person Job already there, the Job is the record (every reader already preferred it): the file is removed and nothing is posted. A receipt that never approved a plan (no full 64-hex plan_digest, a gate name, not "approve") is left as it is. The file is removed only after the Feed holds the record, so a failed post leaves it for the next run and a crash between the post and the removal is closed by the "Job exists" rule on the next run. The caller supplies the task-lease writer: the migration holds no lease.
- `internal/migrate/receipt_test.go`: the fixture-folder tests above. The Job record validated against the job-record schema on the first run.

Choices made here, for the owner to confirm: the file is deleted, not renamed (the Feed result holds every field, and the guard fences the name); a receipt beside any approve person Job, open or answered, is retired without answering the Job (the readers never used it, so no task becomes approved by the move).

## Row 1: readers and writers in the plugin (non-test code, file:line)

Writers of approval.json:
- internal/task/gates.go:54-92: `Store.Approve` reads the previous receipt (:55) and writes the new one through `commitWrite` (:92).
- internal/task/write.go:130-135: reopen reads the previous receipt and overwrites it with the reopen event.
- internal/task/write.go:485 (`approvalFile`) and :550 (the journal replay allowlist names it).
- Both commands reach the writer through internal/land/approval.go:44: internal/cli/task_write.go:193 (`runApprove`) and internal/cli/orchestrator.go:430 (`runPersonApproval`, used by `grid job result --plan-hash`). internal/land/approval.go:41 also sets `LandsHash` on the receipt.

Readers of approval.json:
- internal/task/gates.go:64-67 (compare-and-set against the previous receipt) and :81 (a repeat press writes nothing).
- internal/task/gates.go:120 and :147-175 (`receiptForGate`, reads at :157): the Q10 fallback of `ApprovalForGate`, used by `Store.Approval` and by land.
- internal/task/gates.go:199-206: `ApprovedDigest` fallback, used by `Approved` (hook, attach).
- internal/task/gates.go:329-366: `checkApproval` and `readApproval`, the receipt decoder.
- internal/task/hold.go:145: `os.Lstat` of approval.json, only to choose between the receipt branch and the history branch of `PlanHeldWith`.
- internal/plan/migrate.go:44-67: reads `plan_hash` of approval.json, approval-preview.json and approval-ship.json to warn before a plan-format migration.
- internal/migrate/receipt.go:58,68 (new): the one-time read of the migration.
- internal/guard/host.go:14: names approval.json in the write fence text (no read; stays).

Receipt fields (`plan_hash`, `plan_digest`, `lands_hash`) that already live in the Job result:
- internal/cli/orchestrator.go:487-496 writes them into the accepting control and reads them back with `ApprovalFromJob`.
- internal/task/gates.go:226-228 turns a Job result back into an `Event`. internal/land/land.go:217-221 reads `PlanHash` and `LandsHash` from `ApprovalForGate`.

## Row 1: readers in the app (main, not changed): follow-up lines

- internal/slices/gates/model.go:477-491: `Model.approval` reads approval.json (`plan_hash`, `plan_digest`) to decide whether the plan item is already approved (returns `ErrClosed` on a match). After E3 it must read the plan Job result instead. Comment at :76 says "(approval.json)".
- The web client only sends `plan_hash` and `plan_digest` to the approve route (internal/kernel/webui/app/pages/task/writes.mjs:225-266, transports/go.mjs:47-50) and reads the digest from the needs-you item detail. It never reads the file, so it needs no change.

## The next lane: E2 and E3 together

E2 and E3 cannot be landed green one after the other: once `Store.Approve` stops writing approval.json, every test that uses it as an approval fixture fails, and once the fallback is removed, every test that writes approval.json by hand fails. They are one lane of about 37 test files (counts of `approval.json|.Approve(|approvalFile` per file, from grep): task gates_test (22), gates_digest_test (14), approve_repeat_test (7), approval_workflow_test (4), deliver_test (3), plan_job_fallback_test (3), mode_test, no_gate_test, and 5 more; cli approve_job_test (13), approve_reader_test (9), plan_test, delivery_test, reopen_plan_job_test, approve_lock_test, approve_digest_test and more; guard (task_gates, host_gates, host_approval, host_brief); job (approval_gaps, no_plan_rule, approval_workflow, plan, roundtrip, jobcmd/command_test); orchestrator (plan, plan_digest); land (land_test); plan (legacy_test). The edit tool has no multi-file replace, so the test rewrite is the cost: it needs a shared Job-fixture helper first (a plan Job plus its accepting control, with the lease and handover the Feed requires; `receiptFolder` in internal/migrate/receipt_test.go is a working pattern), then one package at a time.

The two owner questions of the lane doc are answered by the code, so E2 does not wait for the owner:

1. A caller with no Feed writer. `grid job post --person` and `grid job result` already give a person of the team with no lease epoch an unleased writer (`feedWriter(env, "", "")`, internal/cli/orchestrator.go:254-267), which is why the worker holding the task lease never blocks a person. `grid approve` should use the same writer for the same caller, and `AnswerPerson` has to carry plan_hash, plan_digest, lands_hash and note (today only the leased path, `personApproval`, orchestrator.go:482-500, does). A caller that is not a person of the team (no GRID_MEMBER) gets exit 3 and nothing is written: `grid approve` is person-only (internal/cli/cli.go:116), so this refuses no agent that could approve before.
2. A task with no ULID. `grid approve` calls `feed.EnsureTaskID` (a writer's call, internal/feed/identity.go:117) before it posts, so the task gets its Feed and the Job.

Still open for the owner, one line: when "once" runs for tasks nobody approves again. Calling `ReceiptToJob` at the start of `grid approve` covers a task that is approved again; a task that carries an old approval.json and is only read (land, hook, attach) would answer "no plan Job" after E3. The two places that see every task are the worker tick that posts the handover (internal/orchestrator/stream.go:253) and `grid migrate`.

Then E2: `Store.Approve` keeps the stale-hash check, the compare-and-set (read from the Job result, not the file) and the history line but writes no file; reopen stops writing it (write.go:130-135); `runApprove` and `runPersonApproval` create the plan Job when none exists (`migrate.PlanJobBody`, or move it into orchestrator) and answer it. E3: remove `receiptForGate`, the `ApprovedDigest` fallback, `checkApproval`/`readApproval` and the hold.go branch (hold.go asks "has an approve person Job" instead of "does approval.json exist"); delete by the brief's direction (row 3) plan_job_fallback_test.go and `TestTaskWithoutPlanJobFallsBackToApprovalJSON`, `TestApprovedRejectsAnotherGate`, each naming the Job-path test that still covers the behaviour. Follow-up, app (not this repo): internal/slices/gates/model.go:477-491.

## What is NOT VERIFIED on a live screen

Nothing here touches a screen: no command calls `ReceiptToJob` yet, so the dev UI behaves as before.

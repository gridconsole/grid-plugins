# E-verdict-12-receipts: stopped at the sizing check, row 1 done, the build split into three lanes

No code changed. The brief does not fit one lane (60 calls): it needs a new migration writer, a changed approval writer, and a rewrite of about twenty test files that use approval.json as their approval fixture. Row 1 (the inventory) is done and is the only deliverable here. The stop came after the inventory, past call 10 (about call 18), because the inventory took the extra calls.

## Proof table

| Row / rule | Status | Evidence |
|---|---|---|
| 1. Every reader and writer of approval.json and receipt fields, plugin and app | HOLDS | The two lists below, from `grep -rnE "approvalFile|approval\.json|\.LandsHash|\.PlanDigest|readApproval|checkApproval|ApprovalForGate|ApprovedDigest|ApprovalFromJob" internal cmd --include=*.go --exclude=*_test.go` (exit 0) and `git grep -nE "approval\.json|plan_digest|plan_hash" main` on the app repo (exit 0). The app reader is a follow-up line, not a change. |
| 2. `grid approve` records the approval only as the person Job result; with no person Job it creates one | NOT DONE | Needs the split below, lane E2. Unchanged: internal/cli/task_write.go:193 still reaches `Store.Approve`, which writes approval.json (internal/task/gates.go:92). |
| 3. receiptForGate and the hold.go branch removed; old approval.json migrated once into a Job result, with a fixture-folder test | NOT DONE | Lane E1 (migration) then E3 (removal). internal/migrate has no Feed or Job code today (grep for Feed, Job, ULID in apply.go, plan.go, tree.go, record.go finds only a note at plan.go:270). |
| 4. A stale plan revision is still refused; the existing stale-hash test passes unchanged | NOT DONE | Nothing changed, so nothing to prove. The test is `TestApproveStaleHashWritesNoJobResult` (docs/lanes/S6c.md:70). It must stay green in E2. |
| 5. `go test ./...` and `make lint`: 0 fail | NOT DONE | Not run: no code changed. |
| Small commits, one-sentence messages, no attribution lines | HOLDS | One commit, this file. |
| New tests fail on base, pass on lane | NOT DONE | No tests written. |
| No guard, gate, cap, hold or blocking hook added | HOLDS | No code changed. |
| Tests of changed packages and importers, lint, Windows and macOS builds | NOT DONE | No code changed. |

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
- internal/guard/host.go:14: names approval.json in the write fence text (no read; stays).

Receipt fields (`plan_hash`, `plan_digest`, `lands_hash`) that already live in the Job result:
- internal/cli/orchestrator.go:487-496 writes them into the accepting control and reads them back with `ApprovalFromJob`.
- internal/task/gates.go:226-228 turns a Job result back into an `Event`. internal/land/land.go:217-221 reads `PlanHash` and `LandsHash` from `ApprovalForGate`.

## Row 1: readers in the app (main, not changed): follow-up lines

- internal/slices/gates/model.go:477-491: `Model.approval` reads approval.json (`plan_hash`, `plan_digest`) to decide whether the plan item is already approved (returns `ErrClosed` on a match). After E3 it must read the plan Job result instead. Comment at :76 says "(approval.json)".
- The web client only sends `plan_hash` and `plan_digest` to the approve route (internal/kernel/webui/app/pages/task/writes.mjs:225-266, transports/go.mjs:47-50) and reads the digest from the needs-you item detail. It never reads the file, so it needs no change.

## The split

- **E1, migration (additive, breaks no existing test).** A function in internal/migrate that, for a task folder with approval.json and no approve person Job, posts a person Job (kind approve, role Person) and one accepting control carrying plan_hash, plan_digest, lands_hash, note, who and when from the receipt, then retires the file. Fixture-folder Go test. Decisions needed first:
  1. Which `feed.Writer` identity a migration uses (a writer needs Lease, Gen and Fence; `grid migrate` holds no lease).
  2. What "once" means: at `grid migrate` apply, or on the first read or approve of that task.
  3. What happens to the file afterwards: deleted, or renamed. The guard fences the name.
- **E2, writer.** `Store.Approve` keeps the stale-hash check, the compare-and-set and the history line but stops writing approval.json; reopen stops writing it too (write.go:130-135); `runApprove` and `runPersonApproval` create the plan Job when none exists and answer it. Decision needed: a task with no ULID has no Feed, and a caller with no `GRID_MEMBER` or lease epoch has no writer. Today those two cases are the only reason approval.json is written. The brief does not say what `grid approve` does there (exit 3 with a message, or give the task an id first).
- **E3, readers and tests.** Remove `receiptForGate`, the `ApprovedDigest` fallback, `checkApproval`/`readApproval` and the hold.go branch (hold.go would ask "has a plan Job" instead of "does approval.json exist"). Rewrite the tests that use approval.json as their approval fixture to Feed Job fixtures: task (gates_test.go, gates_digest_test.go, approval_workflow_test.go, deliver_test.go, fixture_approval_test.go, no_gate_test.go), cli (approve_job_test.go, approve_lock_test.go, approve_no_gate_test.go, approve_reader_test.go, approve_digest_test.go, delivery_test.go, reopen_plan_job_test.go, plan_digest_roundtrip_test.go with its golden receipt keys), orchestrator (plan_test.go, plan_digest_test.go), land (land_test.go), job (no_plan_rule_test.go). Delete by the brief's direction (row 3): plan_job_fallback_test.go and the fallback tests in task (`TestTaskWithoutPlanJobFallsBackToApprovalJSON`, `TestApprovedRejectsAnotherGate`), each naming the Job-path test that still covers the behaviour.
- **Follow-up, app (not this repo):** internal/slices/gates/model.go:477-491, as above.

## What is NOT VERIFIED on a live screen

Nothing was built, so nothing was changed on a screen.

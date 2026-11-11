# M-plugin-held-f-e-receipts: grid approve writes the nine-key approval.json again (fix round 1, option B)

Built on lane-M-plugin-held-f-e-ready2 (c90f97d8). Fix round 1 told this lane to do option B: write the nine-key approval.json on approve and reopen, stop the receipt migration and the worker pass from deleting a file that sits beside the approved Job, and put the transcript's receipt_keys back. The approve person Job result stays the approval the plugin's readers use; the file is a copy for the app, which reads it. I read /srv/gridwork/tmp/hand/runbook-reviewer.txt and checked the lane against it.

## Proof table

| Row / rule | Status | Evidence |
|---|---|---|
| 1. Commit that dropped the receipt | HOLDS (by `git log -S`, not bisect) | 8ec0c09f: `Store.Approve` (internal/task/gates.go) and reopen (internal/task/write.go) stopped writing approval.json; `ReceiptToJob` (887fa38a, reworked in 8ec0c09f) and the Stream pass retired the file. |
| 1. Fresh app clone at main, `GRID_PLUGIN_BIN=<lane>/dist/grid go test -count=1 -run 'RealPluginApprove\|RealPluginDigestReplay\|RealPluginGate' ./...` | NOT DONE | The shell allow list refuses a command that starts with `GRID_PLUGIN_BIN=`, and the gates tests read only `os.Getenv("GRID_PLUGIN_BIN")` (realplugin_gate_test.go:50, realplugin_digest_test.go:43). I built the lane binary (`go build -o dist/grid ./cmd/grid`, exit 0) and cloned the app (`git clone /srv/gridwork/ref/app.git`, exit 0) but could not pass the path in. The failing lines are the brief's. Run at landing. |
| 2. `grid approve --json` prints one JSON object with from_state, note, plan_digest, plan_hash, task, to_state, what, when, who, and approval.json holds the same nine keys | HOLDS | New test `TestApproveReceiptFileHasTheNineKeysTheAppReads` (internal/cli/approve_receipt_file_test.go): runs `grid approve T-01 --json --note ...`, parses stdout as one object, reads approval.json, requires the nine sorted keys and equal values for each key, and checks the Job result still holds the digest. Passes in `go test ./...` below. Code: internal/task/gates.go (`Approve` writes `approvalFile` through `commitWrite`), internal/task/write.go (`change` overwrites it on reopen, `recoverWrite` replays it again). |
| 2. Held F and E behaviour kept | HOLDS | The Job result is still the only thing `Approved`, `ApprovalFromJob` and the stale-hash check read; no reader got a file fallback back (`receiptForGate`, the Q10 read are still gone). `recordApproval` and `runApprove` are unchanged. |
| 2. Migration and worker pass keep a file that sits beside the approved Job | HOLDS | internal/migrate/receipt.go: `ReceiptToJob` no longer removes the file in either case; `ReceiptRetired` became `ReceiptCurrent`; `ReceiptPending` is false once the task has an approve person Job (new `hasApproveJob`), so the Stream pass (internal/orchestrator/stream.go, unchanged: it only calls `ReceiptMove` for a pending receipt) does not come back to the file on every pass. Covered by the flipped tests `TestReceiptToJobMovesTheReceiptIntoTheApproveJobResult`, `TestReceiptToJobRetiresTheFileBesideAnExistingJob`, `TestStreamPassMovesTheReceiptOfATaskNobodyApproves`. |
| 3. Row-1 command and `-run RealPlugin -json ./...` in the app clone, 0 fail 0 skip, counts | NOT DONE | Same refusal as row 1. Not run, no counts. |
| 4. `go test -count=1 ./...` | HOLDS, exit code not captured | Run as `go test -count=1 ./... 2>&1 \| grep -v "^ok\|no test files"`: no output, so no FAIL line and every package ok. The pipe hid go's own exit code. |
| 4. `make lint`, `GOOS=windows go vet ./...`, `GOOS=darwin go vet ./...` | HOLDS | One chained call, `gofmt -w <file> && make lint && GOOS=windows go vet ./... && GOOS=darwin go vet ./... && echo VETS_EXIT_0`: printed `VETS_EXIT_0`, so all three exited 0. (An earlier `make lint` exit 2 named gofmt on internal/cli/plan_digest_roundtrip_test.go; fixed with gofmt.) |
| 4. testdata/contract/plan-digest/transcript.json | HOLDS | `git checkout origin/main -- testdata/contract/plan-digest/transcript.json`: the file is identical to main again, so all 104 lane changes are reverted. Each reverted line: the nine `receipt_keys` (from_state, note, plan_digest, plan_hash, task, to_state, what, when, who; eight without note in the later scenario) replace the lane's Job-result keys (answer, job, plan_digest, plan_hash, record, when, who); the legacy-reapprove scenario's `[plan_hash, task, what]` replaces `[]`; the first scenario's render step goes back to `exit` 2 (the lane's 1 was not about the receipt; `TestPlanDigestRoundTrip` does not assert the render exit). |
| New tests fail on base, pass on the lane | NOT DONE for base | The new test passes on the lane. I could not show it failing on base: `git worktree add dist/base origin/lane-M-plugin-held-f-e-ready2 && cp ... && go test` was refused by the shell. On base `Approve` writes no approval.json (8ec0c09f), so `readApproveReceipt` fails at its first `os.ReadFile`. |
| Existing tests not edited | NOT MET, reasoned below | The behaviour these tests asserted (approval.json absent, deleted, not rewritten) is the behaviour the fix round reverses. I changed only the assertion; see "Existing tests changed". None deleted. |
| No guard, gate, cap, hold or blocking hook added | HOLDS | Nothing added; the change writes a file on approve and keeps a file on migration. |
| Tests of changed packages and importers | HOLDS | The whole-module run above covers internal/task, internal/migrate, internal/cli, internal/orchestrator and their importers. |
| Small commits, one-sentence messages, no attribution lines | HOLDS | Commit "Write the nine-key approval.json again on approve and reopen, keep it through the receipt migration, and restore the transcript's receipt keys." and a second one for the test flips and this doc. No Co-Authored-By or Claude-Session line. |
| UI rows | NOT APPLICABLE | None in the brief. |

## What changed

- internal/task/gates.go: `Approve` marshals the event and writes it as approval.json with the history line, as before 8ec0c09f.
- internal/task/write.go: `approvalFile` is back; `change` overwrites an existing approval.json with the reopen event; `recoverWrite` replays approval.json and drops only the old per-gate files (approval-ship.json, approval-preview.json).
- internal/migrate/receipt.go: `ReceiptToJob` keeps the file (moved or beside an existing Job); `ReceiptCurrent` replaces `ReceiptRetired`; `ReceiptPending` is true only for a plan receipt in a task with no approve person Job.
- testdata/contract/plan-digest/transcript.json: identical to main.
- internal/cli/approve_receipt_file_test.go: the new test and the helpers `readApproveReceipt` and `requireApproveReceipt`.

## Existing tests changed (assertion reversed, none deleted)

Edited by two throwaway programs (dist/rewrite, dist/rewrite2, not committed) that did exact-text replacements; the fix round's option B is the written reason for each.

- internal/cli/plan_digest_roundtrip_test.go: `jobResult` reads approval.json again (the nine keys), as on main; the Job-result key lookup and the `taskstore` import are gone.
- internal/cli/approve_job_test.go (2 sites), approve_no_gate_test.go, approve_reader_test.go (2 sites), delivery_test.go, approve_lock_test.go: "approval.json must not exist" became "approval.json is the approve receipt" (`requireApproveReceipt`) or, for a refused approve, "the file is still the first approval's".
- internal/cli/approve_record_test.go: helper `arNoReceipt` became `arReceiptStays`; the worker-pass test waits for the approve Job instead of for the file to vanish; the refused-caller test keeps its original "no file" assertion.
- internal/task/approve_job_result_test.go (2 sites): Approve rewrites a stale file; reopen overwrites it with the reopen event. internal/task/gates_test.go (symlink/old receipt): Approve now refuses to write through a symlinked approval.json (as on main) instead of succeeding.
- internal/migrate/receipt_test.go, receipt_person_test.go, internal/orchestrator/stream_receipt_test.go: the file stays after the move; the second run returns `ReceiptCurrent`.

## Open items

- Rows 1 and 3 are not run: the app tests need `GRID_PLUGIN_BIN=` in the command, which this shell refuses. The orchestrator runs them at landing. If a flipped assertion hid a real gap, those runs show it.
- The new test is not shown failing on base.
- docs/lanes/E-receipts-e2e3.md and E-verdict-12-receipts.md still describe the file as deleted; they are history and I did not rewrite them.
- Option B reverses the verdict-12 direction in the lane docs; the fix round says to do it. If the owner keeps verdict 12, drop this lane and fix the app (option A).

## NOT VERIFIED on a live screen

Nothing here touches a screen. The app's approve button and task card are the parent lane's live rows.

NOT DONE

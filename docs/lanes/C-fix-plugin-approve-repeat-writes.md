# C-fix-plugin-approve-repeat-writes: a repeat `grid approve` of the same plan writes nothing

Base: e26c0591 (origin/lane-M-plugin-held-f-e-receipts). Tests commit a2a000ab fails on the base; the fix commit follows it.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. New Go test reproduces on the base | HOLDS | `internal/cli/approve_repeat_test.go` `TestApproveRepeatByAPersonWithoutLeaseWritesAndPostsNothing` and `internal/task/approve_repeat_open_job_test.go` `TestRepeatApproveWithTheJobAnswerPendingWritesNothing`. On base e26c0591, `go test -count=1 -run 'TestRepeatApprove\|TestApproveRepeat' ./internal/task ./internal/cli` exit 1: `approve_repeat_test.go:46: repeat approve wrote: approval.json first: {..."when":"2026-11-11T06:37:17.060Z"...} repeat: {..."when":"2026-11-11T06:37:17.078Z"...}` and `approve_repeat_open_job_test.go:66: repeat approve with token "18c137" changed history: 2 approve lines, 2 lines before and 3 after`. The file `internal/task/approve_repeat_test.go` named in the brief already exists on the base (`TestApproveRepeatWritesNothing`, `TestApproveRepeatWithNewLandsHashRewrites`) and I left it unchanged; the task-level tests are in the new `approve_repeat_open_job_test.go`. |
| 2. Cause with file:line | HOLDS | A person with no lease (the app's `grid approve` shell, `GRID_MEMBER` and no `GRID_LEASE_EPOCH`) does not post the accepting control: `internal/orchestrator/person.go:181-187` posts only an `answer` record that waits for the holder's pass. `planJobResult` reads only an accepted `control` (`internal/task/gates.go` at base :271, `r.Type != "control" ... j.Status != contract.Accepted`), so after the first approve it returns `ErrPlanJobOpen` (base :283). `Approve` tolerates that error (base :57) but its repeat test needs `jobErr == nil` (base :78), so the repeat fell through to `commitWrite` with `approvalFile` (base :93): new history line and a new `when` in approval.json. Then `recordApproval` (`internal/cli/approve_record.go` at base :53-64) only skipped an already accepted Job, so it posted a second answer. The read that fails is the Job result (open, no accepting control); the field that differs is `when`, written fresh by the second `commitWrite`. |
| 3. After the fix: approval.json byte-identical, one approve line, one Feed answer | HOLDS | `go test -count=1 -run 'TestRepeatApprove\|TestApproveRepeat' ./internal/task ./internal/cli` exit 0. The cli test asserts `bytes.Equal` of approval.json, equal `--json` output of both presses, one `"what":"approve"` line in history.md, one answer in the Feed (`arAnswers`). The fix: `internal/task/gates.go:63-68` (read approval.json through `keptApproval`, :117-129, when the Job answer is pending), `:92-95` (return it and write nothing when hash, digest and lands hash match), `internal/cli/approve_record.go:42-58` and `:83-87` (no second answer when the Feed already holds one for this plan, digest and lands hash). The receipt keys and the Feed record shape are unchanged. |
| 4. A changed plan still approves again | HOLDS | Existing `TestApproveCompareAndSetReadsTheJobResult` (`internal/task/approve_job_result_test.go:36`, explicit reapproval of planV2, Job result carries the new digest) and new `TestRepeatApproveWithTheJobAnswerPendingStillApprovesAChange` (task package; a new lands hash and a new plan each write a new history line and a new approval.json). `TestRepeatApproveAfterReopenWithTheJobAnswerPendingApprovesAgain`: a reopen overwrites approval.json with `"what":"reopen"` (`write.go:141-146`), `keptApproval` rejects it and the next approve writes. All pass: exit 0. |
| 5a. `go test -count=1 ./...` | HOLDS | exit 0; run as `go test -count=1 ./... 2>&1 \| grep -v -E '^(ok\|\?) '`, which printed nothing: no FAIL line and no non-ok package. |
| 5b. `make lint` | HOLDS | exit 0 (the target is `go vet ./...`; no output). |
| 5c. `GOOS=windows go vet ./...` | HOLDS | exit 0. |
| 5d. `GOOS=darwin go vet ./...` | HOLDS | exit 0. |
| 5e. Goldens / testdata/contract | HOLDS | No golden or testdata/contract file changed (`git diff --stat` shows only gates.go, approve_record.go and the two new test files). |
| Packages that import the changed ones | HOLDS | `go list` of importers of internal/task and internal/cli: cmd/grid, contract/formats, internal/braincmd, internal/cli, internal/golden, internal/guard, internal/hook, internal/job, internal/job/jobcmd, internal/job/jobtest, internal/land, internal/mcp, internal/migrate, internal/orchestrator, internal/packaging, internal/question, internal/settings, internal/signal, internal/task, internal/workflow, lib. All covered by the exit-0 `go test -count=1 ./...` above. |
| No test edited, none deleted | HOLDS | `git diff e26c0591 --stat` lists no existing `_test.go` file. (I overwrote `internal/task/approve_repeat_test.go` once by mistake and restored it with `git checkout`; the file is unchanged.) |
| No guard, gate, cap, hold or blocking hook added | HOLDS | The change only makes a repeat return the approval it already has; no command refuses anything new. |
| Commits: one sentence, no attribution lines, no personal paths | HOLDS | `git log e26c0591..HEAD --format=%B`. |
| Nothing in the app repo; no change to the receipt's keys or the Feed record shape | HOLDS | Only internal/task and internal/cli (plus tests and this file) changed. |

## What changed

- `internal/task/gates.go`: `Approve` reads approval.json (`keptApproval`) when the plan Job's answer is still pending (`ErrPlanJobOpen`), and a repeat whose plan hash, digest and lands hash equal it returns that receipt and writes nothing. A receipt that is not an approve receipt of this task (a reopen overwrites it), or has no full digest or names a gate, is not used.
- `internal/cli/approve_record.go`: `recordApproval` posts no second answer when the Feed already holds an answer to the plan Job with the same plan_hash, plan_digest and lands_hash. If the first press wrote the receipt but failed before posting its answer, the repeat still posts it (the Feed is checked, not the file).
- Tests: `internal/task/approve_repeat_open_job_test.go`, `internal/cli/approve_repeat_test.go`.

## Open items

- Only the pending-answer case (`ErrPlanJobOpen`) falls back to approval.json. A task with no plan Job at all (`ErrNoPlanJob`) still writes on a repeat, as before; `grid approve` first moves an old receipt into a Job (`migrate.ReceiptToJob`) so it does not meet that case.
- The app test `TestRealPluginApprovePlanOnce` lives in the app repo; I could not run it here. The cli test reproduces its sequence (person with no lease, task.md put back to waiting with its ulid line, same --plan-hash). Run it at landing.

## NOT VERIFIED on a live screen

Nothing in this lane is UI. The app's real-plugin test above needs the app repo.

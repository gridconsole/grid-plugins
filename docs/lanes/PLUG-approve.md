# PLUG-approve: one approval per digest, decided under the lock

Lane PLUG-approve on main 9dc5d049. Packages touched: internal/orchestrator, internal/cli.

## What changed

- `orchestrator.WriteUnless` is `Write` with a `done()` check made under `.orchestrator.lock`, in the same pass as the write (`Write` is `WriteUnless` with no check). `personApproval` uses it: the "already accepted with this digest" test (`ApprovalFromJob` matching job and digest) now runs inside the lock. The earlier `Load` before the lock is gone (row 1).
- `orchestrator.CheckPersonAnswer` (the 3500-character cap) and `orchestrator.PersonOpen` (is a person Job, not closed, its `after` Jobs accepted) are cut out of `AnswerPerson` with the same messages. `AnswerPerson` calls them; `runPersonApproval` calls them before `Approve`, so a refused answer writes neither `approval.json` nor a control (row 4).
- `Env.afterApprove` is an unexported test seam, called after `Approve` returns and before the post, in `grid approve` and in `runPersonApproval`. `approve_lock_test.go` parks the command there on channels instead of polling inside 5 s and 10 s windows (row 5). That existing test is rewritten by the brief's direction (row 5); it asserts the same thing: approval.json written, `.task.lock` free while `.orchestrator.lock` is held, result posted once released.
- Rows 2 and 3 needed no production change: `planJobResult` already takes only a Person Job of kind approve whose fold status is accepted, and already reads the Feed with `store.ReadOnly`/`FoldReadAt`, so `ApprovalFromJob` writes no `state.json` and no reader cache. Two tests now pin both; they pass on base too, so they did not fail first.

## New tests (all in internal/cli/approve_digest_test.go)

On base (production code unchanged), `go test ./internal/cli -run '<the four> ' -count=20`:

```
--- FAIL: TestPersonApprovalSimultaneousPressesPostOneControl
    approve_digest_test.go:65: two presses posted 2 controls, want 1
--- FAIL: TestRunPersonApprovalAnswerLengthAndPersonJob/answer_over_the_cap
    approve_digest_test.go:153: 0
--- FAIL: TestRunPersonApprovalAnswerLengthAndPersonJob/worker_Job
    approve_digest_test.go:153: 0
--- FAIL: TestRunPersonApprovalAnswerLengthAndPersonJob/unknown_Job
    approve_digest_test.go:153: 1 grid: unknown Job J-9
```

Each failed in all 20 runs. On the lane the same command passes (5 tests x 20 = 100 PASS lines with the lock test).

| Row | Test | Base | Lane |
| --- | --- | --- | --- |
| 1 | TestPersonApprovalSimultaneousPressesPostOneControl | FAIL 20/20 | pass, -count=20 |
| 2 | TestApprovalFromJobIgnoresAcceptedPersonJobOfAnotherKind | pass (already held) | pass |
| 3 | TestApprovalFromJobWritesNoState | pass (already held) | pass |
| 4 | TestRunPersonApprovalAnswerLengthAndPersonJob (cap, worker Job, unknown Job) | FAIL 20/20 | pass |
| 5 | TestApprovalPostDoesNotHoldTaskLockWhileWaitingForOrchestratorLock | n/a (rewritten) | pass, -count=20 |

## Checks

- `go build ./...`: 0. `GOOS=windows go build ./...`: 0. `GOOS=darwin CGO_ENABLED=0 go build ./...`: 0.
- `gofmt -l internal cmd`: empty. `go vet ./internal/cli ./internal/orchestrator`: 0.
- Packages that import the changed ones (`go list` over Deps): cmd/grid, internal/cli.
- `go test ./internal/cli ./internal/orchestrator ./internal/task -count=1`: ok, ok, ok.
- `go test ./... -count=1` (row 6): no FAIL, no non-ok line; its exit status was not captured because the output was filtered through grep.
- The five tests above with `-count=20`: ok.

## NOT VERIFIED

- `make lint` was not run (not allowed to the lane); the orchestrator runs it.
- No UI changed.
- Row 1 covers two presses in one process; the lock is a flock on a separate open file, which is the same exclusion two processes get, but a two-process run was not made.
- `ApprovalFromJob` inside the lock still compares with the current plan.md, so a press for a digest that plan.md has since left finds "changed" and posts; unchanged from before.

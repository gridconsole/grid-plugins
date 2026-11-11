# r0-fix-cancelled-dep

Brief: a Job that waits on a cancelled Job says so, and a Job agent's `grid job cancel` works.
Base: main `13b11fe1` (contains the brief's `55c46cbd`; `git merge-base --is-ancestor 55c46cbd main` exits 0).
Code head: `ae5df753` (the proof table commit comes after it).

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. TestReadyReasonForCancelledDependency fails on main, passes on the lane | HOLDS | Lane: `go test ./internal/jobgraph/... -run CancelledDependency` exit 0. Base (the three source files restored to `13b11fe1`, same tests): exit 1, `cancelled_dependency_test.go:27: edge "done", exhausted false: got ready [] waiting [{Job:J-3 Target:J-2 Kind:done Reason:waiting for J-2 to be accepted}], want waiting [{Job:J-3 Target:J-2 Kind:done Reason:waiting for J-2, which was cancelled}]`. Test: internal/jobgraph/cancelled_dependency_test.go:13 (done and bare edge, Exhausted true and false). |
| 2. Failed edge to a Cancelled and Exhausted Job still satisfied | HOLDS | Existing: internal/jobgraph/graph_test.go:189-190 ("failed cancelled", "cancelled with a retry left"), pass. Added anyway in internal/jobgraph/cancelled_dependency_test.go:57 (TestFailedEdgeStillSatisfiedByCancelledExhausted): Cancelled + Exhausted is ready, Cancelled with an attempt left waits with "to fail with no attempt left". Exit 0 on the lane and on base (it pins unchanged behaviour). |
| 3. Held-Job wording | HOLDS | internal/orchestrator/cancelled_dependency_test.go:43 (TestHeldJobReasonForCancelledOrRejectedTarget) goes through `orchestrator.Ready` with J-3 held (its second target J-9 is not posted): cancelled -> "waiting for J-2, which was cancelled", rejected -> "..., which was rejected", posted/running -> "to be accepted", J-9 keeps "which is not posted". Lane exit 0. Base exit 1: `cancelled_dependency_test.go:55: J-2 cancelled: reason "waiting for J-2 to be accepted", want "waiting for J-2, which was cancelled"`. Source: internal/orchestrator/ready.go:224 now calls `jobgraph.DoneReason`. |
| 4. TestJobCancelUsesTaskEpochForJobAgent | HOLDS | internal/cli/job_cancel_task_epoch_test.go:39. Env GRID_LEASE=J-1, GRID_LEASE_EPOCH=<J-1 epoch>, GRID_TASK_EPOCH=<task epoch>: `job cancel --task <t> --job J-3 --reason x` exits 0, J-3 is cancelled, and the control record carries lease `task` at the task fence. Lane exit 0. Base exit 1: `job_cancel_task_epoch_test.go:46: job cancel by a Job agent with GRID_TASK_EPOCH: exit 3 stale lease`. |
| 4b. Typed `--lease-epoch` wins | HOLDS | TestJobCancelTypedLeaseEpochBeatsTaskEpoch (:60): a stale typed epoch (global before the command, and after it) is used and refused with exit 3, J-3 stays not cancelled; a current typed epoch (both places) over a stale GRID_TASK_EPOCH exits 0; with no flag a stale GRID_TASK_EPOCH is refused (job post). Lane exit 0. Base exit 1: `:84: current global --lease-epoch 0.3 over stale GRID_TASK_EPOCH 0.1: exit 3 stale lease`. |
| 4c. `post --type control` still sends the Job epoch | HOLDS | TestJobPostControlStillSendsJobEpoch (:104): with the same env (task epoch current), `job post J-1 --type control` exits 3, as on main (the Job epoch is sent; were GRID_TASK_EPOCH used it would be accepted); with `--lease task --lease-epoch <task epoch>` typed it exits 0 and the record is lease `task` at the task fence. Lane exit 0. Passes on base too (unchanged behaviour). Note the brief says `--kind control`; the flag is `--type`. |
| 5a. `git diff --stat main` names only the Touches files | HOLDS | `git diff --stat main` before this report: internal/cli/job_cancel_task_epoch_test.go, internal/cli/orchestrator.go, internal/jobgraph/cancelled_dependency_test.go, internal/jobgraph/graph.go, internal/orchestrator/cancelled_dependency_test.go, internal/orchestrator/ready.go. The only other file is this report, docs/lanes/r0-fix-cancelled-dep.md, which the runbook asks for. |
| 5b. events.go untouched | HOLDS | `git diff --stat main -- internal/orchestrator/events.go` prints nothing, exit 0. |
| 5c. Tests of changed packages and importers | HOLDS | `go test -count=1 ./internal/jobgraph/... ./internal/orchestrator/... ./internal/cli/... ./cmd/grid/... ./contract/formats/... ./internal/golden/... ./internal/mcp/... ./internal/packaging/... ./internal/question/... ./internal/settings/... ./lib/...` exit 0, 0 fail. Importer list from `go list -f '{{.ImportPath}} {{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./...` filtered for internal/jobgraph, internal/orchestrator, internal/cli: cmd/grid, contract/formats, internal/cli, internal/golden, internal/jobgraph, internal/mcp, internal/orchestrator, internal/packaging, internal/question, internal/settings, lib. |
| 5d. gofmt, lint | HOLDS | `gofmt -l internal` prints nothing, exit 0. `make lint` (go vet ./...) exit 0. |
| 5e. Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0. `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| Rule: new tests fail on base, pass on lane | HOLDS | Rows 1, 3, 4, 4b above (failing lines pasted). Rows 2 and 4c pin unchanged behaviour and pass on both. |
| Rule: no existing test edited or deleted | HOLDS | `git diff --stat main` shows only new test files. |
| Rule: no guard, gate, cap, hold, blocking hook, automatic cancel or new wake | HOLDS | Only a reason string and an epoch choice changed. J-3 stays posted. events.go untouched. |
| Rule: small commits, one sentence, no attribution lines; public repo | HOLDS | `git log main..HEAD`. No personal paths, hosts or emails in the diff. |
| Rule: Windows/macOS-safe, no sudo, nothing outside the clone | HOLDS | Builds above; only the clone and /srv/gridwork/tmp/hand were written. |

## What changed

- internal/jobgraph/graph.go: new exported `DoneReason(target, status)`; Ready's Done case uses it. A Done edge to a Cancelled Job is still unsatisfied, with "waiting for J-2, which was cancelled", the same form as Rejected. The Failed edge case is untouched. `Released` is untouched.
- internal/orchestrator/ready.go:224: a held Job's wait on a posted, unreleased target uses the same `DoneReason`, so cancelled and rejected targets are named.
- internal/cli/orchestrator.go:254-269: the GRID_TASK_EPOCH branch now also runs for `verb == "cancel"`. With GRID_TASK_EPOCH set and no `--lease` typed, `--lease` is cleared so the lease stays `task` (as the verb's default is), and unless `--lease-epoch` was typed (flag or global) the epoch is GRID_TASK_EPOCH. Same rule as `job post --type job`. No role check added. `post --type control`, `ack`, `result` keep today's path.

For the lease reviewer: the cancel record now carries lease `task` at the task fence (asserted in TestJobCancelUsesTaskEpochForJobAgent). A side effect of reusing the post rule: a Job agent that types only `--lease-epoch` (no `--lease`) on `cancel` now gets lease `task` with the typed epoch; on main the lease stayed GRID_LEASE (the Job) and the write was refused. Typing both `--lease` and `--lease-epoch` is unchanged.

## NOT VERIFIED on a live screen

No UI row. Not run live: a real worker handing a real agent GRID_TASK_EPOCH, and a real orchestrator reacting to the new reason. The proof is on a fixture home with the same lease and epoch rules the worker uses (`newAgentTask`, as for `job post`).

## Open items

None inside the brief. Still open, as the brief says: who re-plans after a cancel when no orchestrator Job is running (INDEX.md); no wake for a cancel was added.

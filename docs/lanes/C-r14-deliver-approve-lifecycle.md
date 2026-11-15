# C-r14-deliver-approve-lifecycle

Lane head: the merge of origin/main (d04bdf1f) onto 5343d045. The doc commit follows it.

## Decisions

| # | Decision | Why |
|---|----------|-----|
| D1 (F1) | Rebind, not withdraw. The ship (approve) person Job stays bound to the task's delivery. It is not ready, and `grid approve` answers `job_waiting` (exit 3), while any started deliver Job of the task is not accepted. A rejected deliver Job counts as done only once its reworks are accepted (jobgraph `reworkDone`). A ship Job that is accepted anyway, for example an app answer folded by the holder's tick, lands nothing. The land step puts it back to `posted` with the reason "the delivery is not accepted: ...". | The plugin cannot see the Judge's rejection when it happens (the Judge is in the app), so the check reads the Feed when the person approves and when land runs. Withdrawing would make the person answer a second Job for the same delivery. The rework's `grid deliver` finds the open ship Job, posts no second one and rewrites task.md, so the one ship Job approves the rework's result. The same check also covers the time before any verdict: a running or submitted deliver Job keeps the ship Job waiting. |
| D2 (F2) | A deliver Job is the non-person Job titled "Deliver the task" (skills/plan-jobs/SKILL.md:47). In `jobgraph.Ready` it also waits for every person Job of kind `ask` (the question Job) that is not accepted, cancelled or rejected. | This is a readiness wait on a Job that has not started, the same kind of wait as a dependency edge. It never stops a running agent, so it is not a guard or hook on agents. |
| D3 (F6) | `grid deliver` writes `jobs/<id>/brief.md` for the ship Job it posts: the Job metadata, plus the sections saying the person approves the delivered work. If the write fails, it says so on stderr and the delivery still goes on. | A person Job is never started or rendered, so nothing else would make its folder. `grid job render` refuses person Job bodies (its lint reads `intent.owner` as a pointer). |

## What changed

- `internal/jobgraph/deliver.go` (new): `IsDeliver` (:22), `questionWaits` (:36) and `DeliveryWait` (:55).
- `internal/jobgraph/graph.go:382`: `Ready` adds the question waits for a deliver Job.
- `internal/cli/deliver_person_job.go`: `isShipJobID` (:66), `shipWait` (:81), `renderShipJob` (:100) and `saveShipBrief` (:106).
- `internal/cli/deliver.go:113`: deliver writes the ship Job's folder after it posts the Job.
- `internal/cli/orchestrator.go:511` and `internal/cli/task_write.go:327`: both approve paths refuse a ship Job while `shipWait` waits.
- `internal/cli/ship_land.go:68`: `landAfterShipAs` lands nothing while `shipWait` waits. `reopenShipWaiting` (:116) puts the ship Job back to `posted`. It leaves the land once-guard (`store.Reopened`) untouched, because no land ran.
- Tests:
  - `internal/cli/r14_deliver_approve_lifecycle_test.go`: `TestR14ApproveWaitsForTheDeliverRework`, `TestR14AcceptedShipJobDoesNotLandARejectedDelivery` and `TestR14ShipJobShows`.
  - `internal/jobgraph/r14_deliver_question_test.go`: `TestR14DeliverWaitsForAnOpenQuestion`.
- No existing test was edited.

## Proof

| Row | Result | Proof |
|-----|--------|-------|
| F1: deliver, approve Job posted, review rejects, approve refused, landing only after the rework is accepted | HOLDS | `TestR14ApproveWaitsForTheDeliverRework`: J-2 is rejected and its rework J-3 is open. `grid approve T-01` exits 3 with J-3 on stderr, main does not move, and the task stays waiting. Once J-3 is accepted, approve exits 0, main is at the lane sha and the task is closed. `go test -count=1 ./internal/cli/...` exit 0. |
| F1: a ship Job accepted outside grid approve does not land a rejected delivery | HOLDS | `TestR14AcceptedShipJobDoesNotLandARejectedDelivery`: main does not move and the ship Job is `posted` again. Exit 0 on the lane. |
| F2: a deliver Job does not start while a person question (posted by another Job) is open | HOLDS | `TestR14DeliverWaitsForAnOpenQuestion`: while question J-6 is open, J-3 is not ready, a wait J-3 to J-6 is listed, and J-5 (not a deliver Job) is ready. Once J-6 is accepted, J-3 is ready. `go test -count=1 ./internal/jobgraph/...` exit 0. |
| F6: an approve person Job has its jobs/<id> folder, and `grid job show` works | HOLDS | `TestR14ShipJobShows`: `jobs/<ship>/brief.md` exists after `grid deliver`, and `grid job show <dir> <ship>` exits 0. |
| Tests fail on base | HOLDS | With the production files at origin/main and the new tests in place, `go test -count=1 -run TestR14 ./internal/cli/ ./internal/jobgraph/` exits 1. Output below. |
| Brief checks: `go test ./internal/cli/... ./internal/jobgraph/... ./internal/approvefix/...` | HOLDS | Exit 0 on the merged head: cli ok 34.2s, jobgraph ok, approvefix has no test files. |
| Importers (`go list`: cmd/grid, internal/cli, internal/orchestrator, lib) | HOLDS | `go test -count=1 ./internal/orchestrator/... ./cmd/grid/... ./lib/...` exit 0: orchestrator ok 84.5s, lib ok, cmd/grid has no test files. |
| `make lint` | HOLDS | Exit 0 (`go vet ./...`). |
| Windows and macOS builds | HOLDS | `GOOS=windows go build ./...` exit 0, and `GOOS=darwin go build ./...` exit 0. |
| Walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0, and `go test -count=1 ./internal/job/lint/` exit 0, both after the merge. |
| Merge of origin/main before the last proof run | HOLDS | `git fetch origin && git merge origin/main` merged cleanly at d04bdf1f. Every row above was run after it. |
| No guard, cap or limit on agents | HOLDS | D2 is a readiness wait on a deliver Job that has not started. D1 is a refusal of the person's approve, plus a land that does not run. Nothing stops a running agent. |
| Do not touch the app | HOLDS | Only internal/cli, internal/jobgraph and this doc changed (`git diff --stat origin/main...HEAD`). |
| push, curl, sudo | not run, the brief forbids it | |
| A live screen or a live worker tick | not run, not asked | |

Fail on base (the production files checked out from origin/main, with the new test files in place):

```
--- FAIL: TestR14ApproveWaitsForTheDeliverRework (0.08s)
    r14_deliver_approve_lifecycle_test.go:55: approve of J-1 while the rework of the rejected delivery is open: exit 0, want 3; stderr
--- FAIL: TestR14AcceptedShipJobDoesNotLandARejectedDelivery (0.05s)
    r14_deliver_approve_lifecycle_test.go:99: ship Job J-1 is accepted after a land that waits, want posted (back with its person); stderr land: T-01 after J-1: land refused: plan has not been approved: the plan Job J-1 has no result yet; run grid approve to post it
--- FAIL: TestR14ShipJobShows (0.02s)
    r14_deliver_approve_lifecycle_test.go:112: ship Job J-2 has no brief: stat .../jobs/J-2/brief.md: no such file or directory (stderr )
    r14_deliver_approve_lifecycle_test.go:117: grid job show J-2: exit 1  job: lstat .../jobs: no such file or directory
FAIL	github.com/gridconsole/grid-plugins/internal/cli	0.162s
--- FAIL: TestR14DeliverWaitsForAnOpenQuestion (0.00s)
    r14_deliver_question_test.go:38: deliver J-3 is ready while question J-6 is open: ready [J-3 J-5 J-6], waiting []
FAIL	github.com/gridconsole/grid-plugins/internal/jobgraph	0.005s
```

On the lane, the same command exits 0, and so does the full run in the rows above.

## Open items and NOT VERIFIED

- On base, `TestR14AcceptedShipJobDoesNotLandARejectedDelivery` fails on the ship Job's status. Main does not move there either, because the base land refuses for an unrelated reason: the fixture has no plan answer. So that test proves that the ship Job is put back. The proof that landing waits for the rework is `TestR14ApproveWaitsForTheDeliverRework`.
- F2 sees only questions that have a person Job (kind `ask`). A question with no person Job behind it is not seen.
- The app answer path, where the holder's tick folds an accepted ship Job, is covered only by the direct `landAfterShip` call in the test. It is NOT VERIFIED on a live worker tick.
- A deliver Job is found by its title "Deliver the task". A deliver Job with a different title is not covered.
- A live screen is NOT VERIFIED.

Verdict: DONE

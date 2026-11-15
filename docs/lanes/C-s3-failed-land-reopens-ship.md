# C-s3-failed-land-reopens-ship: a failed land reopens the ship Job with the failure text; a second approve retries

Base: main 842493ba. `git fetch origin && git merge origin/main` (brought in C-s2-ship-answer-doc-line) before the last proof run. Plugin only.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. New test fails on main, passes on the lane | HOLDS | `TestS3FailedLandReopensShipJob` (internal/cli/s3_failed_land_reopens_test.go). Base: main's production files checked out (`git checkout origin/main -- <the 6 prod files>`), the test kept; `go test -count=1 -run TestS3 ./internal/cli/` exit 1, lines below. Lane: same run `--- PASS: TestS3FailedLandReopensShipJob`, `ok`, exit 0. |
| 2. `make lint`, internal/land and internal/cli package tests pass | HOLDS | `make lint` exit 0 (`go vet ./...`). `go test -count=1 ./internal/land/ ./internal/cli/ ./internal/orchestrator/ ./internal/mcp/ ./internal/migrate/ ./internal/job/lint/ ./cmd/...` exit 0: land 0.5s, cli 34.8s, orchestrator 85.4s, mcp 0.2s, migrate 2.7s, job/lint 0.1s. |
| 3. This lane doc | HOLDS | docs/lanes/C-s3-failed-land-reopens-ship.md |
| Do 2: failed `grid land` → ship Job back to waiting for its person, failure text (exit code, first stderr lines) in its note | HOLDS | ship_land.go `reopenShipJob`: a control `status: posted`, `reason: "land failed: grid land exited 7: boom remote refused"`. The Job reads Not started, as a fresh ship Job does. The test asserts status posted and the reason text (s3 test :28-35). |
| Do 2: a second approve runs land again; success records as today | HOLDS | s3 test :46-60: after press 2 there are two `land started` lines, `land landed`, main at the delivered sha, the task closed and one ship Job. |
| Do 2: no automatic retry | HOLDS (leased path); worker path by code only | s3 test :42-45: a second `landAfterShip` before the person's answer runs no land. Worker pass: `personAnswers` and `answered()` skip answers at or before the reopen (`orchestrator.ReopenedAt`), so the old answer does not re-accept the Job on the next tick. No test drives a worker pass after a failed land (see open items). |
| Do 2: C-r7 "no deploy topic" unchanged | HOLDS | `TestR7*` in `go test ./internal/land/` exit 0, unedited. |
| Do 3: fake land fails once, then succeeds | HOLDS | Deploy command `test -f once \|\| { touch once && echo boom remote refused >&2 && exit 7 ; } && git merge -q --ff-only lane`. |
| Tests of changed packages and importers | HOLDS | `go list` importers of internal/land and internal/orchestrator: internal/cli, internal/mcp, internal/migrate (plus cmd/grid, no tests). All in row 2. |
| Walk tests (plugin) | HOLDS | TestHsIpWalksEveryCommand and TestIlGuestSiteList ran inside `go test -count=1 ./internal/cli/` exit 0, after the merge. `go test -count=1 ./internal/job/lint/` exit 0. |
| Windows and macOS builds | HOLDS | `GOOS=windows go build ./...`, `GOOS=darwin go build ./...` and `GOOS=windows go vet ./internal/land/ ./internal/cli/ ./internal/orchestrator/` exit 0. The new test is `//go:build !windows` (sh syntax). |
| No guard, gate, cap, hold or blocking hook | HOLDS | One land per approve. The once-marker is re-armed only by the reopen. No limit or count was added. |
| Deleted test | HOLDS | `TestShipLandFailurePostsNoSecondJob` (ship_land_test.go) pinned "two presses after a failed land run land once". The owner's default for board q-failed-land-retry, written in this lane's brief (end review 0212Z S3), removed that behaviour: a second approve runs land again. Its "no second ship Job" assertion is now in TestS3FailedLandReopensShipJob :24 and :61. |
| No attribution lines, no personal paths | HOLDS | `git log origin/main..HEAD` |
| Reviewer runbook read | HOLDS | /srv/gridwork/tmp/hand/runbook-reviewer.txt read |
| Budget | over | About 75 calls against 60. |
| Whole-module test, cross-repo set, realplugin tables | not run, not asked | Run by the orchestrator at landing. The realplugin tables are an app-repo check. |

Red on base (row 1):

```
--- FAIL: TestS3FailedLandReopensShipJob (0.06s)
    s3_failed_land_reopens_test.go:29: ship Job J-1 is &{ID:J-1 ... Kind:approve Repo: Status:accepted ...} after a failed land, want posted again (waiting for its person)
FAIL	github.com/gridconsole/grid-plugins/internal/cli	0.068s
```

## What changed

- internal/land/ship.go
  - When the deploy command ran and failed or timed out, AfterShip returns `*FailedError`. It unwraps to the land error, keeps its text, and carries `Text`: "grid land exited N: <first 5 stderr lines>".
  - New `Store.Reopened` writes `ship answered · J-N reopened after a failed land; the next approve runs grid land again` under .land.lock.
  - `markShip` now re-arms after that line, so the next acceptance of the same Job runs land once.
  - A land that stops before its command runs keeps its old behaviour: a `land failed` line, the Job stays accepted. `TestApproveAnswersShipPersonJob` (unedited) pins that.
- internal/cli/ship_land.go: `landAfterShipAs` takes the writer; `landAfterShip` uses `approvalWriter(env)`.
  - `reopenShipJob` posts the control under the task lease, refolds, and only then calls `Reopened`. If the post fails, the guard stays set and nothing lands again on its own.
- internal/cli/approve_record.go `answered()`, internal/orchestrator/person.go `personAnswers` + new `ReopenedAt`: an answer from before the reopen does not count again. **Outside the brief's two named packages:** without it, the holder's next tick would re-accept the Job from the old answer and land again on every pass.
- internal/orchestrator/stream.go: `Ticked func(dir string, w feed.Writer)`, so the worker pass can reopen under its task lease. internal/cli/worker.go passes it on.
- internal/cli/s3_failed_land_reopens_test.go (new); ship_land_test.go (one test deleted, see above).

## Open items

- No test drives the unleased answer → worker pass → failed land → reopen → new answer → worker pass loop. The leased path is tested; the worker path shares `landAfterShipAs` and the `ReopenedAt` filter, but its own trigger is not proven.
- "Needs you" on the app screen: the reopened Job has status posted and the task stays waiting, the same shape as a fresh ship Job. Whether the app lists it is NOT VERIFIED.
- A land that stops before its command runs (approval mismatch, malformed topic, no folder on this machine) still leaves the Job accepted, as before. Reopening for those too is an owner question; it would change TestApproveAnswersShipPersonJob.

## NOT VERIFIED

- The task in Needs you after a failed land, on a live app screen.

DONE

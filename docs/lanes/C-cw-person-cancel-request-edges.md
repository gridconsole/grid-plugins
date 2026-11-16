# C-cw-person-cancel-request-edges

Tests only. Two new files; no product code changed. Review: none (no fault found, so the lease path is untouched).

Base: origin/main 6cc26809 (the brief named bacb3f76; main moved on by lane docs only). `git fetch origin && git merge origin/main`: Already up to date.

Base check, quoted on 6cc26809: internal/cli/orchestrator.go:321-327 is
`if verb == "cancel" && *epoch == "" && (...) && contract.TierFor(personRole) == contract.PersonTeam {` /
`held, readErr := (lease.TaskStore{Feed: store}).Read("task")` /
`if readErr == nil && held.Holder != nil && *held.Holder == env.getenv("GRID_MEMBER")+"@"+localMachine(env) {` /
`leaseName, *epoch = "task", ...` / `} else {` / `request, unleased = true, true`.
internal/orchestrator/person.go:172-181 is `reopened := ReopenedAt(snap, j)` ... `case closed:` /
`case r.At <= reopened: note = fmt.Sprintf("%s closed and was reopened at %s", id, reopened)`.
`git grep -n -e "was reopened" -e ReopenedAt -- '*_test.go'` was empty on base, so no row dropped.

## Proof table

| Row | Result |
|---|---|
| 1 Lease read error | HOLDS. `TestCwPcLeaseReadErrorSavesTheRequest` (internal/cli/cw_person_cancel_lease_error_test.go): 2 lease files (invalid JSON; the task lease rewritten as the lease of J-1, same holder, which passes the schema and gives "lease identity mismatch") x 2 members (`worker`, the holder's own name; `dana`). Each case checks Read fails first, then exit 0, r20ClSaved on stderr, exactly one new control, `Lease == nil`, `Fence == 0`, author the person, body status cancelled, J-1 Running, no new leased control. `go test -count=1 -v -run TestCwPc ./internal/cli/ ./internal/orchestrator/` exit 0. |
| 2 Reopened target | HOLDS. `TestCwPcRequestBeforeReopenIsNotUsed` (internal/orchestrator/cw_person_cancel_reopened_test.go). Closing status: `accepted`. The fold line that takes `posted`: internal/contract/fold.go:251-256 (a leased control's status is set on the Job and the control joins its History; no transition table), so J-1 reads `posted` after the reopen, as the test asserts. One `f.tick(t)` writes exactly one record re the request: a `post` under the task lease, id `personCancelID(req, "unused")`, text holds "closed and was reopened at <reopen control's At>"; no control re the request. J-1 is not Cancelled, Rejected or Accepted. Note: the same pass assigns the reopened Job again, so it reads `assigned` after the tick and not `posted`; the test checks it is not closed. A second tick writes nothing more re the request. |
| 3 New request after reopen | HOLDS. `TestCwPcNewRequestAfterReopenCancels`. No pass between reopen and the new request, so only the `r.At <= reopened` cut-off (person_write.go:63) separates the old request from the new: `PostPersonUnless` returns skipped=false, a new record, one more record on the Feed. The next tick writes one leased control re the new request (J-1 Cancelled); the old request has only its not-used post. |
| 4 Mutation proof | HOLDS. Edit 1: orchestrator.go:323 `readErr == nil &&` -> `readErr != nil \|\|`. Failing: `--- FAIL: TestCwPcLeaseReadErrorSavesTheRequest` with all 4 subtests `--- FAIL`, each `exit 3 stale lease`. Edit 2: person.go:180 `case r.At <= reopened:` -> `case false && r.At <= reopened:`. Failing: `--- FAIL: TestCwPcRequestBeforeReopenIsNotUsed` (got the leased cancel control re the request) and `--- FAIL: TestCwPcNewRequestAfterReopenCancels` (got the "dana asked first" not-used post instead of the leased cancel). Both edits reverted with `git checkout -- <file>`; `git status --short` was empty. |
| 5 Fault found | no fault found. |
| 6 Tests | HOLDS. TestCwPc: `--- PASS` for `TestCwPcLeaseReadErrorSavesTheRequest` (4 subtests), `TestCwPcRequestBeforeReopenIsNotUsed`, `TestCwPcNewRequestAfterReopenCancels`; `ok` for both packages; no FAIL. `go test -count=1 -timeout 40m ./internal/cli/` ok (300.7s). `go test -count=1 -timeout 40m ./internal/orchestrator/` ok (1032.6s). Walk tests `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok. `make lint` exit 0. `make cross` exit 0 (darwin/arm64 and windows/amd64, vet and test compile). |
| Packages importing the changed ones | `go list` of importers of internal/cli or internal/orchestrator: cmd/grid, internal/cli, internal/mcp, internal/migrate, lib. Run: internal/mcp ok, internal/migrate ok, lib ok, cmd/grid no test files, internal/cli ok, internal/orchestrator ok. |
| No guard on agents | HOLDS. No product code changed. |
| New tests fail on base | Both files test existing behaviour that has no test; the failing lines are the mutation runs of row 4 (the branch turned to the other path). |
| Existing tests | None edited or deleted. |
| App `*realplugin*` rig / `make webui-test` | not run, not asked (the change alters no command, refusal or Feed record shape; this is the plugin repo). |
| Mismatch proof of a decoded tag | not run, not asked. |

## What changed

- internal/cli/cw_person_cancel_lease_error_test.go (new): helpers `cwPcLeasePath`, `cwPcInvalidJSON`, `cwPcOtherLease`; test `TestCwPcLeaseReadErrorSavesTheRequest`.
- internal/orchestrator/cw_person_cancel_reopened_test.go (new): helpers `cwPcClock`, `cwPcReopened`; tests `TestCwPcRequestBeforeReopenIsNotUsed`, `TestCwPcNewRequestAfterReopenCancels`.
- `cwPcClock` sets `Feed.Now` to a clock that moves one millisecond per record, so "request after the reopen" is later by the clock and not by the real millisecond (no sleep, no real clock wait).

## Notes

- `internal/orchestrator` tests take about 17 minutes on this loaded VM. The first run with the default 10-minute timeout panicked inside the existing `guCtTicks` (gu_replan_ticks_test.go) lock-contention tests when run in parallel with other packages; the rerun alone with `-timeout 40m` passed. Nothing in this lane touches that test.

## NOT VERIFIED on a live screen

Nothing here is a UI row.

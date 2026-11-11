# L-jobcmd-feedjob-save-tests

Behaviour tests for loading a posted Job for render (`loadJob`, `pickJob`, `identity`, `decodeJob`, `badField`, `problem`) and for `save`. One new file: `internal/job/jobcmd/feedjob_save_test.go`. No change to `feedjob.go`, `save.go` or any existing file.

## Proof table

| Row / rule | Result | Proof |
|---|---|---|
| 1 loadJob: plan J-1 returns plan Job, Feed not read | HOLDS | `TestLoadJobFromPlanDoesNotReadTheFeed`; readFeed stub calls t.Fatal. `go test -count=1 ./internal/job/jobcmd/` exit 0 |
| 1 no plan item + Feed post J-2 returns the posted Job | HOLDS | `TestLoadJobFallsBackToTheFeedWhenThePlanHasNoSuchJob`, same command, exit 0 |
| 1 no plan item + empty Feed matches `job.ErrJobNotInPlan` | HOLDS | `TestLoadJobWithEmptyFeedReturnsThePlanError`, exit 0 |
| 1 Feed read error joined with plan error (both `errors.Is`) | HOLDS | `TestLoadJobJoinsThePlanErrorAndTheFeedReadError`, exit 0 |
| 2 selector "3" finds the Job with `item: 3` | HOLDS | `TestPickJobSelectsByItemNumber`, exit 0 |
| 2 a `stale` record is never chosen | HOLDS | `TestPickJobNeverChoosesAStaleRecord` (by id, by item, and a newer stale post does not displace a counted one), exit 0 |
| 2 body with no `id`, record `job: "J-4"` is J-4 | HOLDS | `TestIdentityTakesTheRecordJobWhenTheBodyHasNoID`, exit 0 |
| 3 `"after": "J-1"` names record id, field `after`, "want ..., got string" | HOLDS | `TestDecodeJobNamesRecordAndFieldForAStringWhereAListIsWanted`, exit 0 |
| 3 untyped bad field names first bad key in sorted order; "(unknown)"; `problem` plain text | HOLDS | `TestBadFieldWithoutATypedPathNamesTheFirstBadKeyInSortedOrder` (after sorts before max_rounds), `TestBadFieldAndProblemFallbacks`, exit 0 |
| 3 decoded Job with no id takes selector id and defaults | HOLDS | `TestDecodeJobFillsTheSelectorIDAndDefaults` (J-4, briefed, none, attempt 1), exit 0 |
| 4 id "bad id" errors "invalid Job id" | HOLDS | `TestSaveRefusesAnInvalidID`, exit 0 |
| 4 two identical saves return the same brief.md path | HOLDS | `TestSaveRepeatedIdenticalRenderReturnsTheSamePath`, exit 0 |
| 4 changed brief errors, first files unchanged | HOLDS | `TestSaveRefusesAChangedRenderAndKeepsTheFirstFiles` (brief.md, inputs.json, job.yaml compared byte for byte), exit 0 |
| 4 no `.render-*` directory after success | HOLDS | `TestSaveLeavesNoTempDirectoryAfterSuccess`, exit 0 |
| 5 `go test -count=1 ./internal/job/jobcmd/` 0 fail | HOLDS | `ok ... jobcmd 1.736s`, exit 0 |
| Packages importing jobcmd tested | HOLDS | `go list` importers: `internal/cli`, `internal/job/jobcmd`. `go test -count=1 ./internal/job/jobcmd/ ./internal/cli/` exit 0 |
| Lint | HOLDS | `gofmt -l internal/job/jobcmd` empty; `go vet ./internal/job/jobcmd/` exit 0 |
| Windows build | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0 |
| macOS build | HOLDS | `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0 |
| New tests fail on base | HOLDS, with a note | The code under test already exists, so the tests cannot fail on an unchanged base; they were proven by mutation instead. With the `e.Verdict == "stale"` skip removed from the first loop of `pickJob` (feedjob.go:68), `TestPickJobNeverChoosesAStaleRecord` fails: `--- FAIL: TestPickJobNeverChoosesAStaleRecord` / `panic: runtime error: index out of range [-1]` at feedjob.go:98. The mutation was reverted (`git checkout`); the diff holds only the new file. |
| No existing test edited or deleted | HOLDS | `git status`: only `feedjob_save_test.go` added |
| No guard, gate, cap or hold added | HOLDS | tests only |
| Bugs found | None | |

## What changed

- New `internal/job/jobcmd/feedjob_save_test.go`: 15 tests, using the `readFeed` variable for fixture reads and the existing `fixture` and `entry` helpers (the plan.md fixture holds only J-1).
- This report.

## NOT VERIFIED on a live screen

Nothing here is UI. No live check is needed.

Note: the `panic` in the mutation run shows `pickJob` indexes `records[newest]` with no guard if `newest` is -1. It cannot happen in the unmutated code, so it is not a bug.

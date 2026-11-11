# C-pcov-session-store-reads: tests for session Activities, Activity, End, Show and List

Branch lane-C-pcov-session-store-reads, from main. One new file: `internal/session/store_reads_test.go`. No non-test file changed.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1 Activities on a fresh home: empty non-nil, no error, no `sessions` folder after | HOLDS | `TestActivitiesOnFreshHomeCreatesNothing`, store_reads_test.go:29; `go test ./internal/session/ -count=1` exit 0 |
| 2 Two sessions, stray sub-folder, notes.txt: exactly two; bad JSON file -> error | HOLDS | `TestActivitiesSkipsStraysAndFailsOnABadFile`, store_reads_test.go:43; exit 0 |
| 3 Activity: `../x` and `` -> "invalid session id"; unknown -> zero Activity with SessionID; a.json with session_id b -> "activity identity mismatch" | HOLDS | `TestActivityReadsAndRefusesBadIdentity` store_reads_test.go:73 (also covers no hooks folder, a missing file, bad JSON); `TestActivityFailsWhenHooksPathIsAFile` for the storageDir error path; exit 0 |
| 4 End: Finished with Ended set and Slept nil; second End(Errored) returns the first record; Running and Sleeping refused | HOLDS | `TestEndKeepsTheFirstEndAndRefusesNonEndStates` store_reads_test.go:110. It sleeps the record first, so "clears Slept" is exercised; it also ends fresh records as Errored and StoppedByYou; exit 0 |
| 5 Show: round trip; copy under another id -> identity error; pid 0 -> pid error; Zombie -> state error | HOLDS | `TestShowRoundTripsAndRefusesBadRecords` store_reads_test.go:149 (also empty pid start, bad JSON, unsafe id, absent file); `TestValidateTenureJobFields` covers the Job-liveness branch; exit 0 |
| 6 List: no folder -> empty; two machines -> both; `bad name` folder and x.tmp skipped; malformed record -> error | HOLDS | `TestListSkipsStraysAndFailsOnABadRecord` store_reads_test.go:210 (also skips a `.hooks` folder and a sub-folder named `sub.json`; checks List creates no folder); `TestListFailsWhenSessionsPathIsAFile`; exit 0 |
| 7a `go test ./internal/session/ -count=1`: 0 fail | HOLDS | `ok ... internal/session 0.130s coverage: 87.9%`, exit 0 |
| 7b coverage before and after | HOLDS | before (untouched clone, linux): `coverage: 79.2% of statements`; after: `coverage: 87.9% of statements` |
| 7c per-function: Activities, Activity, End, Show each above 85%; List above 80% | HOLDS | `go tool cover -func`: Activity 94.1% (activity.go:45), Activities 94.1% (activity.go:135), End 90.0% (session.go:304), Show 100.0% (session.go:368), List 91.3% (session.go:397), validateTenure 100.0% (session.go:151), atomicJSON 81.2% (files.go:59, was 62.5%) |
| 7d `GOOS=windows go test -c -o /dev/null ./internal/session/` | compiled, needs Windows run | exit 0; run with `go test ./internal/session/ -count=1` on a Windows host |
| 7e `GOOS=darwin go test -c -o /dev/null ./internal/session/` | compiled, needs macOS run | exit 0; run with `go test ./internal/session/ -count=1` on a macOS host |
| 7f `make lint` | HOLDS | runs `go vet ./...`; exit 0, no output. `gofmt -l internal/session/` prints nothing |
| Packages that import session | HOLDS | `go list` shows internal/cli, internal/hook, internal/lease, internal/orchestrator, internal/task. `go test -count=1` on all five: ok, exit 0 (cli 29.986s, hook 2.394s, lease 0.069s, orchestrator 24.858s, task 1.427s) |
| Tests only, no non-test file changed | HOLDS | `git diff --stat origin/main..HEAD` lists the new test file and this report only |
| New tests fail on base | NOT APPLICABLE | The brief adds coverage for behaviour that already exists and changes no code, so every test passes on base and on the lane. No test exposed a bug, so no row was dropped |
| No guard, gate, cap or hold added; no existing test edited or deleted; `newFixture` reused unedited | HOLDS | diff touches only the new file and this report |
| ReadHeartbeat and lease fields left alone | HOLDS | no test calls ReadHeartbeat; lease fields appear only in `validateTenure` input |

## What changed

New `internal/session/store_reads_test.go` with 10 tests: the rows above, plus `atomicJSON` (a folder where the file belongs -> "must be regular", a parent that is a file, an unencodable value leaves no temp file, replacing an existing file) and the "path is a file, not a folder" error from `storageDir` for Activity, Activities and List.

Not covered, as the brief predicted: the two `os.ReadDir` failures in List (session.go:406-409 and :414-417) and the `atomicJSON` sync, close and rename failures, which have no sandbox trigger.

## NOT VERIFIED on a live screen

None; there is no UI. Windows and macOS were compiled only (rows 7d and 7e).

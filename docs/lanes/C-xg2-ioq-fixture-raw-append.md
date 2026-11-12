# C-xg2-ioq-fixture-raw-append

Verdict: DONE. This finishes C-xg-job-id-held-counts (docs/lanes/C-xg-job-id-held-counts.md). It carries that lane's change unchanged and rebuilds the fixture of `TestIoqOldFeedPairFoldsAsToday` so the one NOT DONE row of C-xg now holds.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. `git merge origin/main`, name each resolved file | HOLDS | `git merge origin/main` printed `Already up to date.` Lane-C-xg was branched from origin/main 103b9157, which is still the head of origin/main. No file was resolved. |
| 2. Fixture rebuilt with raw appends, as TestIcjJobExistsRefused does for J-9 | HOLDS | TestIcjJobExistsRefused is `internal/feed/icj_job_exists_test.go:52`; its J-9 raw appends are at :81-96. New helper `ioqRawJob` (`internal/cli/ioq_question_ask_free_job_id_test.go`, above the test) appends each J-2 record to the writer file of the J-1 record with `os.O_APPEND`, seq = highest seq + 1, so `Store.Post` and its Job-id check are not in the path. `TestIoqOldFeedPairFoldsAsToday` calls it twice (held "First J-2", then "Second J-2"). Its assertions are untouched. |
| 2b. Red on lane-C-xg before the change | HOLDS | `go test -count=1 ./internal/cli/ -run TestIoqOldFeedPairFoldsAsToday` at 865a0cf8: `--- FAIL: TestIoqOldFeedPairFoldsAsToday (0.04s)` / `ioq_question_ask_free_job_id_test.go:142: post J-2 3: grid feed: Job J-2 already exists (post 01M76RZYJKEAJV7QFG871DW93Z); revisions need a new Job` / `FAIL`. |
| 2c. Green after | HOLDS | `go test -count=1 -v ./internal/cli/ -run TestIoq` exit 0: `--- PASS: TestIoqAskSkipsHeldJobID`, `--- PASS: TestIoqHeldJobResolvesBesideQuestionJob`, `--- PASS: TestIoqOldFeedPairFoldsAsToday`. |
| 3a. C-xg row 1 to 4, `-v -run TestXg` | HOLDS | `go test -count=1 -v -run TestXg ./internal/feed/` exit 0: `--- PASS: TestXgHeldJobIDRefused`, `TestXgHeldRetryAllowed`, `TestXgHeldThenLands`, `TestXgCountedStillRefused`, `TestXgHeldVsHeldRefused`. |
| 3b. C-xg row 5: `go test -count=1 ./internal/feed/ ./internal/orchestrator/` | HOLDS | exit 0: `ok internal/feed 20.234s`, `ok internal/orchestrator 240.431s`. |
| 3c. `go test -count=1 ./internal/cli/` | HOLDS | exit 0: `ok github.com/gridconsole/grid-plugins/internal/cli 289.494s`. |
| 3d. Walk tests, plugin set | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0 (`ok internal/cli 2.234s`); `go test -count=1 ./internal/job/lint/` exit 0 (`ok internal/job/lint 0.171s`). The app-only walk tests do not apply to this repo. |
| 3e. `make lint` | HOLDS | exit 0; last line `go vet ./...`, no findings. |
| 3f. `make cross` (Windows and macOS builds) | HOLDS | exit 0: `cross: darwin/arm64 vet`, `cross: darwin/arm64 test compile`, `cross: windows/amd64 vet`, `cross: windows/amd64 test compile`, no failure. |
| Packages that import the changed package | HOLDS | Changed package: `internal/cli`. `go list` importers: `cmd/grid` (`[no test files]`), `lib` (`go test -count=1 ./cmd/grid/ ./lib/` exit 0: `ok lib 124.368s`). |
| Edit only the brief's files | HOLDS | `git diff origin/main --stat`: `docs/lanes/C-xg-job-id-held-counts.md`, `internal/cli/ioq_question_ask_free_job_id_test.go`, `internal/feed/jobid.go`, `internal/feed/xg_held_job_id_test.go`, plus this doc. These are the Touches of C-xg plus the one cli test file. In the cli file only the import block, the new `ioqRawJob` helper and the fixture lines of `TestIoqOldFeedPairFoldsAsToday` changed. |
| No test deleted or loosened | HOLDS | No test deleted. No assertion of any existing test changed. |
| `*RealPlugin*` tests | not applicable | No RealPlugin test covers this change and the brief names none. |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | Only a test fixture changed in this lane; C-xg's own change widens one refusal that already existed. |

## What changed

- `internal/cli/ioq_question_ask_free_job_id_test.go`: imports `os` and `path/filepath`; new helper `ioqRawJob`; `TestIoqOldFeedPairFoldsAsToday` builds its J-2 pair with it instead of two `feed post` calls, since Post now refuses the second J-2. It still proves the fold keeps the lower-seq record and ignores the later one.
- Everything else is C-xg: `internal/feed/jobid.go` (`checkJobFree` refuses a counted or held match) and `internal/feed/xg_held_job_id_test.go`.

## NOT VERIFIED on a live screen

Nothing here is a UI change.

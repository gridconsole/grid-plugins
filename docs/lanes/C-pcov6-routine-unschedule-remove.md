# C-pcov6-routine-unschedule-remove

Test-only lane: one new file, `internal/routine/unschedule_remove_test.go` (helpers prefixed `ur`). No code under test changed.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1 Coverage before | HOLDS | `go test -count=1 -coverprofile=.../C-pcov6-routine-unschedule-remove-before.out ./internal/routine/` exit 0, `coverage: 79.5%`; `go tool cover -func=... \| tail -1` gives `total: (statements) 78.8%` (func report differs from the test line by 0.7; both noted) |
| 2 ReadDir failure :341 | HOLDS | `TestUnscheduleReadDirFailure`; profile `schedule.go:341.3,342.1 1 1` |
| 3 Skipped entries :345, :360 | HOLDS | `TestUnscheduleSkipsOtherEntries` (calls exactly `remove,after`; README and other record kept; daily state gone); `345.4,345.12 1 1`, `360.4,360.12 1 1` |
| 4 Bad record :357 | HOLDS | `TestUnscheduleBadRecord` (errors.As *json.SyntaxError); `357.4,358.1 1 1` |
| 5 Nameless record :363 | HOLDS | `TestUnscheduleNamelessRecord` (exact text); `363.4,364.1 1 1` |
| 6 Unreadable record :353 | HOLDS | `TestUnscheduleUnreadableRecord` (non-nil, no runner call); `353.4,354.1 1 1` |
| 7 Foreign timer file :384, :366 | HOLDS | `TestUnscheduleRefusesForeignTimerFile` (both calls return the exact text; state and foreign file intact; no calls); `384.3,385.1 1 1`, `366.4,367.1 1 1` |
| 8 remove refusals :374, :378 | HOLDS | `TestRemoveRefusals`; `374.3,375.1 1 1`, `378.3,379.1 1 1` |
| 9 AfterRemove failure :400 | HOLDS | `TestRemoveAfterRemoveFailureKeepsStateForRetry` (`ur after`; calls `remove,after`; timer file gone; state kept; retry nil, state gone); `400.4,401.1 1 1` |
| 10 RemoveRoutine :420, :424 | HOLDS | `TestRemoveRoutineRefusals` (store.Get still succeeds); `420.3,421.1 1 1`, `424.3,425.1 1 1` |
| 11 Preview :441, :445, :449 | HOLDS | `TestPreviewRefusals` (three assertions); `441.3,442.1 1 1`, `445.4,445.12 1 1`, `449.4,450.1 1 1` |
| 12 Coverage after and gates | HOLDS | after profile: `go test -count=1 -coverprofile=.../after.out ./internal/routine/` exit 0, `82.3%`; `cover -func \| tail -1` `81.7%` (before 79.5% / 78.8%). One `grep -E` over the after profile shows every block of rows 2-11 at count 1; the "Not in this lane" blocks (:324, :334, :338, :349, :395, :405, :415) stay at 0 as the brief says. `go test -count=1 ./internal/routine/` exit 0; `make lint` (go vet ./...) exit 0; `GOOS=windows go vet ./internal/routine/` exit 0; `GOOS=darwin go vet ./internal/routine/` exit 0; `gofmt -l internal/routine` empty |
| Importers tested | HOLDS | `go list` import scan: only `internal/cli` imports `internal/routine` (`cmd/grid` and `lib` import cli). `go test -count=1 ./internal/routine/ ./internal/cli/` exit 0 |
| New tests fail on base | NOT APPLICABLE | Test-only lane over unchanged code: the tests pass on base too, so no failing line exists. Instead the profile moves the named blocks from count 0 to 1 and the package from 79.5% to 82.3% |
| No guard/gate/cap on agents | HOLDS | Only a test file added |
| No edit of existing tests, none deleted | HOLDS | `git diff --stat origin/main` shows only the new test file and this report |
| Portability | HOLDS | No chmod, Mkfifo, syscall or symlink; OS error text asserted only as non-nil (rows 2, 6) |
| No approval.json / Approve use | HOLDS | Not referenced |

## What changed

- `internal/routine/unschedule_remove_test.go`: `urBackend`, `urRunner`, `urSetup` and helpers, plus ten tests, one per row 2-11.
- `docs/lanes/C-pcov6-routine-unschedule-remove.md`: this report.

## NOT VERIFIED on a live screen

Nothing here touches a UI. The tests use a fake backend and runner, so no real OS timer registration was exercised.

DONE

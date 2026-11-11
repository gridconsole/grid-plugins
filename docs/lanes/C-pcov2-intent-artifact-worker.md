# C-pcov2-intent-artifact-worker: proof

Tests only: three new files, no non-test file changed, no existing test edited, nothing deleted.

Files: `internal/intent/broken_files_test.go`, `internal/artifact/broken_records_test.go`, `internal/worker/empty_home_test.go`.

## Proof table

| Row | Result | Evidence |
|---|---|---|
| 1 coverage before/after | HOLDS | Before (untouched clone, linux/amd64 go1.27.3): `intent total: 86.5%`, `artifact total: 86.1%`, `worker total: 86.2%`. After: `intent total: 90.5%`, `artifact total: 89.6%`, `worker total: 87.9%`. Command per package: `go test -count=1 -coverprofile=.cov-lane/<pkg>.out ./internal/<pkg>/ && go tool cover -func=.cov-lane/<pkg>.out \| tail -1`, exit 0. (The coverprofile folder is `.cov-lane` in the clone, untracked and not committed: `mktemp -d` and a folder under `/srv/gridwork/tmp` were refused in this sandbox.) Linux before figures equal the darwin ones in the brief. worker rose by its one block (`worker.go:134-135` count 1 in the after profile). |
| 2 intent directory | HOLDS | `TestDirectoryRefusals` (3 subtests) in broken_files_test.go: `List(false)` on `Store{TaskDir:""}` gives exactly "task directory is required"; Enqueue on a missing TaskDir is `errors.Is os.ErrNotExist` and the folder is still missing; List(false) on it gives a non-nil empty list and nil; Enqueue on a regular file errors and the bytes are unchanged. Run: `go test -count=1 ./internal/intent/`, exit 0. |
| 3 intent broken files via List | HOLDS | `TestListRefusesBrokenFiles`: second `<ULID>.json` with another id -> "invalid intent record"; release marker as a directory after a Claim -> "intent file must be regular" (skips on Windows: opening a directory for reading fails with another error there); done file `{"token":"..."}{}` -> "unexpected data after intent JSON". Each subtest compares a byte snapshot of the task folder before and after List. Exit 0. |
| 4 intent Enqueue and Done | HOLDS | `TestEnqueueKeyOnlyForSend` (verb "pause", key "k" -> "idempotency keys apply only to send", no intents/ written). `TestDoneRefusals`: bad id and bad token -> `ErrClaim`; fresh ULID with no record after an Enqueue -> `os.ErrNotExist`; Claim, Done ok, Done with "boom" -> `ErrDone`, Done with the same text again returns the same Completion (token, created, error) and nil. Exit 0. |
| 5 artifact task refusals | HOLDS | `TestTaskRefusals`: TaskPath "" -> "a task is required" through Link and List; missing task -> `errors.Is os.ErrNotExist` and still missing; regular file -> "task is not a directory" through Link and List, bytes unchanged, no `artifacts` below it. Exit 0. |
| 6 artifact broken records | HOLDS | `TestBrokenRecords`, six subtests on a record made by Link: no record for a valid id -> `os.ErrNotExist`; record a directory -> "artifact record must be a regular file" (still a directory); "not json" -> starts "artifact <id>: "; id field changed and kind "other" -> "invalid artifact record <id>"; file "a/b" -> "artifact <id>: invalid file name". Publish and List give the same error, and the record bytes are unchanged after each. Exit 0. |
| 7 artifact source label | HOLDS | `TestImportSourceLabelOutsideMissingProject`: ProjectFolder missing, Source matches `^report\.txt#[0-9a-f]{16}$` (artifact.go:201-214, resolveFull :226-231 now covered). Exit 0. |
| 8 worker | HOLDS | `TestListWithoutGridHome`: `Store{GridHome:""}.List()` is a non-nil empty slice and nil. Exit 0. |
| 9 checks | HOLDS | `go test -count=1 ./internal/intent/ ./internal/artifact/ ./internal/worker/ ./internal/task/ ./internal/cli/` all `ok`, exit 0 (importers by `go list`: intent -> cli; artifact -> cli, task; worker -> cli). `GOOS=windows go test -c -o /dev/null` and `GOOS=darwin go test -c -o /dev/null` for each of the three packages: exit 0 (compiled, needs Windows and macOS runs to execute). `make lint` (`go vet ./...`): exit 0. `gofmt -l` on the three packages: no output. |
| Rule: new tests fail on base | NOT APPLICABLE as stated | The lane changes no code, so the new tests pass on base code too. Their proof is coverage: every targeted block named in the brief shows count 0 in the before profile and is absent from the count-0 list in the after profile (intent: no count-0 block at files.go:19/25/30/74/82 or intent.go:165/243/267/417/422/431; artifact: :58, :226-231, :343, :354 covered; worker :134-135 count 1). |
| Rule: tests only, no guard, nothing removed | HOLDS | `git diff --stat origin/main..HEAD` shows only the three new test files plus this report. |
| Rule: no attribution, no personal data | HOLDS | One-sentence commit message, no trailer lines. |

## Outcome notes

- Where the code differed from the brief: nothing. Every message was asserted as read in the source. Row 6 also asserts that List returns the same error as Publish for each broken record (artifact `List` calls the same `read`).
- Existing helpers reused: `testStore`, `enqueue`, `sourceFile`. `appHome` was not needed (GridHome "" is the point of row 8).
- No bug found.

## What changed

Three new test files (395 lines) and this report.

## NOT VERIFIED on a live screen

None; there is no UI. Windows and macOS test runs were not executed, only compiled.

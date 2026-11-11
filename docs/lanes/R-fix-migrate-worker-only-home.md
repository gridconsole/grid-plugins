# R-fix-migrate-worker-only-home

`grid migrate` on a home holding only `worker.lock` and `worker-status.json` exits 0 with "migrate: nothing to migrate".

Base: cc011519 (main with lane-J-migrate-fresh-home merged; merged head 02f91642).

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. New test fails on base through runMigrate, no --from, home with only worker.lock and worker-status.json | HOLDS | `go test ./internal/cli/ -run TestMigrateWorker -count=1` on base (fix not applied), exit 1: `--- FAIL: TestMigrateWorkerOnlyHome` / `migrate_workeronly_test.go:21: exit 1, stdout "", stderr "grid migrate: no old projects folder found\n"` |
| 2. That home prints "migrate: nothing to migrate" on stdout, returns 0; home with other content keeps the error | HOLDS | `go test ./internal/cli/ -run TestMigrate -count=1 -v` exit 0: TestMigrateWorkerOnlyHome PASS; TestMigrateWorkerFilesPlusOtherContentStillErrors PASS (exit 1, "no old projects folder found"). Fix: `onlyWorkerFiles` in internal/cli/migrate.go:57 and below |
| 3. Explicit --from that does not exist exits 1 with the path; TestMigrateErrors unchanged | HOLDS | TestMigrateFromMissingNamesPath and TestMigrateErrors/--from_missing PASS in the same run; the fix is only on the no --from path (migrate.go:55-57) |
| 4. Home with old data still runs Scan | HOLDS | TestMigrateInPlaceDefault, TestMigrateCommand PASS, unchanged |
| 5. `go test ./internal/cli/ ./internal/migrate/` 0 fail; TestMigrateFreshHome, TestMigrateFromMissingNamesPath unchanged | HOLDS | exit 0: `ok internal/cli 28.584s`, `ok internal/migrate 2.623s` |
| Tests of importers | HOLDS | `go list` imports: only cmd/grid imports internal/cli; `go test ./cmd/grid/` exit 0 (no test files). Nothing imports internal/migrate except internal/cli |
| Lint | HOLDS | `go vet ./internal/cli/ ./internal/migrate/ ./cmd/grid/` exit 0; `gofmt -l internal/cli internal/migrate` empty |
| Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0 |
| New test in its own file, no existing test edited, none deleted | HOLDS | internal/cli/migrate_workeronly_test.go (new); `git diff cc011519 --stat` shows migrate.go and the new test only |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | None added |
| No attribution lines, no personal paths, hosts or emails | HOLDS | commit message is one sentence, no trailers |

## What changed

- internal/cli/migrate.go: with no `--from`, a home whose entries are all plain files named `worker.lock` or `worker-status.json` (or none) is treated as fresh. The check is in a small `onlyWorkerFiles` helper.
- internal/cli/migrate_workeronly_test.go: new tests for the worker-only home and for a home with worker files plus other content, which still errors.
- internal/migrate/plan.go was not touched. The brief allowed it, but the CLI check was the smaller fix and the error at plan.go:149 stays for any home with other content.
- The comment in migrate_fresh_test.go lines 23-25 is now out of date. I left it because the lane's existing test file must stay unchanged.

## NOT VERIFIED on a live screen

None. This is CLI behaviour, proven by the tests above.

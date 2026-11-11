# J-migrate-fresh-home

Base: main 9eea4651 (the brief names 94fd8b07; the clone was cut at 9eea4651).

## Proof table

| Row | Result |
|---|---|
| 1. Test fails on main through runMigrate, no --from | HOLDS for (a) and (b); (c) fails with a different message (see below). Command: `go test ./internal/cli/ -run TestMigrateFreshHome` on base, exit 1. |
| 1a. missing home, base | `exit 1 ... stderr "grid migrate: old Grid data folder does not exist: .../home/.grid"` |
| 1b. empty home, base | `exit 1 ... stderr "grid migrate: no old projects folder found"` |
| 1c. worker.lock + worker-status.json, base | `exit 1 ... stderr "grid migrate: no old projects folder found"`. Another message than the "does not exist" the brief expected, so (c) is left alone and is not in the test file. |
| 2. missing or empty home, no --from: "nothing to migrate" on stdout, exit 0 | HOLDS: internal/cli/migrate.go:53-60; `go test ./internal/cli/ -run TestMigrateFreshHome` exit 0 (subtests missing, empty). |
| 2c. worker-files-only home | NOT DONE: the error is `no old projects folder found`, raised at internal/migrate/plan.go:149 (the home is non-empty, so Scan gets past :124). The brief bars internal/migrate. |
| 3. explicit --from missing: exit 1, path in message | HOLDS: TestMigrateErrors unchanged and passing; new TestMigrateFromMissingNamesPath checks the path. Exit 0 of `go test ./internal/cli/ -run Migrate`. |
| 4. old data still runs Scan | HOLDS: TestMigrateInPlaceDefault and TestMigrateCommand pass unchanged (same run). |
| 5. `go test ./internal/cli/ -run Migrate` 0 fail | HOLDS, exit 0. |
| Importers tested | `go list` importers of internal/cli: cmd/grid only. `go test ./internal/cli/ ./cmd/grid/` exit 0 (cmd/grid has no test files). |
| Lint | `gofmt -l internal/cli` empty; `go vet ./internal/cli/ ./cmd/grid/` exit 0. |
| Windows build | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0. |
| macOS build | `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| No existing test edited or deleted | HOLDS: only migrate.go changed and migrate_fresh_test.go added. |
| No guard, gate, cap or hold on agents | HOLDS: none added. |
| No personal paths, hosts, emails | HOLDS. |

## What changed

- internal/cli/migrate.go: with no --from, a Grid home that does not exist or has no entries prints `migrate: nothing to migrate` and returns 0, before Scan. This checks the folder, not an error string. --rollback and an explicit --from take the same path as before.
- internal/cli/migrate_fresh_test.go (new): TestMigrateFreshHome (missing, empty) and TestMigrateFromMissingNamesPath.

## Open

- (c) a home holding only worker.lock and worker-status.json still exits 1 with "no old projects folder found" (internal/migrate/plan.go:149). Fixing it needs a change in internal/migrate (for example Scan returning a "nothing to migrate" result when the source has no projects and nothing else to move) or a list of the worker files in cli. Both are outside this brief.
- `--apply` on a fresh home also prints "nothing to migrate" and exits 0, writing no VERSION.

## NOT VERIFIED on a live screen

Nothing here is UI; the real-run check on a new Grid home is for the orchestrator.

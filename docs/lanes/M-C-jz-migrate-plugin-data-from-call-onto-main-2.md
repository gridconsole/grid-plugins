# M-C-jz-migrate-plugin-data-from-call-onto-main-2

Second merge of lane M-C-jz-migrate-plugin-data-from-call-onto-main (the C-jz-migrate-plugin-data-from-call change) onto plugin main `b6160963`.

Merge commit: `d47663dd` (`git merge origin/main --no-edit`). One conflict, `lib/lib.go`, in the package doc comment. Main added the Feed-write member sentence (git config user.email from the Call's environment); the lane added the sentence that `migrate` takes the old plugin data folder from CLAUDE_PLUGIN_DATA in Environ, with a typed `--plugin-data` winning. Both sentences are kept in one paragraph. The lane's other `lib/lib.go` edit (dropping "and the migrate plan reads CLAUDE_PLUGIN_DATA from the host (internal/migrate)" from the list of host reads) merged cleanly and stays, since `Scan` no longer reads it. `lib/lib.go` is a Go file, not a golden. Nothing from either side was dropped.

## Proof table

| Row | Result | Command / place |
|---|---|---|
| 1. origin/main is an ancestor; diff names only lane files | HOLDS | `git merge-base --is-ancestor origin/main HEAD` exit 0. `git diff origin/main --stat` (before this report): `docs/lanes/C-jz-migrate-plugin-data-from-call.md`, `docs/lanes/M-C-jz-migrate-plugin-data-from-call-onto-main.md`, `internal/cli/migrate.go`, `internal/migrate/jz_plugin_data_test.go`, `internal/migrate/plan.go`, `internal/migrate/recover_test.go`, `lib/lib.go`, `lib/t2_jz_migrate_test.go`: all lane files. |
| 2.1 (C-jz row 1) Scan with no WithPluginData takes no plugin data although the process names a folder | HOLDS | `go test -count=1 -v -run 'TestJzMg' ./internal/migrate/` exit 0: `--- PASS: TestJzMgScanReadsNoProcessVariable`, `--- PASS: TestJzMgWithPluginDataIsTheOnlyWayIn`. |
| 2.2 (C-jz row 2) lib migrate apply takes callDir from Environ, hostDir unchanged | HOLDS | `go test -count=1 -v -run 'TestJzMg' ./lib/` exit 0: `--- PASS: TestJzMgLibMigrateTakesPluginDataFromCallEnviron`. |
| 2.3 (C-jz row 3) typed `--plugin-data` wins over Environ | HOLDS | same run: `--- PASS: TestJzMgLibMigratePluginDataFlagWinsOverEnviron`; also `--- PASS: TestJzMgLibMigrateEnvironWithoutPluginDataTakesNone`. |
| 2.4 (C-jz row 4) package tests | HOLDS | `go test -count=1 ./internal/migrate/ ./internal/cli/ ./lib/ ./cmd/grid/`: `ok internal/migrate 6.467s`, `ok internal/cli 25.306s`, `ok lib 5.886s`, `cmd/grid` no test files. `go test -count=1 -run 'Migrate' ./internal/cli/` exit 0 (`ok 0.265s`). |
| 2.5 (C-jz row 5) `make lint`, `make cross` | HOLDS | see row 4. |
| 3. walk tests (plugin) | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand|TestIlGuestSiteList'` exit 0 (`ok 0.032s`); `go test -count=1 ./internal/job/lint/` exit 0 (`ok 0.059s`). App walk tests belong to the app repo. |
| 4. `make lint` | HOLDS | exit 0 (`go vet ./...`, no findings). |
| 4. `make cross` | HOLDS | exit 0: `cross: darwin/arm64 vet`, `test compile`, `cross: windows/amd64 vet`, `test compile`, every package `ok`. |
| Rule: importers of changed packages (go list) | HOLDS | `go list -f '{{.ImportPath}}: {{join .Deps " "}}' ./... \| grep -E 'internal/migrate( \|$)\|internal/cli( \|$)'` gives `cmd/grid`, `internal/cli`, `lib`. `internal/migrate` itself, `internal/cli`, `lib` tested (row 2.4); `cmd/grid` has no tests. `migrate.Scan(` has one non-test caller on the merged head: `internal/cli/migrate.go:74`. |
| Rule: new tests fail on base, pass on lane | HOLDS | with `plan.go` and `cli/migrate.go` checked out from origin/main (then restored with `git checkout HEAD --`; `git status --short` clean): `--- FAIL: TestJzMgScanReadsNoProcessVariable` `jz_plugin_data_test.go:45: the plan took the process variable: plugin data ".../host-plugin-data/sessions", 2 files`; `--- FAIL: TestJzMgLibMigrateTakesPluginDataFromCallEnviron` `t2_jz_migrate_test.go:159: the manifest records plugin data ".../host-data/sessions" with 2 files, want ".../call-data/sessions"`; `--- FAIL: TestJzMgLibMigrateEnvironWithoutPluginDataTakesNone` `t2_jz_migrate_test.go:199: ... want none`. They pass on the lane head. |
| Rule: gofmt, build | HOLDS | `gofmt -l internal/ lib/` prints nothing; `go build ./...` exit 0. |
| Rule: no existing test edited to pass; nothing deleted | HOLDS, one helper edit | nothing deleted. The lane's one edit of an existing test file is `internal/migrate/recover_test.go:36-42` (`TestCrashHelper`, the crash subprocess, passes `WithPluginData` from its env as `grid migrate` does; no assertion changed), explained in `docs/lanes/C-jz-migrate-plugin-data-from-call.md`. |
| Rule: no guard, gate, cap, hold or blocking hook on agents | HOLDS | none added. |
| Rule: small commits, no attribution lines, public repo | HOLDS | merge commit and this report carry one sentence, no attribution lines, no personal paths or hosts. |
| Rule: *RealPlugin* tests | N/A | no row names one. |

## What changed

Only the merge and this report. `lib/lib.go` doc comment holds both sides' sentences.

## NOT VERIFIED on a live screen

Nothing here is a UI change. Not run: the built binary with the variable set in the shell and no flag.

Verdict: DONE

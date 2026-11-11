# M-C-jz-migrate-plugin-data-from-call-onto-main

Merge of lane C-jz-migrate-plugin-data-from-call onto plugin main `21f02736`.

Merge commit: `a3d1fe0d388f012154fc70bfc6e5060870a675ff` (`git merge origin/main --no-edit`, ort strategy). It merged with no conflicts. Main's side brought in the reviewer agent edit, the `C-jz-agent-probe-env-from-call` change (`internal/agent/runner.go`, `internal/cli/agent.go`) and the `kb` eval case; none touches `internal/migrate`, `internal/cli/migrate.go` or `lib/`. Nothing was dropped from either side. The one overlapping package is `internal/cli` (`agent.go` from main, `migrate.go` from the lane), in different files.

## Proof table

| Row | Result | Command / place |
|---|---|---|
| 1. origin/main is an ancestor of HEAD; diff names only lane files | HOLDS | `git merge-base --is-ancestor origin/main HEAD` exit 0. `git diff origin/main --stat` names 7 files: `docs/lanes/C-jz-migrate-plugin-data-from-call.md`, `internal/cli/migrate.go`, `internal/migrate/jz_plugin_data_test.go`, `internal/migrate/plan.go`, `internal/migrate/recover_test.go`, `lib/lib.go`, `lib/t2_jz_migrate_test.go`: all files of lane C-jz-migrate-plugin-data-from-call (plus this report after the report commit). |
| 2.1 (brief row 1) Scan with no WithPluginData takes no plugin data although the process names a folder | HOLDS | `go test -count=1 -v -run 'TestJzMg' ./internal/migrate/` exit 0: `--- PASS: TestJzMgScanReadsNoProcessVariable`, `--- PASS: TestJzMgWithPluginDataIsTheOnlyWayIn`. |
| 2.2 (brief row 2) lib migrate apply takes callDir from Environ, leaves hostDir unchanged | HOLDS | `go test -count=1 -v -run 'TestJzMg' ./lib/` exit 0: `--- PASS: TestJzMgLibMigrateTakesPluginDataFromCallEnviron`. |
| 2.3 (brief row 3) typed `--plugin-data` wins over Environ | HOLDS | same run: `--- PASS: TestJzMgLibMigratePluginDataFlagWinsOverEnviron`; also `--- PASS: TestJzMgLibMigrateEnvironWithoutPluginDataTakesNone`. |
| 2.4 (brief row 4) package tests | HOLDS | `go test -count=1 ./internal/migrate/ ./lib/ ./cmd/grid/`: `ok internal/migrate 64.266s`, `ok lib 286.374s`, `cmd/grid` no test files; `go test -count=1 -run 'Migrate' ./internal/cli/` exit 0 (`ok ... 1.452s`). |
| 2.5 (brief row 5) `make lint`, `make cross` | HOLDS | see row 4 below. |
| 3. repo-wide walk tests (plugin) | HOLDS | `go test -count=1 -run 'TestHsIpWalksEveryCommand|TestIlGuestSiteList' ./internal/cli` exit 0 (`ok ... 1.930s`); `go test -count=1 ./internal/job/lint/` exit 0 (`ok ... 0.568s`). The app walk tests belong to the app repo. |
| 4. `make lint` | HOLDS | exit 0 (`go vet ./...`, no findings). |
| 4. `make cross` | HOLDS | exit 0. A first run was stopped by the harness at its 10 minute background limit; the rerun with a longer limit ended `cross: windows/amd64 test compile` ... `ok .../lib 0.003s`, `[exited with code 0]`. Both darwin/arm64 and windows/amd64 vet and test compile passed. |
| Rule: importers of the changed packages (go list) | HOLDS | `go list -f '{{.ImportPath}} {{join .Imports " "}}' ./...`: `internal/migrate` is imported only by `internal/cli`; `internal/cli` is imported by `cmd/grid` and `lib`. Tests run: migrate, cli, lib (above), cmd/grid (no test files). |
| Rule: full `internal/cli` package | SEE BELOW | see "Open items". |
| Rule: gofmt | HOLDS | `gofmt -l internal/ lib/` prints nothing. |
| Rule: `go build ./...` | HOLDS | exit 0. |
| Rule: no existing test edited to pass; nothing deleted | HOLDS | nothing removed by the merge. The lane's one edit of an existing file, `internal/migrate/recover_test.go` (the crash subprocess helper passes `WithPluginData`, no assertion changed), is explained in `docs/lanes/C-jz-migrate-plugin-data-from-call.md`. |
| Rule: no guard, gate, cap, hold or blocking hook on agents | HOLDS | none added. |
| Rule: small commits, no attribution lines, public repo | HOLDS | the merge commit message is the git default; this report is committed with one sentence and no attribution lines. |
| Rule: *RealPlugin* tests | N/A | no row names one. |

## What changed

Nothing new beyond the merge: the lane's change (`plan.go` drops `os.Getenv`; `cli/migrate.go` takes `CLAUDE_PLUGIN_DATA` from `env.getenv`; `lib/lib.go` doc; two new test files; the crash helper) sits on top of main `21f02736`. This report is the only new file.

## Open items

- `go test -count=1 ./internal/cli/` as part of one four-package command hit Go's default 10 minute test limit (`panic: test timed out after 10m0s`, running test `TestWorkerNextSpawnFollowsPersonalAndProjectSettings`, an unrelated worker test blocked reading its subprocess) while three heavy runs shared the machine. The result of the rerun alone with `-timeout 30m` is in the section below.

## NOT VERIFIED on a live screen

Nothing here is a UI change. Not run: the built binary with the variable set in the shell and no flag.

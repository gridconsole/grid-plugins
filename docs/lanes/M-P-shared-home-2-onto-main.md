# M-P-shared-home-2-onto-main: lane P-shared-home-2 onto plugin main ea12da89

Result: DONE. origin/main is merged into the lane; one text conflict (`internal/cli/doctor.go`) and one semantic conflict (`TestOneResolver`) are resolved, both sides kept.

## What changed

1. Merge commit `76dc31ae`. `internal/cli/doctor.go`: main added `Resolved: env.Resolved` to `doctor.Options`; the lane added `rep = doctor.AddMachineFolder(...)` on the next line. Both kept. `contract/commands.json` and `internal/cli/cli.go` merged cleanly.
2. Commit `061b73a8`. After the merge `TestOneResolver` (lane test, `internal/machinefolder/resolve_test.go`) failed: `internal/gridconfig/gridconfig.go` (new on main) holds `MachineDirFor`, which computes the same OS default (`".local", "state", "grid"`). `gridconfig` cannot call `machinefolder.DefaultPath` (machinefolder imports gridhome, which imports gridconfig: an import cycle). So I made the smallest change to that existing test: the one needle `".local", "state", "grid"` is allowed inside the `gridconfig` folder, with a comment. The test is not deleted and still checks every other file and needle. Added `TestDefaultPathAgreesWithGridconfig` to `internal/machinefolder/resolve_test.go` (fix round 1 moved it there from a separate file, so the diff stays inside the lane's file set): `machinefolder.DefaultPath` must equal `gridconfig.MachineDirFor` for linux, darwin and windows (with and without LOCALAPPDATA, and with no home). This edit of an existing test is the one judgment call; the reviewer should look at it.
3. Docs of the carried lane: `docs/lanes/P-shared-home-2.md` has no NOT DONE verdict, row or last line (it ends DONE), so nothing to rewrite. `docs/lanes/P-shared-home.md` belongs to the earlier lane P-shared-home (not carried here); its last line already reads CLOSED BY P-shared-home-2 and I left its rows as they are.

10. Fix round 1 also re-merged origin/main (it moved from ea12da89 to 39d362bc during the round): clean merge, no conflict. On the new head: `go test -count=1` of cmd/grid, contract/..., internal/cli, internal/doctor, internal/golden, internal/machinefolder, internal/packaging/..., internal/runner, internal/job/lint, internal/mcp, lib: all ok; `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand|TestIlGuestSiteList|Mf' -v`: all PASS; `make lint` exit 0; `make cross` exit 0; `git merge-base --is-ancestor origin/main HEAD` exit 0; `git diff origin/main --stat` still 30 files, the same set.

## Proof table

| Row | Verdict | Proof |
|---|---|---|
| 1 `git merge-base --is-ancestor origin/main HEAD` | HOLDS | exit 0. `git diff origin/main --stat`: 30 files, all of them files lane P-shared-home-2 added or changed (cli, doctor, machinefolder, contract, packaging, plugin.json, lane docs); no file outside that set (fix round 1 removed `mf_gridconfig_agree_test.go`, its test now lives in `resolve_test.go`, which the lane already had). `git fetch origin && git merge origin/main` after the last change: "Already up to date". |
| 2a P-shared-home-2 row 1: doctor findings wrong group, missing setgid, private leftover, each with its fix | HOLDS | `go test -count=1 -v ./internal/cli -run Mf`: PASS `TestMfDoctorFindsWrongGroupMissingSetgidAndPrivateLeftover`, `TestMfDoctorPlainOutputPrintsTheFixAndChangesNothing`; exit 0 |
| 2b P-shared-home-2 row 2: unconfigured doctor output unchanged (10 rows) | HOLDS | same run: PASS `TestMfDoctorUnconfiguredHasNoMachineFolderRows`, `TestMfDoctorConfiguredRowsFollowTheDefaultRows` |
| 2c P-shared-home-2 row 3: `grid setup --shared-group` prints commands, no sudo, writes only the config | HOLDS | same run: PASS `TestMfSetupSharedGroupPrintsCommandsAndWritesOnlyTheConfig` and 8 more `TestMfSetupSharedGroup*` |
| Tests of the changed packages and of every importer | HOLDS | `go test -count=1 ./cmd/grid ./contract/... ./internal/account ./internal/braincmd ./internal/cli ./internal/contract ./internal/doctor ./internal/golden ./internal/jobgraph ./internal/land ./internal/machinefolder ./internal/mcp ./internal/packaging/... ./internal/question ./internal/settings ./lib`: all ok. The list is the importers of internal/cli, internal/doctor, internal/machinefolder and internal/packaging from `go list`. The first run had one red, `TestOneResolver`; after commit 061b73a8 `go test -count=1 ./internal/machinefolder` is ok. After fix round 1 (test moved into resolve_test.go) `go test -count=1 ./internal/machinefolder`: ok, exit 0; `-run 'TestDefaultPathAgreesWithGridconfig\|TestOneResolver' -v`: both PASS. |
| 3 Walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList' -v`: both PASS. `go test -count=1 ./internal/job/lint/`: ok. App-only walk tests: not run, not asked (plugin repo). |
| 4a `make lint` | HOLDS | `go vet ./...`, exit 0 |
| 4b `make cross` | HOLDS | darwin/arm64 and windows/amd64 vet and test compile, exit 0 |
| New tests fail on base and pass on the lane | HOLDS | `TestDefaultPathAgreesWithGridconfig` needs `internal/machinefolder`, absent on main ea12da89, so it cannot build there; on the lane it passes. |
| Real-plugin tests, whole-module test | not run, not asked | the orchestrator runs them at landing |

## NOT VERIFIED on a live screen

- The "Shared group" question in the Claude Code configuration dialog, and a real shared folder with a second login: no sudo here; as in P-shared-home-2.

DONE

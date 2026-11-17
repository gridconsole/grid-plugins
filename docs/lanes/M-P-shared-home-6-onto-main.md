# M-P-shared-home-6-onto-main: lane P-shared-home-6 merged onto plugin main 63d58a47

Result: **NOT DONE** on row 2 (CLOSED BY M-P-shared-home-6-onto-main-fix1). The merge is done and clean in text, but five `internal/cli` tests of main fail on the merged head. They need a decision this brief does not give. The lane is not handed off as DONE with that row red.

## Proof table

| # | Row | Verdict | Command and result |
|---|---|---|---|
| 1a | `origin/main` is an ancestor of HEAD | HOLDS | `git merge-base --is-ancestor origin/main HEAD`: exit 0 |
| 1b | `git diff origin/main --stat` names only files the lane touched | HOLDS | 57 files, all from the lane chain P-shared-home-3 to -6 (docs P-shared-home-3, -4, -5, -6 and `P-shared-home.md`, and `internal/` files of migrate, machinefolder, gridhome, gridconfig, doctor, feed, job, where, routine, question, brain, cli). `git diff --name-only HEAD^2...HEAD^1` lists every one of them. No file outside what the lane branch changed. |
| 2a | P-shared-home-6 rows: migrate and machinefolder tests (apply, rollback, `rewriteWorktrees`, `rewriteSessions`, unconfigured path stays put) | HOLDS | `go test -count=1 ./internal/migrate/... ./internal/machinefolder/...`: `ok internal/migrate 4.630s`, `ok internal/machinefolder 0.054s` (the same run exited 1 only because of `internal/cli`, below) |
| 2b | P-shared-home-6 step 3: `go test -count=1 ./internal/cli/...` | **NOT DONE** | exit 1. Five tests fail: `TestMfDoctorConfiguredRowsFollowTheDefaultRows` (`mf_doctor_test.go:130`), `TestMfDoctorFindsWrongGroupMissingSetgidAndPrivateLeftover` (`:153`, `:169`, `:189`), `TestMfDoctorPlainOutputPrintsTheFixAndChangesNothing` (`:222`), `TestMfSetupSharedGroupPrintsCommandsAndWritesOnlyTheConfig` (`mf_setup_test.go:133`), `TestMfSetupSharedGroupMakesTheConfigWhenThereIsNone` (`:161`). Cause below. |
| 2c | Other changed packages and importers | HOLDS | `go test -count=1` over gridhome, gridconfig, doctor, feed, job/..., where, routine, question, brain, hook: all `ok`, exit 0. `make cross` also builds and test-compiles every package on darwin/arm64 and windows/amd64. |
| 3 | Walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'`: ok. `go test -count=1 ./internal/job/lint/`: ok. |
| 4a | `make lint` | HOLDS | exit 0 (`go vet ./...`, no output) |
| 4b | `make cross` | HOLDS | exit 0 (darwin/arm64 and windows/amd64: vet and test compile, all `ok`) |
| 5 | First commit rewrites NOT DONE of the carried lane to CLOSED BY M-P-shared-home-6-onto-main | HOLDS (nothing to rewrite) | `grep -n -E "NOT DONE\|CLOSED\|^DONE" docs/lanes/P-shared-home-6.md` finds only line 126 `DONE`. The NOT DONE verdicts of P-shared-home-5 were already closed by `06df0c07`. |
| 6 | Not run | not run, not asked | The `make realplugin` guest walk tables, `make webui-test` and the app-repo walks are app-repo checks; this is the plugin repo. The whole-module test is the orchestrator's at landing. |

## Conflict resolved

One file, `docs/lanes/P-shared-home.md`: both sides appended a last line (`CLOSED BY P-shared-home-6: accepted 05:38 UTC` on the lane, `CLOSED BY P-shared-home-2: accepted 03:40 UTC` on main). Both kept, main's first, since neither is ordered by the file. Nothing dropped. The other merged files (`internal/brain/index.go`, `internal/cli/cli.go`, `internal/cli/doctor.go`, `internal/machinefolder/resolve_test.go`) merged automatically.

## Why row 2b is NOT DONE, and the decision needed

Main has `grid setup --shared-group` (`internal/cli/setup_shared.go:84`, which calls `machinefolder.WriteConfig`) and the doctor machine-folder rows, with tests `internal/cli/mf_doctor_test.go` and `mf_setup_test.go`. Those tests seed the config with the snake_case key `shared_group` (`mf_doctor_test.go:122,144,161,177,205`) and expect setup to write only a two-key file with no other file (`mf_setup_test.go:133,161`). The lane chain (P-shared-home-5 row 2, by the owner's direction) stopped reading the snake_case keys (`TestOldSnakeCaseKeysAreNotRead`) and writes through `gridconfig`, which gives a version 3 file with `workspaces` and a `.v1` backup. The same five tests fail on P-shared-home-5's head merged with main without any commit of P-shared-home-6, so the cause is not `internal/migrate`.

Two ways to make them pass; I took neither, because each breaks a rule of this brief:

- **Change main's side** (seed `sharedGroup` in `mf_doctor_test.go`, accept `v`, `workspaces` and a `.v1` file in `mf_setup_test.go`): this edits existing tests to make them pass, and those files are outside the diff row 1 allows.
- **Change the lane's side** (read `shared_group` again, or write a bare two-key file without a backup): this fails `TestOldSnakeCaseKeysAreNotRead` and reverses the owner's key decision; no test of either side may be dropped.

The orchestrator or the owner picks. The merged head is kept so that the choice can be judged on it. The earlier fix round for P-shared-home-6 asked to leave the merge with main to its own lane and to stop saying DONE while a required check is red; this lane is that merge and the check is still red, so it says NOT DONE.

## What changed

- One merge commit of `origin/main` into the lane, one conflict (above).
- This doc.
- No code, test or contract file changed by this lane.

## NOT VERIFIED

- The five failing tests were not changed, so their behaviour on a fixed head is not shown.
- Behaviour on a real shared folder with two logins, and macOS and Windows runtime (build and vet only), as in P-shared-home-5 and -6.

CLOSED BY M-P-shared-home-6-onto-main-fix1

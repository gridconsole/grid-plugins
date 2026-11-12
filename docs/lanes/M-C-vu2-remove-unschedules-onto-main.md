# M-C-vu2-remove-unschedules-onto-main: C-vu on top of main's remove

Base: `origin/main` 219ca2bb (Pint150). `git merge origin/main` into lane-C-vu-project-remove-unschedules: no conflicts, no file resolved by hand.

## The commit on main that changed what `project remove` leaves behind

`ef8a2eb0` "Make grid project remove move the project's data folder to projects/.removed/<name> instead of rewriting project.md." (`internal/projects/projects.go`). Its doc comment on `Store.Remove`: "Remove unregisters the project and deletes nothing: one rename moves its data folder, project.md and all, to projects/.removed/<name>, or <name>-2, <name>-3 when that is taken ... It answers the record with this machine's folders and keys dropped."

Main pins it in `internal/projects/xn_remove_move_test.go` (data folder moved whole to `projects/.removed/alpha`, free name picked, other projects alone) and in `TestRelinkAndRemoveKeepData` (fc08584a: removed data read under `projects/.removed/example`, gone from `projects/example`).

So after remove the project's folder is gone from `projects/alpha` and the project is not listed. C-vu's two tests had been written for the older remove (folder stays, project listed with no folders).

## Does C-vu's point still hold? Yes.

Main's remove does not touch the OS timers: it renames a folder. The timers' ownership records live in `<gridhome>/local/routine-timers`, outside `projects/<name>`, and C-vu's code (`UnscheduleProject`, `projectRemoved` in `internal/cli/projects.go`) reads those manifests, not the project folder. So no production change was needed, and no main test covers the timers coming down.

## Change

Only `internal/cli/vu_project_remove_unschedules_test.go` (commit 27a5c1ba), the two failing tests plus one helper `removedFolderMentions`:
- `TestVuRmRemoveTakesDownTheProjectsTimer`: the routine definition is no longer read through `routine.Store` for alpha (the project is gone); it is looked for in `projects/.removed/alpha`. Every timer row is unchanged (JSON `"unscheduled":["daily"]`, schema valid, OS calls, manifests).
- `TestVuRmRemoveKeepsTheProjectRemovedWhenTheOSRefuses`: exit 1, stderr naming project, routine and cause, and the answer listing none as down are unchanged; the project list now expects alpha unlisted (the remove completed, as main's remove always does) and the definition kept in `.removed/alpha`. The retry of `UnscheduleProject` finishing the job is unchanged.
No test of main was edited.

## Proof table

| # | Row / rule | Result | Command and evidence |
|---|---|---|---|
| 1 | `go test -count=1 ./internal/cli -run 'TestVuRm\|ProjectRemove\|TestHsIpWalksEveryCommand'` | HOLDS | exit 0 (`-v`): `--- PASS: TestHsIpWalksEveryCommand`, `--- PASS: TestVuRmRemoveTakesDownTheProjectsTimer`, `--- PASS: TestVuRmRemoveWithoutTimersAnswersEmptyList`, `--- PASS: TestVuRmRemoveTextListsEachTimer`, `--- PASS: TestVuRmRemoveKeepsTheProjectRemovedWhenTheOSRefuses`, `--- PASS: TestYnProjectRemove`, `ok .../internal/cli 0.098s`. Before the edit (same merge): `FAIL ... vu_project_remove_unschedules_test.go:148: alpha's routine definition is gone: openat projects/alpha: no such file or directory` and `:209: project list = [] , want alpha still listed with no folder on this machine`. |
| 2 | `go test -count=1 ./internal/cli ./internal/routine/... ./internal/project/... ./internal/projects/...` | HOLDS | exit 0: `ok internal/cli 29.772s`, `ok internal/routine 0.410s`, `ok internal/project 0.005s`, `ok internal/projects 0.312s`. |
| 3 | `make lint` | HOLDS | exit 0 (`go vet ./...`). |
| 4 | `make cross` | HOLDS | exit 0: darwin/arm64 and windows/amd64 vet and test compile of every package. |
| 5 | Importers of the changed package (`internal/cli`), from `go list` | HOLDS | `cmd/grid`, `lib`: `go test -count=1 ./cmd/grid/ ./lib/` exit 0 (`cmd/grid` no test files, `ok lib 4.632s`). |
| W1 | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS | exit 0, `ok .../internal/cli 0.039s`. |
| W2 | `go test -count=1 ./internal/job/lint/` | HOLDS | exit 0, `ok .../internal/job/lint 0.087s`. |
| R1 | Main's tests not weakened | HOLDS | no file of main edited; only the two C-vu tests changed and each still checks every timer row it checked before. |
| R2 | No guard, gate, cap, hold or blocking hook | HOLDS | none added. |
| R3 | Public repo: no personal paths, hosts, emails | HOLDS | `t.TempDir()` only. |
| R4 | Budget (about 40 calls) | HOLDS | about 30. |

## NOT VERIFIED on a live screen

No UI row. No real `systemctl --user` or `launchctl` run: the OS scheduler is a recording runner, as in C-vu's own report.

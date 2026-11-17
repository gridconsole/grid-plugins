# Lane P-shared-home-4: the last readers of the old per-machine paths, and the two calls of P-shared-home-3

Result: **NOT DONE**. Three of the four readers now go through `machinefolder.Resolve` (`where`, `doctor/bundle`, `routine/timer_plist`), each with a test that fails on base. The fourth, `internal/migrate/plan.go:216-228`, is not changed: it needs a new feature, named in row 4. The two decisions of the owner are recorded (rows 5 and 6). Item 6 of the earlier list (the app reads the machine folder) is a separate app lane and is not in this one.

Code head: `8512ef38` (the last code commit). The doc commit comes after it; the tip of the bundle is the lane head. The bundle range `origin/main..lane-P-shared-home-4` also carries P-shared-home and P-shared-home-3 (they are not on main yet). The commits of this lane are `3814301e` (merge of origin/main), `0fa92281`, `ade4e588`, `673ad0d7`, `1ca51d8c`, `3fcc32e5`, `8512ef38`, and the doc commit.

## How it works

A `Folder` from `machinefolder.Resolve` is **active** when `Folder.Active()` is true (`Path != "" && (Source != SourceDefault || Shared())`). A zero or inactive `Folder` keeps the old path byte for byte, so every existing test and every install without a machine folder is unchanged.

- `where` (`internal/where/where.go`, `worktreeTask`): the legacy layout `<grid home>/worktrees/<ulid>[...]` is tried first, as before. With an active folder it also reads `<machine>/worktrees/<ulid>...` and `<machine>/worktrees/<project>/<ulid>...` (`Folder.WorktreeParts`, new in `place.go`). The folder `<project>` is checked against the task's project, so a task of another project is not returned. The resolver is called in `where` itself (`machinefolder.Resolve(Env{Home: r.Home})`, `where.go:334`), with the same inputs as `cli.OSEnv` (`Env{Home: home}`, `cli/cli.go:151`; `Resolver.Home` is `env.Home`, the user's home, at `cli/cli.go:440`), so `where` and the orchestrator resolve the same folder, whether it comes from `GRID_MACHINE_DIR` or from the config file. `guard/settings.go:60`, which builds a second `where.Resolver` with `Home: env["HOME"]`, needs no change. The tests set `GRID_MACHINE_DIR` only; the config-file source is the same `Resolve` call and is covered by the `machinefolder` tests, not again here.
- `routine` (`timer_plist.go`, `schedule.go`): the launchd timer log goes to `launcher/<login>/routines/<label>` in an active folder (`Folder.RoutineLogDir`, new in `place.go`; the directory is made through `Folder.MkdirAll`, so it gets the shared modes) and stays `<grid home>/logs/routines/<label>` otherwise. `TimerSpec` is a struct with `Machine` and `Login`; `Scheduler` carries both.
- `doctor` (`bundle.go`): when the folder is active and the login is known, the support bundle also takes the `*.log` files under `launcher/<login>/` of the folder. They go into the archive as `logs/machine/launcher/...`, so they never collide with `logs/` of the Grid home, and the same 5 MiB tail rule applies. Not the lock or the pid beside them, and not another login's logs.

## Proof table

Rows of the brief:

| # | Row | Result | Proof |
|---|-----|--------|-------|
| 1 | `where.go:327` reads through `machinefolder.Resolve`, with a test that fails on base | HOLDS | `internal/where/where.go` `worktreeTask` (+ `worktreeNamed`); `internal/where/pc_machine_worktree_test.go`: `TestWorktreeFolderInTheMachineFolderResolvesItsTask`, `TestWorktreeFolderOfAnotherProjectIsNotThisTask`, `TestWorktreeFolderWithoutAMachineFolderIsNotATask`. Base failure below. |
| 2 | `doctor/bundle.go:75`: the support bundle includes the launcher logs of a machine folder, with a test that fails on base | HOLDS | `bundle.go` (`BundleOptions.Machine`, `.Login`; `collectBundleFiles`); wired at `internal/cli/doctor.go:48`; `internal/doctor/pc_machine_bundle_test.go` `TestBundleIncludesTheLauncherLogsOfTheMachineFolder` (expects `doctor.txt`, `logs/grid-open.log`, `logs/machine/launcher/routines/nightly/stdout.log`, `logs/machine/launcher/server.log`, `versions.txt`; not the lock, the pid or another login's log; without a machine folder the zip has the 3 entries it had). Base failure below. |
| 3 | `routine/timer_plist.go:23` reads through `machinefolder.Resolve`, with a test that fails on base | HOLDS | `timer_plist.go` (`logDir`), `schedule.go` (`makeLogDir`); wired at `internal/cli/routine.go:221` and `internal/cli/projects.go:231`; `internal/routine/pc_machine_log_test.go`: `TestLaunchdLogGoesToTheMachineFolderPerLogin`, `TestLaunchdLogUnconfiguredStaysInTheGridHome`. Base failure below. |
| 4 | `migrate/plan.go:216-228` reads through `machinefolder.Resolve`, with a test that fails on base | **NOT DONE** | Not changed. Why: every `Action.To` in a migrate plan is relative to the new Grid home (`p.result[a.To]`), and `rewriteWorktrees` (`worktree.go`) and `rewriteSessions` read the `paths` map built at `plan.go:225` to repoint git worktree files and session `cwd`. To put the old `<home>/worktrees/<name>` folders into `<machine>/worktrees/<project>` the plan needs an action whose target is outside the home, its apply and rollback (`apply.go`), and the pointer and cwd rewrites to that target. That is a new migrate feature, not a path swap, and a Note that says the folder was left would read as a HOLDS. The split for a follow-up lane: "migrate relocates the old worktrees folders into the machine folder: `Action` with an absolute target, apply, rollback, `rewriteWorktrees` and `rewriteSessions` against it, one test of each". |
| 5 | Decision recorded: the brain cache stays in the brain root, per person | HOLDS | Comment at `internal/brain/index.go:247-250` before the `atomicWrite` of `knowledge/backlinks.json` (the cache, `index.md` included, stays in this brain root; a machine folder shared by several logins must not show one person's summaries and backlinks to the others). No code changed there. |
| 6 | Decision recorded: the launcher pid is not wired, `Launcher.Start` is not called from `grid open` | HOLDS | Here: `grid open` does not call `machinefolder.Launcher.Start`, because `Start` returns `ErrRunning` and that would add a refusal to `grid open`; Grid adds no limit of its own. `grep -rn -E '\.Start\(' internal cmd lib --include=*.go --exclude=*_test.go` hits only `processid.Start` (`ports`, `migrate`), `exec.Cmd.Start` (`env/env.go:139`, `cli/open.go:83`, `cli/open_start.go:255`, `agent/iu_probe_process_unix.go:21`) and `sessions.Start` (`task/attach.go:135`); none is `machinefolder.Launcher.Start`. |
| 7 | Merge origin/main and say how it was resolved | HOLDS | See "The merge" below. `git fetch origin` then `git merge origin/main` on the final tree printed `Already up to date.` |
| 8 | `go test -race` of the five packages, and `go vet ./...` | HOLDS | Race run below, exit 0. `go vet ./...` exit 0. |
| 9 | `docs/lanes/P-shared-home-3.md`: its NOT DONE rows rewritten to CLOSED BY P-shared-home-4 | HOLDS | Row 1, the writer-table Brain cache row, the grep notes for `where`, `doctor/bundle.go` and `routine/timer_plist.go`, and open items 1, 2 and 4 are rewritten, and the closing `NOT DONE` line is replaced by a status line. The migrate reader is the one exception: it stays open, as row 4 says, and row 1 and open item 4 of that doc say so. |

The race run and the importer run below started after the last code commit `8512ef38` (committed 04:19:21Z; the runs ended 04:21:32Z and 04:22:46Z, and the slowest packages took 76s and 144s, so they began after it). The race run, as printed (exit 0): `go test -race -count=1 ./internal/machinefolder/... ./internal/where/... ./internal/migrate/... ./internal/doctor/... ./internal/routine/... ./internal/gridconfig/... ./internal/gridhome/...`

```
ok  	github.com/gridconsole/grid-plugins/internal/machinefolder	3.849s
ok  	github.com/gridconsole/grid-plugins/internal/where	42.206s
ok  	github.com/gridconsole/grid-plugins/internal/migrate	56.999s
ok  	github.com/gridconsole/grid-plugins/internal/doctor	76.181s
ok  	github.com/gridconsole/grid-plugins/internal/routine	9.378s
ok  	github.com/gridconsole/grid-plugins/internal/gridconfig	1.411s
ok  	github.com/gridconsole/grid-plugins/internal/gridhome	1.191s
```

`internal/machinefolder` includes `TestOneResolver`: it passes.

New tests on base (the new test files built against the code without this lane's non-test changes; the `where` test builds and fails by assertion, the other two fail to build, so no new test passes there):

```
internal/routine/pc_machine_log_test.go:42:33: unknown field Machine in struct literal of type TimerSpec
internal/routine/pc_machine_log_test.go:62:22: unknown field Login in struct literal of type TimerSpec
internal/doctor/pc_machine_bundle_test.go:63:50: unknown field Machine in struct literal of type BundleOptions
internal/doctor/pc_machine_bundle_test.go:63:79: unknown field Login in struct literal of type BundleOptions
--- FAIL: TestWorktreeFolderInTheMachineFolderResolvesItsTask (0.00s)
    pc_machine_worktree_test.go:27: <temp>/srv/grid/worktrees/01M72K22CXZ5Q8W8B9PB3KVQ9M/J-4-a1: got {Project:J-4-a1 ID: Rule:folder Folder:<temp>...
```

The test files carry the prefix `pc_machine_`, so they are new files. No existing test file is edited: `git diff --diff-filter=M --name-only origin/main...HEAD -- '*_test.go'` prints nothing.

## The merge

`git merge origin/main` (merge commit `3814301e`, no conflict section in its message) brought in the second machine-folder notion on main: `gridconfig` (key `machineFolder`, `Config.MachineDir`, `MachineDirFor`) and `gridhome.Home.Machine` / `MachineSource`. Three things had to be fixed after it, one commit each:

1. `0fa92281`: `internal/cli/orchestrator.go:516: undefined: filepath`. `jobFromBrief` from main uses `filepath`, and the lane's side of the import block had dropped it. The import is back.
2. `ade4e588` (part 1): `TestOneResolver` was red, because `gridconfig` now spells `".local", "state", "grid"`. `gridconfig.MachineDirFor` now returns `machinefolder.DefaultPath(...)` (the `ios` alias to `darwin` is kept), so only `machinefolder` spells the OS standard place. Same answer as before.
3. `ade4e588` (part 2): that made an import cycle (`machinefolder/check.go` imported `gridhome`, `gridhome` imports `gridconfig`, `gridconfig` imports `machinefolder`). `check.go` now has a local `realPath` with the algorithm of `gridhome.Real`, with a comment saying why.

`git fetch origin` before the last run fetched one new branch (`lane-P-jobs-land-g2`, not main) and `git merge origin/main` printed `Already up to date.`

## Rules

| Rule | Result | Proof |
|------|--------|-------|
| Never push | HOLDS | No `git push`; the result leaves as a bundle. |
| Never touch /srv/gmf, /root/.grid, /srv/grid, other users' homes, another lane's clone | HOLDS | Every command ran in this clone; the new tests use `t.TempDir()` and `t.Setenv`. |
| Never sudo, never `pkill -f` | HOLDS | None run. |
| Commits: small, one sentence, no attribution lines | HOLDS | `git log --oneline origin/main..HEAD` lists one-sentence commits; `git log --oneline -i --grep=Co-Authored --grep=Claude --grep=Generated origin/main..HEAD` prints nothing. |
| No personal paths, hosts or emails in committed files | HOLDS | The doc and the tests use `<temp>` and `t.TempDir()` paths only. |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | None added. Row 6 is the one place where the choice was to add no refusal. Every new path falls back to the old one when no folder is active. |
| Unconfigured paths stay byte-identical | HOLDS | `TestWorktreeFolderWithoutAMachineFolderIsNotATask`, `TestLaunchdLogUnconfiguredStaysInTheGridHome`, and the bundle test's no-folder case (3 entries). |
| Existing tests not edited; new tests fail on base | HOLDS | Above. |

## Runbook checks

| Check | Result | Proof |
|-------|--------|-------|
| Tests of changed packages | HOLDS | Changed: `machinefolder`, `where`, `routine`, `doctor`, `gridconfig`, `brain` (a comment), `cli`. The first four and `gridconfig` ran with `-race` (above). |
| Tests of every importer | HOLDS | `go list -f '{{$p := .ImportPath}}{{range .Imports}}{{if or (eq . ".../machinefolder") (eq . ".../where") (eq . ".../routine") (eq . ".../doctor") (eq . ".../gridconfig") (eq . ".../cli")}}{{$p}} imports {{.}}{{"\n"}}{{end}}{{end}}' ./...` printed: `cmd/grid` imports cli, routine; `braincmd` imports where; `cli` imports doctor, machinefolder, routine, where; `doctor` imports gridconfig, machinefolder; `feed` imports machinefolder; `gridconfig` imports machinefolder; `gridhome` imports gridconfig; `guard`, `hook`, `job/jobcmd`, `job/jobtest`, `land`, `projectmd`, `projects`, `settings`, `signal`, `task`, `updates`, `workflow` import where; `question` imports machinefolder, where; `lib` imports cli. All ran, exit 0, without `-race`: `braincmd` 10.508s, `cli` 144.282s, `feed` 54.654s, `guard` 8.523s, `hook` 22.049s, `job/jobcmd` 13.339s, `land` 6.252s, `projectmd` 1.192s, `projects` 6.117s, `question` 12.428s, `settings` 4.473s, `signal` 1.820s, `task` 25.799s, `workflow` 19.425s, `brain` 10.454s, `lib` 14.054s; `cmd/grid`, `job/jobtest` and `updates` have no test files. `gridhome` ran in the race run. |
| `make lint` | HOLDS | `make lint` ran `go vet ./...` and printed nothing else (exit 0, no failure line). `gofmt -l internal cmd lib` prints nothing. |
| Windows and macOS builds | HOLDS | `env GOOS=windows CGO_ENABLED=0 go build ./...` exit 0 and `env GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0; `go vet` for both exit 0. |
| Walk tests after the merge | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` printed `ok  	github.com/gridconsole/grid-plugins/internal/cli	0.094s`; `go test -count=1 ./internal/job/lint/` printed `ok  	github.com/gridconsole/grid-plugins/internal/job/lint	0.218s`. No command or flag was added. |
| App repo checks | not run, not asked | The brief makes item 6 a separate app lane. |
| `go test ./...` over the whole module | not run, not asked | The brief names the touched packages; those and their importers ran. |
| Live check with two logins on a shared folder | not run, not asked | A step after landing. |

## Files outside the five packages of the brief

- `internal/cli/routine.go`, `internal/cli/projects.go`, `internal/cli/doctor.go`: one line each, `Machine: env.MachineFolder, Login: env.Login`. Forced: without it the two new fields stay empty and the new behaviour never runs.
- `internal/cli/orchestrator.go`: the import restored after the merge (merge fix 1).
- `internal/gridconfig/gridconfig.go`: `MachineDirFor` delegates to `machinefolder.DefaultPath` (merge fix 2; `TestOneResolver` demands it).
- `internal/brain/index.go`: a comment only (row 5, named by the brief).

`internal/machinefolder/place.go` and `check.go` are in the named package.

## What changed

- `machinefolder/place.go`: `Folder.WorktreeParts`, `Folder.RoutineLogDir`. `machinefolder/check.go`: a local `realPath`.
- `where`: `worktreeTask` reads the machine folder's worktrees; `worktreeNamed` is the old ULID check as a helper.
- `routine`: `TimerSpec` and `Scheduler` gained `Machine` and `Login`; `makeLogDir` makes the log directory.
- `doctor`: `BundleOptions` gained `Machine` and `Login`; `bundleFiles` became `collectBundleFiles` with a name filter (`bundleFiles` stays as the nil-filter call).
- New test files: `where/pc_machine_worktree_test.go`, `routine/pc_machine_log_test.go`, `doctor/pc_machine_bundle_test.go`.

## Open items

1. **migrate (row 4)**: NOT DONE, with the split named there. This lane is NOT DONE for that row only.
2. **Two resolvers on main.** `gridconfig` / `gridhome.Home.Machine` read the config key `machineFolder` (camelCase) and `machinefolder.Resolve` reads `machine_folder`. They can disagree when the config sets only one of them. This lane branches only on `machinefolder.Resolve`; never branch on `Home.Machine`, because it is set by default and unconfigured paths would move. A lane that makes one of the two the owner is needed.
3. **Not on main**: the P-shared-home-2 doctor rows and `grid setup --shared-group` (no use of `SharedGroup` outside `machinefolder`). Open item 7 of P-shared-home-3 stays open.
4. **The app** reads `feed/state.json` and the logs from the Grid home; it must read the machine folder (item 6, a separate app lane).
5. **A new failure path in `grid doctor --bundle`.** With an active folder and a known login, `doctor.Bundle` returns an error if `Launcher(login)` or the walk of `launcher/<login>/` fails (`bundle.go:88-95`), so `--bundle` can fail where it succeeded on base. A missing `launcher/<login>/` is not an error (`collectBundleFiles` returns nothing). If the owner wants the bundle to be written without those logs, the two `return` lines become a skip; not changed here because that is the owner's call.
6. **Worktrees folder maker**: nothing in this repo makes `worktrees/<project>` (see P-shared-home-3 item 5b); `where` only reads it.

## NOT VERIFIED

- A live run with two real logins on a real shared folder. The tests use a temp folder.
- macOS and Windows runtime: only built and vetted there (`go vet` compiles the tests too).
- `go test ./...` over the whole module: not run, not asked.

NOT DONE

# Lane P-shared-home-5: one config key for the machine folder, a bundle that skips missing launcher logs, and migrate (not built)

Result: **NOT DONE**. Two of the three items are built and proven: the one resolver and its one config key (row 2) and the best-effort support bundle (row 3). Row 1, migrate relocating the old worktrees into the machine folder, is **not built**: the tool budget of the brief (about 70 calls) was used up on rows 2 and 3 and on reading `internal/migrate`, and the budget rule says to stop building then. That code moves a person's data and has a crash journal, so a half-built version would be worse than none. The split for the next lane, with the design, is under row 1 and in "Open items".

Code head: `d05abd57` (the last code commit); the doc commit comes after it and the tip of the bundle is the lane head. The commits of this lane are `7286661b` (row 3) and `d05abd57` (row 2), then the doc commit. The bundle range `origin/main..lane-P-shared-home-5` also carries the earlier shared-home lanes, which are not on main yet.

## How it works

**Row 3, the bundle** (`internal/doctor/bundle.go`). The launcher logs of this login in the machine folder are collected by `machineLauncherLogs` (the old `Launcher(login)` plus the walk of `launcher/<login>/`). When either fails, `Bundle` no longer returns the error: it keeps the logs of the Grid home, adds one entry `skipped.txt` with the line `skipped the launcher logs of <login> in the machine folder: <why>`, and writes the bundle. `skipped.txt` exists only when something was skipped, so a bundle with nothing skipped has the entries it had (the existing test still sees exactly 5 and 3). The line goes through the same redaction as every entry.

**Row 2, one resolver, one key.**

- The keys are `machineFolder` and `sharedGroup`. `machine_folder` and `shared_group` are not read any more: Grid is not released, so no config holds them (the brief's decision). A config that still holds them names no folder (`TestOldSnakeCaseKeysAreNotRead`).
- `gridconfig` parses and writes both keys (`Config.MachineFolder`, `Config.SharedGroup`; a value that is not text is read as empty and listed in `Config.NotText`, so `Resolve` still reports a Problem).
- `machinefolder.Resolve` stays the one resolver (env `GRID_MACHINE_DIR`, then the config, then the OS default). It reads the config through `gridconfig.Load(gridconfig.PathFor(...), gridconfig.OldPath(...))`, the call `gridhome` uses, so there is no second parser of the file. `ReadConfig` and `WriteConfig` are kept with their signatures; `WriteConfig` loads the config (adopting the older `~/.grid/config.json` once, as every reader does), sets the two keys, and writes through `gridconfig.Write` (version 3, a `.v<n>` copy of an older file, 0600, atomic rename). It refuses a file that is not a JSON object and a file still in the version 1 shape, so nothing of a person's is replaced or lost.
- `gridhome.Home.Machine` and `MachineSource` come from `machinefolder.Resolve` (`gridhome.go`, `resolve`); `Home.Machine` is kept because `LauncherDirs`, `FindLauncherDir`, `cmd/grid/main.go` and the doctor home row read it.
- New leaf package `internal/machinefolder/osplace`: the two key names, the OS default path and `Absolute` (a leading `~/` is the login's home). `gridconfig` imports it instead of `machinefolder`, which removes the edge `gridconfig` to `machinefolder` and lets `machinefolder` import `gridconfig` with no cycle. It lives under a folder named `machinefolder`, so `TestOneResolver` (which skips that whole subtree) is unchanged and still passes: nothing outside the subtree spells the key or the OS path.
- `Config.MachineDir` and `MachineDirFor` stay (existing tests use them) and use `osplace`, so a config holding only `machineFolder` gives one folder from `machinefolder.Resolve`, `Home.Machine` and `Config.MachineDir`, including for a `~/` value.
- A path that is not configured does not move: with no config and no env, `Home.Machine` is the OS default with `MachineSource` default and `Folder.Active()` is false, so every per-machine file stays where it was. `TestUnconfiguredMachineFolderStaysPut` pins it (it also passes on base, by design).

Behaviour that changes, on purpose: `GRID_MACHINE_DIR` now reaches `Home.Machine`, so `LauncherDirs`, `FindLauncherDir` and `cmd/grid/main.go` see an env-named folder too (before, only the config did); the doctor home row says `from the environment` for it (`doctor/pc_home_source.go`; no test reads that text, see NOT VERIFIED). A `~/` in `machineFolder` is the login's home in every reader (before, `Config.MachineDir` ignored it). The 1 MiB bound that the old private parser put on the file is gone; `gridconfig` reads the file as `gridhome` always did.

## Proof table

Rows of the brief:

| # | Row | Result | Proof |
|---|-----|--------|-------|
| 1 | migrate moves the old `<home>/worktrees/<name>` folders into `<machine>/worktrees/<project>`: an `Action` with an absolute target outside the home, its apply and rollback, `rewriteWorktrees` and `rewriteSessions` against it, one test of each that fails on base | **NOT DONE** | Not built; `internal/migrate` is unchanged. Why: the budget ran out (above). The design, for the lane that builds it, is below this table. |
| 2 | One config key for the machine folder: `machineFolder` and `sharedGroup`, `machinefolder.Resolve` the one resolver reading through `gridconfig`, `Home.Machine` from it, snake_case dropped, an unconfigured path pinned, and a test that a config holding only `machineFolder` gives the same folder from every reader | HOLDS | `internal/gridconfig/gridconfig.go` (`SharedGroup`, `NotText`, `machineKey`, `MachineDir`), `internal/machinefolder/machinefolder.go` (`load`, `ReadConfig`, `WriteConfig`, `Resolve`), `internal/machinefolder/osplace/osplace.go`, `internal/gridhome/gridhome.go` (`resolve`). Tests: `internal/gridhome/pc_machine_one_folder_test.go` `TestConfigWithOnlyMachineFolderGivesOneFolderFromEveryReader` (absolute and `~/`), `TestHomeMachineFollowsTheEnvOverride`, `TestUnconfiguredMachineFolderStaysPut`; `internal/machinefolder/pc_config_keys_test.go` `TestResolveBadConfigFallsBackCamelCase`, `TestOldSnakeCaseKeysAreNotRead`, `TestWriteConfigKeepsOtherKeysCamelCase`, `TestWriteConfigAdoptsTheOlderConfig`. Base failures below. `grid setup` writes nothing of the two keys today and has no `--shared-group` flag (see "Open items"), so there is no caller to change. |
| 3 | `grid doctor --bundle` is best effort: a failing `Launcher(login)` or walk of `launcher/<login>/` adds a line naming what was skipped and why, and the bundle is written; a test with an unreadable launcher folder | HOLDS | `internal/doctor/bundle.go` (`machineLauncherLogs`, `skipped.txt`); `internal/doctor/pc_bundle_skip_test.go` `TestBundleSkipsAnUnreadableLauncherFolderAndSaysSo` (mode 000 on `launcher/<login>`: bundle written, entries `doctor.txt,logs/grid-open.log,skipped.txt,versions.txt`, `skipped.txt` names the login and `permission denied`; skipped on Windows and as root) and `TestBundleSkipsALauncherThatHasNoPlainLoginName` (the `Launcher(login)` failure). Base failures below. |
| 4 | Merge origin/main and say how it was resolved | HOLDS | `git fetch origin` then `git merge origin/main` printed `Already up to date.` (the fetch brought in other lanes' branches only). Nothing to resolve. |
| 5 | `go test -race` of the packages that exist, and `go vet ./...` | HOLDS | Race run below, exit 0; `go vet ./...` exit 0. `internal/setup` does not exist (`ls internal/setup` exit 2), so it is skipped as the brief says. |
| 6 | P-shared-home-4: the NOT DONE row 4 and open items 2 and 5 rewritten | HOLDS for items 2 and 5; row 4 stays NOT DONE | `docs/lanes/P-shared-home-4.md` open items 2 and 5 now say CLOSED BY P-shared-home-5. Row 4 is not rewritten to closed, because it is not closed: open item 1 there now points to row 1 here. |

### Row 1: the design for the split

Brief for the lane that builds it: "migrate relocates the old worktrees folders into the machine folder". Invariants that came out of reading `plan.go`, `apply.go`, `recover.go` and `rollback.go`:

1. Option, not a call: `migrate.WithMachineFolder(machinefolder.Folder)`, passed from `internal/cli/migrate.go:65-75` (`env.MachineFolder`); `Scan` never calls `Resolve`. An inactive folder gives today's plan byte for byte (the existing migrate tests and goldens are that pin). The target comes from `Folder.WorktreesDir(home, project)` (`worktrees/<project>`), not spelled in migrate. A target inside the home or the source is not relocated.
2. Outside entries live in their own tree, not in `p.result` (which is the exact set verified for the staged home). Audit every loop that indexes `p.result[a.To]` or walks `a.To` upward with `path.Dir`: the in-place `used` loop (`plan.go:264-276`), `rewriteWorktrees`, `rewriteSessions`. Setting `paths[<old worktrees dir>] = <machine>/worktrees/<project>` points both rewrites at the new place.
3. Apply: copy and verify the outside tree (`writeEntries`, `verifyEntries`, a copy, not a rename) into `<machine>/worktrees/.migrate-stage-<token>/<project>` before the `publishing` journal; the journal (a new `Externals` field) names the targets before the first byte lands there, so `fail`, `discard` and `back` can remove them; rename the child into place in `finishPublication` before `applyPointers`, so the pointer rewrite never names a folder that is not there yet.
4. Never overwrite or delete what this run did not create: a plan Conflict when the target child exists; `os.Mkdir` for the child; remove only journaled children; a `validJournal` check that every external is a child of `<machine>/worktrees` with a clean name.
5. Rollback: verify the outside copy is unchanged (digests in the receipt), reverse the pointers, then remove the outside copy; repeatable after a crash with a verified flag in the style of `PreviousVerified`.
6. Four tests, in new files, each failing on base: apply (the folder is in the machine folder), rollback (the old path is back and the machine child is gone), the worktree pointer rewrite (`git worktree list --porcelain` names the machine path), the session `cwd` rewrite. Crash tests with `crashAt` at each new step.

## Runs

Race run, as printed (exit 0): `go test -race -count=1 ./internal/machinefolder/... ./internal/gridconfig/... ./internal/gridhome/... ./internal/migrate/... ./internal/doctor/...`

```
ok  	github.com/gridconsole/grid-plugins/internal/machinefolder	2.124s
?   	github.com/gridconsole/grid-plugins/internal/machinefolder/osplace	[no test files]
ok  	github.com/gridconsole/grid-plugins/internal/gridconfig	1.038s
ok  	github.com/gridconsole/grid-plugins/internal/gridhome	1.184s
ok  	github.com/gridconsole/grid-plugins/internal/migrate	19.452s
ok  	github.com/gridconsole/grid-plugins/internal/doctor	37.004s
```

`internal/machinefolder` includes `TestOneResolver`: it passes, unedited.

New tests on base. Each fix was taken out with `git checkout HEAD~1 -- <files>` and put back with `git restore --source=HEAD --staged --worktree -- <files>`; `git status --short` was empty afterwards. Row 3 (base `bundle.go`):

```
--- FAIL: TestBundleSkipsAnUnreadableLauncherFolderAndSaysSo (0.00s)
    pc_bundle_skip_test.go:56: an unreadable launcher folder must not stop the bundle: open <temp>/002/launcher/alice: permission denied
--- FAIL: TestBundleSkipsALauncherThatHasNoPlainLoginName (0.00s)
    pc_bundle_skip_test.go:75: a login that is not a plain name must not stop the bundle: login is not one plain name: a/b
```

Row 2 (base `machinefolder.go`, `place.go`, `gridconfig.go`, `gridhome.go`, `pc_home_source.go`):

```
--- FAIL: TestResolveBadConfigFallsBackCamelCase/folder_is_a_number (0.00s)
    pc_config_keys_test.go:37: a config that cannot be read falls back to the default and says why: {Path:<temp>/.local/state/grid Source:default SharedGroup: Problem:}
--- FAIL: TestOldSnakeCaseKeysAreNotRead (0.00s)
    pc_config_keys_test.go:58: the old spellings name nothing: {Path:/srv/grid Source:config SharedGroup:grid Problem:}
--- FAIL: TestWriteConfigKeepsOtherKeysCamelCase (0.00s)
    pc_config_keys_test.go:81: config lost "machineFolder": "/srv/grid":   (base wrote "machine_folder" and "shared_group" beside the old "machineFolder")
--- FAIL: TestWriteConfigAdoptsTheOlderConfig (0.00s)
    pc_config_keys_test.go:120: the new config must hold the older one's workspaces and the key:  {"machine_folder": "/srv/grid"}
--- FAIL: TestConfigWithOnlyMachineFolderGivesOneFolderFromEveryReader/absolute (0.00s)
    pc_machine_one_folder_test.go:50: machinefolder.Resolve = {Path:<temp>/.local/state/grid Source:default ...}, want <temp>/shared from the config
--- FAIL: TestConfigWithOnlyMachineFolderGivesOneFolderFromEveryReader/home (0.00s)
    (same: Resolve gives the default, the config names <temp>/shared)
--- FAIL: TestHomeMachineFollowsTheEnvOverride (0.00s)
    pc_machine_one_folder_test.go:70: Home.Machine = "<temp>/002/shared" (config), want <temp>/003/other (env)
```

`TestUnconfiguredMachineFolderStaysPut` passes on base and on the lane: it is the pin, not a fix. The two cases `array` and `not json` of `TestResolveBadConfigFallsBackCamelCase` also pass on base.

Existing tests: none edited. Two were deleted from `internal/machinefolder/resolve_test.go`, by the owner's direction in the brief that `machine_folder` and `shared_group` are dropped (they asserted those spellings): `TestResolveBadConfigFallsBack` (replaced by `TestResolveBadConfigFallsBackCamelCase`, same three cases with the new key, plus a non-text `sharedGroup`) and `TestWriteConfigKeepsOtherKeys` (replaced by `TestWriteConfigKeepsOtherKeysCamelCase`). `TestWriteConfigRefusesAFileThatIsNotAnObject` and `TestWriteConfigNewFileIsPrivate` still run, unedited, and pass on the new `WriteConfig`; `pc_resolve_test.go` of `gridhome`, `gridconfig_test.go` and `hook/pc_machine_launcher_test.go` pass unedited.

## Rules

| Rule | Result | Proof |
|------|--------|-------|
| Never push | HOLDS | No `git push`; the result leaves as a bundle. |
| Never touch /srv/gmf, /root/.grid, /srv/grid, other users' homes, another lane's clone | HOLDS | Every command ran in this clone; the new tests use `t.TempDir()`. (The string `/srv/grid` in the tests is a value written into a temp config, never opened.) |
| Never sudo, never `pkill -f` | HOLDS | None run. |
| One command per Bash call | HOLDS | Two calls the shell refused (a `sed`/`tr` pipeline and an `echo` after `;`) were re-issued as single commands. |
| Commits: small, one sentence, no attribution lines | HOLDS | `git log --format=%s origin/main..HEAD` lists one-sentence commits; no `Co-Authored-By` or session line was added, as the owner's rule says. |
| No personal paths, hosts or emails in committed files | HOLDS | The doc and tests use `<temp>` and `t.TempDir()`. |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | None added. Row 3 removes a refusal; row 2 adds none. |
| Unconfigured paths stay byte-identical | HOLDS | `TestUnconfiguredMachineFolderStaysPut`; the bundle test of P-shared-home-4 (3 entries without a folder, 5 with) still passes unedited. |
| Stop at 85% of the budget and say so | HOLDS, late | The rule's first check was missed: the brief should have been split at call 10, before reading `migrate`. It is split here instead, with the design above. |

## Runbook checks

| Check | Result | Proof |
|-------|--------|-------|
| Tests of changed packages | HOLDS | Changed: `machinefolder` (and new `osplace`), `gridconfig`, `gridhome`, `doctor`. All four ran with `-race` (above), exit 0. |
| Tests of every importer | HOLDS | `go list -f '{{$p := .ImportPath}}{{range .Deps}}{{if eq . ".../internal/gridconfig"}}{{$p}}{{"\n"}}{{end}}{{end}}' ./...` printed 46 packages, which with `gridconfig` imports are the importers of `machinefolder` and `gridhome` too: `cmd/grid`, `account`, `agent`, `approvefix`, `brain`, `braincmd`, `cli`, `doctor`, `env`, `feed`, `gitx`, `gridhome`, `guard`, `hook`, `job`, `job/jobcmd`, `job/jobtest`, `job/lint`, `jobgraph`, `land`, `lease`, `localdata`, `machinefolder`, `mcp`, `migrate`, `orchestrator`, `packaging`, `packaging/cmd`, `people`, `ports`, `project`, `projectenv`, `projectmd`, `projects`, `proof`, `question`, `routine`, `settings`, `signal`, `task`, `updates`, `usage`, `where`, `worker`, `workflow`, `lib`. Run as `go test -count=1 ./...` over the whole module; result in the next row. |
| `go test -count=1 ./...` over the whole module | HOLDS | Exit 0 (`[exited with code 0]`), 81 package lines, every one `ok` or `no test files`, none `FAIL`; it covers all the importers above, `internal/migrate` (27.986s), `internal/cli` (292.776s) and `internal/orchestrator` (409.389s) included. |
| `make lint` | HOLDS | `make lint` ran `go vet ./...` and printed nothing else (exit 0). |
| `go vet ./...` | HOLDS | exit 0. |
| Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0; `go vet` on both exit 0. |
| Walk tests after the merge | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` printed `ok  	github.com/gridconsole/grid-plugins/internal/cli	0.194s`; `go test -count=1 ./internal/job/lint/` printed `ok  	github.com/gridconsole/grid-plugins/internal/job/lint	0.595s`. No command or flag was added. |
| RealPlugin tests | not run, not asked | No row names them. |
| App repo checks | not run, not asked | The brief leaves the app to a separate lane. |
| Live check with two logins on a shared folder | not run, not asked | A step after landing. |

## Files outside the named packages

None. `internal/cli` is not touched (row 1, which needed the one line in `cli/migrate.go`, is not built). `internal/doctor/pc_home_source.go` is in `doctor`, a named package. Docs: `docs/lanes/P-shared-home-5.md` and the three rewritten items of `docs/lanes/P-shared-home-4.md`.

## What changed

- `doctor`: `Bundle` skips a launcher folder it cannot read and says so in `skipped.txt`; the home row names `from the environment` for an env-named folder.
- `gridconfig`: `SharedGroup`, `NotText`; imports `osplace`, not `machinefolder`.
- `machinefolder`: reads and writes the two keys through `gridconfig`; `osplace` holds the keys, the OS default and `Absolute`.
- `gridhome`: `Home.Machine` and `MachineSource` from `machinefolder.Resolve`.
- New test files: `doctor/pc_bundle_skip_test.go`, `machinefolder/pc_config_keys_test.go`, `gridhome/pc_machine_one_folder_test.go`.

## Open items

1. **migrate (row 1)**: NOT DONE, with the design above. It is the reason this lane ends NOT DONE. It needs `internal/cli/migrate.go` (one line, `migrate.WithMachineFolder(env.MachineFolder)`) as a file outside the named packages.
2. **The app reads the machine folder**: not this lane, still open (the app reads `feed/state.json` and the logs from the Grid home).
3. **Worktrees folder maker**: not this lane, still open; nothing in this repo makes `worktrees/<project>`, `where` only reads it.
4. **No `grid setup --shared-group`**: no flag or caller exists, and nothing calls `machinefolder.WriteConfig` in production, so the new `sharedGroup` key has no writer outside tests. The brief says `grid setup` writes the keys; there was nothing to change, and adding a flag is a new command that the walk tests would have to learn.
5. **One unreadable log still stops the bundle.** Row 3 covers the two failures the brief names (`Launcher(login)` and the walk). A single launcher log that exists but cannot be read still fails at `readBundleTail`, as a Grid home log does today.

## NOT VERIFIED

- A live run with two real logins on a real shared folder; the tests use temp folders.
- The doctor home row text `machine folder ... from the environment`: no test reads it.
- macOS and Windows runtime: only built and vetted there.
- The bundle test needs a folder the login cannot read, so it is skipped on Windows and as root; on those it proves nothing, and the second test (`Launcher(login)` failing) still runs.

NOT DONE

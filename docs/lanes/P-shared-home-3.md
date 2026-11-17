# Lane P-shared-home-3: the per-machine files move onto the shared machine folder

Result: **NOT DONE**. Rows 2 and 3 hold. Row 1 does not: the launcher lock and log, the task `state.json` and the Job worktrees folder now go through `machinefolder.Resolve`, and so does the `job verify` lookup of a recorded worktree. The brain cache is not switched (on purpose, see Open items). The launcher pid and the videos have no writer in this repo, so there was nothing to switch. The open items are at the end.

Code head: `ce60ffdc` (the last code commit). The doc commit comes after it; the tip of the bundle is the lane head. The bundle range `origin/main..lane-P-shared-home-3` also carries the earlier lane P-shared-home (the package `internal/machinefolder`, commits `abfe3a29` to `61264bf9`), because that lane is not on main yet. The commits of this lane are `7e881a34`, `dd9ed3bf` (merge of origin/main), `7dde556f` and `ce60ffdc`.

## How it works

`internal/machinefolder/place.go` (new) is the one place that knows both the old path and the new one for each file. Callers ask it and do not join the path themselves.

- A `Folder` is **active** when `Path != "" && (Source != SourceDefault || Shared())`. This is `Folder.Active()`. It is deliberately not `Configured()`: a config with a `Problem`, or the private OS default place, never moves a file. A zero `Folder` is not active, so every old path stays as it was and every existing test stays hermetic.
- `cli.OSEnv()` resolves the folder once (`Env.MachineFolder`) and the login once (`Env.Login`, falling back to `machinefolder.CurrentLogin`). `feed.Store`, `question.Store`, `question.Env`, `job.VerifyOptions` (a `WorktreesDir` func, nil means the old path) and `jobcmd.Env` carry it down.
- Shared mode: directories `2770` (`os.ModeSetgid|0o770`), files `0660`, by explicit chmod so the umask cannot change them. Shared writes go through `internal/atomicfile` (write a temp file, rename).

## Proof table

Acceptance rows (the brief):

| # | Row | Result | Proof |
|---|-----|--------|-------|
| 1 | Every listed writer uses `Resolve`; grep shows no other path computation | NOT DONE | Why: the brain cache still writes inside the brain root (`internal/brain/index.go:247` and `:251`, `atomicWrite` under `locked(root)`). Switched: the launcher lock (`open_start.go:136`), the launcher log (`open_start.go:239`), `state.json` (`feed/fold.go:86`, `:135`), the Job worktrees dir (`cli/orchestrator.go:693`, `job/verify_paths.go:71`). No writer exists for the launcher pid or the videos (grep below). The grep for the other computations is in the writer table. |
| 2 | Unconfigured paths byte-identical (tested); `TestOneResolver` passes | HOLDS | `TestPlaceUnconfiguredPathsAreTheOldOnes`, `TestStartLockUnconfiguredIsTheOldFile`, `TestStartDetachedUnconfiguredKeepsTheOldLog`, `TestOpenStartLockAndLogsStayInTheGridHomeWithoutAMachineFolder`, `TestSaveStateUnconfiguredWritesFeedStateJSON`, `TestWorktreePathInFollowsTheMachineFolder` (zero folder, default folder and a `Problem`-only folder all give the old path). The race run below includes `TestOneResolver` in `internal/machinefolder`: exit 0. |
| 3 | Two fake logins share one folder: each writes its launcher files, neither breaks the other's, `state.json` has no partial read under `-race` | HOLDS | `go test -race -count=1` of `internal/machinefolder internal/feed internal/question internal/job internal/job/frontmatter internal/job/jobcmd internal/job/lint internal/cli` exit 0 (times below). Tests: `TestStartDetachedTwoLoginsShareOneMachineFolder` (two logins write `launcher/<login>/<name>.log`, neither log is touched by the other), `TestStartLockTwoLoginsShareOneMachineFolder` (two logins hold `launcher/<login>/lock` at the same time; the same login is refused a second time), `TestOpenStartLockAndLogsGoToTheMachineFolder`, `TestSaveStateTwoLoginsShareOneMachineFolder` (two stores on one folder, 15 distinct Jobs each, 4 reader goroutines parse `state.json` the whole time and never see a partial file; modes `0660` and `2770` under umask 077; no `feed/state.json`; `needs-you.json` stays in `feed/`). |

The race run, as printed (exit 0):

```
ok  	github.com/gridconsole/grid-plugins/internal/machinefolder	1.632s
ok  	github.com/gridconsole/grid-plugins/internal/feed	104.006s
ok  	github.com/gridconsole/grid-plugins/internal/question	17.912s
ok  	github.com/gridconsole/grid-plugins/internal/job	15.550s
ok  	github.com/gridconsole/grid-plugins/internal/job/frontmatter	1.095s
ok  	github.com/gridconsole/grid-plugins/internal/job/jobcmd	22.834s
?   	github.com/gridconsole/grid-plugins/internal/job/jobtest	[no test files]
ok  	github.com/gridconsole/grid-plugins/internal/job/lint	4.574s
ok  	github.com/gridconsole/grid-plugins/internal/cli	235.699s
```

Writer table (what each writer used before, and now):

| File | Writer (file:line) | Old path, kept when no folder is set | Path with a machine folder | Result |
|------|--------------------|---------------------------------------|----------------------------|--------|
| Launcher lock | `internal/cli/open_start.go:136` (`StartLockPath`, `OpenStartLock`) | `<grid home>/open-start.lock` | `launcher/<login>/lock` | switched |
| Launcher log | `internal/cli/open_start.go:239` (`LogPath`, `OpenLog`) | `<grid home>/logs/<name>.log` | `launcher/<login>/<name>.log` | switched |
| `state.json` | `internal/feed/fold.go:86` (`stateFile`), `:135` (publish through `Machine.WriteStateFile`) | `<feed>/state.json` | `tasks/<task>/state.json`, atomic rename | switched |
| Job worktrees dir | `internal/cli/orchestrator.go:693` (`configureTickStore`) | `<grid home>/projects/<project>/worktrees` | `worktrees/<project>` | switched |
| Recorded worktree lookup | `internal/job/verify_paths.go:71` (`WorktreePathIn`) | same old path | same new path | switched |
| Brain cache | `internal/brain/index.go:247`, `:251` | inside the brain root | not changed | NOT DONE |
| Launcher pid | none | none | none | no writer |
| Videos | none (`internal/proof/proof.go:129`, `:141` only accept a video as a proof file copied into the task proof folder) | none | none | no writer |

The wiring that carries the folder: `internal/cli/cli.go` (`Env.MachineFolder`, `Env.Login`, `OSEnv`), `internal/cli/feed.go:223`, `internal/cli/orchestrator.go:490`, `internal/cli/question.go:28`, `internal/cli/job.go:27`, `internal/question/question.go`, `internal/question/feed.go:29`, `internal/question/command.go`, `internal/job/verify.go`, `internal/job/jobcmd/command.go:48`, `internal/job/jobcmd/verify.go:41`.

Grep proof that no other code computes these paths (non-test Go files under `internal`, `cmd`, `lib`):

`grep -rn -E 'open-start\.lock|"state\.json"|"logs"|"worktrees"' internal cmd lib --include=*.go --exclude=*_test.go` hits:

- `internal/machinefolder/place.go` and `files.go`: the one place (`place.go:33`, `:60`, `:91`, `:111`, `:113`, `files.go:186`, `:193`).
- `internal/cli/open_start.go:26`: the constant `OpenStartLock`, used only as a name in messages; the path comes from `StartLockPath`.
- `internal/job/verify_paths.go:44`: the old path, used only when the caller passes no `WorktreesDir` func.
- `internal/migrate/plan.go:216-228`: the migration that reads the old layout, left alone on purpose.
- `internal/where/where.go:327`: a different layout (`<grid home>/worktrees/<ulid>`), left alone.
- `internal/doctor/bundle.go:75`: reads `<grid home>/logs` for the support bundle (see Open items).
- `internal/routine/timer_plist.go:23`: launchd routine logs, one set per login, left alone.

`grep -rn -E '"pid"\)|\.pid"|\.mp4|\.webm|"videos"' internal cmd lib --include=*.go --exclude=*_test.go` hits only `machinefolder` (`files.go:189`, `launcher.go:83`), a test server fixture (`internal/env/testdata/server/main.go:64`) and the proof file extension list (`internal/proof/proof.go:129`, `:141`). The other `pid` hits in a wider grep are JSON record fields (session, worker, ports, lease), not files in a machine folder.

Rules:

| Rule | Result | Proof |
|------|--------|-------|
| Never push | HOLDS | No `git push` was run; the result leaves as a bundle. |
| Never touch `/srv/gmf`, `/root/.grid` or another user's home | HOLDS | Every command ran inside the lane folder or a Go temp dir; the new tests use `t.TempDir()`. |
| Never `pkill -f`, never sudo | HOLDS | None run. The package prints sudo commands as text only (`Finding.Fix`). |
| Commits: small, one sentence, no attribution lines | HOLDS | `git log --oneline origin/main..HEAD` lists the lane commits, each one sentence; `git log --grep=Co-Authored --grep=Claude --grep=Generated --oneline origin/main..HEAD` prints nothing. |
| No personal paths, hosts or emails in committed files | HOLDS | The doc and the code use `/srv/grid` as the only example path. |
| Existing tests not edited | HOLDS | `git diff --stat origin/main...HEAD` shows only new `*_test.go` files, plus the lane doc of the earlier lane. |
| New tests fail on base and pass on the lane | HOLDS | Failing lines below; all pass on the lane (race run above). |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | No hook, gate or refusal was added. A bad config or an unreachable folder falls back to the old path. |
| Shared writes are atomic renames; shared dirs 2770 and files 0660 | HOLDS | `state.json` goes through `WriteStateFile` (`internal/atomicfile`); the modes are asserted under umask 077 in `TestSaveStateTwoLoginsShareOneMachineFolder` and `TestStartLockTwoLoginsShareOneMachineFolder`. |

New tests on base (the new test files built against the code without this lane's non-test changes, which has no machine folder wiring; each package fails to build, so no new test passes there):

```
internal/feed/pc_machine_state_test.go:24:13: logins[i].Machine undefined (type Store has no field or method Machine)
internal/cli/pc_machine_log_unix_test.go:29:19: env.MachineFolder undefined (type Env has no field or method MachineFolder)
internal/machinefolder/place_test.go:27:8: f.Active undefined (type machinefolder.Folder has no field or method Active)
internal/machinefolder/place_lock_unix_test.go:22:18: f.StartLockPath undefined (type machinefolder.Folder has no field or method StartLockPath)
internal/job/pc_machine_worktree_test.go:23:15: undefined: WorktreePathIn
FAIL ... [build failed]
```

The cli lock test (`pc_machine_lock_unix_test.go`) sets `env.MachineFolder` and `env.Login`, so it fails to compile on base in the same way as the log test. The test files carry the prefix `pc_machine_` so they are new files.

Runbook checks:

| Check | Result | Proof |
|-------|--------|-------|
| Tests of changed packages and every importer | HOLDS | Changed packages: `machinefolder`, `feed`, `question`, `job`, `job/jobcmd`, `cli`. `go list -f '{{$p := .ImportPath}}{{range .Imports}}{{if or (eq . ".../machinefolder") (eq . ".../feed") (eq . ".../question") (eq . ".../job") (eq . ".../job/jobcmd") (eq . ".../cli")}}{{$p}} imports {{.}}{{"\n"}}{{end}}{{end}}' ./...` printed: `cmd/grid` imports cli; `approvefix` imports feed; `cli` imports feed, job, jobcmd, machinefolder, question; `feed` imports machinefolder; `hook` imports feed, job; `jobcmd` imports feed, job; `job/lint` imports job; `jobgraph` imports job; `lease` imports feed, job; `mcp` imports feed; `migrate` imports feed, job; `orchestrator` imports feed, job, question; `ports` imports job; `question` imports feed, machinefolder; `task` imports feed, question; `usage` imports feed; `lib` imports cli. The changed packages ran with `-race` (above). The other importers ran without it, exit 0: `approvefix` (no test files), `hook` 8.481s, `lease` 0.252s, `mcp` 0.725s, `migrate` 10.919s, `orchestrator` 194.351s, `task` 8.605s, `usage` 0.343s, `jobgraph` 0.407s, `ports` 2.599s, `cmd/grid` (no test files), `lib` 16.197s. |
| `-race` where concurrency changed | HOLDS | `feed` (state.json writers and readers), `machinefolder`, `question`, `cli`, `job`: exit 0 in the race run above. |
| `make lint` | HOLDS | `go vet ./...` and the gofmt check: exit 0. |
| Windows and macOS builds | HOLDS | `env GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `env GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. `make cross` (vet and test compile for darwin/arm64 and windows/amd64 over every package) printed `ok` for each package and no failure. Tests that need unix modes, groups or flock carry a `unix` build tag. |
| Walk tests after `git fetch origin && git merge origin/main` | HOLDS | Merge commit `dd9ed3bf`. After it: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` printed `ok ... 0.109s`, exit 0; `go test -count=1 ./internal/job/lint/` printed `ok ... 0.327s`, exit 0. No command or flag was added, so no golden, contract or walk file changed. |
| Live check on /srv/grid with two logins | not run, not asked | The brief makes it a step after landing. |
| `make test` / `go test ./...` over the whole repo | not run, not asked | The brief names the touched packages and their importers; those ran. |

## What changed

- `internal/machinefolder/place.go` (new): `Active`, `LogPath`, `OpenLog`, `StartLockPath`, `OpenStartLock`, `StateFile`, `WriteStateFile`, `WorktreesDir`. Names that escape the folder (`..`, a separator) are refused (`TestPlaceRefusesNamesThatEscape`).
- `internal/cli`: `grid open` takes its start lock and writes the server and worker logs through the folder; `Env` carries the folder and the login; the feed, question and orchestrator stores get `Machine`.
- `internal/feed`: `Store.Machine`; the task `state.json` is published through the folder (the `reader.json` and `needs-you.json` files stay in `feed/`).
- `internal/question`, `internal/job`, `internal/job/jobcmd`: the folder is plumbed; `WorktreePathIn` looks for a recorded worktree under the folder.
- New test files (all new, none existing edited): `place_test.go`, `place_lock_unix_test.go`, `pc_machine_lock_unix_test.go`, `pc_machine_log_unix_test.go`, `pc_machine_state_test.go`, `pc_machine_worktree_test.go`.

Choices the reviewer may question:

- The log of a program in a shared folder is `launcher/<login>/<name>.log` (the old `logs/<name>.log` per program, moved under the login).
- The start lock is per login (`launcher/<login>/lock`), the same shape as the package's launcher files. Two logins can start a server at the same time; the old single lock kept two `grid open` calls of one Grid home apart, and that still holds for one login. Cross-login serialization of server start is not provided.
- `Active()` and not `Configured()`: a bad config or the private OS default place keeps the old paths, so a typo never hides files.
- The active `OpenStartLock` makes the file through `OpenCreate` (O_EXCL, chmod 0660), closes it, and opens it again with `fslock.OpenNoFollow`, so a planted link is not followed.

## Open items (the reason for NOT DONE)

1. **Brain cache not switched** (`internal/brain/index.go:247`, `:251`). The home-level brain is one person's, so a shared folder would show its summaries to every login. The cache is written inside an `os.Root` under `locked(root)`, so it needs a design for a second root and a place in `brain.Store`; that is a decision for the owner, not a path swap.
2. **Launcher pid:** the plugin has no pid writer. `machinefolder.Launcher.Start` writes one, but `grid open` does not call it; wiring it would add a new refusal (`ErrRunning`) to `grid open`.
3. **Videos:** no writer in this repo (proof only copies a video into the task proof folder).
4. **Left on the old layout** (read or written elsewhere): `internal/where/where.go:327`, `internal/migrate/plan.go:216-228`, `internal/doctor/bundle.go:75` (the support bundle does not include the launcher logs of a machine folder), `internal/routine/timer_plist.go:23`.
5. **Every Store that folds a task must carry the same `Machine`**: `cli/feed.go`, `refoldAfterWrite`, `configureTickStore` and `question`. A store without it writes `<feed>/state.json` as before; two such stores can differ in liveness inside one shared file.
6. **The app repo** reads `feed/state.json` and the logs from the Grid home; it must read the machine folder when one is set.
7. `grid doctor` and `grid setup --shared-group` (open items 1 and 2 of lane P-shared-home) are still not written.

## NOT VERIFIED

- A live run with two real logins on a real shared folder (`/srv/grid`, group `grid`). The tests use a temp folder in the test's own group.
- macOS and Windows runtime: only built, vetted and test-compiled there.
- `go test ./...` over the whole module: not run, not asked.

NOT DONE

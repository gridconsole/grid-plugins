# Lane P-shared-home: one machine folder shared by several logins

Result: **NOT DONE**. The new package `internal/machinefolder` is built and tested: acceptance rows 1, 2, 3, 4, 6 and 7 hold. The doctor findings of row 5 exist as a function (`machinefolder.Check`) but `grid doctor` does not call it, and `grid setup --shared-group` is not written. Nothing outside the package uses it yet. The open items are listed at the end.

Code head: `abfe3a29` (the package and its tests). The lane then merges `origin/main` (82a26411), adds this doc and corrects it; the tip of the bundle is the lane head.

## Proof table

Acceptance rows (the brief):

| # | Row | Result | Proof |
|---|-----|--------|-------|
| 1 | Path resolution per OS from config, env override, default; one resolver | HOLDS | `go test -count=1 ./internal/machinefolder/` exit 0: `TestResolvePerOS` (linux, darwin, windows with and without LOCALAPPDATA), `TestResolveConfigThenEnv`, `TestResolveHomeAndRelative`, `TestResolveBadConfigFallsBack`. `Resolve` is `internal/machinefolder/machinefolder.go:227`. `TestOneResolver` fails if a non-test Go file outside the package holds `GRID_MACHINE_DIR`, `"machine_folder"` or `".local", "state", "grid"`. No other code places a machine folder today: `grep -rn -E 'Application Support\|"\.local", "state"\|GRID_MACHINE_DIR\|machine_folder' --include=*.go --exclude-dir=machinefolder .` hits only Claude's managed-settings path (`internal/settings/claude_user.go:319`), the golden replay home (`internal/golden/replay.go:123`) and env-name allow lists; `grep -rn -E 'UserCacheDir\|UserConfigDir\|XDG_\|\.local/state' --include=*.go` (non-test, outside the package) hits only env passthrough lists, the eval and golden sandboxes and the systemd timer folder (`internal/routine/timer_systemd.go:23`). `internal/localdata` is not a second resolver: its functions (`ID`, `Safe`, `Init`, `Machine`, `Lock`, `LockExisting`) work inside the Grid home that the caller passes (the `VERSION` marker, the machine id, flock files), not in a machine folder. |
| 2 | Shared dirs 2770 and files 0660 in a temp folder in the test's own group; private 0700 and 0600 | HOLDS | `TestSharedModeDirs2770Files0660` (run under umask 022, 077 and 0, so the modes come from chmod and not the umask), `TestPrivateModeDirs0700Files0600`, `TestNeverChmodsOutsideOrExisting` (a folder or file that already exists keeps its permission bits; a path outside the folder is refused and the folder above is not chmod). Exit 0. |
| 3 | Two launchers under different logins keep separate pid and lock files; A's stale lock is untouched by B | HOLDS | `TestLaunchersOfTwoLoginsAreSeparate`, `TestStaleLockOfOneLoginIsLeftAloneByAnother` (bytes and mtime of A's pid, lock and log are the same after B starts, logs and stops). Exit 0. |
| 4 | Concurrent state.json writers by atomic rename; a reader never parses a partial file; 50 runs with -race | HOLDS | `go test -race -count=50 -run TestStateJSONConcurrentWritersReaderNeverSeesPartial ./internal/machinefolder/` printed `ok ... 19.541s`, exit 0. 4 writers, 4 readers, 25 rewrites each, documents of about 160 KB. |
| 5 | Doctor findings for wrong group, missing setgid, private leftover folder, each with the fix command | NOT DONE | Why: `grid doctor` does not run these checks. What exists: `machinefolder.Check` rows with the fix command, tested by `TestCheckWrongGroup` (`sudo chgrp -R <group> <folder>`), `TestCheckMissingSetgid` (`sudo chmod 2770 <folder>`), `TestCheckPrivateLeftoverFolder` (a `cp -a` chain; nothing is moved), `TestCheckLoginNotInGroup`, `TestCheckMissingSharedFolderPrintsSetupCommands`, `TestCheckEveryFailureHasAFixAndChangesNothing`; all exit 0. The leftover row looks at the OS standard place (`DefaultPath`); since no other code places a machine folder today, that is where a private folder would be. |
| 6 | Secrets check over a seeded folder finds none; a planted token file is reported | HOLDS | `TestSecretsCheckFindsNoneInASeededFolder` (folder filled by Grid's own writers: launcher files, log, state.json, brain cache, a worktree, a video), `TestSecretsCheckReportsPlantedFiles` (a planted `ghp_` token file, `.credentials.json`, `.claude/`, `.claude-work/`, `.env`, `.pem`, an `api_key =` file; each hit names the path and the reason, never the token), `TestSecretsCheckDoesNotFollowLinks`. The content check is `brain.LooksSecret`, passed in. Exit 0. |
| 7 | New tests in new files; `go test` for touched packages passes; `go vet` clean | HOLDS | The lane adds 13 Go files (all new, in `internal/machinefolder/`) and this doc: `git diff --stat origin/main...HEAD` shows 14 files and no deletion. `go test -count=1 ./internal/machinefolder/` exit 0; `go test -race -count=3 ./internal/machinefolder/` exit 0; `go vet ./internal/machinefolder/` exit 0; `make lint` (`go vet ./...`) exit 0. |

Brief build items:

| Item | Result | Proof |
|------|--------|-------|
| 1 Config keys `machine_folder`, `shared_group`, env `GRID_MACHINE_DIR`, OS default, one function | HOLDS | `Resolve` in `machinefolder.go:227`; `ReadConfig` and `WriteConfig` keep every other key of the config (`TestWriteConfigKeepsOtherKeys`) and refuse a file that is not a JSON object (`TestWriteConfigRefusesAFileThatIsNotAnObject`). Callers do not exist yet, so one resolver means one function that nothing else duplicates (row 1). |
| 2 Shared mode modes by explicit chmod; group checked; doctor finding with the fix | NOT DONE | Why: the `grid doctor` rows are not wired. The modes are row 2; the group and setgid checks are `Check` (row 5). |
| 3 Per-login launcher files; shared parts by atomic rename | NOT DONE | Why: no existing launcher, brain-cache or state.json code calls the new API. What exists: `launcher.go` (`launcher/<login>/pid`, `lock`, `log`) and `files.go` (`WriteState`, `BrainCache`, `Worktrees`, `WriteFile` through `internal/atomicfile`), tested in rows 3 and 4. |
| 4 No secrets in the folder; a test asserts the check finds none | HOLDS | Row 6. |
| 5 `grid setup --shared-group <g>` and the userConfig question | NOT DONE | Why: the command, its parser, its contract entries and the userConfig question are not written. What exists: `machinefolder.PlanShared` (looks, makes nothing, returns the exact `groupadd`, `mkdir -p`, `chgrp`, `chmod 2770`, `usermod -aG` commands when the folder is missing or not writable; `TestPlanShared`) and `WriteConfig`. |
| 6 Migration: leave a private folder, doctor reports it, the move is a person's step | NOT DONE | Why: the `grid doctor` row is not wired. The leftover finding and its fix text exist (row 5) and nothing moves any file. |

Rules:

| Rule | Result | Proof |
|------|--------|-------|
| Never use sudo or act as root; Grid runs no sudo | HOLDS | The package never runs `sudo` or any command; the sudo commands are text in `Finding.Fix` and `SharedPlan.Commands` (`check.go`). |
| Never chmod outside the folder, only paths Grid creates | HOLDS | `Folder.MkdirAll`, `WriteFile` and `OpenCreate` refuse a path outside the folder (`ErrOutside`) and chmod only the directories and files they just made (`files.go:89`, `:134`, `:148`); `TestNeverChmodsOutsideOrExisting`. |
| Findings, never refusals; no guard on agents | HOLDS | `Check` returns rows and blocks nothing; the package adds no hook, gate or cap. |
| Secrets never in the folder | HOLDS | Row 6; the package writes only the launcher pid, lock and log, state.json, and what callers hand to `WriteFile`. |
| Commits: one sentence, no attribution lines | HOLDS | `git log origin/main..HEAD --format=%B` lists the messages: each one sentence, no trailer. |
| No private paths, hosts or emails in committed files | HOLDS | The doc and the code use `/srv/grid` as the only example path. |
| Files outside the brief | HOLDS | The brief has no Touches list. The only file outside the new package is this doc, `docs/lanes/P-shared-home.md`. No golden, walk, contract or data file changed, because no command or flag was added. |

Runbook checks:

| Check | Result | Proof |
|-------|--------|-------|
| Tests of changed packages and every importer | HOLDS | Changed package: `internal/machinefolder`. Command run: `go list -f '{{if .Imports}}{{.ImportPath}} {{join .Imports " "}}{{end}}' ./...`. The only line that names `internal/machinefolder` is its own: `github.com/gridconsole/grid-plugins/internal/machinefolder bytes encoding/json errors fmt github.com/gridconsole/grid-plugins/internal/atomicfile github.com/gridconsole/grid-plugins/internal/fslock github.com/gridconsole/grid-plugins/internal/gridhome io io/fs os os/user path/filepath runtime sort strconv strings syscall`. No other package imports it, so there is no importer to test. |
| Walk tests after `git fetch origin && git merge origin/main` | HOLDS | After the merge (main at 82a26411): `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` printed `ok github.com/gridconsole/grid-plugins/internal/cli 0.810s`, exit 0; `go test -count=1 ./internal/job/lint/` printed `ok github.com/gridconsole/grid-plugins/internal/job/lint 0.878s`, exit 0. |
| Windows and macOS builds | HOLDS | `GOOS=windows go build ./internal/machinefolder/`, `GOOS=darwin go build ./internal/machinefolder/` and `GOOS=windows go vet ./internal/machinefolder/` all exit 0. Tests that need unix modes, groups or rename over an open file carry a `unix` build tag (`modes_unix_test.go`, `check_unix_test.go`, `state_race_unix_test.go`). |
| New tests fail on base and pass on the lane | NOT DONE | Why: I did not run the test files against a base tree. What I have: the package is absent on base (`git show origin/main:internal/machinefolder/machinefolder.go` prints `fatal: path 'internal/machinefolder/machinefolder.go' exists on disk, but not in 'origin/main'`), so the tests cannot compile there; on the lane they pass (rows 1 to 4 and 6). |
| Live check on /srv/grid with two logins | not run, not asked | The brief makes it an orchestrator step after landing. |
| `make test` over the whole repo | not run, not asked | The brief names the touched package and its importers; there are none. |

## What changed

New package `internal/machinefolder` (no existing file edited):

- `machinefolder.go`: constants, `Env`, `DefaultPath`, `ConfigPath`, `ReadConfig`, `WriteConfig`, `Folder`, `Resolve` (the one resolver: env, then config, then OS default).
- `files.go`: modes (shared dir `os.ModeSetgid|0o770`, file `0o660`; private `0o700` and `0o600`), `MkdirAll`, `WriteFile` (atomic rename through `internal/atomicfile`), `OpenCreate`, the layout (`launcher/`, `brain-cache/`, `worktrees/`, `videos/`, `tasks/<T>/state.json`), `WriteState`, `ReadState`.
- `launcher.go`: `launcher/<login>/pid|lock|log`, `Start` (flock through `internal/fslock`), `Stop`, `OpenLog`, `Logins`, `SafeLogin`, `CurrentLogin`.
- `secrets.go`: `ScanSecrets` (names of credential files and Claude or Codex config folders, and content through a caller-supplied check).
- `check.go`: `Check` rows (`machine folder`, `... group`, `... mode`, `... login`, `... leftover`), `SharedCommands`, `PlanShared`.
- `gid_unix.go` and `gid_other.go`: the owning group of a folder.
- Six test files, all new.

Choices the reviewer may question:

- The config reader is the package's own and tolerant (it keeps unknown keys) because the config-reader lane (P-grid-config-read) is not on main. When it lands, `ReadConfig` and `WriteConfig` become thin calls to it; `Resolve` stays the one place that picks the path.
- The setgid bit is required in the mode row only on Linux; macOS and BSD folders inherit the group from the parent without it.
- A lock file stays after `Stop` so a launcher starting at the same moment never locks an unlinked file; only the pid file is removed.
- `machine_folder` is written by setup as the path that resolves now (so `GRID_MACHINE_DIR=/srv/grid grid setup --shared-group grid` records `/srv/grid`); no new path flag was invented.

## Open items (the reason for NOT DONE)

1. Wire `machinefolder.Check` into `grid doctor` as rows that show only when the folder is configured (the README pins the default list to 10 rows, `TestGjReadmeDoctorChecksMatch`).
2. Add `grid setup --shared-group <g>` (its own parser, dispatched before `settings.ClaudeSetupCommand`, which reads a positional as a project), with the `contract/commands.json`, `contract/schemas/cmd-setup.json` and README rows it forces, then the userConfig "Shared group" question in the plugin manifest. The setup usage text pinned at `internal/cli/setup_claude_test.go:118` must stay byte-identical.
3. Switch the launcher pid, lock and log files, the brain cache, the task state.json cache, worktrees and videos to the `Folder` API.

## NOT VERIFIED

- Behaviour on a real shared folder with two logins (`/srv/grid`, group `grid`): the orchestrator's live step. The tests use a temp folder in the test's own group.
- macOS and Windows runtime: only built and vetted there, not run.
- The `grid doctor` and `grid setup --shared-group` output, because they are not written.

NOT DONE

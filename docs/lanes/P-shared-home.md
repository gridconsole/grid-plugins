# Lane P-shared-home: one machine folder shared by several logins

Result: **NOT DONE**. The new package `internal/machinefolder` is built and tested (acceptance rows 1 to 4, 6 and 7 hold, and the doctor findings of row 5 exist as a function). Nothing calls it yet: `grid doctor` and `grid setup --shared-group` are not wired (brief items 5 and the doctor part of 2 and 6), and the launcher, brain cache and state.json writers in the other packages do not use it. Those are the open items below.

Code head: `abfe3a29` (the package and its tests). The lane then merges `origin/main` (82a26411) and adds this doc.

## Proof table

Acceptance rows (the brief):

| # | Row | Result | Proof |
|---|-----|--------|-------|
| 1 | Path resolution per OS from config, env override, default; one resolver | HOLDS | `go test -count=1 ./internal/machinefolder/` exit 0: `TestResolvePerOS` (linux, darwin, windows with and without LOCALAPPDATA), `TestResolveConfigThenEnv`, `TestResolveHomeAndRelative`, `TestResolveBadConfigFallsBack`. `Resolve` is `internal/machinefolder/machinefolder.go:227`. `TestOneResolver` fails if any non-test Go file outside the package holds `GRID_MACHINE_DIR`, `"machine_folder"` or `".local", "state", "grid"`. The grep: `grep -rn -E 'Application Support\|"\.local", "state"\|GRID_MACHINE_DIR\|machine_folder' --include=*.go --exclude-dir=machinefolder .` lists no machine-folder code; the only hits are Claude's managed-settings path (`internal/settings/claude_user.go:319`), the golden replay home (`internal/golden/replay.go:123`) and env-name allow lists (LOCALAPPDATA), none of which place the machine folder. |
| 2 | Shared dirs 2770 and files 0660 in a temp folder in the test's own group; private 0700 and 0600 | HOLDS | `TestSharedModeDirs2770Files0660` (run under umask 022, 077 and 0, so the modes come from chmod and not the umask), `TestPrivateModeDirs0700Files0600`, `TestNeverChmodsOutsideOrExisting` (a folder or file that already exists keeps its mode; a path outside the folder is refused and the folder above is not chmod). Exit 0. |
| 3 | Two launchers under different logins keep separate pid and lock files; A's stale lock is untouched by B | HOLDS | `TestLaunchersOfTwoLoginsAreSeparate`, `TestStaleLockOfOneLoginIsLeftAloneByAnother` (bytes and mtime of A's pid, lock and log are the same after B starts, logs and stops). Exit 0. |
| 4 | Concurrent state.json writers by atomic rename; a reader never parses a partial file; 50 runs with -race | HOLDS | `go test -race -count=50 -run TestStateJSONConcurrentWritersReaderNeverSeesPartial ./internal/machinefolder/` printed `ok ... 19.541s`, exit 0. 4 writers, 4 readers, 25 rewrites each, 160 KB documents. |
| 5 | Doctor findings for wrong group, missing setgid, private leftover folder, each with the fix command | PARTLY | The findings and their fix commands exist and are tested: `TestCheckWrongGroup` (`sudo chgrp -R <group> <folder>`), `TestCheckMissingSetgid` (`sudo chmod 2770 <folder>`), `TestCheckPrivateLeftoverFolder` (a `cp -a` chain, nothing moved), `TestCheckLoginNotInGroup`, `TestCheckMissingSharedFolderPrintsSetupCommands`, `TestCheckEveryFailureHasAFixAndChangesNothing`. They are `machinefolder.Check` rows. `grid doctor` does not call it yet: **NOT DONE**, see open items. |
| 6 | Secrets check over a seeded folder finds none; a planted token file is reported | HOLDS | `TestSecretsCheckFindsNoneInASeededFolder` (folder filled by Grid's own writers: launcher files, log, state.json, brain cache, a worktree, a video), `TestSecretsCheckReportsPlantedFiles` (a planted `ghp_` token file, `.credentials.json`, `.claude/`, `.claude-work/`, `.env`, `.pem`, an `api_key =` file; hits name the path and the reason, never the token), `TestSecretsCheckDoesNotFollowLinks`. The content check is `brain.LooksSecret`, passed in. Exit 0. |
| 7 | New tests in new files; `go test` for touched packages passes; `go vet` clean | HOLDS | All 13 changed files are new (`git diff --stat origin/main...HEAD`: 13 files, 2257 insertions, 0 deletions). `go test -count=1 ./internal/machinefolder/` exit 0; `go test -race -count=3 ./internal/machinefolder/` exit 0; `go vet ./internal/machinefolder/` and `make lint` (`go vet ./...`) exit 0. |

Brief build items:

| Item | Result | Proof |
|------|--------|-------|
| 1 Config keys `machine_folder`, `shared_group`, env `GRID_MACHINE_DIR`, OS default, one function | HOLDS | `Resolve` in `machinefolder.go:227`; `ReadConfig` and `WriteConfig` keep every other key of the config (`TestWriteConfigKeepsOtherKeys`) and refuse a file that is not a JSON object (`TestWriteConfigRefusesAFileThatIsNotAnObject`). |
| 2 Shared mode modes by explicit chmod, group checked, doctor finding with fix | PARTLY | Modes: row 2. Group and setgid checks: `Check` (row 5). The `grid doctor` rows: NOT DONE. |
| 3 Per-login launcher files; shared parts by atomic rename | HOLDS for the package | `launcher.go` (`launcher/<login>/pid`, `lock`, `log`), `files.go` `WriteState`, `BrainCache`, `Worktrees` and `WriteFile` (via `internal/atomicfile`). No existing launcher, brain-cache or state.json code calls them yet: NOT DONE for the callers. |
| 4 No secrets in the folder; a test asserts the check finds none | HOLDS | Row 6. |
| 5 `grid setup --shared-group <g>` and the userConfig question | NOT DONE | `machinefolder.PlanShared` (looks, makes nothing, returns the exact `groupadd`, `mkdir -p`, `chgrp`, `chmod 2770`, `usermod -aG` commands when the folder is missing or not writable; `TestPlanShared`) and `WriteConfig` exist. The command, its parser, its contract entry and the userConfig question are not written. |
| 6 Migration: leave a private folder, doctor reports it, the move is a person's step | PARTLY | The leftover finding and its fix text exist (row 5) and nothing moves any file. The `grid doctor` row is not wired. |

Rules:

| Rule | Result | Proof |
|------|--------|-------|
| Never use sudo or act as root; Grid runs no sudo | HOLDS | No call to `sudo` or `exec` in the package; the sudo commands are text in `Finding.Fix` and `SharedPlan.Commands` (`check.go`). |
| Never chmod outside the folder, only paths Grid creates | HOLDS | `Folder.MkdirAll`, `WriteFile` and `OpenCreate` refuse a path outside the folder (`ErrOutside`) and chmod only directories and files they just made (`files.go:89`, `:134`, `:148`); `TestNeverChmodsOutsideOrExisting`. |
| Findings, never refusals; no guard on agents | HOLDS | `Check` returns rows and blocks nothing; the package adds no hook, gate or cap. |
| Secrets never in the folder | HOLDS | Row 6; the package writes only launcher pid, lock and log, state.json, and what callers hand to `WriteFile`. |
| Commits: one sentence, no attribution lines | HOLDS | `git log origin/main..HEAD --format=%B` has one sentence per commit and no trailer. |
| No private paths, hosts or emails in committed files | HOLDS | The doc and the code use `/srv/grid` as the only example path. |
| Check against the reviewer runbook: files outside the brief | HOLDS | The brief has no Touches list. Files outside the new package: this doc only (`docs/lanes/P-shared-home.md`). No golden, walk, contract or data file is changed, because no command or flag was added. |

Runbook checks:

| Check | Result | Proof |
|-------|--------|-------|
| Tests of changed packages and every importer | HOLDS | Changed package: `internal/machinefolder`. Its importers: none. `go list -f '{{.ImportPath}} {{join .Imports " "}}' ./...` shows `internal/machinefolder` in no other package's imports (it imports `atomicfile`, `fslock`, `gridhome` and the standard library). |
| Walk tests after `git fetch origin && git merge origin/main` | HOLDS | After the merge (head `82a26411` of main): `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` printed `ok github.com/gridconsole/grid-plugins/internal/cli 0.810s`, exit 0. `go test -count=1 ./internal/job/lint/` printed `ok ... 0.878s`, exit 0. |
| Windows and macOS builds | HOLDS | `GOOS=windows go build ./internal/machinefolder/`, `GOOS=darwin go build ./internal/machinefolder/` and `GOOS=windows go vet ./internal/machinefolder/` all exit 0. The tests that need unix modes, groups or rename-over-open-file carry a `unix` build tag (`modes_unix_test.go`, `check_unix_test.go`, `state_race_unix_test.go`). |
| New tests fail on base and pass on the lane | HOLDS (weak) | On base the package does not exist: `git show origin/main:internal/machinefolder/machinefolder.go` prints `fatal: path 'internal/machinefolder/machinefolder.go' exists on disk, but not in 'origin/main'`, so every test of it fails there (no package). I did not run the test files against a base copy with stubs. |
| Live check on /srv/grid with two logins | not run, not asked | The brief makes it an orchestrator step after landing. |
| `make test` over the whole repo | not run, not asked | The brief names the touched package and importers; there are none. |

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
2. Add `grid setup --shared-group <g>` (own parser dispatched before `settings.ClaudeSetupCommand`, since it reads a positional as a project), with `contract/commands.json`, `contract/schemas/cmd-setup.json` and the README rows forced by it, then the userConfig "Shared group" question in the plugin manifest. The setup usage text pinned at `internal/cli/setup_claude_test.go:118` must stay byte-identical.
3. Switch the launcher pid, lock and log files, the brain cache, the task state.json cache, worktrees and videos to the `Folder` API.

## NOT VERIFIED

- Behaviour on a real shared folder with two logins (`/srv/grid`, group `grid`): the orchestrator's live step. The tests use a temp folder in the test's own group.
- macOS and Windows runtime: only built and vetted there, not run.
- The `grid doctor` and `grid setup --shared-group` output, because they are not written.

NOT DONE

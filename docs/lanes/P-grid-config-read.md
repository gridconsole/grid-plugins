# P-grid-config-read: proof table

Lane P1-grid-config-read from `docs/research/first-run-config.md` sections 3-4 and 6, with the
ruling of 17 Nov 01:58 UTC: the config is at `~/.config/grid/config.json` (macOS, Linux) or
`%APPDATA%\Grid\config.json` (Windows), and the older `~/.grid/config.json` is read as a
fallback and copied once. The machine folder is at the OS standard place, and the launcher is
looked up there first. A brain folder key is read. The member name is only read, with its
default worked out.

The proof runs below were made after `git fetch origin && git merge origin/main` (merge
c5257980). The last code commit is f2fce452.

## Acceptance rows

| Row | Verdict | Proof |
| --- | --- | --- |
| (1) A redacted owner-shaped file with data in ~/.grid/<name> lists notes and projects, with no copy | HOLDS | `internal/cli/pc_config_home_test.go:69` TestPcConfigDataFolderIsRead with `contract/gridconfig/owner-v3/old-config.json`. With no flag and no GRID_HOME, `grid project list`, `grid brain topics` and `grid brain show` read `~/.grid/studio`. No `projects`, `knowledge` or `VERSION` appears in `~/.grid`. The config is copied byte for byte to the new path and the old file is left in place. `go test -count=1 ./internal/cli/`: ok, exit 0. The owner's file was **not readable** from this lane, so owner-v3 is a hand-written, redacted v3 sample, as the brief allows |
| (2) GRID_HOME and --grid-home still win | HOLDS | `internal/cli/pc_config_home_test.go:140` TestPcEnvAndFlagWinOverConfig and `:126` TestPcFlagPicksItsWorkspace. `internal/gridhome/pc_resolve_test.go:121` TestPcResolveEnvWinsOverConfig, and the contract case `env` key. All are in `go test ./internal/cli/ ./internal/gridhome/`: ok, exit 0 |
| (3) No config and no data gives ~/.grid | HOLDS | `internal/cli/pc_config_home_test.go:155` TestPcNoConfigGivesDotGrid, and the contract case `none` in `internal/gridhome/pc_resolve_test.go:49`. ok, exit 0 |
| (4) The v1 top-level shape resolves by the config.rs rules | HOLDS | The contract cases `v1-elsewhere`, `v1-login`, `v1-member` and `v1-workspace` in TestPcResolveContractCases (`internal/gridhome/pc_resolve_test.go:49`, which requires at least 9 cases; there are 11). Also `internal/gridconfig/gridconfig_test.go:89` TestMigrateV1WritesNoOldKeys. ok, exit 0 |
| (5) `grep -rn 'gridhome.Dir(' internal cmd` lists only the resolver | HOLDS | The command prints nothing (exit 1, no match). The only `Dir` left is the resolver's own `func Dir` at `internal/gridhome/gridhome.go:136`, kept because the existing `gridhome_test.go` TestDir calls it. Every caller uses `gridhome.Resolve(...).Dir` |
| (6) The launcher is found with data elsewhere | HOLDS | `internal/hook/pc_machine_launcher_test.go:17` TestPcMachineLauncherWithDataElsewhere: with data in `~/.grid/acme`, SessionStart puts `~/.grid/bin` on PATH when only that holds `grid`. Once the OS machine folder's `bin/grid` exists, that one goes on PATH. `internal/gridhome/pc_resolve_test.go:198` TestPcLauncherDirs covers the lookup order (machine folder, then ~/.grid, then data) and which folders the binary refreshes. `go test ./internal/hook/`: ok, exit 0 |
| (7) Walk tests | HOLDS | After the merge: `go test -count=1 -v ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` gives `--- PASS: TestHsIpWalksEveryCommand`, `--- PASS: TestIlGuestSiteList` and `ok internal/cli 0.337s`, exit 0. `go test -count=1 ./internal/job/lint/` gives `ok internal/job/lint 1.494s`, exit 0 |

## Ruling rows (17 Nov 01:58 UTC)

| Ruling | Verdict | Proof |
| --- | --- | --- |
| 1. Config at ~/.config/grid/config.json, or %APPDATA%\Grid\config.json; old file read as fallback and copied once, old file kept | HOLDS | `internal/gridconfig/gridconfig.go:77` PathFor, `:62` OldPath, `:209` Load, `:241` copyOnce (temp file and rename, never over an existing file). `internal/gridconfig/gridconfig_test.go:119` TestLoadCopiesOldFileOnce covers: no file; old only (copied, old kept); an edited new file is kept on the next load; the copy cannot be made, so old is read in place; a broken old file is reported and not copied. The contract cases `owner-v3` and `old-and-new` (new wins, no copy) |
| 2. Machine folder at the OS standard place, config key may move it; launcher found there, ~/.grid/bin kept as fallback | HOLDS | `internal/gridconfig/gridconfig.go:98` MachineDirFor and `:394` Config.MachineDir (top-level key `machineFolder`). `internal/gridhome/gridhome.go:190` FindLauncherDir and `:167` LauncherDirs. `cmd/grid/main.go` creates the machine folder and refreshes its launcher. `internal/gridhome/pc_resolve_test.go:155` TestPcMachineFolderMacAndLinux covers darwin, linux, the override and Windows |
| 3. Brain folder key, read only; absent means the brain stays where it is | HOLDS | `internal/gridconfig/gridconfig.go:385` BrainFolder (absolute paths only). Home.Brain is set only from the workspace in use. `internal/brain` is untouched. Proof: `internal/gridconfig/gridconfig_test.go:194`, the contract case `brain-machine`, and TestPcFlagPicksItsWorkspace |
| 4. Name in Grid: default is the oauthAccount displayName, else the OS login; read and default only | HOLDS | `internal/gridhome/gridhome.go:217` DefaultMember reads `$CLAUDE_CONFIG_DIR/.claude.json`, else `~/.claude.json`, then USER, USERNAME or LOGNAME. Home.Member is the config's member. `internal/gridhome/pc_resolve_test.go:249` TestPcDefaultMember also checks that nothing is written. Doctor names where the default comes from, not the name itself |
| 5. Package doc, README and tests on the new paths; a fallback and copy-once test; a macOS and Linux machine folder test | HOLDS | The package doc is at the top of `internal/gridconfig/gridconfig.go`, plus `contract/gridconfig/README.md`, TestLoadCopiesOldFileOnce and TestPcMachineFolderMacAndLinux |
| UI Change 12: no Handle field; member comes from the shared config | HOLDS | v3 `workspaces[].member` is read into Home.Member, and DefaultMember gives P2 its default. The shape is unchanged for the app |

## Rules

| Rule | Verdict | Proof |
| --- | --- | --- |
| Tests of changed packages and of every importer | HOLDS | Importers from `go list -f '{{.ImportPath}}: {{join .Imports " "}}' ./... \| grep -E "internal/(gridconfig\|gridhome)( \|$)"` are account, agent, braincmd, cli, doctor, gitx, gridhome, guard, hook, job/jobcmd, land, localdata, migrate, ports, project, projects, routine, settings, signal, task, where, worker, workflow and lib. `go test -count=1` of all of them plus gridconfig and cmd/grid: every one ok, exit 0. internal/cli was rerun after the merge: ok |
| `make lint` with 0 fail | HOLDS | `make -C . lint` runs `go vet ./...` and the gofmt check, with no output and no error, exit 0. The first run flagged `internal/cli/cli.go`, which this lane had changed; it was fixed in f2fce452 |
| Windows and macOS builds | HOLDS | `GOOS=windows go build ./... && GOOS=darwin go build ./...`: exit 0 |
| New tests fail on base and pass on the lane | HOLDS | The lane's test files and fixtures were checked out onto an `origin/main` worktree and run with `go test ./internal/gridconfig/ ./internal/gridhome/ ./internal/cli/ ./internal/hook/`. Output: `gridconfig_test.go:17:37: undefined: File`, `:21:21: undefined: Read`, `:25:12: undefined: Write`, ..., then `FAIL internal/gridconfig [build failed]`, `FAIL internal/gridhome [build failed]`, `FAIL internal/cli [build failed]`, `FAIL internal/hook [build failed]`. On the lane all pass |
| Existing tests not edited to pass | HOLDS, with a note | `internal/gridhome/gridhome_test.go` is untouched (TestDir still calls `Dir`). `internal/golden/corpus_test.go:74` has a one-line call-site change, `gridhome.Dir(getenv, home)` to `gridhome.Resolve(getenv, home).Dir` (commit ac337da5), so that row (5)'s grep lists only the resolver. Its assertions are unchanged |
| Only the brief's packages | HOLDS, with a note | The changes are in gridconfig, gridhome, cli, doctor, hook, cmd/grid and contract/gridconfig. guard, lib and golden carry only the row (5) call-site changes from `gridhome.Dir(` to `gridhome.Resolve(...).Dir` |
| No guard, gate, cap, hold, delay, limit or blocking hook | HOLDS | No hook, Job or agent path got a new refusal. A broken or missing config falls through to the next source and is reported by doctor as an offer |
| Small commits, one sentence each, no attribution lines | HOLDS | `git log origin/main..HEAD` shows ac337da5, 32dc3cf5, 9f30289b, the merge c5257980 and f2fce452, none with trailers |
| No personal paths, hosts or emails in committed files | HOLDS | Fixtures use `~`, `sam`, `acme` and `studio` |
| Never sudo, root, push or another lane's clone | HOLDS | Only this clone was written. The base check used a temporary worktree inside this clone, which was removed afterwards |

## What changed

- `internal/gridconfig`:
  - Path functions for the new config path, the old path, the data root and the OS machine folder (with an explicit goos so the tests run on any OS).
  - `Load`: the new file wins; otherwise the old file is read and copied once by temp file and rename; the old file is left in place.
  - The top-level `machineFolder` key, and `BrainFolder()` on a workspace.
- `internal/gridhome`:
  - Resolve now records which config file was read and whether it was copied, plus the machine folder and its source, the workspace in use (also when GRID_HOME or --grid-home names its folder), and its brain and member.
  - `LauncherDirs` (which folders the binary refreshes) and `FindLauncherDir` (machine folder, then ~/.grid, then data).
  - `DefaultMember`.
- Launcher lookups in `hook.go`, `cli/routine.go` and `doctor/vs_upgrade.go` use `FindLauncherDir`. `cmd/grid/main.go` creates the machine folder and refreshes its launcher. `--grid-home` re-resolves, so its workspace and brain are known.
- The doctor home line names the config file, the copy, the machine folder, the brain and where the member default comes from.
- New contract cases `owner-v3`, `brain-machine` and `old-and-new`. The README lists the new paths and expect keys.

## NOT VERIFIED

- A live run on macOS or Windows. Paths there are checked by unit tests with an explicit goos, and the builds pass, but nothing was run on those machines.
- The owner's real config file was not readable from this lane, so row (1) uses a redacted, hand-written v3 sample.

## Open items

- The key name `machineFolder` is this lane's choice; the ruling did not name it.
- `DefaultMember` returns the raw displayName, not a handle. P2 decides whether to pass it through `HandleFromLogin`.
- XDG_CONFIG_HOME and XDG_STATE_HOME are not read; the ruling named fixed paths.
- Only the launcher moved to the machine folder. Locks, caches, worktrees, runner logs and videos still sit where they were (P-shared-home and later).
- The brain key is read and surfaced as `Home.Brain`, but `internal/brain` does not use it yet.
- The research doc's doctor line about the baked timer is not done in this lane.

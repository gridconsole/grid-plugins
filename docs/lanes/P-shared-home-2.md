# Lane P-shared-home-2: the shared machine folder in grid doctor and grid setup

Branch `lane-P-shared-home-2`, built on `lane-P-shared-home` (the `internal/machinefolder` package: Resolve, Check, PlanShared, WriteConfig). This lane's own commits are the three after the base: `a0ce256c`, `32a58e54`, `ce79710e`. `origin/main..lane-P-shared-home-2` also carries the P-shared-home commits, because they are not on `origin/main` yet. The head also contains the merge of `origin/main`, which was clean and docs only.

## Proof table

Rows 1 to 3 are the brief's acceptance rows. Every `go test` line below printed `ok`, never `FAIL`, so its exit is 0. The tool that ran the commands showed the output and not the numeric exit.

| # | Row | Result | Command or place |
|---|---|---|---|
| 1 | `grid doctor` shows the three findings, wrong group, missing setgid, private leftover, each with its fix command, on a seeded folder | HOLDS | `go test -count=1 ./internal/cli` ok. `TestMfDoctorFindsWrongGroupMissingSetgidAndPrivateLeftover` has 3 subtests; each asserts the row name, the failing state and the exact fix (`sudo chgrp -R <g> <path>`, `sudo chmod 2770 <path>`, the `cp -a` chain). `internal/cli/mf_doctor_test.go` |
| 2 | Unconfigured `grid doctor` output is unchanged: the 10 README rows, no machine folder rows | HOLDS | `TestMfDoctorUnconfiguredHasNoMachineFolderRows` ok; `TestGjReadmeDoctorChecksMatch` ok (calls `doctor.Run`, which this lane did not touch). `internal/doctor/machinefolder.go:16` returns the report unchanged when `!f.Configured()` |
| 3 | `grid setup --shared-group <g>` prints the commands (mkdir, chgrp, chmod 2770, usermod per login), runs no sudo, writes only the config | HOLDS | `TestMfSetupSharedGroupPrintsCommandsAndWritesOnlyTheConfig` ok. It snapshots the whole temp tree before and after: only `home/.config/grid/config.json` changed, the folder was not made, and a fake `sudo`, `usermod`, `groupadd`, `chgrp`, `chmod` and `mkdir` on PATH never wrote its marker. `internal/cli/mf_setup_test.go` |
| 4 | Contract entries and the userConfig "Shared group" question | HOLDS | `contract/commands.json` (flag `--shared-group`, value), `contract/schemas/cmd-setup.json` (third `oneOf` branch), `contract/fixtures/cmd-setup/{valid,invalid}/shared-group*.json`, `contract/README.md`. `internal/packaging/layout.go` adds `userConfig.shared_group` ("Shared group") to `.claude-plugin/plugin.json` only; `make generate` output equals the committed file (`TestGeneratedLayout` ok) |
| 5 | Setup uses PlanShared and WriteConfig | HOLDS | `internal/cli/setup_shared.go`, `runSetupShared` calls `machinefolder.PlanShared` and `machinefolder.WriteConfig` and nothing else writes |
| 6 | Tests of the touched packages | HOLDS | `go test -count=1 ./internal/machinefolder/ ./internal/doctor/ ./internal/cli/ ./internal/contract/ ./internal/packaging/... ./internal/settings/ ./cmd/grid/ ./lib/ ./contract/...` all `ok` (cli 110.9s, lib 28.5s); `cmd/grid` has no test files |
| 7 | The whole module | HOLDS | `go test -count=1 ./...` printed no line other than `ok` and `no test files` |
| 8 | Importers of the changed packages | HOLDS | `go list -f '{{.ImportPath}}: {{join .Imports " "}}' ./...` shows: `internal/machinefolder` is imported by `internal/cli` and `internal/doctor`; `internal/doctor` by `internal/cli`; `internal/cli` by `cmd/grid` and `lib`; `internal/packaging` by `internal/packaging/cmd`; the embedded `contract` package by `internal/contract`. All of them are in the run of row 6, and the module run of row 7 covers the rest |
| 9 | `make lint` | HOLDS | `make lint` ran `go vet ./...` and printed nothing else, no diagnostics. `gofmt -l internal/cli internal/doctor internal/packaging` printed nothing |
| 10 | Windows and macOS builds | HOLDS | `GOOS=windows go build ./...` and `GOOS=darwin go build ./...` printed nothing, no errors |
| 11 | Walk tests, after merging `origin/main` | HOLDS | `git fetch origin` then `git merge origin/main` (clean). `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` printed `ok  internal/cli 0.208s`. `go test -count=1 ./internal/job/lint/` printed `ok  internal/job/lint 0.351s` |
| 12 | New tests fail on base, with the failing lines | HOLDS | Run on the lane's test files with the wiring not yet in place (before `a0ce256c` and `32a58e54`). Doctor: `mf_doctor_test.go:130: machine folder rows = [], want [` and `mf_doctor_test.go:153: fix = ` (empty fix), also lines 169, 189, 193, 222. Setup: `mf_setup_test.go:102: exit 2: flag provided but not defined: -shared-group`, also lines 162, 180, 202, 214, 225, 287 |
| 13 | No sudo, no root | HOLDS | `git grep -n -E 'exec\.Command|os/exec'` on `internal/cli/setup_shared.go` and `internal/doctor/machinefolder.go` finds nothing: neither file can start a process. Setup only prints `sudo ...` strings; row 3 proves nothing ran |
| 14 | No guard, gate, cap, hold or blocking hook on agents | HOLDS | None added. A failing machine folder row sets `rep.OK = false` and prints its fix; it blocks no command, and `grid doctor --bundle` still runs after it |
| 15 | Small one-sentence commits, no attribution lines | HOLDS | `git log --format=%B a0ce256c~1..ce79710e` shows three one-sentence subjects and no `Co-Authored-By` or `Claude-Session` line. The system reminder asked for a `Co-Authored-By` line; the owner's rule wins |
| 16 | No personal paths, hosts or emails in committed files | HOLDS | `git grep` of the changed files finds only `/home/example/...` in the contract fixtures, the same placeholder the existing `basic.json` and `populated.json` fixtures use |
| 17 | No existing test edited to make it pass | HOLDS | The diff of `a0ce256c~1..ce79710e` touches no existing `_test.go` file; both test files are new |
| 18 | Live two-login check on the shared folder | not run, not asked | The brief names no live check; the orchestrator owns the run on the real shared folder |

## What changed

- `internal/doctor/machinefolder.go` (new): `AddMachineFolder(rep, env, opts)` resolves the folder and, only when it is configured, appends the `machinefolder.Check` rows (`machine folder`, group, mode, login, leftover) with their `Fix` strings. `doctor.Run` is untouched, so the README's 10 rows and `TestGjReadmeDoctorChecksMatch` do not move.
- `internal/cli/doctor.go`: one line calls `AddMachineFolder` just before the `--bundle` step. `internal/cli/cli.go`: two unexported test seams on `Env` (`machineCheck`, `machineGOOS`).
- `internal/cli/setup_shared.go` (new) and `internal/cli/settings.go` (3 lines): `grid setup --shared-group <group> [--json]`. It resolves the folder, calls `PlanShared`, writes `shared_group` and `machine_folder` with `WriteConfig` (other config keys kept, file replaced atomically), then prints the config path, the group, the folder, and either "The folder exists and this login can write in it." or the commands a person with sudo runs, two spaces indented. It runs no sudo. It exits 0 once the config is written, even when it prints commands, because the commands are the person's next step. A bad group name, an operand or a broken config file exits 2 or 1 and writes nothing.
- Contract: `commands.json`, `cmd-setup.json` (third output shape: group, path, config, ready, commands, optional why), two fixtures, and `contract/README.md` (the setup citation and a paragraph on the machine folder doctor rows).
- Manifest: `internal/packaging/layout.go` gives the Claude manifest a `userConfig.shared_group` question, "Shared group", not required. The Codex manifest has no such key and is unchanged. `.claude-plugin/plugin.json` is the regenerated file.
- Tests (new files): `internal/cli/mf_doctor_test.go` (4 tests, 3 subtests) and `internal/cli/mf_setup_test.go` (9 tests). They use a map-backed `Getenv`, so the process environment never leaks in.

Files outside doctor, setup and their tests are forced by the new code: `contract/commands.json` is generated by `TestContractCommandsParserInventory` and fails without the flag, `.claude-plugin/plugin.json` is generated by `internal/packaging/layout.go` and `TestGeneratedLayout` fails if it is stale, and the schema, fixtures and README rows are the contract entries the brief names.

## Open items

- The "Shared group" answer is declared in the manifest only. Claude Code hands a `userConfig` value to hook environments (`CLAUDE_PLUGIN_OPTION_SHARED_GROUP`), not to `grid` run through Bash, and nothing here copies it into the Grid config. The config is written by `grid setup --shared-group`. A follow-up can have the session hook run that command with the answer.
- `--shared-group` is not in the text of `grid setup --help`, because the test `setup_claude_test.go:118` pins that text byte for byte and this lane may not edit an existing test. `grid setup --shared-group <g> --help` prints `Usage: grid setup --shared-group <group> [--json]` (`TestMfSetupSharedGroupHelp`); the flag is listed in `contract/commands.json`.
- The launcher, brain cache, `state.json` and similar files still do not read the Folder API; that was open item 3 of the previous lane and is not in this brief.

## NOT VERIFIED on a live screen

- The "Shared group" question in the Claude Code configuration dialog: never shown in a real Claude Code session; the manifest is checked by `TestGeneratedLayout` only.
- A real second login on a real shared folder: no sudo here, so the group, setgid and `usermod` steps were never run for real. The doctor findings were seeded with real files on a temp folder in the test, with the group and mode changed by the test itself.
- Windows and macOS were built, not run; `mf_doctor_test.go` and `mf_setup_test.go` carry `//go:build unix`.

DONE

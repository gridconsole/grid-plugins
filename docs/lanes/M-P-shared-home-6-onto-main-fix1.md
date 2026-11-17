# M-P-shared-home-6-onto-main-fix1: machine-folder CLI tests follow the decided config format

Closes the NOT DONE of M-P-shared-home-6-onto-main (row 2b, five `internal/cli` tests). Ruling (orchestrator, 17 Nov 06:08 UTC): the decided format wins (P-shared-home-5 item 2, owner direction: camelCase keys `machineFolder` and `sharedGroup`, written through `gridconfig`, version 3 file with `workspaces` and a `.v1` backup). No production file changed, no test deleted.

## What each test pins, and that it still holds

`git log -S` finds one introducing commit per file: `a0ce256c` ("Add the machine folder rows to grid doctor when the folder is configured") for `mf_doctor_test.go`, `32a58e54` ("Add grid setup --shared-group, ...") for `mf_setup_test.go`. Neither names a past incident; they pin the behaviour of those two features. Only the seed key and the file-shape expectation changed; every assertion on a finding or a fix is untouched.

| Test | Pins | Change | Still holds |
|---|---|---|---|
| `TestMfDoctorConfiguredRowsFollowTheDefaultRows` | a configured folder adds the five rows after the ten default rows, all OK with no fix | seed `sharedGroup` | same rows, same order, same OK/no-fix checks |
| `TestMfDoctorFindsWrongGroupMissingSetgidAndPrivateLeftover` | wrong group, missing setgid and private leftover are each a failing row with its fix command, exit 1, and the old folder is not moved or removed | seed `sharedGroup` (3 subtests) | all three subtests keep their fix, detail, exit and "old folder untouched" checks |
| `TestMfDoctorPlainOutputPrintsTheFixAndChangesNothing` | plain output prints each failing row with its `fix:` line; doctor changes nothing in the folder | seed `sharedGroup` | same output lines, same mode and empty-folder checks |
| `TestMfSetupSharedGroupPrintsCommandsAndWritesOnlyTheConfig` | setup prints the four sudo commands in order, runs none, makes no folder, writes only the config, and a second run changes nothing | expected changes are `config.json` and `config.json.v1`; added: `.v1` holds the old bytes, file is version 3 with `workspaces`, `theme` kept, `machineFolder`/`sharedGroup` set | commands, no-run marker, no folder, second-run bytes equal: unchanged |
| `TestMfSetupSharedGroupMakesTheConfigWhenThereIsNone` | with no config, setup records the folder and the group, file mode 0600, and Resolve reads them back | config has 4 keys (`v`=3, `workspaces`, `machineFolder`, `sharedGroup`); config folder holds only `config.json` (no old file, so no `.v1`) | mode 0600 and Resolve check unchanged |

The `.v1` backup exists only when an old config existed; the no-config test therefore asserts there is none, and the other test asserts it.

## Failing on base (the merged head e4a4fb89), before the edit

```
mf_doctor_test.go:130: machine folder rows = ["machine folder" "machine folder leftover"], want [... group mode login leftover]
mf_doctor_test.go:153: fix = "", want "sudo chgrp -R gridtest <folder>"
mf_doctor_test.go:169: fix = "", want "sudo chmod 2770 <folder>"
mf_doctor_test.go:189: fix "Stop the launchers, then: cp -a ..." lacks "chgrp -R gridtest"
mf_doctor_test.go:222: output lacks "fail machine folder group: "
mf_setup_test.go:133: setup changed ["home/.config/grid/config.json" "home/.config/grid/config.json.v1"], want only ["home/.config/grid/config.json"]
mf_setup_test.go:161: config = map[machineFolder:... sharedGroup:gridtest v:3 workspaces:[]]
```

## Proof table

| # | Row | Verdict | Command and result |
|---|---|---|---|
| 1 | doctor tests seed `sharedGroup`; every finding still checked | HOLDS | `internal/cli/mf_doctor_test.go:122,144,161,177,205`; `go test -count=1 ./internal/cli -run 'TestMfDoctor\|TestMfSetup' -v`: all 13 PASS, exit 0 |
| 2 | setup tests expect the gridconfig output | HOLDS | `internal/cli/mf_setup_test.go:130-153` (version 3, workspaces, `.v1` backup with old bytes, values set, no other file) and `:166-176` (4 keys, only `config.json`); both PASS |
| 3 | `go test -count=1 ./internal/cli/... ./internal/machinefolder/... ./internal/gridconfig/... ./internal/migrate/...` | HOLDS | exit 0: `ok internal/cli 45.268s`, `ok internal/machinefolder 0.130s`, `ok internal/gridconfig 0.008s`, `ok internal/migrate 4.410s` |
| 3b | `TestOldSnakeCaseKeysAreNotRead` | HOLDS | `go test -count=1 -v ./internal/machinefolder/... -run TestOldSnakeCaseKeysAreNotRead`: `--- PASS`, exit 0 |
| 4 | `make lint` | HOLDS | exit 0 (`go vet ./...`, no output) |
| 4b | first commit rewrites NOT DONE of M-P-shared-home-6-onto-main | HOLDS | commit "Close the NOT DONE of M-P-shared-home-6-onto-main by this fix lane."; `docs/lanes/M-P-shared-home-6-onto-main.md:3` and last line read CLOSED BY M-P-shared-home-6-onto-main-fix1 |
| 5 | importers of the changed package | HOLDS | only test files changed. Importers of `internal/cli` (grep): account, mcp, packaging, settings, braincmd, golden, jobgraph, land, question, lib, cmd/grid, contract/formats: `go test -count=1` on all: `ok` (cmd/grid has no test files), exit 0 |
| 6 | Windows and macOS builds | HOLDS | `make cross`: exit 0 (darwin/arm64 and windows/amd64 vet and test compile, all `ok`) |
| 7 | Walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'`: ok; `go test -count=1 ./internal/job/lint/`: ok |
| 8 | `git fetch origin && git merge origin/main` before the last run | HOLDS | `Already up to date.` |
| 9 | New tests fail on base, pass on lane | HOLDS | failing lines pasted above; pass: row 1 |
| 10 | App-repo walks (`make webui-test`, `make realplugin` guest tables) | not run, not asked | plugin repo |
| 11 | Whole-module test, cross-repo set | not run, not asked | the orchestrator's at landing |

## What changed

- `docs/lanes/M-P-shared-home-6-onto-main.md`: NOT DONE closed (first commit).
- `internal/cli/mf_doctor_test.go`: five seeds `shared_group` to `sharedGroup`.
- `internal/cli/mf_setup_test.go`: two tests expect the gridconfig file shape.
- This doc. No production file, no deleted test.

## NOT VERIFIED

- Behaviour on a real shared folder with two logins, and macOS and Windows runtime (build and vet only).

DONE

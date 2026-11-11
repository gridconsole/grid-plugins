# F-routine-launchd: routines on an OS schedule on macOS and Windows

Lane head: see the report message. Branch lane-F-routine-launchd, two commits on main. Only test files and golden files were added in `internal/routine/`; no production file changed.

## Proof table

| # | Row | Result | Evidence |
|---|-----|--------|----------|
| 1 | Map install, list, remove for each OS to file:line; name which have a test | HOLDS | Table "Map" below |
| 2 | timer_plist.go output for daily, hourly, weekday checked against golden plists (StartCalendarInterval keys, quoted grid path, log path under Grid home) | HOLDS for golden, quoted path and log path; NOT DONE for `StartCalendarInterval` (by design, see below) | `go test ./internal/routine -run TestLaunchdPlistGolden -v`: 3 subtests PASS, exit 0. Goldens `internal/routine/testdata/golden/launchd-{daily,hourly,weekday}.plist` and `.cmd`. Test reads the plist itself: `timer_golden_test.go:146-190` (keys at `:163`, `StartInterval` at `:167`) |
| 3 | timer_schtasks.go output for the same three schedules checked against golden command lines | HOLDS | `go test ./internal/routine -run TestSchtasksGolden -v`: 3 subtests PASS, exit 0. Goldens `schtasks-{daily,hourly,weekday}.xml` (Exec Command and Arguments) and `.cmd` (`schtasks /Create ...`, `schtasks /Delete ...`). Test also decodes the XML: `timer_golden_test.go:193-232` |
| 4 | Install then remove on darwin through a fake launchctl runner leaves no plist (temp LaunchAgents folder) | HOLDS | `launchd_lifecycle_test.go:17` `TestLaunchdInstallThenRemoveLeavesNoPlist`, subtests unschedule, remove-routine, failed-bootstrap: PASS, exit 0. Asserts the call order `bootstrap gui/501 <plist>` then `bootout gui/501/<label>`, plist present at both calls, `Library/LaunchAgents` empty afterwards, ownership record gone |
| 5 | `GOOS=darwin` and `GOOS=windows` `go vet ./internal/routine/...` and `go test -c ./internal/routine/` | HOLDS | Four commands, each exit 0 (`go test -c -o /dev/null` so no binary is left in the tree). `make cross` (vet and test compile of every package for darwin/arm64 and windows/amd64) exit 0 |
| 6 | `go test ./internal/routine/...` and `make lint`: 0 fail | HOLDS | `go test ./internal/routine/...` exit 0 (`ok ... 2.206s`). `make lint` exit 0 (go vet ./... and gofmt -l empty) |
| R1 | New tests fail on base, pass on lane | HOLDS (by mutation, see note) | Mutated `timer_plist.go` (StartInterval replaced by a StartCalendarInterval dict, `bootout` replaced by `unload`): `launchd_lifecycle_test.go:77: remove calls: ["launchctl unload gui/501/com.gridconsole.routine.74b6... plist-present=yes"], want ["launchctl bootout ...` and `timer_golden_test.go:158: launchd-daily.plist differs from the renderer output` (also hourly, weekday). File restored with `git checkout`; tree clean |
| R2 | Existing tests not edited, none deleted | HOLDS | `git diff origin/main..HEAD --stat` shows only added files |
| R3 | Tests of changed packages and every importer | HOLDS | `go list` importer scan (`.Imports` and `.TestImports` of `./...`) lists one package: `internal/cli`. `go test ./internal/routine/...` exit 0. `go test ./internal/cli -run Routine` exit 0 (`ok ... 1.833s`). The full `go test ./internal/cli/...` ran past the 120 s foreground limit, finished in the background: `ok github.com/gridconsole/grid-plugins/internal/cli 203.270s`, exit 0. No production file changed, so no cli behaviour moved |
| R4 | Windows and macOS builds | HOLDS | row 5 |
| R5 | No guard, gate, cap, hold or blocking hook added | HOLDS | Diff adds only `*_test.go` and `testdata/golden/*` |
| R6 | No attribution lines, no personal paths, hosts or emails | HOLDS | Commit messages are one sentence; goldens use `{{HOME}}`, `{{GRID_HOME}}`, `{{LABEL}}` tokens |

Note on R1: the new tests pin behaviour that already works, so they pass on base once the test files exist; the mutation run above is the proof that they fail when the renderer or the launchctl calls change. A base checkout has no golden files, so the golden tests fail there for that reason too.

## Why row 2 has no StartCalendarInterval

The brief expects `StartCalendarInterval` keys. The code and an existing test say otherwise, and I did not change that:

- `timer_plist.go:40` emits `StartInterval` 60 and nothing else for timing. `schedule_test.go:74-76` (`TestLaunchdIntervalAndLogs`) fails if `StartCalendarInterval` appears ("calendar trigger remains").
- The schedule lives in the routine entry. The OS ticks once a minute and runs `grid routine run <name> --due` (`TimerSpec.argv`, `schedule.go:84`); `Interval.Due` (`interval.go:79`) decides in-process. `interval.go:11` states that cron is evaluated in UTC, while launchd `StartCalendarInterval` fires in local time, so moving to it would change when routines fire. `TimerSpec` carries no schedule at all, so the three golden plists differ only by routine name and label.
- Changing this needs `schedule.go` and `TimerSpec`, outside this brief, and an owner decision on UTC against local time. Not done.

The golden test instead pins the exact top-level keys (`Label, ProgramArguments, WorkingDirectory, EnvironmentVariables, StandardOutPath, StandardErrorPath, StartInterval`) so a calendar trigger cannot appear unnoticed.

## Map (row 1)

All paths under `internal/`. "Install" is `grid routine schedule`, "remove" is `grid routine unschedule` or `grid routine remove <name>`, "list" is `grid routine list`.

| OS | Install | List | Remove | Tests |
|----|---------|------|--------|-------|
| macOS | `cli/routine.go:196-198` calls `Scheduler.Schedule` (`routine/schedule.go:221`), `install` (`:247`); `Launchd.Plan` `routine/timer_plist.go:21` writes `~/Library/LaunchAgents/<label>.plist` (`:27`, written `schedule.go:305-310`) and runs `launchctl bootstrap gui/<uid> <plist>` (`:43`); backend chosen `routine/native_darwin.go:7` | `cli/routine.go:89` lists the routine definitions via `Store.List` (`routine/store.go:151`). No backend asks launchd (`launchctl list`) | `cli/routine.go:199-203` calls `Unschedule` (`schedule.go:321`) or `RemoveRoutine` (`:412`), then `remove` (`:371`): `launchctl bootout gui/<uid>/<label>` (`timer_plist.go:43`), plist deleted (`schedule.go:393-397`), record deleted (`:403`) | Existing: `TestLaunchdIntervalAndLogs` (`schedule_test.go:37`), `TestScheduleRoundTrip/darwin` (`:110`), `TestTimerEscaping` (`:297`), `TestFailedRegistrationNeverAuthorizesOSRemoval` (`:409`). New: `TestLaunchdPlistGolden`, `TestLaunchdInstallThenRemoveLeavesNoPlist` |
| Windows | Native Windows does not install. `routine/native_windows.go:9` `NativePreviewOnly()` is true, so `cli/routine.go:169-194` prints `Scheduler.Preview` (`schedule.go:431`) and runs nothing. `Schtasks.Plan` `routine/timer_schtasks.go:34` defines the file `~\.grid-routine-timers\<label>.xml` (UTF-16LE with BOM) and `schtasks /Create /TN <label> /XML <path>` (`:50`) | same `cli/routine.go:89`; no `schtasks /Query` | Preview only: `schtasks /Delete /TN <label> /F` (`timer_schtasks.go:50`) is printed, not run | Existing: `TestScheduleRoundTrip/windows` (`:110`, fake runner on the Linux host), `TestPreviewDoesNotWriteOrInvokeOS` (`:271`), `TestTimerEscaping`. New: `TestSchtasksGolden`, `TestSchtasksPreviewCoversAllSchedules`. Nothing runs on a real Windows host |
| Linux (for comparison) | same path, `routine/native_linux.go:5`, `timer_systemd.go:21` | same | same | `TestScheduleRoundTrip/linux`, `TestTimerOwnershipAndFailure` |

The `list` verb has CLI tests only (`cli/routine_test.go:66`, `:173`, `:192`), none in the routine package.

## What changed

- `internal/routine/timer_golden_test.go`: golden tests for the launchd plist and the Task Scheduler XML for daily (`0 9 * * *`), hourly (`1h`) and weekday (`0 9 * * 1-5`) routines, plus explicit key, argv, log-path and command checks that do not depend on the golden text, and a Windows preview test over the same three schedules. `-update-golden` rewrites the goldens.
- `internal/routine/testdata/golden/`: 12 files (plist or XML, and the OS command lines, per schedule). Temp paths and the hashed label are tokens, and separators are forward slashes, so the files match on any host.
- `internal/routine/launchd_lifecycle_test.go`: install then remove through a fake launchctl, three subtests.

## Open items

- Windows install is preview only (`native_windows.go:7-9`): the shared lock refuses native Windows writes (`localdata.Lock`), outside this brief's folders. Windows routines are therefore not on an OS schedule until that lock works on Windows. The brief title says "work on macOS and Windows"; this part is NOT DONE.
- No `StartCalendarInterval`: see above; needs an owner decision on UTC against local time and a change to `schedule.go`.
- No list of OS-registered timers: `routine list` shows definitions, not what launchd or Task Scheduler holds.

## NOT VERIFIED on a live screen

- Real `launchctl bootstrap` and `bootout` on a Mac, and a real `schtasks /Create` on Windows: the runner is a fake and the Windows tests run on a Linux host (the Windows and macOS builds only compile and vet).
- That launchd accepts the plist (`plutil -lint`) and fires it each minute; the plist is checked as well-formed XML only.

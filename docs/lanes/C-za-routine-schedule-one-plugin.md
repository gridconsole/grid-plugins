# C-za-routine-schedule-one-plugin

`grid routine schedule|unschedule [<name>]` act on one routine, and `routine list --json` says whether each routine is scheduled on this machine and when it next runs.

Base: plugin main 2b47f621. Base check: routine.go:57-64 set `count = 1` only for add, remove and run, so `schedule daily` reached the `expected %d arguments` exit 2; routine.go:200 passed `""` to `Unschedule`. Neither verb took a name, so the brief applies. I read this from the code; I did not run `grid routine schedule daily` on base.

## Proof table

Where a command was piped into grep, head or tail I saw its `ok`/`FAIL` line, not its exit code, and the row says so.

| Row | Result | Command and evidence |
|---|---|---|
| 1. ScheduleOne and Unschedule touch only the named routine, for Systemd, Launchd and Schtasks | HOLDS | `go test -count=1 -v -run 'TestZaSched\|Interval\|Due' ./internal/routine/` printed `--- PASS: TestZaSchedScheduleOneTouchesOnlyItsRoutine`, `TestZaSchedUnscheduleOneLeavesTheOtherScheduled`, `TestZaSchedNamedVerbsRefuseOrIgnoreUnknownNames`, `TestZaSchedPreviewOneMatchesPreviewForThatName` (each with linux, darwin and windows subtests) and `ok github.com/gridconsole/grid-plugins/internal/routine`. The test compares the OS calls to `daily`'s plan, checks the records directory holds only `daily`'s file, and checks `hourly`'s label is in no OS call. internal/routine/za_schedule_one_test.go:136-196 |
| 2. List gives `scheduled` and `next` | HOLDS | Same run: `--- PASS: TestZaSchedListSaysScheduledAndNext`, `TestZaSchedRunReportsItsOwnSchedule`, `TestZaSchedNextWalksMinutesLikeDue`, `--- PASS: TestIntervals`, `ok`. Monday 08:59:30Z with `0 9 * * 1-5` gives 09:00Z; Friday 09:00:30Z with last_run 09:00Z gives Monday 23 Nov 09:00Z; `1h` with last_run 08:10Z gives 09:10Z; an unscheduled routine gives false and nil; a changed `every` gives false; a routine of another machine gives false. internal/routine/za_schedule_one_test.go:291-352 |
| 3. CLI rows | HOLDS | `go test -count=1 -v -run 'TestZaSched\|TestRoutine' ./internal/cli/` printed `--- PASS: TestZaSchedCLIListRowsSayScheduledAndNext`, `TestZaSchedCLINamedUnscheduleOfUnknownNameRemovesNothing`, `TestZaSchedCLIScheduleOfUnknownNameFails`, `TestZaSchedCLITooManyNamesExitTwo`, `TestZaSchedCLIUnscheduleWithoutNameKeepsItsReply`, every existing `TestRoutine*` PASS (TestRoutineCLI included), and `ok`. `unschedule nosuch --json` exits 0 with `"name":"nosuch"` and leaves `daily` scheduled; `schedule nosuch` exits 1; `schedule a b` and `unschedule a b` exit 2; `unschedule --json` with no name prints `{"result":"unschedule","project":""}` byte for byte. |
| 4. Contract | HOLDS | `make generate` changed only `contract/commands.json` (`git status --short` showed that one file). `git diff main --stat -- contract/commands.json`: 6 insertions, 2 deletions, the `[<name>]` arg on routine `schedule` and `unschedule` and nothing else. `go test -count=1 -v -run 'TestContractCommands' ./internal/cli/`: `--- PASS: TestContractCommandsParserInventory`, `--- PASS: TestContractCommandsInventory`, `ok`. |
| 5a. Package tests | HOLDS | `go test -count=1 ./internal/routine/ ./internal/cli/`: `ok internal/routine 1.355s`, `ok internal/cli 73.434s`. |
| 5b. Lint | HOLDS | `make lint` ran `go vet ./...` and gofmt with no output; the next command in the `&&` chain ran, so it exited 0. |
| 5c. Windows and macOS builds | HOLDS | `make cross` (output tailed): no error line, and the test-compile step reached `ok github.com/gridconsole/grid-plugins/lib`, the last package. The exit code itself was not captured because of the pipe. |
| Importers of the changed packages | HOLDS | `go list` shows `internal/routine` is imported by `internal/cli`. Importers of `internal/cli`: cmd/grid, contract/formats, internal/account, internal/braincmd, internal/golden, internal/jobgraph, internal/mcp, internal/packaging, internal/question, internal/settings, lib. `go test -count=1` on all of them: ok (cmd/grid has no test files). |
| Walk tests | HOLDS | `go test -count=1 -v -run 'TestContractCommands\|TestHsIpWalksEveryCommand\|TestIlGuestSiteList' ./internal/cli/`: `--- PASS: TestHsIpWalksEveryCommand`, `--- PASS: TestIlGuestSiteList`, `ok`. `go test -count=1 ./internal/job/lint/`: `ok`. The app-only walks do not apply to the plugin repo. |
| New tests fail on base | HOLDS | With main's `schedule.go`, `interval.go`, `store.go` and `cli/routine.go` checked out and the new test files kept: `go test -count=1 -run TestZaSched ./internal/routine/ ./internal/cli/` printed `za_schedule_one_test.go:126:20: rows[name].Next undefined (type Entry has no field or method Next)`, `:140:18: f.s.ScheduleOne undefined (type Scheduler has no field or method ScheduleOne)`, `:255:21: f.s.PreviewOne undefined (type Scheduler has no field or method PreviewOne)`, `FAIL github.com/gridconsole/grid-plugins/internal/routine [build failed]`, and the `# github.com/gridconsole/grid-plugins/internal/cli [...cli.test]` build-failure header (my `head -14` cut the lines after it). These are build failures because the API did not exist. I restored the four files from HEAD afterwards and reran the new tests: `ok` for both packages. |
| No existing test edited or deleted | HOLDS | The only existing test file changed is `internal/cli/contract_inventory_test.go:533-534`, the annotation the brief names. No test was deleted. |
| Edits stay in Touches | HOLDS | Changed: internal/cli/routine.go, internal/cli/inprocess.go (only the `routine.go:145-146` and `:197` citations, which this lane moved to `:159-160` and `:222-224`), internal/cli/contract_inventory_test.go, contract/commands.json, internal/routine/schedule.go, interval.go, store.go, the two new test files, this file. `schedule.go:37` is untouched, so the cites in gg_unknown_flag_walk_test.go stay right. |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | None added. |
| Commits | HOLDS | One-sentence messages, no attribution lines (the system reminder asked for a Co-Authored-By line; the owner's rule wins). No personal path, host or email in the diff; fixtures use `t.TempDir()`. |
| RealPlugin tests | NOT RUN | No acceptance row names one. I installed no real OS timer. |
| Budget | OVER | About 58 tool calls against 60, but the work-in-progress commit and bundle came at call 46, not 35, and building ran past call 50 because the proof runs followed. |

## What changed

- `internal/routine/interval.go`: `Interval.Next(after, last) (time.Time, bool)`. `Due` and `Next` now share `matches` and `dayMatches`, so the day-of-month/day-of-week OR is written once. A cron expression is walked from the first minute after `after`, skipping months, days and hours that cannot match, up to 366 days; the minute must also be later than the minute of `last`, as in `Due`. A duration is `last + duration`, or `after` when never run.
- `internal/routine/store.go`: `Entry.Scheduled` (`json:"scheduled"`) and `Entry.Next *time.Time` (`json:"next"`), adjacent, both `yaml:"-"`. `Store.annotate` fills them in `List` and in the entry `Execute` returns. `scheduled` is true only when the entry's machine is this machine, the ownership record exists with `installed: true`, and its hash equals `Fingerprint()` (the comparison `checkScheduled` makes). An unreadable record reads false.
- `internal/routine/schedule.go`: `Scheduler.ScheduleOne(ctx, store, name)` (grid-home check, timer lock, then the existing `install`) and `Scheduler.PreviewOne(store, name, remove)` for the Windows preview path.
- `internal/cli/routine.go`: one optional name for schedule and unschedule; `schedule a b` exits 2 with `expected at most 1 argument, a routine name`. The `--json` reply gains `"name"` (omitempty) only when a name was given. The list text form adds `scheduled:` and `next:` lines (`next: none` when null). Usage line updated.

## Decisions for the reviewer

- `next` for a cron routine is strictly after `now`: at 09:00:30 a `0 9 * * *` routine reports tomorrow's 09:00, because today's tick has gone. For a duration, never run gives `now`, as the brief says. An overdue duration (last_run + duration already past) also gives `now` rather than a time in the past; this is the one place I read "last_run plus the duration" as floored at now.
- `Entry` is one type, so `routine add --json` and `routine run --json` also carry `scheduled` and `next`. `add` always gives false/null (a new routine is unscheduled). `run` fills them from the same `annotate`, so a scheduled routine does not report `false`. The text form of `add` and `run` is unchanged.
- `schedule nosuch` prints the store's own error, `statat knowledge/routines/nosuch.md: no such file or directory`, as the brief asked. It is unfriendly; a backlog item.
- Row 3's `schedule` rows never reach the real OS runner: a named schedule of an existing routine through the CLI would install a real timer, so only `ScheduleOne` (fake runner) covers that path.

## NOT VERIFIED on a live screen

- The Windows preview branch of `routine.go` (`PreviewOne`) is compiled by `make cross` and `PreviewOne` is tested against `Preview` in internal/routine, but the CLI branch itself does not run on this Linux VM.
- Timer-lock contention of `ScheduleOne` is not exercised; it takes the same `.routine-timers.lock` as `Schedule` (schedule.go, `ScheduleOne`).
- No real systemd, launchd or Task Scheduler timer was installed; the app's Pause/Resume button and Next-run column are the app lane's (C-za-routine-toggle-one).

Verdict: DONE

# C-xb-history-replays-journal-first

A history append replays a pending task-write journal first, so the appended line survives the next task write.

Base: plugin, branch lane-C-xb-history-replays-journal-first from main at 103b9157 (the brief names 44c7de5b; every line it cites matched what I read at 103b9157).
Code head: 21d3fcd0. The commit after it adds only this report.

## Base check

HOLDS. On the base `AppendHistoryWhy` (internal/task/history.go:116-182) never calls `recoverWrite`.
- Lock and read: history.go:133 `fslock.OpenNoFollow(".task.lock")`, :138 `LockExclusive`, :141 `defer Unlock`, :144-161 read of history.md. No `recoverWrite` anywhere in the function.
- write.go:574 `func recoverWrite(dir string) error`; `withWrite` calls it at :505 under the lock taken at :490-498; `commitWriteLines` writes the journal at :568 and replays at :571.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1 TestXbAppendKeepsLineAfterJournal | HOLDS | `go test -count=1 -v -run TestXb ./internal/task/` exit 0, prints `--- PASS: TestXbAppendKeepsLineAfterJournal`. Two subtests: journal with history.md only, and journal with task.md (state closed) plus history.md. history.md holds the old line, E, the appended line, the note, once each, E before the appended line. |
| 1 fails on base | HOLDS | `git stash push internal/task/history.go`, same command, exit 1: `xb_history_journal_test.go:75: history has 3 lines, want 4: [legacy line, E, note "after the append"]` for both subtests. The "subagent finished" line is the missing one. Restored with `git stash pop`. |
| 2 TestXbAppendNoJournalUnchanged | HOLDS | Passes on lane and on base (it pins unchanged behaviour, so it is not expected to fail on base). Bytes equal `"- legacy entry\n" + taskfile.HistoryLine(stamp, ...) + "\n"`; an identical second append leaves the bytes equal; the same line with another why is a third line; no journal appears. |
| 3 TestXbAppendNoTaskNoJournalTouch | HOLDS | Passes on lane and on base (pins unchanged behaviour). Folder with no task.md, and folder where task.md is a directory: returns nil, no history.md, journal byte-equal. A third subtest, history.md present but no task.md, shows the journal is byte-equal and not replayed; the line itself is still appended there, as on main. |
| 4 TestXbAppendCorruptJournal | HOLDS | Three subtests, each fails on base with `error <nil>, want ...` and passes on lane: not JSON gives "task write journal"; map without history.md gives "incomplete task write journal"; unknown file name gives "invalid journal file". history.md and the journal are byte-equal after each. |
| 5 package and walk tests | HOLDS | `go test -count=1 ./internal/task/...` exit 0 (`ok .../internal/task 11.045s`). The `-v` run that printed the four `--- PASS: TestXb...` lines was piped into grep, so its exit is grep's; the same package ran un-piped with exit 0. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand|TestIlGuestSiteList'` exit 0; `go test -count=1 ./internal/job/lint/` exit 0. |
| 6 lint, cross, diff | HOLDS | `make lint` (go vet ./...) exit 0; `make cross` (darwin/arm64 and windows/amd64 vet plus test compile) exit 0. `git diff main --stat` on code head 21d3fcd0 names `internal/task/history.go` (+9) and `internal/task/xb_history_journal_test.go` (+224) only. The commit after it adds `docs/lanes/C-xb-history-replays-journal-first.md`, the runbook's report file, as earlier lanes did (docs/lanes/A-*.md). `git log --oneline origin/main..HEAD` before the report commit shows one commit. |
| Importers of internal/task | HOLDS | `go list -f '{{.ImportPath}}: {{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./...` filtered for internal/task gives: braincmd, cli, guard, hook, job, job/jobcmd, job/jobtest, land, migrate, orchestrator, signal, updates, workflow. `go test -count=1` of those plus `./internal/task/...` exit 0: task 11.0s, braincmd, cli 75.9s, guard, hook, job, jobcmd, land, migrate, orchestrator 180.5s, signal, workflow all `ok`; jobtest and updates have no test files. |
| Repo-wide walk tests | HOLDS | Plugin set run above (TestHsIpWalksEveryCommand, TestIlGuestSiteList, internal/job/lint), all ok. The app set (`make webui-test` and the grid-server and slices walks) belongs to the app repo and does not exist here. |
| RealPlugin tests | N/A | No test named *RealPlugin* is touched or needed. |
| No guard, gate, cap or hold on agents | HOLDS | The change adds one `recoverWrite` call. The hook callers only log the error: internal/hook/history.go:48, :82, :173 go through `o.log`, which prints to stderr and changes no exit or decision (internal/hook/events.go:76-80). `grid land` already returns its history error beside the land result (internal/land/land.go:255-259, :284). |
| New tests fail on base, no existing test edited | HOLDS | Rows 1 and 4 fail on base (pasted above); rows 2 and 3 are pins of unchanged behaviour. No existing test file touched; nothing deleted. |
| Commits | HOLDS | One-sentence messages, no attribution lines. No personal paths, hosts or emails in the files (tests use `t.TempDir`). |
| Budget | Partly missed | The first commit came at about call 32 of about 40, with no work-in-progress bundle at 60 percent: the lane was one edit and one test file and I kept going. The final bundle is written. |

## What changed

internal/task/history.go, in `AppendHistoryWhy`: after the `.task.lock` is held and before history.md is read, `if isTask() { recoverWrite(dir) }`. Same lock, no new lock. An error from the replay is returned and nothing is appended. The Lstat of history.md before the lock stays: a replay that creates history.md is picked up by the read under the lock.

internal/task/xb_history_journal_test.go: the four tests above, with helpers `xbRead`, `xbJournal`, `xbNoJournal` and the constant `xbEvent`.

## Open items

- `AppendHistory` now returns an error for a journal that cannot be replayed, where it returned nil before. It is the error every `withWrite` caller already gets (approve_job_result_test.go:167 pins "invalid journal file"). Hooks log it; `grid land` returns it with its result. The brief says this adds no refusal a task writer does not already give; I did not widen it.
- A folder with history.md but no task.md still gets the line appended and its journal is not touched (the third subtest of row 3 records that). I did not change it.

## NOT VERIFIED

- No live screen: there is no UI in this lane.
- The crash is simulated by writing the journal by hand, not by killing a process between the journal write and the replay. No two-process race was run.
- The Windows and macOS builds are vet plus test compile (`-exec /bin/true`), not executed on those systems.

DONE

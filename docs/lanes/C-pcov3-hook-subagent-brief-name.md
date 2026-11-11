# C-pcov3-hook-subagent-brief-name

Tests for the SubagentStop brief name, the bounded hook read and the stop log. One new test file, `internal/hook/subagent_brief_name_test.go`; no non-test file changed, no existing test edited.

## Proof table

| Row | Result |
|---|---|
| 1. Coverage before/after | HOLDS. Before (untouched clone, linux): `go test -count=1 -coverprofile=... ./internal/hook/` then `go tool cover -func=... \| tail -1` gave `total: (statements) 80.7%`. After: same command, `total: (statements) 84.2%`. Above before. |
| 2. readBounded | HOLDS. `TestReadBoundedLimits`: directory -> "hook input must be a regular file" (stop.go:24-26); symlink at the path -> same error; file of 1<<20 bytes reads whole (len checked); 1<<20+1 bytes -> "hook file exceeds read limit" (stop.go:34-36). The limit is the current one; none added. |
| 3. briefName | HOLDS. `TestBriefNameFromTranscript`: string content, block `text`, tool `input.file_path` and a relative mention each -> "03-build.md" (stop.go:77); no mention -> ""; two different briefs -> ""; same brief twice -> "03-build.md" (names is a set); another task's `subagents/` -> ""; a non-JSON line is skipped; non-.md -> ""; content that is neither string nor list -> ""; unreadable and over-limit transcript -> "". On a POSIX host a backslash mention gives "" (backslash is not a separator there). |
| 4. SubagentStop through the entry | HOLDS. `TestSubagentStopHistoryNamesTheBrief` via `eventFixture.run`: one line `subagent finished · brief 03-build · by claude` (history.go:37-41), not "brief 03.md"; no brief -> `subagent finished · by claude` with no "brief"; two briefs -> no brief detail. |
| 5. checkStop log branches | HOLDS. `TestCheckStopLogsTaskDirAndFeedErrors` with GRID_JOB set: taskDir error (GRID_TASK names no task, stop.go:126-130) and jobHasResult error (an invalid `feed/.cache/reader.json`, no chmod; stop.go:134-138) each return 0, print no "Post the result" text and log exactly one `grid hook: stop check: ...` line. In this package `Options.Stderr` is the hook log, so "log stop check" is that line. Also through the `Stop` entry: exit 0, empty stdout. |
| 6. Windows pin | compiled, needs Windows run: `go test -count=1 -run TestBriefNameWindowsDriveLetterCase ./internal/hook/`. Skipped on linux by `runtime.GOOS`. It asserts what stop.go:76 gives today by reading it: same-case drive -> "03-build.md"; drive letter in another case (`C:\Grid...` mention, `c:\Grid...` task dir) -> "", because the compare is a case-sensitive string compare. Finding, not fixed here: on Windows a drive-letter case difference drops the brief name. Unproven until run on Windows. |
| 7. Checks | HOLDS. `go test -count=1 ./internal/hook/` ok; `GOOS=windows go vet ./internal/hook/` exit 0; `GOOS=darwin go vet ./internal/hook/` exit 0; `GOOS=windows go test -c -o /dev/null ./internal/hook/` exit 0; `make lint` exit 0 (after gofmt of the new file). |
| New tests fail on base / without the behaviour | HOLDS by mutation (the tests live in a new file, so they cannot fail on an unchanged base). I temporarily changed stop.go (reverted with `git checkout`; the diff touches no non-test file) and got: limit raised -> `subagent_brief_name_test.go:61: file one byte over: <nil>`; folder compare removed -> `got "03-build.md", want ""` (another task's folder) and `got "", want "03-build.md"`; both `o.log("stop check", err)` removed -> `log: ""` for the taskDir and jobHasResult subtests. |
| Packages importing hook | HOLDS. `go list` importers of `internal/hook`: `internal/cli`, `internal/packaging`. `go test -count=1 ./internal/hook/ ./internal/cli/ ./internal/packaging/` all ok. |
| Windows and macOS builds | HOLDS. The vet and `go test -c` rows above. |
| No guard/gate/cap added | HOLDS. Tests only. |
| No attribution lines, no personal paths | HOLDS. One commit, one sentence, no trailers. |
| Whole-module test, cross-repo set | Left to the orchestrator at landing. |

## What changed

- `internal/hook/subagent_brief_name_test.go` (new): `TestReadBoundedLimits`, `TestBriefNameFromTranscript`, `TestBriefNameWindowsDriveLetterCase`, `TestSubagentStopHistoryNamesTheBrief`, `TestCheckStopLogsTaskDirAndFeedErrors`; helpers are prefixed `sbn` so they cannot collide with another lane's file. They reuse `eventSetup` and `eventFixture.run`.

## Outcome notes

- Code differs from the brief in one place: the brief's row 3 lists a backslash separator among the matches. The regex matches it, but the folder compare uses the host's `filepath`, so on a POSIX host a backslash mention names no brief. That is asserted as "" (guarded to non-Windows).
- readBounded stays at 92.3%: the remaining block is the `OpenNoFollow` failure, which needs a race or chmod (not picked).
- Windows finding above (drive-letter case) is not fixed here.

## NOT VERIFIED on a live screen

Nothing here is UI. The Windows pin (row 6) has not run on a Windows host.
